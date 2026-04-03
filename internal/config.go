package internal

import (
	"os"
	"strings"
)

// Config holds application configuration
type Config struct {
	Schedule ScheduleConfig `json:"schedule"`
	Discord  DiscordConfig  `json:"discord"`
	Data     DataConfig     `json:"data"`
}

// ScheduleConfig holds scheduling configuration
type ScheduleConfig struct {
	RefreshHour                  int `json:"refresh_hour"`
	RefreshMinute                int `json:"refresh_minute"`
	IndexingMinutes              int `json:"indexing_minutes"`
	DailyRaceRefreshIntervalMins int `json:"daily_race_refresh_interval_mins"` // How often to refresh Daily Race combinations (minutes)
}

// DiscordConfig holds Discord integration configuration
type DiscordConfig struct {
	BotToken         string `json:"bot_token"`          // Discord bot token (stored in discord_token - DO NOT COMMIT!)
	ChannelID        string `json:"channel_id"`         // Channel ID to monitor
	MessageCheckMins int    `json:"message_check_mins"` // How far back to check for new messages (in minutes)
	Enabled          bool   `json:"enabled"`            // Whether Discord integration is enabled
}

// DataConfig holds data fetching configuration
type DataConfig struct {
	MultiplayerPositionLimit int `json:"multiplayer_position_limit"` // Maximum number of multiplayer positions to track
	APIThrottleMs            int `json:"api_throttle_ms"`            // Delay between API calls in milliseconds (default 20)
}

// GetDefaultConfig returns default configuration
func GetDefaultConfig() Config {
	// Try to read Discord bot token from discord_token file
	discordToken := ""
	if data, err := os.ReadFile("discord_token"); err == nil {
		discordToken = strings.TrimSpace(string(data))
	}

	return Config{
		Schedule: ScheduleConfig{
			RefreshHour:                  3,  // 3 AM
			RefreshMinute:                30, // At the half hour
			IndexingMinutes:              60, // Every 60 minutes during fetching (reduced from 30 to lower memory pressure)
			DailyRaceRefreshIntervalMins: 60, // Refresh Daily Race combinations every hour
		},
		Discord: DiscordConfig{
			BotToken:         discordToken,
			ChannelID:        "1468248477736894648", // Your Discord channel
			MessageCheckMins: 70,                    // Check messages from last 70 minutes
			Enabled:          discordToken != "",    // Enable only if token is set
		},
		Data: DataConfig{
			MultiplayerPositionLimit: 5000, // Track top 5000 multiplayer positions
			APIThrottleMs:            20,   // 20ms between API calls
		},
	}
}
