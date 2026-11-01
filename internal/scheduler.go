package internal

import (
	"log"
	"time"
)

const automaticRefreshInterval = 6 * time.Hour

// Scheduler handles automatic data refresh at scheduled times
type Scheduler struct {
	refreshHour   int // Hour of day (0-23) to refresh
	refreshMinute int // Minute of hour (0-59) to refresh
	stopChan      chan bool
	stopped       bool
}

// NewScheduler creates a new scheduler with the specified refresh time
// refreshHour: 0-23, refreshMinute: 0-59
func NewScheduler(refreshHour, refreshMinute int) *Scheduler {
	return &Scheduler{
		refreshHour:   refreshHour,
		refreshMinute: refreshMinute,
		stopChan:      make(chan bool),
		stopped:       false,
	}
}

// Start begins the background scheduler
func (s *Scheduler) Start(refreshCallback func()) {
	go s.runScheduler(refreshCallback)
}

// Stop stops the background scheduler
func (s *Scheduler) Stop() {
	if !s.stopped {
		s.stopped = true
		close(s.stopChan)
		log.Println("📅 Scheduler stop signal sent")
	}
}

// runScheduler runs the background scheduling loop
func (s *Scheduler) runScheduler(refreshCallback func()) {
	defer func() {
		// Clean up on exit
		log.Println("📅 Scheduler goroutine exiting")
	}()

	for {
		now := time.Now()
		nextRefresh := nextAutomaticRefreshTime(now, s.refreshHour, s.refreshMinute)

		timeUntilRefresh := time.Until(nextRefresh)
		log.Printf("📅 Next automatic refresh scheduled in %v (at %s)", timeUntilRefresh.Round(time.Minute), nextRefresh.Format("2006-01-02 15:04"))

		// Use a timer instead of time.After to allow cleanup
		timer := time.NewTimer(timeUntilRefresh)

		// Wait until refresh time or stop signal
		select {
		case <-timer.C:
			log.Printf("🕓 Automatic cache refresh triggered at %s", time.Now().Format("15:04"))
			refreshCallback()
		case <-s.stopChan:
			timer.Stop()
			log.Println("📅 Scheduler stopped")
			return
		}
	}
}

func nextAutomaticRefreshTime(now time.Time, refreshHour, refreshMinute int) time.Time {
	nextRefresh := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		refreshHour,
		refreshMinute,
		0,
		0,
		now.Location(),
	)
	if !nextRefresh.After(now) {
		elapsed := now.Sub(nextRefresh)
		intervalsUntilRefresh := elapsed/automaticRefreshInterval + 1
		nextRefresh = nextRefresh.Add(intervalsUntilRefresh * automaticRefreshInterval)
	}
	return nextRefresh
}
