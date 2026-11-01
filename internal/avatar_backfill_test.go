package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunAvatarBackfillJobsFetchesAndSavesURLs(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != userProfileAPIEndpoint {
			t.Errorf("Request path = %q, want %q", r.URL.Path, userProfileAPIEndpoint)
		}
		var request struct {
			UserID uint64 `json:"userId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if request.UserID > 0 {
			_, _ = w.Write([]byte(`{"avatarUrl":""}`))
			return
		}
		_, _ = w.Write([]byte(`{"avatarUrl":"https://example.com/avatar.webp"}`))
	}))
	defer server.Close()

	client := newAvatarBackfillTestClient(server)
	jobs := []avatarBackfillJob{
		{pathID: "h55d", nameKey: "ludo flender", letter: "l"},
		{pathID: "8246810", nameKey: "legacy driver", letter: "l"},
	}
	var saved []avatarUpdate
	attempted := make(map[string]struct{})

	runAvatarBackfillJobs(context.Background(), jobs, client, 0, attempted, func(updates []avatarUpdate) error {
		saved = append(saved, updates...)
		return nil
	})

	if requestCount != 2 {
		t.Fatalf("Profile requests = %d, want 2", requestCount)
	}
	if len(saved) != 1 || saved[0].pathID != "h55d" || saved[0].avatarURL != "https://example.com/avatar.webp" {
		t.Fatalf("Saved updates = %+v, want h55d avatar URL", saved)
	}
	if len(attempted) != 2 {
		t.Errorf("Completed profile count = %d, want both URL and empty-avatar profiles", len(attempted))
	}
}

func TestRunAvatarBackfillJobsRetriesLookupFailure(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"avatarUrl":"https://example.com/avatar.webp"}`))
	}))
	defer server.Close()

	client := newAvatarBackfillTestClient(server)
	jobs := []avatarBackfillJob{{pathID: "h55d", nameKey: "ludo flender", letter: "l"}}
	attempted := make(map[string]struct{})
	savedCount := 0
	save := func(updates []avatarUpdate) error {
		savedCount += len(updates)
		return nil
	}

	runAvatarBackfillJobs(context.Background(), jobs, client, 0, attempted, save)
	if _, exists := attempted["h55d"]; exists {
		t.Fatal("Failed lookup must remain eligible for retry")
	}

	runAvatarBackfillJobs(context.Background(), jobs, client, 0, attempted, save)
	if requestCount != 2 {
		t.Fatalf("Profile requests = %d, want 2 after retry", requestCount)
	}
	if savedCount != 1 {
		t.Errorf("Saved avatar count = %d, want 1", savedCount)
	}
	if _, exists := attempted["h55d"]; !exists {
		t.Error("Successfully saved lookup should be marked complete")
	}
}

func TestRunAvatarBackfillJobsRetriesSaveFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"avatarUrl":"https://example.com/avatar.webp"}`))
	}))
	defer server.Close()

	client := newAvatarBackfillTestClient(server)
	jobs := []avatarBackfillJob{{pathID: "h55d", nameKey: "ludo flender", letter: "l"}}
	attempted := make(map[string]struct{})

	runAvatarBackfillJobs(context.Background(), jobs, client, 0, attempted, func([]avatarUpdate) error {
		return errors.New("disk unavailable")
	})
	if _, exists := attempted["h55d"]; exists {
		t.Fatal("Failed save must remain eligible for retry")
	}

	savedCount := 0
	runAvatarBackfillJobs(context.Background(), jobs, client, 0, attempted, func(updates []avatarUpdate) error {
		savedCount += len(updates)
		return nil
	})
	if savedCount != 1 {
		t.Errorf("Retry saved %d avatars, want 1", savedCount)
	}
	if _, exists := attempted["h55d"]; !exists {
		t.Error("Successful retry should mark the profile complete")
	}
}

func TestRunAvatarBackfillJobsHonorsCancellation(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount++
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runAvatarBackfillJobs(ctx,
		[]avatarBackfillJob{{pathID: "h55d", nameKey: "ludo flender", letter: "l"}},
		newAvatarBackfillTestClient(server),
		0,
		make(map[string]struct{}),
		func([]avatarUpdate) error {
			t.Error("Save must not run when canceled before the first profile")
			return nil
		},
	)
	if requestCount != 0 {
		t.Errorf("Profile requests = %d, want 0 after cancellation", requestCount)
	}
}

func TestFindMissingDriverAvatarJobs(t *testing.T) {
	withAvatarBackfillTempDir(t)
	names := DriverNamesIndex{
		"ludo flender": {
			{Name: "Ludo Flender", PathID: "7681081"},
		},
		"legacy driver": {
			{Name: "Legacy Driver", PathID: "8246810", Avatar: "https://example.com/avatar.webp"},
		},
		"ludo alias": {
			{Name: "Ludo Alias", PathID: "7681081"},
		},
	}
	if _, err := writeGzipJSON(filepath.Join(ShardedIndexDir, "l.json.gz"), names); err != nil {
		t.Fatalf("Write metadata fixture: %v", err)
	}
	if _, err := writeGzipJSON(filepath.Join(ShardedIndexDir, "a.json.gz"), DriverNamesIndex{
		"aaron halbert-boucle": {
			{Name: "Aaron Halbert-Bouclé", PathID: "8113325", Avatar: "https://game.raceroomracing.com/cdn/apex-production/DefaultAvatar:9%89%DC%D8%A4%A6%B9%BD%A2%F5%97%80%88%9F"},
		},
	}); err != nil {
		t.Fatalf("Write Aaron metadata fixture: %v", err)
	}

	jobs, err := findMissingDriverAvatarJobs()
	if err != nil {
		t.Fatalf("findMissingDriverAvatarJobs returned error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("Missing or corrupted-avatar jobs = %d, want 2", len(jobs))
	}
	if jobs[0].pathID != "8113325" || jobs[0].letter != "a" {
		t.Errorf("Corrupted-avatar job = %+v, want Aaron Halbert-Bouclé", jobs[0])
	}
	if jobs[1].pathID != "7681081" || jobs[1].letter != "l" {
		t.Errorf("Missing-avatar job = %+v, want Ludo Flender", jobs[1])
	}

	allJobs, err := findDriverAvatarJobs(true)
	if err != nil {
		t.Fatalf("findDriverAvatarJobs(true) returned error: %v", err)
	}
	if len(allJobs) != 3 || allJobs[0].pathID != "8113325" ||
		allJobs[1].pathID != "8246810" || allJobs[2].pathID != "7681081" {
		t.Errorf("Full refresh jobs = %+v, want all three unique profiles in sorted order", allJobs)
	}
}

func TestAvatarRefreshDue(t *testing.T) {
	now := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	freshness := 24 * time.Hour

	tests := []struct {
		name        string
		lastRefresh time.Time
		freshness   time.Duration
		want        bool
	}{
		{name: "never refreshed", want: true},
		{name: "within freshness period", lastRefresh: now.Add(-freshness + time.Second), freshness: freshness},
		{name: "freshness period elapsed", lastRefresh: now.Add(-freshness), freshness: freshness, want: true},
		{name: "zero freshness", lastRefresh: now, freshness: 0, want: true},
		{name: "negative freshness", lastRefresh: now, freshness: -time.Second, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := avatarRefreshDue(test.lastRefresh, now, test.freshness); got != test.want {
				t.Errorf("avatarRefreshDue() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestSaveDriverAvatarUpdates(t *testing.T) {
	withAvatarBackfillTempDir(t)
	names := DriverNamesIndex{
		"ludo flender": {
			{Name: "Ludo Flender", PathID: "7681081"},
			{Name: "Ludo Flender", PathID: "7681082", Avatar: "https://example.com/existing.webp"},
			{Name: "Ludo Flender", PathID: "7681083", Avatar: "https://example.com/stale.webp"},
		},
	}
	if _, err := writeGzipJSON(filepath.Join(ShardedIndexDir, "l.json.gz"), names); err != nil {
		t.Fatalf("Write metadata fixture: %v", err)
	}
	if _, err := writeGzipJSON(filepath.Join(ShardedIndexDir, "a.json.gz"), DriverNamesIndex{
		"aaron halbert-boucle": {
			{Name: "Aaron Halbert-Bouclé", PathID: "8113325", Avatar: "https://game.raceroomracing.com/cdn/apex-production/DefaultAvatar:9%89%DC%D8%A4%A6%B9%BD%A2%F5%97%80%88%9F"},
		},
	}); err != nil {
		t.Fatalf("Write Aaron metadata fixture: %v", err)
	}

	wantURL := "https://game.raceroomracing.com/cdn/user-avatars-prod/7681081/avatar.webp"
	wantDefaultAvatarURL := "https://game.raceroomracing.com/cdn/apex-production/DefaultAvatar:9:default.webp"
	err := saveDriverAvatarUpdates([]avatarUpdate{{
		avatarBackfillJob: avatarBackfillJob{pathID: "7681081", nameKey: "ludo flender", letter: "l"},
		avatarURL:         wantURL,
	}, {
		avatarBackfillJob: avatarBackfillJob{pathID: "7681082", nameKey: "ludo flender", letter: "l"},
		avatarURL:         "https://example.com/should-not-overwrite.webp",
	}, {
		avatarBackfillJob: avatarBackfillJob{
			pathID:  "7681083",
			nameKey: "ludo flender",
			letter:  "l",
			refresh: true,
		},
		avatarURL: "https://example.com/refreshed.webp",
	}, {
		avatarBackfillJob: avatarBackfillJob{pathID: "8113325", nameKey: "aaron halbert-boucle", letter: "a"},
		avatarURL:         wantDefaultAvatarURL,
	}})
	if err != nil {
		t.Fatalf("saveDriverAvatarUpdates returned error: %v", err)
	}

	savedNames, err := LoadLetterNames("l")
	if err != nil {
		t.Fatalf("LoadLetterNames returned error: %v", err)
	}
	identities := savedNames["ludo flender"]
	if identities[0].Avatar != wantURL {
		t.Errorf("Saved avatar = %q, want %q", identities[0].Avatar, wantURL)
	}
	if identities[1].Avatar != "https://example.com/existing.webp" {
		t.Errorf("Existing avatar was overwritten: %q", identities[1].Avatar)
	}
	if identities[2].Avatar != "https://example.com/refreshed.webp" {
		t.Errorf("Stale avatar was not refreshed: %q", identities[2].Avatar)
	}
	savedNames, err = LoadLetterNames("a")
	if err != nil {
		t.Fatalf("LoadLetterNames(a) returned error: %v", err)
	}
	if got := savedNames["aaron halbert-boucle"][0].Avatar; got != wantDefaultAvatarURL {
		t.Errorf("Corrupted avatar was not repaired: got %q, want %q", got, wantDefaultAvatarURL)
	}
}

func TestAvatarBackfillLogsETA(t *testing.T) {
	var output bytes.Buffer
	originalWriter := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(originalWriter)

	logAvatarBackfillProgress(1000, 100, 75, 5, 20, 0, time.Now().Add(-10*time.Second), false)
	message := output.String()
	for _, expected := range []string{
		"100/1000 profiles checked",
		"75 URLs saved",
		"20 lookup failures",
		"profiles/s",
		"ETA",
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("Progress log %q does not contain %q", message, expected)
		}
	}
	if strings.Contains(message, "NaN") || strings.Contains(message, "+Inf") {
		t.Errorf("Progress log contains invalid rate: %q", message)
	}
}

func TestAvatarBackfillManagerScheduleCoalescesSignals(t *testing.T) {
	manager := avatarBackfillManager{started: true, wake: make(chan struct{}, 1)}
	manager.schedule()
	manager.schedule()
	if len(manager.wake) != 1 {
		t.Fatalf("Queued wake signals = %d, want 1 coalesced signal", len(manager.wake))
	}
}

func newAvatarBackfillTestClient(server *httptest.Server) *APIClient {
	client := NewAPIClient()
	client.baseURL = server.URL
	client.client = server.Client()
	return client
}

func withAvatarBackfillTempDir(t *testing.T) {
	t.Helper()
	tempDir, cleanup := TempTestDir(t, "avatar_backfill")
	t.Cleanup(cleanup)
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Restore working directory: %v", err)
		}
	})
}
