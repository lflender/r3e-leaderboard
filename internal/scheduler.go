package internal

import (
	"log"
	"time"
)

// Scheduler handles automatic data refresh at scheduled times
type Scheduler struct {
	refreshTime string // Time in format "15:04" (24h format)
	stopChan    chan bool
}

// NewScheduler creates a new scheduler with default refresh time of 4:00 AM
func NewScheduler() *Scheduler {
	return &Scheduler{
		refreshTime: "04:00",
		stopChan:    make(chan bool),
	}
}

// Start begins the background scheduler
func (s *Scheduler) Start(refreshCallback func()) {
	go s.runScheduler(refreshCallback)
}

// Stop stops the background scheduler
func (s *Scheduler) Stop() {
	s.stopChan <- true
}

// runScheduler runs the background scheduling loop
func (s *Scheduler) runScheduler(refreshCallback func()) {
	for {
		// Calculate time until next 4:00 AM
		now := time.Now()
		next4AM := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())

		// If it's already past 4:00 AM today, schedule for tomorrow
		if now.After(next4AM) {
			next4AM = next4AM.Add(24 * time.Hour)
		}

		timeUntilRefresh := time.Until(next4AM)
		log.Printf("📅 Next automatic refresh scheduled in %v (at %s)", timeUntilRefresh.Round(time.Minute), next4AM.Format("2006-01-02 15:04"))

		// Wait until refresh time or stop signal
		select {
		case <-time.After(timeUntilRefresh):
			log.Println("🕓 Automatic refresh triggered at 4:00 AM")
			refreshCallback()
		case <-s.stopChan:
			log.Println("📅 Scheduler stopped")
			return
		}
	}
}

// ForceRefresh triggers an immediate refresh (for manual "fetch" command)
func (s *Scheduler) ForceRefresh(refreshCallback func()) {
	log.Println("🔄 Manual refresh triggered")
	refreshCallback()
}
