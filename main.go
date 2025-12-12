package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// LeaderboardEntry represents a single leaderboard entry
type LeaderboardEntry struct {
	Pos        int    `json:"pos"`
	Driver     string `json:"driver"`
	LapTime    string `json:"lap_time"`
	Rank       string `json:"rank"`
	Region     string `json:"region"`
	CarClass   string `json:"car_class"`
	Track      string `json:"track"`
	Difficulty string `json:"difficulty"`
	Team       string `json:"team"`
	TrackID    string `json:"track_id"`
	ClassID    string `json:"class_id"`
}

// LeaderboardData represents complete data for one car class + track combination
type LeaderboardData struct {
	CarClass  string             `json:"car_class"`
	ClassID   string             `json:"class_id"`
	Track     string             `json:"track"`
	TrackID   string             `json:"track_id"`
	Entries   []LeaderboardEntry `json:"entries"`
	ScrapedAt time.Time          `json:"scraped_at"`
}

func main() {
	// Check for command line arguments for direct driver search
	if len(os.Args) >= 4 {
		driverName := os.Args[1]
		trackID := os.Args[2]
		classID := os.Args[3]

		log.Printf("🔍 Quick Search: %s on track %s, class %s", driverName, trackID, classID)

		// Direct search mode
		directDriverSearch(driverName, trackID, classID)
		return
	}

	// Show usage information if no arguments provided
	log.Println("🏎️  RaceRoom Leaderboard Driver Search")
	log.Println("Usage: program.exe \"Driver Name\" trackID classID")
	log.Println("Example: program.exe \"Alex Pate\" 9344 1703")
	log.Println("Example: program.exe \"Stefan Krause\" 1693 1703")
	log.Println("Note: Class ID should be just the number (1703), 'class-' is added automatically")
}

func getLeaderboardsForClass(client *http.Client, config ScraperConfig, data RaceRoomData, carClass CarClass) []LeaderboardData {
	leaderboards := []LeaderboardData{}

	// Generate track IDs to test
	trackIDs := generateTrackIDsFromRanges(data.TrackRanges)

	for _, trackID := range trackIDs {
		log.Printf("    Trying track ID: %s", trackID)
		leaderboard, err := scrapeLeaderboard(client, config, data, carClass, trackID)
		if err != nil {
			log.Printf("    ❌ Error for track %s: %v", trackID, err)
			continue // Skip invalid combinations
		}

		if len(leaderboard.Entries) > 0 {
			leaderboards = append(leaderboards, *leaderboard)
			log.Printf("    ✓ %s on %s (%d entries)", carClass.Name, leaderboard.Track, len(leaderboard.Entries))
		}

		// Rate limiting
		time.Sleep(config.RateLimit)
	}

	return leaderboards
}

func generateTrackIDsFromRanges(ranges []TrackIDRange) []string {
	trackIDs := []string{
		"10394", // Known: Donington Park
		"8367",  // Known: Anderstorp
		"1693",  // Hockenheimring Grand Prix
	}

	for _, r := range ranges {
		for id := r.Start; id <= r.End; id += r.Step {
			trackIDs = append(trackIDs, strconv.Itoa(id))
		}
	}

	return trackIDs
}

// JSON API response structures
type APIResponse struct {
	Results []APIResult `json:"results"`
}

type APIResult struct {
	Driver      APIDriver   `json:"driver"`
	Laptime     string      `json:"laptime"`
	Rank        string      `json:"rank"`
	Country     APICountry  `json:"country"`
	CarClass    APICarClass `json:"car_class"`
	Track       APITrack    `json:"track"`
	GlobalIndex int         `json:"global_index"`
	DateTime    string      `json:"date_time"`
}

type APIDriver struct {
	Name string `json:"name"`
}

type APICountry struct {
	Name string `json:"name"`
}

type APICarClass struct {
	Class APIClass `json:"class"`
}

type APIClass struct {
	Name string `json:"name"`
}

type APITrack struct {
	Name string `json:"name"`
}

func scrapeLeaderboard(client *http.Client, config ScraperConfig, data RaceRoomData, carClass CarClass, trackID string) (*LeaderboardData, error) {
	// First visit main leaderboard page to establish session
	mainURL := config.BaseURL + "?car_class=" + carClass.ID + "&track=" + trackID
	req, err := http.NewRequest("GET", mainURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	// Use JSON API endpoint to get all entries
	apiParams := url.Values{}
	apiParams.Add("start", "0")
	apiParams.Add("count", "2000") // Request up to 2000 entries to get all
	apiParams.Add("track", trackID)
	apiParams.Add("car_class", carClass.ID)
	// Use correct API endpoint structure: /leaderboard/listing/0?track=ID&car_class=CLASS&start=0&count=COUNT
	baseAPI := "https://game.raceroom.com/leaderboard/listing/0"
	apiURL := baseAPI + "?track=" + trackID + "&car_class=" + carClass.ID + "&start=0&count=1500"

	// Make JSON API request
	apiReq, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	apiReq.Header.Set("User-Agent", config.UserAgent)
	apiReq.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	apiReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	apiReq.Header.Set("Referer", mainURL)

	apiResp, err := client.Do(apiReq)
	if err != nil {
		return nil, err
	}
	defer apiResp.Body.Close()

	if apiResp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", apiResp.StatusCode)
	}

	// Parse JSON response - the data is in context.c.results, not directly in results
	var rawResponse map[string]interface{}
	if err := json.NewDecoder(apiResp.Body).Decode(&rawResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	// Extract results from context.c.results
	var results []interface{}
	if context, ok := rawResponse["context"].(map[string]interface{}); ok {
		if c, ok := context["c"].(map[string]interface{}); ok {
			if r, ok := c["results"].([]interface{}); ok {
				results = r
			}
		}
	}

	// Check if we have results
	if len(results) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	// Get track name from first result
	var trackName string
	if len(results) > 0 {
		if firstResult, ok := results[0].(map[string]interface{}); ok {
			if track, ok := firstResult["track"].(map[string]interface{}); ok {
				if name, ok := track["name"].(string); ok {
					trackName = name
				}
			}
		}
	}
	if trackName == "" {
		return nil, fmt.Errorf("no track name found")
	}

	// Convert raw results to leaderboard entries
	entries := convertRawResults(results, trackID, carClass.ID)

	return &LeaderboardData{
		CarClass:  carClass.Name,
		ClassID:   carClass.ID,
		Track:     trackName,
		TrackID:   trackID,
		Entries:   entries,
		ScrapedAt: time.Now(),
	}, nil
}

func convertRawResults(rawResults []interface{}, trackID, classID string) []LeaderboardEntry {
	entries := make([]LeaderboardEntry, 0, len(rawResults))

	for i, rawResult := range rawResults {
		if result, ok := rawResult.(map[string]interface{}); ok {
			// Extract driver name
			var driverName string
			if driver, ok := result["driver"].(map[string]interface{}); ok {
				if name, ok := driver["name"].(string); ok {
					driverName = name
				}
			}

			// Extract position
			position := i + 1 // default to array index
			if globalIndex, ok := result["global_index"].(float64); ok {
				position = int(globalIndex) // global_index is already 1-based
			}

			// Extract lap time
			var lapTime string
			if lt, ok := result["laptime"].(string); ok {
				lapTime = lt
			}

			// Extract rank
			var rank string
			if r, ok := result["rank"].(string); ok {
				rank = r
			}

			// Extract country
			var country string
			if countryObj, ok := result["country"].(map[string]interface{}); ok {
				if name, ok := countryObj["name"].(string); ok {
					country = name
				}
			}

			// Extract car class name
			var carClassName string
			if carClass, ok := result["car_class"].(map[string]interface{}); ok {
				if class, ok := carClass["class"].(map[string]interface{}); ok {
					if name, ok := class["name"].(string); ok {
						carClassName = name
					}
				}
			}

			// Extract track name
			var trackName string
			if track, ok := result["track"].(map[string]interface{}); ok {
				if name, ok := track["name"].(string); ok {
					trackName = name
				}
			}

			entry := LeaderboardEntry{
				Pos:        position,
				Driver:     driverName,
				LapTime:    lapTime,
				Rank:       rank,
				Region:     country,
				CarClass:   carClassName,
				Track:      trackName,
				Difficulty: "Challenge",
				Team:       "", // Not available in API
				TrackID:    trackID,
				ClassID:    classID,
			}
			entries = append(entries, entry)
		}
	}

	return entries
}

func convertAPIResults(apiResults []APIResult, trackID, classID string) []LeaderboardEntry {
	entries := make([]LeaderboardEntry, 0, len(apiResults))

	for _, result := range apiResults {
		entry := LeaderboardEntry{
			Pos:        result.GlobalIndex + 1, // API uses 0-based index
			Driver:     result.Driver.Name,
			LapTime:    result.Laptime,
			Rank:       result.Rank,
			Region:     result.Country.Name,
			CarClass:   result.CarClass.Class.Name,
			Track:      result.Track.Name,
			Difficulty: "Challenge",
			Team:       "", // Not available in API
			TrackID:    trackID,
			ClassID:    classID,
		}
		entries = append(entries, entry)
	}

	return entries
}

func searchForDriverInAllBoards(leaderboards []LeaderboardData, searchName string) {
	log.Printf("\n🔍 Searching for %s across all leaderboards...", searchName)
	found := false

	for _, leaderboard := range leaderboards {
		for _, entry := range leaderboard.Entries {
			if strings.Contains(strings.ToLower(entry.Driver), strings.ToLower(searchName)) {
				if !found {
					log.Printf("\n🎯 FOUND %s!", searchName)
					found = true
				}
				log.Printf("👤 Driver: %s", entry.Driver)
				log.Printf("🏆 Position: #%d", entry.Pos)
				log.Printf("⏱️ Lap Time: %s", entry.LapTime)
				log.Printf("🌍 Region: %s", entry.Region)
				log.Printf("🏎️ Car Class: %s", entry.CarClass)
				log.Printf("🏁 Track: %s", entry.Track)
				log.Printf("📍 Track ID: %s", entry.TrackID)
				log.Printf("🆔 Class ID: %s", entry.ClassID)
				if entry.Rank != "" {
					log.Printf("📊 Rank: %s", entry.Rank)
				}
				log.Println()
			}
		}
	}

	if !found {
		log.Printf("❌ %s not found in any leaderboard", searchName)
	}
}

func printSummary(leaderboards []LeaderboardData) {
	log.Println("\n📊 SCRAPING SUMMARY:")
	log.Printf("Total leaderboards: %d", len(leaderboards))

	classCount := make(map[string]int)
	trackCount := make(map[string]int)
	totalEntries := 0

	for _, lb := range leaderboards {
		classCount[lb.CarClass]++
		trackCount[lb.Track]++
		totalEntries += len(lb.Entries)
	}

	log.Println("\n🏎️  Car Classes:")
	for class, count := range classCount {
		log.Printf("  - %s: %d leaderboards", class, count)
	}

	log.Println("\n🏁 Tracks:")
	for track, count := range trackCount {
		log.Printf("  - %s: %d leaderboards", track, count)
	}

	log.Printf("\n👥 Total driver entries: %d", totalEntries)
}

func directDriverSearch(driverName, trackID, classID string) {
	// Always add "class-" prefix to the class ID
	classID = "class-" + classID

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
	}

	// Establish session
	mainURL := "https://game.raceroom.com/leaderboard/?car_class=" + classID + "&track=" + trackID
	req, _ := http.NewRequest("GET", mainURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("❌ Session failed:", err)
	}
	resp.Body.Close()

	// Direct API call
	apiURL := "https://game.raceroom.com/leaderboard/listing/0?track=" + trackID + "&car_class=" + classID + "&start=0&count=1500"

	apiReq, _ := http.NewRequest("GET", apiURL, nil)
	apiReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	apiReq.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	apiReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	apiReq.Header.Set("Referer", mainURL)

	apiResp, err := client.Do(apiReq)
	if err != nil {
		log.Fatal("❌ API failed:", err)
	}
	defer apiResp.Body.Close()

	if apiResp.StatusCode != 200 {
		log.Fatal("❌ HTTP", apiResp.StatusCode)
	}

	// Parse JSON
	var rawResponse map[string]interface{}
	if err := json.NewDecoder(apiResp.Body).Decode(&rawResponse); err != nil {
		log.Fatal("❌ JSON parse failed:", err)
	}

	var results []interface{}
	if context, ok := rawResponse["context"].(map[string]interface{}); ok {
		if c, ok := context["c"].(map[string]interface{}); ok {
			if r, ok := c["results"].([]interface{}); ok {
				results = r
			}
		}
	}

	if len(results) == 0 {
		log.Fatal("❌ No entries found for this track/class combination")
	}

	log.Printf("📊 Searching %d entries...", len(results))

	// Search for the driver
	searchTerms := strings.Fields(strings.ToLower(driverName))
	found := false

	for i, rawEntry := range results {
		if entry, ok := rawEntry.(map[string]interface{}); ok {
			var driverNameFromAPI string
			if driver, ok := entry["driver"].(map[string]interface{}); ok {
				if name, ok := driver["name"].(string); ok {
					driverNameFromAPI = name
				}
			}

			if driverNameFromAPI != "" {
				driverLower := strings.ToLower(driverNameFromAPI)

				// Check if all search terms are in the driver name
				allTermsMatch := true
				for _, term := range searchTerms {
					if !strings.Contains(driverLower, term) {
						allTermsMatch = false
						break
					}
				}

				if allTermsMatch {
					position := i + 1
					if globalIndex, ok := entry["global_index"].(float64); ok {
						position = int(globalIndex)
					}

					var lapTime string
					if lt, ok := entry["laptime"].(string); ok {
						lapTime = lt
					}

					var country string
					if countryObj, ok := entry["country"].(map[string]interface{}); ok {
						if name, ok := countryObj["name"].(string); ok {
							country = name
						}
					}

					var trackName string
					if track, ok := entry["track"].(map[string]interface{}); ok {
						if name, ok := track["name"].(string); ok {
							trackName = name
						}
					}

					log.Printf("\n🎯 FOUND: %s", driverNameFromAPI)
					log.Printf("🏆 Position: #%d", position)
					log.Printf("⏱️ Lap Time: %s", lapTime)
					log.Printf("🌍 Country: %s", country)
					log.Printf("🏁 Track: %s", trackName)
					log.Printf("📍 Track ID: %s", trackID)
					log.Printf("🏎️ Class ID: %s", classID)

					found = true
					return
				}
			}
		}
	}

	if !found {
		log.Printf("❌ '%s' not found in %d entries on track %s, class %s", driverName, len(results), trackID, classID)
	}
}

func saveToJSON(data []LeaderboardData, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
