package internal

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestAPIClient_FetchRankedSeries(t *testing.T) {
	const endpoint = "/iapi/abcdef0123456789"
	seriesPayload := encodeTestRankedSeriesResponse([]byte("series-1"), "Touring Car Series")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ranked/series":
			_, _ = w.Write([]byte(`<script src="/assets/series.js"></script>`))
		case "/assets/series.js":
			_, _ = w.Write([]byte(
				`rankedSeriesItem=createSchema({fields:[[1,"rounds",items,1],[2,"currentRound",3]]}),` +
					`seriesSchema=createSchema({fields:[[1,"series",rankedSeriesItem,1],[2,"responseStatus",status]]}),` +
					`fetchSeries=params=>request({url:"` + endpoint + `",decoder:seriesSchema.decode},params),` +
					`headers={"xr":"series-token"}, limit=Math.min(512,bytes.length);for(let i=0;i<limit;i++)bytes[i]^=51+i*5&255`,
			))
		case endpoint:
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
			}
			var requestBody struct {
				KnownVersion string `json:"knownVersion"`
			}
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Errorf("decode request body: %v", err)
			}
			if requestBody.KnownVersion != "" {
				t.Errorf("knownVersion = %q, want empty initial version", requestBody.KnownVersion)
			}
			if r.Header.Get("xr") != "series-token" {
				t.Errorf("xr header = %q, want series-token", r.Header.Get("xr"))
			}
			w.Header().Set("Content-Type", "application/javascript-binast")
			_, _ = w.Write(seriesPayload)
		default:
			t.Errorf("Unexpected request path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewAPIClient()
	defer client.Close()
	client.baseURL = server.URL
	var seriesStatusCleared bool
	client.discoveryStatus = func(operation string, err error) error {
		seriesStatusCleared = operation == "series" && err == nil
		return nil
	}

	series, err := client.FetchRankedSeries(context.Background())
	if err != nil {
		t.Fatalf("FetchRankedSeries returned error: %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("series count = %d, want 1", len(series))
	}
	if !seriesStatusCleared {
		t.Error("successful ranked-series discovery did not clear its warning")
	}
	if series[0].ID != "series-1" || series[0].Name != "Touring Car Series" {
		t.Fatalf("series = %+v, want the Touring Car Series fixture", series[0])
	}
	if series[0].CurrentRound != 2 || len(series[0].Rounds) != 1 {
		t.Fatalf("series current round/rounds = %d/%d, want 2/1", series[0].CurrentRound, len(series[0].Rounds))
	}
	round := series[0].Rounds[0]
	if round.Track.TrackID != "13352" || round.Track.LayoutID != "7" ||
		round.Track.Name != "Nürburgring" || round.Track.Layout != "Nordschleife" {
		t.Errorf("track = %+v, unexpected", round.Track)
	}
	wantClass := RankedSeriesCarClass{
		ID:   "123",
		Name: "WTCR 2018",
		Cars: []RankedSeriesCar{{ID: "1234", Name: "Honda Civic"}},
	}
	if !reflect.DeepEqual(round.EligibleClasses, []RankedSeriesCarClass{wantClass}) {
		t.Errorf("eligible classes = %+v, unexpected", round.EligibleClasses)
	}
	if !reflect.DeepEqual(series[0].EligibleClasses, []RankedSeriesCarClass{wantClass}) {
		t.Errorf("series eligible classes = %+v, unexpected", series[0].EligibleClasses)
	}
	if round.EventID != "event-2" || !reflect.DeepEqual(round.RaceIDs, []string{"race-2"}) ||
		round.StartsAt != 1700000000 || round.EndsAt != 1700003600 || round.Status != "active" {
		t.Errorf("round details = %+v, unexpected", round)
	}
	if round.Registration.OpensAt != 1699990000 || round.Registration.ClosesAt != 1700000000 ||
		round.Registration.Count != 25 || !reflect.DeepEqual(round.Registration.DriverIDs, []int{101, 202}) {
		t.Errorf("registration = %+v, unexpected", round.Registration)
	}
	format := series[0].Format
	if format.PracticeMinutes != 5 || format.QualifyingType != "leaderboard" || format.QualifyingMinutes != 10 ||
		!format.FixedSetup || format.MandatoryPitstop != "required" || format.FuelUsage != "normal" ||
		format.TireWear != "normal" || format.IncidentLimit != 20 || format.ReversedGridPlaces != 8 ||
		format.Damage != "full" {
		t.Errorf("format = %+v, unexpected", format)
	}
	wantRaces := []RankedSeriesRace{{Format: "laps", Length: 3, RollingStart: true, EstimatedMinutes: 12}}
	if !reflect.DeepEqual(format.Races, wantRaces) {
		t.Errorf("races = %+v, want %+v", format.Races, wantRaces)
	}
	wantSchedule := RankedSeriesSchedule{
		CadenceMinutes: 120,
		AnchorMinutes:  []int{0, 60},
		DailyTimes:     []int{480, 1200},
		Weekdays:       []int{1, 3, 5},
	}
	if !reflect.DeepEqual(series[0].Schedule, wantSchedule) {
		t.Errorf("schedule = %+v, want %+v", series[0].Schedule, wantSchedule)
	}
}

func TestDecodeRankedSeriesResponse_MalformedPayload(t *testing.T) {
	if _, err := decodeRankedSeriesResponse([]byte{0}, apiProtocol{}); err == nil {
		t.Fatal("expected malformed response to return an error")
	}
}

func TestRankedSeriesTrackClassCombinations_CurrentRoundsOnlyAndDeduplicated(t *testing.T) {
	series := []RankedSeries{
		{
			CurrentRound: 2,
			Rounds: []RankedSeriesRound{
				{
					Number: 1,
					Track:  RankedSeriesTrack{TrackID: "100"},
					EligibleClasses: []RankedSeriesCarClass{
						{ID: "1", Name: "Past"},
					},
				},
				{
					Number: 2,
					Track:  RankedSeriesTrack{TrackID: "200"},
					EligibleClasses: []RankedSeriesCarClass{
						{ID: "20", Name: "Current"},
						{ID: "invalid", Name: "Invalid"},
					},
				},
			},
		},
		{
			CurrentRound: 2,
			Rounds: []RankedSeriesRound{{
				Number: 2,
				Track:  RankedSeriesTrack{TrackID: "200"},
				EligibleClasses: []RankedSeriesCarClass{
					{ID: "20", Name: "Duplicate"},
					{ID: "21", Name: "Also current"},
				},
			}},
		},
	}

	got := RankedSeriesTrackClassCombinations(series)
	want := []string{"200-20", "200-21"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("combinations = %v, want %v", got, want)
	}
}

func TestRankedSeriesFetchTargets_UsesDiscoveredClassMetadata(t *testing.T) {
	series := []RankedSeries{{
		CurrentRound: 1,
		Rounds: []RankedSeriesRound{{
			Number: 1,
			Track:  RankedSeriesTrack{TrackID: "91234", Name: "New Track", Layout: "Short"},
			EligibleClasses: []RankedSeriesCarClass{
				{ID: "98765", Name: "New Class"},
			},
		}},
	}}

	targets, tracks, classes := rankedSeriesFetchTargets(series)
	if !reflect.DeepEqual(targets, []targetCombo{{trackID: "91234", classID: "98765"}}) {
		t.Errorf("targets = %+v, want the discovered track/class pair", targets)
	}
	if !reflect.DeepEqual(tracks, []TrackConfig{{Name: "New Track - Short", TrackID: "91234"}}) {
		t.Errorf("tracks = %+v, want track metadata from the series response", tracks)
	}
	if !reflect.DeepEqual(classes, []CarClassConfig{{Name: "New Class", ClassID: "98765"}}) {
		t.Errorf("classes = %+v, want class metadata from the series response", classes)
	}
}

func TestExportRankedSeries_WritesSortedGzipJSON(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "ranked_series_export_test")
	defer cleanup()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()

	series := []RankedSeries{
		{ID: "2", Name: "Z Series", CurrentRound: 1, Rounds: []RankedSeriesRound{}},
		{ID: "1", Name: "A Series", CurrentRound: 3, Rounds: []RankedSeriesRound{}},
	}
	if err := ExportRankedSeries(series); err != nil {
		t.Fatalf("ExportRankedSeries returned error: %v", err)
	}

	compressed, err := os.ReadFile(filepath.Join(tempDir, RankedSeriesFile))
	if err != nil {
		t.Fatalf("read exported ranked-series file: %v", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("open exported gzip JSON: %v", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("decompress exported ranked-series JSON: %v", err)
	}
	var exported RankedSeriesExport
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatalf("unmarshal exported ranked-series JSON: %v", err)
	}
	if exported.UpdatedAt.IsZero() || time.Since(exported.UpdatedAt) > time.Minute {
		t.Errorf("updated_at = %v, want a recent timestamp", exported.UpdatedAt)
	}
	if len(exported.Series) != 2 || exported.Series[0].Name != "A Series" || exported.Series[1].Name != "Z Series" {
		t.Errorf("exported series order = %+v, want alphabetical order", exported.Series)
	}
}

func encodeTestRankedSeriesResponse(id []byte, name string) []byte {
	var car []byte
	car = appendTestProtoVarint(car, 1, 1234)
	car = appendTestProtoBytes(car, 2, []byte("Honda Civic"))

	var eligibleClass []byte
	eligibleClass = appendTestProtoVarint(eligibleClass, 1, 123)
	eligibleClass = appendTestProtoBytes(eligibleClass, 2, []byte("WTCR 2018"))
	eligibleClass = appendTestProtoBytes(eligibleClass, 3, car)

	var raceFormat []byte
	raceFormat = appendTestProtoBytes(raceFormat, 1, []byte("laps"))
	raceFormat = appendTestProtoVarint(raceFormat, 2, 3)
	raceFormat = appendTestProtoVarint(raceFormat, 3, 1)
	raceFormat = appendTestProtoVarint(raceFormat, 4, 12)

	var format []byte
	format = appendTestProtoVarint(format, 1, 5)
	format = appendTestProtoBytes(format, 2, []byte("leaderboard"))
	format = appendTestProtoVarint(format, 3, 10)
	format = appendTestProtoBytes(format, 4, raceFormat)
	format = appendTestProtoVarint(format, 5, 1)
	format = appendTestProtoBytes(format, 6, []byte("required"))
	format = appendTestProtoBytes(format, 7, []byte("normal"))
	format = appendTestProtoBytes(format, 8, []byte("normal"))
	format = appendTestProtoVarint(format, 9, 20)
	format = appendTestProtoVarint(format, 10, 8)
	format = appendTestProtoBytes(format, 11, []byte("full"))

	var schedule []byte
	schedule = appendTestProtoVarint(schedule, 1, 120)
	schedule = appendTestProtoVarint(schedule, 2, 0)
	schedule = appendTestProtoVarint(schedule, 2, 60)
	schedule = appendTestProtoVarint(schedule, 3, 480)
	schedule = appendTestProtoVarint(schedule, 3, 1200)
	packedWeekdays := appendTestRawVarint(nil, 1)
	packedWeekdays = appendTestRawVarint(packedWeekdays, 3)
	packedWeekdays = appendTestRawVarint(packedWeekdays, 5)
	schedule = appendTestProtoBytes(schedule, 4, packedWeekdays)

	var registration []byte
	registration = appendTestProtoVarint(registration, 1, 1699990000)
	registration = appendTestProtoVarint(registration, 2, 1700000000)
	registration = appendTestProtoVarint(registration, 3, 25)
	packedDrivers := appendTestRawVarint(nil, 101)
	packedDrivers = appendTestRawVarint(packedDrivers, 202)
	registration = appendTestProtoBytes(registration, 4, packedDrivers)

	var track []byte
	track = appendTestProtoVarint(track, 1, 7)
	track = appendTestProtoBytes(track, 2, []byte("Nordschleife"))
	track = appendTestProtoVarint(track, 3, 13352)
	track = appendTestProtoBytes(track, 4, []byte("Nürburgring"))

	var round []byte
	round = appendTestProtoVarint(round, 1, 2)
	round = appendTestProtoBytes(round, 2, []byte("event-2"))
	round = appendTestProtoBytes(round, 3, []byte("race-2"))
	round = appendTestProtoVarint(round, 4, 1700000000)
	round = appendTestProtoVarint(round, 5, 1700003600)
	round = appendTestProtoBytes(round, 6, []byte("active"))
	round = appendTestProtoBytes(round, 7, track)
	round = appendTestProtoBytes(round, 8, registration)
	round = appendTestProtoBytes(round, 9, eligibleClass)

	var series []byte
	series = appendTestProtoBytes(series, 1, id)
	series = appendTestProtoBytes(series, 2, []byte(name))
	series = appendTestProtoBytes(series, 3, []byte("Current ranked series"))
	series = appendTestProtoBytes(series, 5, eligibleClass)
	series = appendTestProtoBytes(series, 6, format)
	series = appendTestProtoBytes(series, 7, schedule)
	series = appendTestProtoBytes(series, 8, round)
	series = appendTestProtoVarint(series, 10, 2)

	var response []byte
	response = appendTestProtoBytes(response, 1, series)
	response = appendTestProtoVarint(response, 2, 1)
	xorAPIResponsePrefix(response, testResponseXORSeed, testResponseXORStep, 512)
	return response
}

func appendTestRawVarint(data []byte, value uint64) []byte {
	for value >= 0x80 {
		data = append(data, byte(value)|0x80)
		value >>= 7
	}
	return append(data, byte(value))
}
