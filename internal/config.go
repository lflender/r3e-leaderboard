package internal

import (
	"os"
	"strings"
)

// Config holds application configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Schedule ScheduleConfig `json:"schedule"`
	Discord  DiscordConfig  `json:"discord"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port int `json:"port"`
}

// ScheduleConfig holds scheduling configuration
type ScheduleConfig struct {
	RefreshHour     int `json:"refresh_hour"`
	RefreshMinute   int `json:"refresh_minute"`
	IndexingMinutes int `json:"indexing_minutes"`
}

// DiscordConfig holds Discord integration configuration
type DiscordConfig struct {
	BotToken         string `json:"bot_token"`          // Discord bot token (stored in discord_token - DO NOT COMMIT!)
	ChannelID        string `json:"channel_id"`         // Channel ID to monitor
	MessageCheckMins int    `json:"message_check_mins"` // How far back to check for new messages (in minutes)
	Enabled          bool   `json:"enabled"`            // Whether Discord integration is enabled
}

// GetDefaultConfig returns default configuration
func GetDefaultConfig() Config {
	// Try to read Discord bot token from discord_token file
	discordToken := ""
	if data, err := os.ReadFile("discord_token"); err == nil {
		discordToken = strings.TrimSpace(string(data))
	}

	return Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Schedule: ScheduleConfig{
			RefreshHour:     4,  // 4 AM
			RefreshMinute:   45, // At the top of the hour
			IndexingMinutes: 30, // Every 30 minutes during fetching
		},
		Discord: DiscordConfig{
			BotToken:         discordToken,
			ChannelID:        "1468248477736894648", // Your Discord channel
			MessageCheckMins: 15,                    // Check messages from last 5 minutes
			Enabled:          discordToken != "",    // Enable only if token is set
		},
	}
}
