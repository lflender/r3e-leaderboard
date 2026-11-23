package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const RankedSeriesFile = "cache/ranked_series.json.gz"

const rankedSeriesDiscoveryInterval = time.Hour

type RankedSeriesExport struct {
	UpdatedAt time.Time      `json:"updated_at"`
	Series    []RankedSeries `json:"series"`
}

type RankedSeries struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description,omitempty"`
	EligibleClasses []RankedSeriesCarClass `json:"eligible_classes"`
	Format          RankedSeriesFormat     `json:"format"`
	Schedule        RankedSeriesSchedule   `json:"schedule"`
	CurrentRound    int                    `json:"current_round"`
	Rounds          []RankedSeriesRound    `json:"rounds"`
}

type RankedSeriesRound struct {
	Number          int                      `json:"number"`
	EventID         string                   `json:"event_id,omitempty"`
	RaceIDs         []string                 `json:"race_ids"`
	StartsAt        int64                    `json:"starts_at,omitempty"`
	EndsAt          int64                    `json:"ends_at,omitempty"`
	Status          string                   `json:"status,omitempty"`
	Track           RankedSeriesTrack        `json:"track"`
	Registration    RankedSeriesRegistration `json:"registration"`
	EligibleClasses []RankedSeriesCarClass   `json:"eligible_classes"`
}

type RankedSeriesTrack struct {
	TrackID  string `json:"track_id"`
	LayoutID string `json:"layout_id"`
	Name     string `json:"name"`
	Layout   string `json:"layout"`
}

type RankedSeriesCarClass struct {
	ID   string            `json:"id"`
	Name string            `json:"name"`
	Cars []RankedSeriesCar `json:"cars,omitempty"`
}

type RankedSeriesCar struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RankedSeriesFormat struct {
	PracticeMinutes    int                `json:"practice_minutes"`
	QualifyingType     string             `json:"qualifying_type"`
	QualifyingMinutes  int                `json:"qualifying_minutes"`
	Races              []RankedSeriesRace `json:"races"`
	FixedSetup         bool               `json:"fixed_setup"`
	MandatoryPitstop   string             `json:"mandatory_pitstop,omitempty"`
	FuelUsage          string             `json:"fuel_usage,omitempty"`
	TireWear           string             `json:"tire_wear,omitempty"`
	IncidentLimit      int                `json:"incident_limit"`
	ReversedGridPlaces int                `json:"reversed_grid_places"`
	Damage             string             `json:"damage,omitempty"`
}

type RankedSeriesRace struct {
	Format           string `json:"format"`
	Length           int    `json:"length"`
	RollingStart     bool   `json:"rolling_start"`
	EstimatedMinutes int    `json:"estimated_minutes"`
}

type RankedSeriesSchedule struct {
	CadenceMinutes int   `json:"cadence_minutes"`
	AnchorMinutes  []int `json:"anchor_minutes"`
	DailyTimes     []int `json:"daily_times"`
	Weekdays       []int `json:"weekdays"`
}

type RankedSeriesRegistration struct {
	OpensAt   int64 `json:"opens_at,omitempty"`
	ClosesAt  int64 `json:"closes_at,omitempty"`
	Count     int   `json:"count"`
	DriverIDs []int `json:"driver_ids,omitempty"`
}

func (api *APIClient) FetchRankedSeries(ctx context.Context) ([]RankedSeries, error) {
	protocol, err := api.discoverAPIProtocol(ctx, "series")
	if err != nil {
		return nil, fmt.Errorf("discover ranked-series protocol: %w", err)
	}

	requestURL := strings.TrimRight(api.baseURL, "/") + protocol.endpoint
	requestBody, err := json.Marshal(struct {
		KnownVersion string `json:"knownVersion"`
	}{})
	if err != nil {
		return nil, fmt.Errorf("encode ranked-series request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create ranked-series request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("xr", protocol.xr)

	resp, err := api.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch ranked series: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("ranked-series API returned status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ranked-series response: %w", err)
	}
	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("parse ranked-series response content type: %w", err)
	}
	if contentType != "application/javascript-binast" {
		return nil, fmt.Errorf("unexpected ranked-series response content type %q", contentType)
	}

	series, err := decodeRankedSeriesResponse(body, protocol)
	if err != nil {
		return nil, fmt.Errorf("decode ranked-series response: %w", err)
	}
	if len(series) == 0 {
		return nil, fmt.Errorf("ranked-series response contained no series")
	}
	api.reportDiscoveryStatus(ctx, "series", nil)
	return series, nil
}

func decodeRankedSeriesResponse(data []byte, protocol apiProtocol) ([]RankedSeries, error) {
	xorAPIResponsePrefix(data, protocol.responseSeed, protocol.responseStep, protocol.responseLimit)
	fields, err := parseProtobufFields(data)
	if err != nil {
		return nil, fmt.Errorf("parse series response: %w", err)
	}
	series := make([]RankedSeries, 0)
	for _, field := range fields {
		if field.number != 1 {
			continue
		}
		if field.wireType != 2 {
			return nil, fmt.Errorf("series entry has unexpected wire type %d", field.wireType)
		}
		item, err := decodeRankedSeries(field.bytes)
		if err != nil {
			return nil, fmt.Errorf("decode series %d: %w", len(series), err)
		}
		series = append(series, item)
	}
	return series, nil
}

func decodeRankedSeries(data []byte) (RankedSeries, error) {
	fields, err := parseProtobufFields(data)
	if err != nil {
		return RankedSeries{}, err
	}
	series := RankedSeries{
		EligibleClasses: make([]RankedSeriesCarClass, 0),
		Rounds:          make([]RankedSeriesRound, 0),
		Format:          RankedSeriesFormat{Races: make([]RankedSeriesRace, 0)},
		Schedule: RankedSeriesSchedule{
			AnchorMinutes: make([]int, 0),
			DailyTimes:    make([]int, 0),
			Weekdays:      make([]int, 0),
		},
	}
	for _, field := range fields {
		switch field.number {
		case 1:
			if field.wireType != 2 {
				return RankedSeries{}, fmt.Errorf("series ID has unexpected wire type %d", field.wireType)
			}
			series.ID = string(field.bytes)
		case 2:
			if field.wireType != 2 {
				return RankedSeries{}, fmt.Errorf("series name has unexpected wire type %d", field.wireType)
			}
			series.Name = string(field.bytes)
		case 3:
			if field.wireType != 2 {
				return RankedSeries{}, fmt.Errorf("series description has unexpected wire type %d", field.wireType)
			}
			series.Description = string(field.bytes)
		case 5:
			if field.wireType != 2 {
				return RankedSeries{}, fmt.Errorf("series eligible class has unexpected wire type %d", field.wireType)
			}
			carClass, err := decodeRankedSeriesCarClass(field.bytes)
			if err != nil {
				return RankedSeries{}, fmt.Errorf("decode series eligible class: %w", err)
			}
			series.EligibleClasses = append(series.EligibleClasses, carClass)
		case 6:
			if field.wireType != 2 {
				return RankedSeries{}, fmt.Errorf("series format has unexpected wire type %d", field.wireType)
			}
			series.Format, err = decodeRankedSeriesFormat(field.bytes)
			if err != nil {
				return RankedSeries{}, fmt.Errorf("decode series format: %w", err)
			}
		case 7:
			if field.wireType != 2 {
				return RankedSeries{}, fmt.Errorf("series schedule has unexpected wire type %d", field.wireType)
			}
			series.Schedule, err = decodeRankedSeriesSchedule(field.bytes)
			if err != nil {
				return RankedSeries{}, fmt.Errorf("decode series schedule: %w", err)
			}
		case 8:
			if field.wireType != 2 {
				return RankedSeries{}, fmt.Errorf("round has unexpected wire type %d", field.wireType)
			}
			round, err := decodeRankedSeriesRound(field.bytes)
			if err != nil {
				return RankedSeries{}, fmt.Errorf("decode round %d: %w", len(series.Rounds), err)
			}
			series.Rounds = append(series.Rounds, round)
		case 10:
			if field.wireType != 0 {
				return RankedSeries{}, fmt.Errorf("current round has unexpected wire type %d", field.wireType)
			}
			series.CurrentRound = int(field.value)
		}
	}
	if series.ID == "" || series.Name == "" {
		return RankedSeries{}, fmt.Errorf("series is missing its ID or name")
	}
	return series, nil
}

func decodeRankedSeriesRound(data []byte) (RankedSeriesRound, error) {
	fields, err := parseProtobufFields(data)
	if err != nil {
		return RankedSeriesRound{}, err
	}
	round := RankedSeriesRound{
		RaceIDs:         make([]string, 0),
		EligibleClasses: make([]RankedSeriesCarClass, 0),
	}
	for _, field := range fields {
		switch field.number {
		case 1:
			if field.wireType != 0 {
				return RankedSeriesRound{}, fmt.Errorf("round number has unexpected wire type %d", field.wireType)
			}
			round.Number = int(field.value)
		case 2:
			if field.wireType != 2 {
				return RankedSeriesRound{}, fmt.Errorf("event ID has unexpected wire type %d", field.wireType)
			}
			round.EventID = string(field.bytes)
		case 3:
			if field.wireType != 2 {
				return RankedSeriesRound{}, fmt.Errorf("race ID has unexpected wire type %d", field.wireType)
			}
			round.RaceIDs = append(round.RaceIDs, string(field.bytes))
		case 4:
			if field.wireType != 0 {
				return RankedSeriesRound{}, fmt.Errorf("round start time has unexpected wire type %d", field.wireType)
			}
			round.StartsAt = int64(field.value)
		case 5:
			if field.wireType != 0 {
				return RankedSeriesRound{}, fmt.Errorf("round end time has unexpected wire type %d", field.wireType)
			}
			round.EndsAt = int64(field.value)
		case 6:
			if field.wireType != 2 {
				return RankedSeriesRound{}, fmt.Errorf("round status has unexpected wire type %d", field.wireType)
			}
			round.Status = string(field.bytes)
		case 7:
			if field.wireType != 2 {
				return RankedSeriesRound{}, fmt.Errorf("track has unexpected wire type %d", field.wireType)
			}
			round.Track, err = decodeRankedSeriesTrack(field.bytes)
			if err != nil {
				return RankedSeriesRound{}, fmt.Errorf("decode track: %w", err)
			}
		case 8:
			if field.wireType != 2 {
				return RankedSeriesRound{}, fmt.Errorf("registration has unexpected wire type %d", field.wireType)
			}
			round.Registration, err = decodeRankedSeriesRegistration(field.bytes)
			if err != nil {
				return RankedSeriesRound{}, fmt.Errorf("decode registration: %w", err)
			}
		case 9:
			if field.wireType != 2 {
				return RankedSeriesRound{}, fmt.Errorf("eligible class has unexpected wire type %d", field.wireType)
			}
			carClass, err := decodeRankedSeriesCarClass(field.bytes)
			if err != nil {
				return RankedSeriesRound{}, fmt.Errorf("decode eligible class: %w", err)
			}
			round.EligibleClasses = append(round.EligibleClasses, carClass)
		}
	}
	if round.Number == 0 {
		return RankedSeriesRound{}, fmt.Errorf("round has no round number")
	}
	return round, nil
}

func decodeRankedSeriesTrack(data []byte) (RankedSeriesTrack, error) {
	fields, err := parseProtobufFields(data)
	if err != nil {
		return RankedSeriesTrack{}, err
	}
	var track RankedSeriesTrack
	for _, field := range fields {
		switch field.number {
		case 1:
			track.LayoutID, err = protobufIntegerString(field)
		case 2:
			if field.wireType != 2 {
				return RankedSeriesTrack{}, fmt.Errorf("layout name has unexpected wire type %d", field.wireType)
			}
			track.Layout = string(field.bytes)
		case 3:
			track.TrackID, err = protobufIntegerString(field)
		case 4:
			if field.wireType != 2 {
				return RankedSeriesTrack{}, fmt.Errorf("track name has unexpected wire type %d", field.wireType)
			}
			track.Name = string(field.bytes)
		}
		if err != nil {
			return RankedSeriesTrack{}, err
		}
	}
	if track.TrackID == "" || track.LayoutID == "" || track.Name == "" {
		return RankedSeriesTrack{}, fmt.Errorf("track is missing ID or name")
	}
	return track, nil
}

func decodeRankedSeriesCarClass(data []byte) (RankedSeriesCarClass, error) {
	fields, err := parseProtobufFields(data)
	if err != nil {
		return RankedSeriesCarClass{}, err
	}
	carClass := RankedSeriesCarClass{Cars: make([]RankedSeriesCar, 0)}
	for _, field := range fields {
		switch field.number {
		case 1:
			carClass.ID, err = protobufIntegerString(field)
		case 2:
			if field.wireType != 2 {
				return RankedSeriesCarClass{}, fmt.Errorf("eligible class name has unexpected wire type %d", field.wireType)
			}
			carClass.Name = string(field.bytes)
		case 3:
			if field.wireType != 2 {
				return RankedSeriesCarClass{}, fmt.Errorf("car has unexpected wire type %d", field.wireType)
			}
			car, err := decodeRankedSeriesCar(field.bytes)
			if err != nil {
				return RankedSeriesCarClass{}, fmt.Errorf("decode car: %w", err)
			}
			carClass.Cars = append(carClass.Cars, car)
		}
		if err != nil {
			return RankedSeriesCarClass{}, err
		}
	}
	if carClass.ID == "" || carClass.Name == "" {
		return RankedSeriesCarClass{}, fmt.Errorf("eligible class is missing ID or name")
	}
	return carClass, nil
}

func decodeRankedSeriesCar(data []byte) (RankedSeriesCar, error) {
	fields, err := parseProtobufFields(data)
	if err != nil {
		return RankedSeriesCar{}, err
	}
	var car RankedSeriesCar
	for _, field := range fields {
		switch field.number {
		case 1:
			car.ID, err = protobufIntegerString(field)
		case 2:
			if field.wireType != 2 {
				return RankedSeriesCar{}, fmt.Errorf("car name has unexpected wire type %d", field.wireType)
			}
			car.Name = string(field.bytes)
		}
		if err != nil {
			return RankedSeriesCar{}, err
		}
	}
	if car.ID == "" || car.Name == "" {
		return RankedSeriesCar{}, fmt.Errorf("car is missing ID or name")
	}
	return car, nil
}

func decodeRankedSeriesFormat(data []byte) (RankedSeriesFormat, error) {
	fields, err := parseProtobufFields(data)
	if err != nil {
		return RankedSeriesFormat{}, err
	}
	format := RankedSeriesFormat{Races: make([]RankedSeriesRace, 0)}
	for _, field := range fields {
		switch field.number {
		case 1:
			format.PracticeMinutes, err = protobufInteger(field, "practice duration")
		case 2:
			format.QualifyingType, err = protobufString(field, "qualifying type")
		case 3:
			format.QualifyingMinutes, err = protobufInteger(field, "qualifying duration")
		case 4:
			if field.wireType != 2 {
				return RankedSeriesFormat{}, fmt.Errorf("race format has unexpected wire type %d", field.wireType)
			}
			race, err := decodeRankedSeriesRace(field.bytes)
			if err != nil {
				return RankedSeriesFormat{}, fmt.Errorf("decode race format: %w", err)
			}
			format.Races = append(format.Races, race)
		case 5:
			if field.wireType != 0 {
				return RankedSeriesFormat{}, fmt.Errorf("fixed setup has unexpected wire type %d", field.wireType)
			}
			format.FixedSetup = field.value != 0
		case 6:
			format.MandatoryPitstop, err = protobufString(field, "mandatory pitstop")
		case 7:
			format.FuelUsage, err = protobufString(field, "fuel usage")
		case 8:
			format.TireWear, err = protobufString(field, "tire wear")
		case 9:
			format.IncidentLimit, err = protobufInteger(field, "incident limit")
		case 10:
			format.ReversedGridPlaces, err = protobufInteger(field, "reversed grid places")
		case 11:
			format.Damage, err = protobufString(field, "damage")
		}
		if err != nil {
			return RankedSeriesFormat{}, err
		}
	}
	return format, nil
}

func decodeRankedSeriesRace(data []byte) (RankedSeriesRace, error) {
	fields, err := parseProtobufFields(data)
	if err != nil {
		return RankedSeriesRace{}, err
	}
	var race RankedSeriesRace
	for _, field := range fields {
		switch field.number {
		case 1:
			race.Format, err = protobufString(field, "race format")
		case 2:
			race.Length, err = protobufInteger(field, "race length")
		case 3:
			if field.wireType != 0 {
				return RankedSeriesRace{}, fmt.Errorf("rolling start has unexpected wire type %d", field.wireType)
			}
			race.RollingStart = field.value != 0
		case 4:
			race.EstimatedMinutes, err = protobufInteger(field, "estimated race duration")
		}
		if err != nil {
			return RankedSeriesRace{}, err
		}
	}
	return race, nil
}

func decodeRankedSeriesSchedule(data []byte) (RankedSeriesSchedule, error) {
	fields, err := parseProtobufFields(data)
	if err != nil {
		return RankedSeriesSchedule{}, err
	}
	schedule := RankedSeriesSchedule{
		AnchorMinutes: make([]int, 0),
		DailyTimes:    make([]int, 0),
		Weekdays:      make([]int, 0),
	}
	for _, field := range fields {
		switch field.number {
		case 1:
			schedule.CadenceMinutes, err = protobufInteger(field, "schedule cadence")
		case 2:
			schedule.AnchorMinutes, err = appendProtobufIntegers(schedule.AnchorMinutes, field, "schedule anchor")
		case 3:
			schedule.DailyTimes, err = appendProtobufIntegers(schedule.DailyTimes, field, "daily time")
		case 4:
			schedule.Weekdays, err = appendProtobufIntegers(schedule.Weekdays, field, "weekday")
		}
		if err != nil {
			return RankedSeriesSchedule{}, err
		}
	}
	return schedule, nil
}

func decodeRankedSeriesRegistration(data []byte) (RankedSeriesRegistration, error) {
	fields, err := parseProtobufFields(data)
	if err != nil {
		return RankedSeriesRegistration{}, err
	}
	registration := RankedSeriesRegistration{DriverIDs: make([]int, 0)}
	for _, field := range fields {
		switch field.number {
		case 1:
			if field.wireType != 0 {
				return RankedSeriesRegistration{}, fmt.Errorf("registration open time has unexpected wire type %d", field.wireType)
			}
			registration.OpensAt = int64(field.value)
		case 2:
			if field.wireType != 0 {
				return RankedSeriesRegistration{}, fmt.Errorf("registration close time has unexpected wire type %d", field.wireType)
			}
			registration.ClosesAt = int64(field.value)
		case 3:
			registration.Count, err = protobufInteger(field, "registration count")
		case 4:
			registration.DriverIDs, err = appendProtobufIntegers(registration.DriverIDs, field, "registration driver ID")
		}
		if err != nil {
			return RankedSeriesRegistration{}, err
		}
	}
	return registration, nil
}

func protobufString(field protobufField, fieldName string) (string, error) {
	if field.wireType != 2 {
		return "", fmt.Errorf("%s has unexpected wire type %d", fieldName, field.wireType)
	}
	return string(field.bytes), nil
}

func protobufInteger(field protobufField, fieldName string) (int, error) {
	if field.wireType != 0 {
		return 0, fmt.Errorf("%s has unexpected wire type %d", fieldName, field.wireType)
	}
	return int(field.value), nil
}

func appendProtobufIntegers(values []int, field protobufField, fieldName string) ([]int, error) {
	switch field.wireType {
	case 0:
		return append(values, int(field.value)), nil
	case 2:
		offset := 0
		for offset < len(field.bytes) {
			value, err := readProtobufVarint(field.bytes, &offset)
			if err != nil {
				return nil, fmt.Errorf("decode packed %s: %w", fieldName, err)
			}
			values = append(values, int(value))
		}
		return values, nil
	default:
		return nil, fmt.Errorf("%s has unexpected wire type %d", fieldName, field.wireType)
	}
}

func protobufIntegerString(field protobufField) (string, error) {
	if field.wireType != 0 {
		return "", fmt.Errorf("integer ID has unexpected wire type %d", field.wireType)
	}
	return strconv.FormatUint(field.value, 10), nil
}

func ExportRankedSeries(series []RankedSeries) error {
	if len(series) == 0 {
		return fmt.Errorf("cannot export an empty ranked-series list")
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Name < series[j].Name })
	_, err := writeGzipJSON(RankedSeriesFile, RankedSeriesExport{
		UpdatedAt: time.Now().UTC(),
		Series:    series,
	})
	return err
}

func FetchAndExportRankedSeries(ctx context.Context) ([]RankedSeries, error) {
	api := newStatusReportingAPIClient()
	defer api.Close()
	return fetchAndExportRankedSeries(ctx, api)
}

func fetchAndExportRankedSeries(ctx context.Context, api *APIClient) ([]RankedSeries, error) {
	series, err := api.FetchRankedSeries(ctx)
	if err != nil {
		return nil, err
	}
	if err := ExportRankedSeries(series); err != nil {
		return nil, fmt.Errorf("export ranked series: %w", err)
	}
	if err := UpdateStatusData(func(status *StatusData) {
		status.RankedSeriesCount = len(series)
		status.LastRankedSeriesRefresh = time.Now()
	}); err != nil {
		return nil, fmt.Errorf("update ranked-series status: %w", err)
	}
	return series, nil
}

// StartRankedSeriesDiscovery refreshes ranked-series metadata independently of leaderboard fetches.
func StartRankedSeriesDiscovery(ctx context.Context) {
	go runRankedSeriesDiscovery(ctx, rankedSeriesDiscoveryInterval, func(ctx context.Context) error {
		series, err := FetchAndExportRankedSeries(ctx)
		if err == nil {
			log.Printf("✅ Refreshed %d ranked series", len(series))
		}
		return err
	})
}

func runRankedSeriesDiscovery(ctx context.Context, interval time.Duration, refresh func(context.Context) error) {
	for ctx.Err() == nil {
		if err := refresh(ctx); err != nil && ctx.Err() == nil {
			log.Printf("⚠️ Failed to refresh ranked series: %v", err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
