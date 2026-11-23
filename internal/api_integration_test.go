package internal

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRaceRoomLeaderboardDiscoveryIntegration(t *testing.T) {
	if os.Getenv("R3E_LEADERBOARD_INTEGRATION") != "1" {
		t.Skip("set R3E_LEADERBOARD_INTEGRATION=1 to call the live RaceRoom leaderboard API")
	}

	client := NewAPIClient()
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	results, err := client.fetchLeaderboardPage(ctx, "1671", "1703", 0)
	if err != nil {
		t.Fatalf("live leaderboard discovery and fetch: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("live Monza GTR 3 leaderboard returned no results")
	}
	driver, ok := results[0]["driver"].(map[string]interface{})
	name, hasName := driver["name"].(string)
	lapTime, hasLapTime := results[0]["laptime"].(string)
	if !ok || !hasName || name == "" || !hasLapTime || lapTime == "" {
		t.Fatalf("live leaderboard result missing driver or lap time: %v", results[0])
	}
	t.Logf("Discovered endpoint and decoded %d leaderboard results", len(results))
}

func TestRaceRoomRankedSeriesDiscoveryIntegration(t *testing.T) {
	if os.Getenv("R3E_LEADERBOARD_INTEGRATION") != "1" {
		t.Skip("set R3E_LEADERBOARD_INTEGRATION=1 to call the live RaceRoom ranked-series API")
	}

	client := NewAPIClient()
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	series, err := client.FetchRankedSeries(ctx)
	if err != nil {
		t.Fatalf("live ranked-series discovery and fetch: %v", err)
	}
	active := 0
	for _, item := range series {
		for _, round := range item.Rounds {
			if round.Number == item.CurrentRound {
				active++
				t.Logf("%s: round %d, track %s, %d eligible classes", item.Name, item.CurrentRound, round.Track.TrackID, len(round.EligibleClasses))
			}
		}
	}
	if active == 0 || len(currentRoundCombinations(series)) == 0 {
		t.Fatal("live response has no usable current-round combinations")
	}
}
