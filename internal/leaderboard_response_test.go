package internal

import (
	"fmt"
	"testing"
)

func TestDecodeLeaderboardResponse(t *testing.T) {
	results, err := decodeLeaderboardResponse(encodeTestLeaderboardResponse(0, 1), "1703")
	if err != nil {
		t.Fatalf("decodeLeaderboardResponse returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Decoded %d results, want 1", len(results))
	}

	result := results[0]
	if result["index"] != float64(0) {
		t.Errorf("Index = %v, want 0", result["index"])
	}
	if result["laptime"] != "1:30.000" {
		t.Errorf("Lap time = %v, want 1:30.000", result["laptime"])
	}

	driver, ok := result["driver"].(map[string]interface{})
	if !ok {
		t.Fatalf("Driver = %T, want map", result["driver"])
	}
	if driver["name"] != "Driver 1" {
		t.Errorf("Driver name = %v, want Driver 1", driver["name"])
	}
	if driver["path"] != "https://game.raceroom.com/users/info/1000000/" {
		t.Errorf("Driver path = %v, unexpected", driver["path"])
	}

	country, ok := result["country"].(map[string]interface{})
	if !ok {
		t.Fatalf("Country = %T, want map", result["country"])
	}
	if country["name"] != "Germany" || country["code"] != "DE" {
		t.Errorf("Country = %+v, want Germany (DE)", country)
	}
}

func TestDecodeLeaderboardResponse_MalformedPayload(t *testing.T) {
	if _, err := decodeLeaderboardResponse([]byte{0x00}, "1703"); err == nil {
		t.Fatal("Expected malformed response to return an error")
	}
}

func encodeTestLeaderboardResponse(start, count int) []byte {
	var response []byte
	for index := start; index < start+count; index++ {
		response = appendTestProtoBytes(response, 1, encodeTestLeaderboardResult(index))
	}
	response = appendTestProtoVarint(response, 2, 1)

	for i := 0; i < len(response) && i < 128; i++ {
		response[i] ^= byte(51 + i*5)
	}
	return response
}

func encodeTestLeaderboardResult(index int) []byte {
	driverID := uint64(1000000 + index)
	var driver []byte
	driver = appendTestProtoVarint(driver, 1, driverID)
	driver = appendTestProtoBytes(driver, 2, []byte(fmt.Sprintf("Driver %d", index+1)))
	driver = appendTestProtoBytes(driver, 3, []byte("A"))
	driver = appendTestProtoBytes(driver, 4, []byte(fmt.Sprintf("driver_%d", index+1)))

	var result []byte
	result = appendTestProtoVarint(result, 1, uint64(index))
	result = appendTestProtoVarint(result, 2, uint64(index+1))
	result = appendTestProtoBytes(result, 3, driver)
	result = appendTestProtoVarint(result, 4, 1700000000000)
	result = appendTestProtoBytes(result, 5, []byte("A"))
	result = appendTestProtoVarint(result, 7, 1)
	result = appendTestProtoBytes(result, 8, []byte("DE"))
	result = appendTestProtoBytes(result, 9, []byte("Germany"))
	result = appendTestProtoBytes(result, 10, []byte("Test Car"))
	result = appendTestProtoVarint(result, 11, 42)
	result = appendTestProtoBytes(result, 13, []byte("Test Team"))
	result = appendTestProtoVarint(result, 15, 90000)
	return result
}

func appendTestProtoVarint(data []byte, fieldNumber, value uint64) []byte {
	data = appendTestVarint(data, fieldNumber<<3)
	return appendTestVarint(data, value)
}

func appendTestProtoBytes(data []byte, fieldNumber uint64, value []byte) []byte {
	data = appendTestVarint(data, fieldNumber<<3|2)
	data = appendTestVarint(data, uint64(len(value)))
	return append(data, value...)
}

func appendTestVarint(data []byte, value uint64) []byte {
	for value >= 0x80 {
		data = append(data, byte(value)|0x80)
		value >>= 7
	}
	return append(data, byte(value))
}
