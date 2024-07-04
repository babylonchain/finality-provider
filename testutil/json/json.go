package jsonutil

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ReadJSONValue reads a value from a JSON file given a key
func ReadJSONValue(filePath string, key string) (interface{}, error) {
	// Open the JSON file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	// Read the file's content
	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %v", err)
	}

	// Unmarshal the JSON content into a map
	var result map[string]interface{}
	if err := json.Unmarshal(byteValue, &result); err != nil {
		return nil, fmt.Errorf("could not unmarshal JSON: %v", err)
	}

	// Retrieve the value associated with the given key
	value, exists := result[key]
	if !exists {
		return nil, fmt.Errorf("key not found in JSON: %s", key)
	}

	return value, nil
}

func ReadJSONValueToString(filePath string, key string) (string, error) {
	value, err := ReadJSONValue(filePath, key)
	if err != nil {
		return "", err
	}

	// Convert the value to a string
	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value is not a string: %v", value)
	}

	return strValue, nil
}

func ReadJSONValueToUint64(filePath string, key string) (uint64, error) {
	value, err := ReadJSONValue(filePath, key)
	if err != nil {
		return 0, err
	}

	// Check if the value is a float64 (default type for JSON numbers)
	// https://stackoverflow.com/a/41136623/1991574
	float64Value, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("value is not a float64 (and thus not a JSON number): %T, %v", value, value)
	}

	// Convert the float64 to uint64
	uint64Value := uint64(float64Value)

	// Check if the value is an integer
	if float64Value != float64(uint64(uint64Value)) {
		return 0, fmt.Errorf("value is not an integer: %v", float64Value)
	}

	return uint64Value, nil
}
