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
