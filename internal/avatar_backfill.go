package internal

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	avatarBackfillSaveBatchSize = 100
	avatarBackfillLogInterval   = 1000
	avatarBackfillFirstLogAt    = 100
	avatarBackfillScanInterval  = 30 * time.Minute
	avatarProfileRequestTimeout = 15 * time.Second
)

type avatarBackfillJob struct {
	pathID  string
	nameKey string
	letter  string
}

type avatarUpdate struct {
	avatarURL string
	avatarBackfillJob
}

type avatarBackfillManager struct {
	mu      sync.Mutex
	started bool
	wake    chan struct{}
}

var driverAvatarBackfill = avatarBackfillManager{wake: make(chan struct{}, 1)}

// StartDriverAvatarBackfill starts the non-blocking profile URL backfill.
// Successful URLs are written to the metadata shards, so unfinished work is
// naturally rediscovered after a restart.
func StartDriverAvatarBackfill(ctx context.Context) {
	driverAvatarBackfill.mu.Lock()
	if driverAvatarBackfill.started {
		driverAvatarBackfill.mu.Unlock()
		driverAvatarBackfill.schedule()
		return
	}
	driverAvatarBackfill.started = true
	driverAvatarBackfill.mu.Unlock()

	go driverAvatarBackfill.run(ctx)
}

// ScheduleDriverAvatarBackfill asks the background worker to scan for new
// drivers after an index export. It never waits for profile requests.
func ScheduleDriverAvatarBackfill() {
	driverAvatarBackfill.schedule()
}

func (manager *avatarBackfillManager) schedule() {
	manager.mu.Lock()
	started := manager.started
	manager.mu.Unlock()
	if !started {
		return
	}

	select {
	case manager.wake <- struct{}{}:
	default:
	}
}

func (manager *avatarBackfillManager) run(ctx context.Context) {
	client := NewAPIClient()
	defer client.Close()

	attempted := make(map[string]struct{})
	ticker := time.NewTicker(avatarBackfillScanInterval)
	defer ticker.Stop()

	for {
		jobs, err := findMissingDriverAvatarJobs()
		if err != nil {
			log.Printf("⚠️ Avatar URL backfill could not scan metadata: %v", err)
		} else {
			pending := jobs[:0]
			for _, job := range jobs {
				if _, wasAttempted := attempted[job.pathID]; !wasAttempted {
					pending = append(pending, job)
				}
			}
			jobs = pending
		}
		if err == nil && len(jobs) > 0 {
			log.Printf("🖼️ Avatar URL backfill queued %d driver profiles; only profile metadata URLs are requested, not image files. ETA will be reported after the first %d profiles.",
				len(jobs), avatarBackfillLogInterval)
			runAvatarBackfillJobs(ctx, jobs, client, apiThrottle, attempted, saveDriverAvatarUpdates)
		} else if err == nil {
			log.Println("🖼️ Avatar URL backfill scan complete: no unattempted missing avatar URLs")
		}

		select {
		case <-ctx.Done():
			log.Println("⏹️ Avatar URL backfill stopped")
			return
		case <-manager.wake:
		case <-ticker.C:
		}
	}
}

func findMissingDriverAvatarJobs() ([]avatarBackfillJob, error) {
	names, err := LoadShardedNamesIndex()
	if err != nil {
		return nil, fmt.Errorf("load driver metadata: %w", err)
	}

	jobs := make([]avatarBackfillJob, 0)
	seenPathIDs := make(map[string]struct{})
	for nameKey, identities := range names {
		for _, identity := range identities {
			if isStoredAvatarURLValid(identity.Avatar) || identity.PathID == "" || !profileSlugRegex.MatchString(identity.PathID) {
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

	updatesByLetter := make(map[string]map[string]map[string]string)
	for _, update := range updates {
		if updatesByLetter[update.letter] == nil {
			updatesByLetter[update.letter] = make(map[string]map[string]string)
		}
		if updatesByLetter[update.letter][update.nameKey] == nil {
			updatesByLetter[update.letter][update.nameKey] = make(map[string]string)
		}
		updatesByLetter[update.letter][update.nameKey][update.pathID] = update.avatarURL
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
				if avatarURL, exists := driverUpdates[identities[i].PathID]; exists && !isStoredAvatarURLValid(identities[i].Avatar) {
					identities[i].Avatar = avatarURL
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
