package main

import "time"

// ScraperConfig holds configuration for the scraper
type ScraperConfig struct {
	BaseURL        string
	UserAgent      string
	RequestTimeout time.Duration
	RateLimit      time.Duration
	MaxRetries     int
	OutputFilename string
}

// GetDefaultConfig returns default scraper configuration
func GetDefaultConfig() ScraperConfig {
	return ScraperConfig{
		BaseURL:        "https://game.raceroom.com/leaderboard/",
		UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		RequestTimeout: 30 * time.Second,
		RateLimit:      500 * time.Millisecond,
		MaxRetries:     3,
		OutputFilename: "raceroom_leaderboards.json",
	}
}

// GetTestConfig returns configuration optimized for testing
func GetTestConfig() ScraperConfig {
	config := GetDefaultConfig()
	config.OutputFilename = "quick_test_results.json"
	config.RateLimit = 200 * time.Millisecond // Faster for testing
	return config
}
