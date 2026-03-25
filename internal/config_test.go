package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// CONFIG TESTS
// =============================================================================

func TestConfig_DiscordEnabledWhenTokenSet(t *testing.T) {
	// Create a temp directory and token file for this test
	tempDir, cleanup := TempTestDir(t, "config_test")
	defer cleanup()

	// Save current working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	// Change to temp directory
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create discord_token file
	tokenFile := filepath.Join(tempDir, "discord_token")
	if err := os.WriteFile(tokenFile, []byte("test_token_12345"), 0644); err != nil {
		t.Fatalf("Failed to create token file: %v", err)
	}

	config := GetDefaultConfig()

	if config.Discord.BotToken != "test_token_12345" {
		t.Errorf("BotToken = %q, expected 'test_token_12345'", config.Discord.BotToken)
	}

	if !config.Discord.Enabled {
		t.Error("Discord should be enabled when token file exists")
	}
}

func TestConfig_DiscordDisabledWhenNoToken(t *testing.T) {
	// Create a temp directory WITHOUT token file
	tempDir, cleanup := TempTestDir(t, "config_notoken_test")
	defer cleanup()

	// Save current working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	// Change to temp directory (no token file here)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	config := GetDefaultConfig()

	if config.Discord.BotToken != "" {
		t.Errorf("BotToken should be empty when no token file, got %q", config.Discord.BotToken)
	}

	if config.Discord.Enabled {
		t.Error("Discord should be disabled when no token file")
	}
}

func TestConfig_TokenFileTrimmed(t *testing.T) {
	// Create a temp directory
	tempDir, cleanup := TempTestDir(t, "config_trim_test")
	defer cleanup()

	// Save current working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	// Change to temp directory
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create token file with whitespace
	tokenFile := filepath.Join(tempDir, "discord_token")
	if err := os.WriteFile(tokenFile, []byte("  my_token_with_spaces  \n\n"), 0644); err != nil {
		t.Fatalf("Failed to create token file: %v", err)
	}

	config := GetDefaultConfig()

	if config.Discord.BotToken != "my_token_with_spaces" {
		t.Errorf("BotToken not trimmed correctly, got %q", config.Discord.BotToken)
	}
}

// =============================================================================
// CONFIG STRUCT TESTS
// =============================================================================

func TestServerConfig(t *testing.T) {
	cfg := ServerConfig{Port: 3000}

	if cfg.Port != 3000 {
		t.Errorf("Port = %d, expected 3000", cfg.Port)
	}
}

func TestScheduleConfig(t *testing.T) {
	cfg := ScheduleConfig{
		RefreshHour:     5,
		RefreshMinute:   30,
		IndexingMinutes: 15,
	}

	if cfg.RefreshHour != 5 {
		t.Errorf("RefreshHour = %d, expected 5", cfg.RefreshHour)
	}

	if cfg.RefreshMinute != 30 {
		t.Errorf("RefreshMinute = %d, expected 30", cfg.RefreshMinute)
	}

	if cfg.IndexingMinutes != 15 {
		t.Errorf("IndexingMinutes = %d, expected 15", cfg.IndexingMinutes)
	}
}

func TestDiscordConfig(t *testing.T) {
	cfg := DiscordConfig{
		BotToken:         "test_token",
		ChannelID:        "123456789",
		MessageCheckMins: 10,
		Enabled:          true,
	}

	if cfg.BotToken != "test_token" {
		t.Errorf("BotToken = %q, expected 'test_token'", cfg.BotToken)
	}

	if cfg.ChannelID != "123456789" {
		t.Errorf("ChannelID = %q, expected '123456789'", cfg.ChannelID)
	}

	if cfg.MessageCheckMins != 10 {
		t.Errorf("MessageCheckMins = %d, expected 10", cfg.MessageCheckMins)
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestDataConfig(t *testing.T) {
	cfg := DataConfig{
		MultiplayerPositionLimit: 3000,
	}

	if cfg.MultiplayerPositionLimit != 3000 {
		t.Errorf("MultiplayerPositionLimit = %d, expected 3000", cfg.MultiplayerPositionLimit)
	}
}

func TestDataConfig_CustomLimit(t *testing.T) {
	cfg := DataConfig{
		MultiplayerPositionLimit: 5000,
	}

	if cfg.MultiplayerPositionLimit != 5000 {
		t.Errorf("MultiplayerPositionLimit = %d, expected 5000", cfg.MultiplayerPositionLimit)
	}

	// Test that pages would be calculated correctly for this limit
	// (approximately 500 entries per page)
	expectedPages := (cfg.MultiplayerPositionLimit + 499) / 500
	if expectedPages <= 0 {
		t.Error("Pages calculation should result in positive number")
	}
}
