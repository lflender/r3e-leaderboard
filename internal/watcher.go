package internal

import (
	"context"
	"log"
	"net/url"
	"os"
	"strings"
	"time"
)

// RefreshTriggerCallback is called when a refresh is triggered
type RefreshTriggerCallback func(trackIDs []string, origin string)

// RefreshWatcher watches a file for refresh triggers
type RefreshWatcher struct {
	triggerPath   string
	checkInterval time.Duration
	ctx           context.Context
	onRefresh     RefreshTriggerCallback
	isBusy        func() bool
}

// NewRefreshWatcher creates a new refresh file watcher
func NewRefreshWatcher(ctx context.Context, triggerPath string, checkIntervalSeconds int, onRefresh RefreshTriggerCallback, isBusy func() bool) *RefreshWatcher {
	if checkIntervalSeconds < 1 {
		checkIntervalSeconds = 30
	}
	return &RefreshWatcher{
		triggerPath:   triggerPath,
		checkInterval: time.Duration(checkIntervalSeconds) * time.Second,
		ctx:           ctx,
		onRefresh:     onRefresh,
		isBusy:        isBusy,
	}
}

// Start begins watching for the trigger file
func (w *RefreshWatcher) Start() {
	go func() {
		log.Printf("🪙 Refresh file trigger watching %s every %v", w.triggerPath, w.checkInterval)
		ticker := time.NewTicker(w.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.checkTrigger()
			case <-w.ctx.Done():
				log.Println("⏹️ Refresh file trigger watcher stopping")
				return
			}
		}
	}()
}

// parseRefreshTrackIDs extracts refresh tokens from the trigger file content.
// Supported input formats:
// - whitespace-separated track IDs: "1693 1778"
// - newline-separated track IDs: "1111\n2222"
// - query strings: "track=6164&class=1685"
// - full URLs with query parameters: "https://.../?track=6164&class=1685"
func parseRefreshTrackIDs(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	fields := strings.Fields(content)
	trackIDs := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}

		if parsed := parseTrackIDField(field); parsed != "" {
			trackIDs = append(trackIDs, parsed)
		}
	}
	return trackIDs
}

func parseTrackIDField(field string) string {
	if strings.Contains(field, "track=") && strings.Contains(field, "class=") {
		rawQuery := field
		if idx := strings.Index(field, "?"); idx != -1 {
			rawQuery = field[idx+1:]
		}

		values, err := url.ParseQuery(rawQuery)
		if err == nil {
			trackID := strings.TrimSpace(values.Get("track"))
			classID := strings.TrimSpace(values.Get("class"))
			if trackID != "" && classID != "" {
				return trackID + "-" + classID
			}
		}
	}

	return field
}

// checkTrigger checks for the trigger file and handles it
func (w *RefreshWatcher) checkTrigger() {
	// Ultra-lightweight existence check
	if _, err := os.Stat(w.triggerPath); err != nil {
		// File doesn't exist, nothing to do
		return
	}

	// Found trigger file
	log.Printf("🪙 Refresh trigger file detected: %s", w.triggerPath)

	// Read file contents before deleting to check for track IDs
	fileContent, readErr := os.ReadFile(w.triggerPath)
	var trackIDs []string
	if readErr == nil {
		trackIDs = parseRefreshTrackIDs(string(fileContent))
	}

	// Attempt to remove to avoid repeated triggers
	if rmErr := os.Remove(w.triggerPath); rmErr != nil {
		log.Printf("⚠️ Could not remove trigger file: %v", rmErr)
	}

	// Skip if already fetching
	if w.isBusy != nil && w.isBusy() {
		log.Println("⏭️ Skipping manual refresh - fetch already in progress")
		return
	}

	// Trigger the refresh callback
	if w.onRefresh != nil {
		w.onRefresh(trackIDs, "manual")
	}
}
