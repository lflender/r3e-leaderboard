package internal

import (
	"log"
	"time"
)

// Scheduler handles automatic data refresh at scheduled times
type Scheduler struct {
	refreshHour     int // Hour of day (0-23) to refresh
	refreshMinute   int // Minute of hour (0-59) to refresh
	refreshInterval time.Duration
	stopChan        chan bool
	stopped         bool
}

// NewScheduler creates a scheduler with a daily anchor time and repeat interval.
// refreshHour: 0-23, refreshMinute: 0-59, refreshIntervalHours: positive number of hours
func NewScheduler(refreshHour, refreshMinute, refreshIntervalHours int) *Scheduler {
	if refreshIntervalHours <= 0 {
		log.Printf("⚠️ Invalid full refresh interval (%d hours); defaulting to %d hours",
			refreshIntervalHours, defaultFullRefreshIntervalHours)
		refreshIntervalHours = defaultFullRefreshIntervalHours
	}

	return &Scheduler{
		refreshHour:     refreshHour,
		refreshMinute:   refreshMinute,
		refreshInterval: time.Duration(refreshIntervalHours) * time.Hour,
		stopChan:        make(chan bool),
		stopped:         false,
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
		nextRefresh := nextAutomaticRefreshTime(now, s.refreshHour, s.refreshMinute, s.refreshInterval)

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

func nextAutomaticRefreshTime(now time.Time, refreshHour, refreshMinute int, refreshInterval time.Duration) time.Time {
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
		intervalsUntilRefresh := elapsed/refreshInterval + 1
		nextRefresh = nextRefresh.Add(intervalsUntilRefresh * refreshInterval)
	}
	return nextRefresh
}
