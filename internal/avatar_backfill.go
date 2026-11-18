package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	avatarBackfillSaveBatchSize = 100
	avatarBackfillLogInterval   = 1000
	avatarBackfillFirstLogAt    = 100
	avatarProfileRequestTimeout = 15 * time.Second
)

type avatarBackfillJob struct {
	pathID  string
	nameKey string
	letter  string
	refresh bool
}

type avatarUpdate struct {
	avatarURL string
	avatarBackfillJob
}

// StartDriverAvatarBackfill schedules a full profile URL refresh once a week.
func StartDriverAvatarBackfill(ctx context.Context, schedule ScheduleConfig) error {
	if schedule.AvatarBackfillWeekday < time.Sunday || schedule.AvatarBackfillWeekday > time.Saturday ||
		schedule.AvatarBackfillHour < 0 || schedule.AvatarBackfillHour > 23 ||
		schedule.AvatarBackfillMinute < 0 || schedule.AvatarBackfillMinute > 59 {
		return fmt.Errorf("invalid avatar backfill schedule: weekday %d, time %02d:%02d",
			schedule.AvatarBackfillWeekday, schedule.AvatarBackfillHour, schedule.AvatarBackfillMinute)
	}
	go runDriverAvatarBackfill(ctx, schedule)
	return nil
}

func nextAvatarBackfillTime(now time.Time, schedule ScheduleConfig) time.Time {
	days := (int(schedule.AvatarBackfillWeekday) - int(now.Weekday()) + 7) % 7
	next := time.Date(now.Year(), now.Month(), now.Day()+days,
		schedule.AvatarBackfillHour, schedule.AvatarBackfillMinute, 0, 0, now.Location())
	if !next.After(now) {
		next = time.Date(now.Year(), now.Month(), now.Day()+days+7,
			schedule.AvatarBackfillHour, schedule.AvatarBackfillMinute, 0, 0, now.Location())
	}
	return next
}

func runDriverAvatarBackfill(ctx context.Context, schedule ScheduleConfig) {
	client := NewAPIClient()
	defer client.Close()

	for {
		next := nextAvatarBackfillTime(time.Now(), schedule)
		log.Printf("🖼️ Next avatar URL backfill scheduled for %s", next.Format(time.RFC3339))
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Println("⏹️ Avatar URL backfill stopped")
			return
		case <-timer.C:
		}

		jobs, err := findDriverAvatarJobs(true)
		if err != nil {
			log.Printf("⚠️ Avatar URL backfill could not scan metadata: %v", err)
			continue
		}
		attempted := make(map[string]struct{})
		if len(jobs) > 0 {
			log.Printf("🖼️ Avatar URL backfill queued %d profiles for a full refresh; only profile metadata URLs are requested, not image files. ETA will be reported after the first %d profiles.",
				len(jobs), avatarBackfillLogInterval)
			runAvatarBackfillJobs(ctx, jobs, client, apiThrottle, attempted, saveDriverAvatarUpdates)
		}
		if ctx.Err() != nil {
			return
		}
		if allAvatarJobsAttempted(jobs, attempted) {
			if err := UpdateLastAvatarRefresh(time.Now()); err != nil {
				log.Printf("⚠️ Failed to persist avatar refresh timestamp: %v", err)
			}
		} else {
			log.Println("⚠️ Avatar URL backfill was incomplete; failed profiles will be retried on the next scheduled run")
		}
	}
}

func findMissingDriverAvatarJobs() ([]avatarBackfillJob, error) {
	return findDriverAvatarJobs(false)
}

func findDriverAvatarJobs(includeFresh bool) ([]avatarBackfillJob, error) {
	names, err := LoadShardedNamesIndex()
	if err != nil {
		return nil, fmt.Errorf("load driver metadata: %w", err)
	}

	jobs := make([]avatarBackfillJob, 0)
	seenPathIDs := make(map[string]struct{})
	for nameKey, identities := range names {
		for _, identity := range identities {
			if (!includeFresh && isStoredAvatarURLValid(identity.Avatar)) ||
				identity.PathID == "" ||
				!profileSlugRegex.MatchString(identity.PathID) {
				continue
			}
			if _, exists := seenPathIDs[identity.PathID]; exists {
				continue
			}
			seenPathIDs[identity.PathID] = struct{}{}
			jobs = append(jobs, avatarBackfillJob{
				pathID:  identity.PathID,
				nameKey: nameKey,
				letter:  ShardKeyForName(nameKey),
				refresh: includeFresh,
			})
		}
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].letter == jobs[j].letter {
			if jobs[i].nameKey == jobs[j].nameKey {
				return jobs[i].pathID < jobs[j].pathID
			}
			return jobs[i].nameKey < jobs[j].nameKey
		}
		return jobs[i].letter < jobs[j].letter
	})
	return jobs, nil
}

func allAvatarJobsAttempted(jobs []avatarBackfillJob, attempted map[string]struct{}) bool {
	for _, job := range jobs {
		if _, exists := attempted[job.pathID]; !exists {
			return false
		}
	}
	return true
}

func runAvatarBackfillJobs(
	ctx context.Context,
	jobs []avatarBackfillJob,
	client *APIClient,
	throttle time.Duration,
	attempted map[string]struct{},
	save func([]avatarUpdate) error,
) {
	if len(jobs) == 0 {
		return
	}

	startedAt := time.Now()
	processed, urlsFound, noAvatar, failures, saveFailures := 0, 0, 0, 0, 0
	updates := make([]avatarUpdate, 0, avatarBackfillSaveBatchSize)
	lastProgressLog := startedAt

	flush := func() {
		if len(updates) == 0 {
			return
		}
		if err := save(updates); err != nil {
			saveFailures += len(updates)
			log.Printf("⚠️ Avatar URL backfill failed to persist %d discovered URL(s): %v", len(updates), err)
		} else {
			urlsFound += len(updates)
			for _, update := range updates {
				attempted[update.pathID] = struct{}{}
			}
		}
		updates = updates[:0]
	}

	for _, job := range jobs {
		if _, wasAttempted := attempted[job.pathID]; wasAttempted {
			continue
		}
		select {
		case <-ctx.Done():
			flush()
			logAvatarBackfillProgress(len(jobs), processed, urlsFound, noAvatar, failures, saveFailures, startedAt, false)
			return
		default:
		}

		requestCtx, cancel := context.WithTimeout(ctx, avatarProfileRequestTimeout)
		avatarURL, err := client.FetchProfileAvatar(requestCtx, job.pathID)
		cancel()
		processed++

		if err != nil {
			failures++
			if failures == 1 {
				log.Printf("⚠️ First avatar URL lookup failure for driver %s: %v", job.pathID, err)
			}
			var statusErr profileAPIStatusError
			if errors.As(err, &statusErr) && (statusErr.statusCode == http.StatusUnauthorized || statusErr.statusCode == http.StatusForbidden) {
				flush()
				log.Printf("⚠️ Avatar URL backfill stopped after HTTP %d; remaining profiles will be retried on the next scheduled run", statusErr.statusCode)
				logAvatarBackfillProgress(len(jobs), processed, urlsFound, noAvatar, failures, saveFailures, startedAt, false)
				return
			}
		} else if avatarURL == "" {
			noAvatar++
			attempted[job.pathID] = struct{}{}
		} else {
			updates = append(updates, avatarUpdate{
				avatarBackfillJob: job,
				avatarURL:         avatarURL,
			})
			if len(updates) >= avatarBackfillSaveBatchSize {
				flush()
			}
		}

		if processed == avatarBackfillFirstLogAt ||
			processed%avatarBackfillLogInterval == 0 ||
			time.Since(lastProgressLog) >= time.Minute {
			logAvatarBackfillProgress(len(jobs), processed, urlsFound, noAvatar, failures, saveFailures, startedAt, false)
			lastProgressLog = time.Now()
		}
		if throttle > 0 {
			select {
			case <-ctx.Done():
				flush()
				logAvatarBackfillProgress(len(jobs), processed, urlsFound, noAvatar, failures, saveFailures, startedAt, false)
				return
			case <-time.After(throttle):
			}
		}
	}

	flush()
	logAvatarBackfillProgress(len(jobs), processed, urlsFound, noAvatar, failures, saveFailures, startedAt, true)
}

func logAvatarBackfillProgress(total, processed, urlsFound, noAvatar, failures, saveFailures int, startedAt time.Time, complete bool) {
	elapsed := time.Since(startedAt)
	rate := 0.0
	if elapsed > 0 {
		rate = float64(processed) / elapsed.Seconds()
	}
	remaining := total - processed
	if remaining < 0 {
		remaining = 0
	}

	eta := "calculating"
	if rate > 0 {
		eta = (time.Duration(float64(remaining)/rate) * time.Second).Round(time.Second).String()
	}
	if complete {
		eta = "0s"
	}

	log.Printf("🖼️ Avatar URL backfill: %d/%d profiles checked, %d URLs saved, %d without avatars, %d lookup failures, %d save failures; %.2f profiles/s, elapsed %s, ETA %s",
		processed, total, urlsFound, noAvatar, failures, saveFailures, rate, elapsed.Round(time.Second), eta)
}

func saveDriverAvatarUpdates(updates []avatarUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	indexUpdateMu.Lock()
	defer indexUpdateMu.Unlock()

	updatesByLetter := make(map[string]map[string]map[string]avatarUpdate)
	for _, update := range updates {
		if updatesByLetter[update.letter] == nil {
			updatesByLetter[update.letter] = make(map[string]map[string]avatarUpdate)
		}
		if updatesByLetter[update.letter][update.nameKey] == nil {
			updatesByLetter[update.letter][update.nameKey] = make(map[string]avatarUpdate)
		}
		updatesByLetter[update.letter][update.nameKey][update.pathID] = update
	}

	var failures []string
	for letter, nameUpdates := range updatesByLetter {
		names, err := LoadLetterNames(letter)
		if err != nil {
			failures = append(failures, fmt.Sprintf("load %s metadata: %v", letter, err))
			continue
		}

		changed := false
		for nameKey, driverUpdates := range nameUpdates {
			identities, exists := names[nameKey]
			if !exists {
				failures = append(failures, fmt.Sprintf("driver name %q not found in %s metadata", nameKey, letter))
				continue
			}
			for i := range identities {
				if update, exists := driverUpdates[identities[i].PathID]; exists &&
					(update.refresh || !isStoredAvatarURLValid(identities[i].Avatar)) &&
					identities[i].Avatar != update.avatarURL {
					identities[i].Avatar = update.avatarURL
					changed = true
				}
			}
			names[nameKey] = identities
		}
		if !changed {
			continue
		}
		if _, err := writeGzipJSON(filepath.Join(ShardedIndexDir, letter+".json.gz"), names); err != nil {
			failures = append(failures, fmt.Sprintf("write %s metadata: %v", letter, err))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("save avatar metadata: %s", strings.Join(failures, "; "))
	}
	return nil
}

func isStoredAvatarURLValid(avatarURL string) bool {
	if avatarURL == "" {
		return false
	}
	parsed, err := url.Parse(avatarURL)
	return err == nil && utf8.ValidString(parsed.Path)
}
