package internal

// Config holds application configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Schedule ScheduleConfig `json:"schedule"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port int `json:"port"`
}

// ScheduleConfig holds scheduling configuration
type ScheduleConfig struct {
	RefreshHour     int `json:"refresh_hour"`
	IndexingMinutes int `json:"indexing_minutes"`
}

// GetDefaultConfig returns default configuration
func GetDefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Schedule: ScheduleConfig{
			RefreshHour:     4, // 4 AM
			IndexingMinutes: 2, // Every 2 minutes
		},
	}
}
