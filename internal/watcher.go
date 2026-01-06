package internal

import (
	"context"
	"log"
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
		// Parse track IDs from file (space or newline separated)
		content := strings.TrimSpace(string(fileContent))
		if content != "" {
			// Split by whitespace (spaces, tabs, newlines)
			fields := strings.Fields(content)
			for _, field := range fields {
				if field != "" {
					trackIDs = append(trackIDs, field)
				}
			}
		}
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
