package main

import (
	"reflect"
	"testing"
)

func TestParseSecrets(t *testing.T) {
	testCases := []struct {
		name        string
		secretsRaw  string
		expected    map[string]string
		expectError bool
	}{
		{
			name:        "Valid secrets",
			secretsRaw:  "SECRET1=value1\nSECRET2=value2",
			expected:    map[string]string{"SECRET1": "value1", "SECRET2": "value2"},
			expectError: false,
		},
		{
			name:        "Empty line",
			secretsRaw:  "\nSECRET1=value1",
			expected:    map[string]string{"SECRET1": "value1"},
			expectError: false,
		},
		{
			name:        "Malformed line",
			secretsRaw:  "SECRET1",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "Missing value",
			secretsRaw:  "SECRET1=",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "Missing key",
			secretsRaw:  "=value1",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "Whitespace around key and value",
			secretsRaw:  "  SECRET1  =  value1  ",
			expected:    map[string]string{"SECRET1": "value1"},
			expectError: false,
		},
		{
			name:        "Multiple equal signs",
			secretsRaw:  "SECRET1=value1=value2",
			expected:    map[string]string{"SECRET1": "value1=value2"},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseKeyValuePairs(tc.secretsRaw)
			if (err != nil) != tc.expectError {
				t.Fatalf("Expected error: %v, got: %v", tc.expectError, err)
			}
			if err == nil && !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected result: %v, got: %v", tc.expected, result)
			}
		})
	}
}
