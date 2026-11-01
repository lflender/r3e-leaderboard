package internal

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRaceRoomAvatarProfileIntegration(t *testing.T) {
	if os.Getenv("R3E_AVATAR_INTEGRATION") != "1" {
		t.Skip("set R3E_AVATAR_INTEGRATION=1 to call the live RaceRoom profile API")
	}

	sampleCount := 20
	if rawCount := os.Getenv("R3E_AVATAR_INTEGRATION_SAMPLES"); rawCount != "" {
		parsedCount, err := strconv.Atoi(rawCount)
		if err != nil || parsedCount < 1 {
			t.Fatalf("invalid R3E_AVATAR_INTEGRATION_SAMPLES %q: expected a positive integer", rawCount)
		}
		sampleCount = parsedCount
	}

	client := NewAPIClient()
	defer client.Close()

	profileID := os.Getenv("R3E_AVATAR_INTEGRATION_PROFILE_ID")
	if profileID == "" {
		profileID = "7681081"
	}
	const sampleDelay = 100 * time.Millisecond
	startedAt := time.Now()
	for sample := 1; sample <= sampleCount; sample++ {
		ctx, cancel := context.WithTimeout(context.Background(), avatarProfileRequestTimeout)
		avatarURL, err := client.FetchProfileAvatar(ctx, profileID)
		cancel()
		if err != nil {
			t.Fatalf("live profile request %d/%d failed: %v", sample, sampleCount, err)
		}
		if avatarURL == "" {
			t.Fatalf("live profile request %d/%d returned no avatar URL", sample, sampleCount)
		}
		if sample == 1 {
			parsedURL, parseErr := url.Parse(avatarURL)
			if parseErr != nil || parsedURL.Host != "game.raceroomracing.com" ||
				!strings.HasPrefix(parsedURL.Path, "/cdn/") {
				t.Fatalf("live avatar URL %q is not a valid RaceRoom CDN URL", avatarURL)
			}
		}

		if sample < sampleCount {
			time.Sleep(sampleDelay)
		}
	}

	elapsed := time.Since(startedAt)
	requestTime := elapsed - sampleDelay*time.Duration(sampleCount-1)
	if requestTime < 0 {
		requestTime = 0
	}
	averageRequestTime := requestTime / time.Duration(sampleCount)
	estimatedProfileInterval := averageRequestTime + apiThrottle
	estimatedTotal := estimatedProfileInterval * 100_000
	t.Logf(
		"Live profile URL backfill estimate: %d serial profile metadata requests took %s including %s spacing. Mean request duration: %s; configured throttle: %s; estimated 100,000-profile run: %s (%s, %.2f profiles/s). No avatar image files were downloaded.",
		sampleCount,
		elapsed.Round(time.Millisecond),
		sampleDelay,
		averageRequestTime.Round(time.Millisecond),
		apiThrottle,
		estimatedTotal.Round(time.Second),
		formatBackfillDuration(estimatedTotal),
		float64(time.Second)/float64(estimatedProfileInterval),
	)
}

func formatBackfillDuration(duration time.Duration) string {
	if duration < time.Minute {
		return duration.Round(time.Second).String()
	}
	days := int(duration / (24 * time.Hour))
	remaining := duration % (24 * time.Hour)
	hours := int(remaining / time.Hour)
	remaining %= time.Hour
	minutes := int(remaining / time.Minute)
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
