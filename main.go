package main

import (
	"log"
	"os"
)

func main() {
	// Check for command line arguments for direct driver search
	if len(os.Args) >= 4 {
		driverName := os.Args[1]
		trackID := os.Args[2]
		classID := os.Args[3]

		log.Printf("🔍 Quick Search: %s on track %s, class %s", driverName, trackID, classID)

		// Perform search using separated modules
		performDriverSearch(driverName, trackID, classID)
		return
	}

	// Show usage information if no arguments provided
	showUsage()
}

// showUsage displays help information
func showUsage() {
	log.Println("🏎️  RaceRoom Leaderboard Driver Search")
	log.Println("Usage: program.exe \"Driver Name\" trackID classID")
	log.Println("Example: program.exe \"Alex Pate\" 9344 1703")
	log.Println("Example: program.exe \"Stefan Krause\" 1693 1703")
	log.Println("Note: Class ID should be just the number (1703), 'class-' is added automatically")
}

// performDriverSearch orchestrates the complete search process
func performDriverSearch(driverName, trackID, classID string) {
	// Initialize modules
	apiClient := NewAPIClient()
	searchEngine := NewSearchEngine()

	// Fetch data from API
	data, apiDuration, err := apiClient.FetchLeaderboardData(trackID, classID)
	if err != nil {
		log.Fatal("❌ API request failed:", err)
	}

	if len(data) == 0 {
		log.Fatal("❌ No entries found for this track/class combination")
	}

	// Log API timing
	log.Printf("📊 API Response: %.3f seconds (%d entries)", apiDuration.Seconds(), len(data))

	// Search for driver
	result, searchDuration := searchEngine.FindDriver(driverName, data, trackID, classID)

	// Log search timing
	log.Printf("🔍 Search Time: %.3f seconds", searchDuration.Seconds())

	// Print results
	printSearchResult(result, len(data), trackID, classID)
}

// printSearchResult displays the search results in a formatted way
func printSearchResult(result DriverResult, totalEntries int, trackID, classID string) {
	if result.Found {
		log.Printf("\n🎯 FOUND: %s", result.Name)
		log.Printf("🏆 Position: #%d", result.Position)
		log.Printf("⏱️ Lap Time: %s", result.LapTime)
		log.Printf("🌍 Country: %s", result.Country)
		log.Printf("🏁 Track: %s", result.Track)
		log.Printf("📍 Track ID: %s", trackID)
		log.Printf("🏎️ Class ID: %s", result.ClassID)
	} else {
		log.Printf("❌ Driver not found in %d entries on track %s, class class-%s", totalEntries, trackID, classID)
	}
}
