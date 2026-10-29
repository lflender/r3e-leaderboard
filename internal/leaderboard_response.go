package internal

import (
	"fmt"
	"strconv"
	"time"
)

type protobufField struct {
	number   uint64
	wireType uint64
	value    uint64
	bytes    []byte
}

func decodeLeaderboardResponse(data []byte, classID string) ([]map[string]interface{}, error) {
	for i := 0; i < len(data) && i < 128; i++ {
		data[i] ^= byte(51 + i*5)
	}

	fields, err := parseProtobufFields(data)
	if err != nil {
		return nil, fmt.Errorf("decode leaderboard response: %w", err)
	}

	results := make([]map[string]interface{}, 0, leaderboardPageSize)
	for _, field := range fields {
		if field.number != 1 {
			continue
		}
		if field.wireType != 2 {
			return nil, fmt.Errorf("leaderboard result has unexpected wire type %d", field.wireType)
		}
		result, err := decodeLeaderboardResult(field.bytes, classID)
		if err != nil {
			return nil, fmt.Errorf("decode leaderboard result %d: %w", len(results), err)
		}
		results = append(results, result)
	}

	return results, nil
}

func decodeLeaderboardResult(data []byte, classID string) (map[string]interface{}, error) {
	fields, err := parseProtobufFields(data)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	driver := make(map[string]interface{})
	country := make(map[string]interface{})
	car := make(map[string]interface{})
	var driverID uint64
	var carName string
	for _, field := range fields {
		switch field.number {
		case 1:
			if field.wireType != 0 {
				return nil, fmt.Errorf("index has unexpected wire type %d", field.wireType)
			}
			result["index"] = float64(field.value)
		case 2:
			if field.wireType != 0 {
				return nil, fmt.Errorf("position has unexpected wire type %d", field.wireType)
			}
			result["position"] = float64(field.value)
		case 3:
			if field.wireType != 2 {
				return nil, fmt.Errorf("driver has unexpected wire type %d", field.wireType)
			}
			driverFields, err := parseProtobufFields(field.bytes)
			if err != nil {
				return nil, fmt.Errorf("decode driver: %w", err)
			}
			for _, driverField := range driverFields {
				switch driverField.number {
				case 1:
					if driverField.wireType != 0 {
						return nil, fmt.Errorf("driver id has unexpected wire type %d", driverField.wireType)
					}
					driverID = driverField.value
				case 2:
					if err := setStringField(driver, "name", driverField); err != nil {
						return nil, err
					}
				case 3:
					if err := setStringField(driver, "rank", driverField); err != nil {
						return nil, err
					}
				case 4:
					if err := setStringField(driver, "username", driverField); err != nil {
						return nil, err
					}
				}
			}
		case 4:
			if field.wireType != 0 {
				return nil, fmt.Errorf("date has unexpected wire type %d", field.wireType)
			}
			result["date_time"] = time.UnixMilli(int64(field.value)).UTC().Format(time.RFC3339)
		case 5:
			if err := setStringField(result, "rank", field); err != nil {
				return nil, err
			}
		case 7:
			if field.wireType != 0 {
				return nil, fmt.Errorf("driving model has unexpected wire type %d", field.wireType)
			}
			result["driving_model"] = strconv.FormatUint(field.value, 10)
		case 8:
			if err := setStringField(country, "code", field); err != nil {
				return nil, err
			}
		case 9:
			if err := setStringField(country, "name", field); err != nil {
				return nil, err
			}
		case 10:
			if err := setStringField(car, "name", field); err != nil {
				return nil, err
			}
			if field.wireType == 2 {
				carName = string(field.bytes)
			}
		case 11:
			if field.wireType != 0 {
				return nil, fmt.Errorf("car id has unexpected wire type %d", field.wireType)
			}
			car["id"] = float64(field.value)
		case 12:
			if field.wireType != 0 {
				return nil, fmt.Errorf("class position has unexpected wire type %d", field.wireType)
			}
			result["class_position"] = float64(field.value)
		case 13:
			if err := setStringField(result, "team", field); err != nil {
				return nil, err
			}
		case 14:
			if err := setStringField(result, "points", field); err != nil {
				return nil, err
			}
		case 15:
			if field.wireType != 0 {
				return nil, fmt.Errorf("lap time has unexpected wire type %d", field.wireType)
			}
			result["laptime"] = formatLeaderboardLapTime(field.value)
		}
	}

	if driverID != 0 {
		driver["path"] = fmt.Sprintf("https://game.raceroom.com/users/info/%d/", driverID)
	}
	if name, ok := driver["name"].(string); ok && name != "" {
		result["driver"] = driver
	}
	if len(country) > 0 {
		result["country"] = country
	}
	if carName != "" {
		result["car_class"] = map[string]interface{}{
			"car":        car,
			"class-name": GetCarClassName(classID),
		}
	}

	return result, nil
}

func setStringField(destination map[string]interface{}, key string, field protobufField) error {
	if field.wireType != 2 {
		return fmt.Errorf("%s has unexpected wire type %d", key, field.wireType)
	}
	destination[key] = string(field.bytes)
	return nil
}

func formatLeaderboardLapTime(milliseconds uint64) string {
	minutes := milliseconds / 60000
	seconds := milliseconds % 60000 / 1000
	millisecondPart := milliseconds % 1000
	return fmt.Sprintf("%d:%02d.%03d", minutes, seconds, millisecondPart)
}

func parseProtobufFields(data []byte) ([]protobufField, error) {
	fields := make([]protobufField, 0)
	for offset := 0; offset < len(data); {
		tag, err := readProtobufVarint(data, &offset)
		if err != nil {
			return nil, err
		}
		number, wireType := tag>>3, tag&7
		if number == 0 {
			return nil, fmt.Errorf("invalid protobuf field number 0")
		}

		field := protobufField{number: number, wireType: wireType}
		switch wireType {
		case 0:
			field.value, err = readProtobufVarint(data, &offset)
			if err != nil {
				return nil, err
			}
		case 1:
			if len(data)-offset < 8 {
				return nil, fmt.Errorf("truncated fixed64 protobuf field")
			}
			field.bytes = data[offset : offset+8]
			offset += 8
		case 2:
			length, err := readProtobufVarint(data, &offset)
			if err != nil {
				return nil, err
			}
			if length > uint64(len(data)-offset) {
				return nil, fmt.Errorf("truncated length-delimited protobuf field")
			}
			field.bytes = data[offset : offset+int(length)]
			offset += int(length)
		case 5:
			if len(data)-offset < 4 {
				return nil, fmt.Errorf("truncated fixed32 protobuf field")
			}
			field.bytes = data[offset : offset+4]
			offset += 4
		default:
			return nil, fmt.Errorf("unsupported protobuf wire type %d", wireType)
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func readProtobufVarint(data []byte, offset *int) (uint64, error) {
	var value uint64
	for shift := uint(0); ; shift += 7 {
		if *offset >= len(data) {
			return 0, fmt.Errorf("truncated protobuf varint")
		}
		current := data[*offset]
		*offset = *offset + 1
		if shift == 63 && current > 1 {
			return 0, fmt.Errorf("protobuf varint exceeds 64 bits")
		}
		value |= uint64(current&0x7f) << shift
		if current < 0x80 {
			return value, nil
		}
		if shift == 63 {
			return 0, fmt.Errorf("protobuf varint exceeds 64 bits")
		}
	}
}
