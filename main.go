package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"
)

// Simplified driver result struct
type DriverResult struct {
	Name     string
	Position int
	LapTime  string
	Country  string
	Track    string
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

func directDriverSearch(driverName, trackID, classID string) {
	// Always add "class-" prefix to the class ID
	classID = "class-" + classID

	// Start timing for total API response
	apiStartTime := time.Now()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 20 * time.Second, // Reduced timeout
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
	apiReq.Header.Set("Accept", "application/json")
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

	// Parse JSON directly to the nested structure we need
	var response struct {
		Context struct {
			C struct {
				Results []map[string]interface{} `json:"results"`
			} `json:"c"`
		} `json:"context"`
	}

	if err := json.NewDecoder(apiResp.Body).Decode(&response); err != nil {
		log.Fatal("❌ JSON parse failed:", err)
	}

	results := response.Context.C.Results
	if len(results) == 0 {
		log.Fatal("❌ No entries found for this track/class combination")
	}

	// Log API timing
	apiDuration := time.Since(apiStartTime)
	log.Printf("📊 API Response: %.3f seconds (%d entries)", apiDuration.Seconds(), len(results))

	// Start timing for search operation
	searchStartTime := time.Now()

	// Optimized search - precompute search terms once
	searchTerms := strings.Fields(strings.ToLower(driverName))

	// Search for driver with early exit
	for _, entry := range results {
		if driver, ok := entry["driver"].(map[string]interface{}); ok {
			if name, ok := driver["name"].(string); ok {
				driverLower := strings.ToLower(name)

				// Quick check - if all search terms match
				allMatch := true
				for _, term := range searchTerms {
					if !strings.Contains(driverLower, term) {
						allMatch = false
						break
					}
				}

				if allMatch {
					// Extract data efficiently
					position := 1
					if globalIndex, ok := entry["global_index"].(float64); ok {
						position = int(globalIndex)
					}

					lapTime, _ := entry["laptime"].(string)

					country := ""
					if countryObj, ok := entry["country"].(map[string]interface{}); ok {
						country, _ = countryObj["name"].(string)
					}

					trackName := ""
					if track, ok := entry["track"].(map[string]interface{}); ok {
						trackName, _ = track["name"].(string)
					}

					// Log search timing
					searchDuration := time.Since(searchStartTime)
					log.Printf("🔍 Search Time: %.3f seconds", searchDuration.Seconds())

					// Output result
					log.Printf("\n🎯 FOUND: %s", name)
					log.Printf("🏆 Position: #%d", position)
					log.Printf("⏱️ Lap Time: %s", lapTime)
					log.Printf("🌍 Country: %s", country)
					log.Printf("🏁 Track: %s", trackName)
					log.Printf("📍 Track ID: %s", trackID)
					log.Printf("🏎️ Class ID: %s", classID)
					return
				}
			}
		}
	}

	// Log search timing even if not found
	searchDuration := time.Since(searchStartTime)
	log.Printf("🔍 Search Time: %.3f seconds", searchDuration.Seconds())
	log.Printf("❌ '%s' not found in %d entries on track %s, class %s", driverName, len(results), trackID, classID)
}
