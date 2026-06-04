package internal

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// DISCORD CLIENT CONSTRUCTION TESTS
// =============================================================================

func TestNewDiscordClient_SetsFields(t *testing.T) {
	cfg := DiscordConfig{
		BotToken:  "test-token-abc",
		ChannelID: "987654321",
	}
	client := NewDiscordClient(cfg)
	if client == nil {
		t.Fatal("NewDiscordClient returned nil")
	}
	if client.token != cfg.BotToken {
		t.Errorf("token = %q, expected %q", client.token, cfg.BotToken)
	}
	if client.channelID != cfg.ChannelID {
		t.Errorf("channelID = %q, expected %q", client.channelID, cfg.ChannelID)
	}
	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestNewDiscordClient_EmptyConfig(t *testing.T) {
	client := NewDiscordClient(DiscordConfig{})
	if client == nil {
		t.Fatal("NewDiscordClient returned nil for empty config")
	}
	if client.token != "" {
		t.Errorf("token should be empty, got %q", client.token)
	}
	if client.channelID != "" {
		t.Errorf("channelID should be empty, got %q", client.channelID)
	}
}

// =============================================================================
// FIND DAILY SPRINT RACES MESSAGE TESTS
// =============================================================================

func TestFindDailySprintRacesMessage_Found(t *testing.T) {
	messages := []DiscordMessage{
		{ID: "1", Content: "Some regular post", Timestamp: time.Now()},
		{ID: "2", Content: "📅 This Week in Ranked Multiplayer\n\nDaily Sprint Races (15 min)\n🏁 GT3 - Monza", Timestamp: time.Now()},
		{ID: "3", Content: "Another post", Timestamp: time.Now()},
	}

	client := NewDiscordClient(DiscordConfig{BotToken: "x", ChannelID: "1"})
	result := client.FindDailySprintRacesMessage(messages)

	if result == nil {
		t.Fatal("expected to find a message, got nil")
	}
	if result.ID != "2" {
		t.Errorf("expected message ID '2', got %q", result.ID)
	}
}

func TestFindDailySprintRacesMessage_ReturnsFirst(t *testing.T) {
	messages := []DiscordMessage{
		{ID: "1", Content: "Daily Sprint Races (15 min)\n🏁 GT3 - Monza"},
		{ID: "2", Content: "Daily Sprint Races (15 min)\n🏁 Super Touring - Zhejiang"},
	}

	client := NewDiscordClient(DiscordConfig{BotToken: "x", ChannelID: "1"})
	result := client.FindDailySprintRacesMessage(messages)

	if result == nil {
		t.Fatal("expected to find a message, got nil")
	}
	if result.ID != "1" {
		t.Errorf("expected first matching message ID '1', got %q", result.ID)
	}
}

func TestFindDailySprintRacesMessage_NotFound(t *testing.T) {
	messages := []DiscordMessage{
		{ID: "1", Content: "Some regular post"},
		{ID: "2", Content: "Another unrelated post"},
	}

	client := NewDiscordClient(DiscordConfig{BotToken: "x", ChannelID: "1"})
	result := client.FindDailySprintRacesMessage(messages)

	if result != nil {
		t.Errorf("expected nil, got message ID %q", result.ID)
	}
}

func TestFindDailySprintRacesMessage_EmptyList(t *testing.T) {
	client := NewDiscordClient(DiscordConfig{BotToken: "x", ChannelID: "1"})
	result := client.FindDailySprintRacesMessage([]DiscordMessage{})
	if result != nil {
		t.Errorf("expected nil for empty list, got message ID %q", result.ID)
	}
}

// =============================================================================
// FETCH RECENT MESSAGES TESTS
// =============================================================================

func TestFetchRecentMessages_NoToken(t *testing.T) {
	client := NewDiscordClient(DiscordConfig{ChannelID: "123"}) // no token
	_, err := client.FetchRecentMessages(context.Background(), 60)
	if err == nil {
		t.Fatal("expected error when token is empty, got nil")
	}
}

// =============================================================================
// CHECK FOR NEW DAILY SPRINT RACES TESTS
// =============================================================================

func TestCheckForNewDailySprintRaces_NoToken(t *testing.T) {
	client := NewDiscordClient(DiscordConfig{ChannelID: "123"}) // no token
	result, err := client.CheckForNewDailySprintRaces(context.Background(), 60)
	if err == nil {
		t.Fatal("expected error when token is empty, got nil")
	}
	if result != nil {
		t.Error("expected nil result when error occurs")
	}
}
