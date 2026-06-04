package internal

import (
"context"
"encoding/json"
"fmt"
"io"
"log"
"net/http"
"strings"
"time"
)

const (
discordAPIBase = "https://discord.com/api/v10"
)
// DiscordMessage represents a message from Discord API
type DiscordMessage struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
}

// DiscordClient handles Discord API interactions
type DiscordClient struct {
	httpClient *http.Client
	token      string
	channelID  string
}

// NewDiscordClient creates a new Discord API client
func NewDiscordClient(config DiscordConfig) *DiscordClient {
	return &DiscordClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token:     config.BotToken,
		channelID: config.ChannelID,
	}
}

// FetchRecentMessages fetches messages from the channel within the specified time window
func (d *DiscordClient) FetchRecentMessages(ctx context.Context, withinMinutes int) ([]DiscordMessage, error) {
	if d.token == "" {
		return nil, fmt.Errorf("discord bot token not configured")
	}

	log.Printf("🔍 Discord API: Fetching recent messages from channel %s", d.channelID)
	url := fmt.Sprintf("%s/channels/%s/messages?limit=10", discordAPIBase, d.channelID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+d.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "R3E-Leaderboard-Bot/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("📡 Discord API response: HTTP %d", resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Discord API error details: %s", string(body))
		return nil, fmt.Errorf("discord API error: %d - %s", resp.StatusCode, string(body))
	}

	var messages []DiscordMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	return messages, nil
}

// FindDailySprintRacesMessage finds a message containing the Daily Sprint Races section
func (d *DiscordClient) FindDailySprintRacesMessage(messages []DiscordMessage) *DiscordMessage {
	for i := range messages {
		if strings.Contains(messages[i].Content, "Daily Sprint Races") {
			return &messages[i]
		}
	}
	return nil
}

// CheckForNewDailySprintRaces checks Discord for new Daily Sprint Races messages
// and returns the parsed result if found
func (d *DiscordClient) CheckForNewDailySprintRaces(ctx context.Context, withinMinutes int) (*DailySprintRacesResult, error) {
	messages, err := d.FetchRecentMessages(ctx, withinMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Discord messages: %w", err)
	}

	// Find message with Daily Sprint Races
	raceMessage := d.FindDailySprintRacesMessage(messages)
	if raceMessage == nil {
		log.Println("ℹ️ No Daily Sprint Races message found")
		return nil, nil
	}

	// Parse the message
	result := ParseDailySprintRaces(raceMessage)
	if result == nil {
		return nil, fmt.Errorf("failed to parse Daily Sprint Races message")
	}

	log.Printf("✅ Parsed %d sprint races and %d feature races from Discord message", len(result.Races), len(result.FeatureRaces))

	// Load full names for better logging
	tracks := GetTracks()
	carClasses := GetCarClasses()
	classNameByID := make(map[string]string, len(carClasses))
	for _, class := range carClasses {
		classNameByID[class.ClassID] = class.Name
	}

	resolveClassDisplay := func(race DailySprintRace) (string, string) {
		// For alias/category races, always show concrete class IDs in logs.
		if len(race.CategoryIDs) > 0 {
			resolvedNames := make([]string, 0, len(race.CategoryIDs))
			for _, id := range race.CategoryIDs {
				if name, ok := classNameByID[id]; ok {
					resolvedNames = append(resolvedNames, name)
				}
			}

			classNameDisplay := race.CarClass
			if len(resolvedNames) > 0 {
				classNameDisplay = strings.Join(resolvedNames, " + ")
			}
			return classNameDisplay, strings.Join(race.CategoryIDs, ",")
		}

		if race.CarClassID != "" {
			if name, ok := classNameByID[race.CarClassID]; ok {
				return name, race.CarClassID
			}
		}

		return race.CarClass, race.CarClassID
	}

	logRaces := func(label string, races []DailySprintRace) {
		if len(races) == 0 {
			return
		}
		log.Printf("📌 %s:", label)
		for _, race := range races {
			status := "❌"
			if race.MatchedOK {
				status = "✅"
			} else if race.ParsedOK {
				status = "⚠️"
			}

			fullClassName, classIDDisplay := resolveClassDisplay(race)
			fullTrackName := race.Track

			if race.TrackID != "" {
				for _, track := range tracks {
					if track.TrackID == race.TrackID {
						fullTrackName = track.Name
						break
					}
				}
			}

			log.Printf("  %s %s - %s", status, race.CarClass, race.Track)
			log.Printf("      → %s (ID: %s) - %s (ID: %s)",
				fullClassName, classIDDisplay, fullTrackName, race.TrackID)
		}
	}

	logRaces("Daily Sprint Races", result.Races)
	logRaces("Daily Feature Races", result.FeatureRaces)

	return result, nil
}
