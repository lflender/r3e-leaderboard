package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"r3e-leaderboard/internal"
	"strings"
	"testing"
)

// helper to build APIServer with given tracks
func makeServerWithTracks(tracks []internal.TrackInfo) *APIServer {
	s := &APIServer{}
	s.tracks = tracks
	// build topCombinations and byTrack similar to UpdateData
	sorted := make([]internal.TrackInfo, len(tracks))
	copy(sorted, tracks)
	// simple sort by len(data) desc
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if len(sorted[j].Data) > len(sorted[i].Data) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if len(sorted) > 1000 {
		s.topCombinations = sorted[:1000]
	} else {
		s.topCombinations = sorted
	}
	byTrack := make(map[string][]internal.TrackInfo)
	for _, t := range tracks {
		byTrack[t.TrackID] = append(byTrack[t.TrackID], t)
	}
	for k := range byTrack {
		arr := byTrack[k]
		// sort
		for i := 0; i < len(arr); i++ {
			for j := i + 1; j < len(arr); j++ {
				if len(arr[j].Data) > len(arr[i].Data) {
					arr[i], arr[j] = arr[j], arr[i]
				}
			}
		}
		byTrack[k] = arr
	}
	s.topCombinationsByTrack = byTrack
	return s
}

func TestTopCombinations_ClassFilter(t *testing.T) {
	// Create tracks with mixed class IDs
	tracks := []internal.TrackInfo{
		{Name: "T1-C1", TrackID: "1", ClassID: "100", Data: make([]map[string]interface{}, 5)},
		{Name: "T1-C2", TrackID: "1", ClassID: "200", Data: make([]map[string]interface{}, 3)},
		{Name: "T2-C1", TrackID: "2", ClassID: "100", Data: make([]map[string]interface{}, 8)},
	}
	srv := makeServerWithTracks(tracks)
	h := NewHandlers(srv)

	req := httptest.NewRequest("GET", "/api/top-combinations?class=100", nil)
	w := httptest.NewRecorder()
	h.HandleTopCombinations(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	results, ok := body["results"].([]interface{})
	if !ok {
		t.Fatalf("results missing or wrong type")
	}
	// Expect only entries with ClassID == "100"
	for _, r := range results {
		m := r.(map[string]interface{})
		if m["class_id"] != "100" {
			t.Fatalf("unexpected class_id in result: %v", m["class_id"])
		}
	}
}

func TestTopCombinations_TrackAndClassFilter(t *testing.T) {
	tracks := []internal.TrackInfo{
		{Name: "T1-C1", TrackID: "1", ClassID: "100", Data: make([]map[string]interface{}, 5)},
		{Name: "T1-C2", TrackID: "1", ClassID: "200", Data: make([]map[string]interface{}, 3)},
		{Name: "T2-C1", TrackID: "2", ClassID: "100", Data: make([]map[string]interface{}, 8)},
	}
	srv := makeServerWithTracks(tracks)
	h := NewHandlers(srv)

	req := httptest.NewRequest("GET", "/api/top-combinations?track=1&class=100", nil)
	w := httptest.NewRecorder()
	h.HandleTopCombinations(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	results, ok := body["results"].([]interface{})
	if !ok {
		t.Fatalf("results missing or wrong type")
	}
	// Expect only the specific track+class pairing
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	m := results[0].(map[string]interface{})
	if m["track_id"] != "1" || m["class_id"] != "100" {
		t.Fatalf("unexpected result: %#v", m)
	}
}

func TestTopCombinations_NoFilter_SortsDesc(t *testing.T) {
	tracks := []internal.TrackInfo{
		{Name: "A", TrackID: "1", ClassID: "100", Data: make([]map[string]interface{}, 2)},
		{Name: "B", TrackID: "2", ClassID: "200", Data: make([]map[string]interface{}, 5)},
		{Name: "C", TrackID: "3", ClassID: "300", Data: make([]map[string]interface{}, 3)},
	}
	srv := makeServerWithTracks(tracks)
	h := NewHandlers(srv)

	req := httptest.NewRequest("GET", "/api/top-combinations", nil)
	w := httptest.NewRecorder()
	h.HandleTopCombinations(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	results := body["results"].([]interface{})
	// Expect first entry to be B (5 entries)
	first := results[0].(map[string]interface{})
	if !strings.HasPrefix(first["track"].(string), "B") {
		t.Fatalf("expected first track B, got %v", first["track"])
	}
}
