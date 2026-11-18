package internal

import (
	"log"
	"time"
)

const defaultFullRefreshIntervalHours = 3

func fullRefreshInterval(hours int) time.Duration {
	if hours <= 0 {
		log.Printf("⚠️ Invalid full refresh interval (%d hours); defaulting to %d hours", hours, defaultFullRefreshIntervalHours)
		hours = defaultFullRefreshIntervalHours
	}
	return time.Duration(hours) * time.Hour
}

// Config holds application configuration
type Config struct {
	Schedule ScheduleConfig `json:"schedule"`
	Data     DataConfig     `json:"data"`
}

// ScheduleConfig holds scheduling configuration
type ScheduleConfig struct {
	RefreshHour           int          `json:"refresh_hour"`
	RefreshMinute         int          `json:"refresh_minute"`
	RefreshIntervalHours  int          `json:"refresh_interval_hours"`
	IndexingMinutes       int          `json:"indexing_minutes"`
	AvatarBackfillWeekday time.Weekday `json:"avatar_backfill_weekday"`
	AvatarBackfillHour    int          `json:"avatar_backfill_hour"`
	AvatarBackfillMinute  int          `json:"avatar_backfill_minute"`
}

// DataConfig holds data fetching configuration
type DataConfig struct {
	MultiplayerPositionLimit int `json:"multiplayer_position_limit"` // Maximum positions to track; -1 tracks all positions
	APIThrottleMs            int `json:"api_throttle_ms"`            // Delay between API calls in milliseconds (default 20)
}

// GetDefaultConfig returns default configuration
func GetDefaultConfig() Config {
	return Config{
		Schedule: ScheduleConfig{
			RefreshHour:           2, // 2 AM
			RefreshMinute:         0, // At the top of the hour
			RefreshIntervalHours:  defaultFullRefreshIntervalHours,
			IndexingMinutes:       60, // Every 60 minutes during fetching (reduced from 30 to lower memory pressure)
			AvatarBackfillWeekday: time.Monday,
			AvatarBackfillHour:    4,
			AvatarBackfillMinute:  0,
		},
		Data: DataConfig{
			MultiplayerPositionLimit: unlimitedMultiplayerPositionLimit,
			APIThrottleMs:            20, // 20ms between API calls
		},
	}
}
