package internal

import (
	"testing"
	"time"
)

// =============================================================================
// CONFIG TESTS
// =============================================================================

func TestConfig_AvatarBackfillDefaultsToMondayAtFour(t *testing.T) {
	config := GetDefaultConfig()
	if config.Schedule.AvatarBackfillWeekday != time.Monday || config.Schedule.AvatarBackfillHour != 4 || config.Schedule.AvatarBackfillMinute != 0 {
		t.Errorf("avatar backfill schedule = %s %02d:%02d, want Monday 04:00",
			config.Schedule.AvatarBackfillWeekday, config.Schedule.AvatarBackfillHour, config.Schedule.AvatarBackfillMinute)
	}
}

func TestConfig_MultiplayerPositionLimitDefaultsToUnlimited(t *testing.T) {
	config := GetDefaultConfig()
	if config.Data.MultiplayerPositionLimit != unlimitedMultiplayerPositionLimit {
		t.Errorf("MultiplayerPositionLimit = %d, want %d", config.Data.MultiplayerPositionLimit, unlimitedMultiplayerPositionLimit)
	}
}

func TestConfig_FullRefreshIntervalDefaultsToSixHours(t *testing.T) {
	config := GetDefaultConfig()
	if config.Schedule.RefreshIntervalHours != 3 {
		t.Errorf("RefreshIntervalHours = %d, want 3", config.Schedule.RefreshIntervalHours)
	}
}
