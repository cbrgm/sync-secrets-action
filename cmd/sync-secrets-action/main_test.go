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

func TestParseSecretsJSON(t *testing.T) {
	testCases := []struct {
		name        string
		secretsRaw  string
		expected    map[string]string
		expectError bool
	}{
		{
			name:        "JSON simple secrets",
			secretsRaw:  `{"SECRET1": "value1", "SECRET2": "value2"}`,
			expected:    map[string]string{"SECRET1": "value1", "SECRET2": "value2"},
			expectError: false,
		},
		{
			name:        "JSON with multi-line value",
			secretsRaw:  `{"SECRET1": "line1\nline2\nline3"}`,
			expected:    map[string]string{"SECRET1": "line1\nline2\nline3"},
			expectError: false,
		},
		{
			name:        "JSON with complex multi-line certificate",
			secretsRaw:  `{"CERTIFICATE": "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJAKHBfpP...\n-----END CERTIFICATE-----"}`,
			expected:    map[string]string{"CERTIFICATE": "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJAKHBfpP...\n-----END CERTIFICATE-----"},
			expectError: false,
		},
		{
			name:        "JSON with private key containing newlines",
			secretsRaw:  `{"PRIVATE_KEY": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0Z3VS...\nmore key data here...\n-----END RSA PRIVATE KEY-----"}`,
			expected:    map[string]string{"PRIVATE_KEY": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0Z3VS...\nmore key data here...\n-----END RSA PRIVATE KEY-----"},
			expectError: false,
		},
		{
			name:        "JSON with multiple multi-line secrets",
			secretsRaw:  `{"SECRET1": "line1\nline2", "SECRET2": "a\nb\nc", "SECRET3": "single"}`,
			expected:    map[string]string{"SECRET1": "line1\nline2", "SECRET2": "a\nb\nc", "SECRET3": "single"},
			expectError: false,
		},
		{
			name:        "JSON with whitespace around braces",
			secretsRaw:  "  { \"SECRET1\": \"value1\" }  ",
			expected:    map[string]string{"SECRET1": "value1"},
			expectError: false,
		},
		{
			name:        "JSON with special characters in value",
			secretsRaw:  `{"SECRET1": "value with = and \t tab and \"quotes\""}`,
			expected:    map[string]string{"SECRET1": "value with = and \t tab and \"quotes\""},
			expectError: false,
		},
		{
			name:        "JSON empty object",
			secretsRaw:  `{}`,
			expected:    map[string]string{},
			expectError: false,
		},
		{
			name:        "JSON malformed",
			secretsRaw:  `{"SECRET1": "value1"`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "JSON with empty key",
			secretsRaw:  `{"": "value1"}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "JSON with whitespace-only key",
			secretsRaw:  `{"  ": "value1"}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "JSON with empty value",
			secretsRaw:  `{"SECRET1": ""}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "JSON key converted to uppercase",
			secretsRaw:  `{"secret1": "value1", "Secret2": "value2"}`,
			expected:    map[string]string{"SECRET1": "value1", "SECRET2": "value2"},
			expectError: false,
		},
		{
			name:        "JSON with carriage return and newline",
			secretsRaw:  `{"SECRET1": "line1\r\nline2\r\nline3"}`,
			expected:    map[string]string{"SECRET1": "line1\r\nline2\r\nline3"},
			expectError: false,
		},
		{
			name:        "JSON with leading and trailing whitespace in value preserved",
			secretsRaw:  `{"SECRET1": "  value with spaces  "}`,
			expected:    map[string]string{"SECRET1": "  value with spaces  "},
			expectError: false,
		},
		{
			name:        "JSON with only newlines in value",
			secretsRaw:  `{"SECRET1": "\n\n\n"}`,
			expected:    map[string]string{"SECRET1": "\n\n\n"},
			expectError: false,
		},
		{
			name:        "JSON multiline formatted",
			secretsRaw:  "{\n  \"SECRET1\": \"value1\",\n  \"SECRET2\": \"value2\"\n}",
			expected:    map[string]string{"SECRET1": "value1", "SECRET2": "value2"},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseKeyValuePairs(tc.secretsRaw)
			if (err != nil) != tc.expectError {
				t.Fatalf("Expected error: %v, got: %v (error: %v)", tc.expectError, err != nil, err)
			}
			if err == nil && !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected result: %v, got: %v", tc.expected, result)
			}
		})
	}
}

func TestParseSecretsKeyValueNotMistakenForJSON(t *testing.T) {
	// These tests ensure that key-value format inputs that happen to contain
	// JSON-like content in their values are not mistakenly parsed as JSON.
	testCases := []struct {
		name        string
		secretsRaw  string
		expected    map[string]string
		expectError bool
	}{
		{
			name:        "Key-value with JSON object as value",
			secretsRaw:  `SECRET1={"nested": "json"}`,
			expected:    map[string]string{"SECRET1": `{"nested": "json"}`},
			expectError: false,
		},
		{
			name:        "Key-value with braces in value",
			secretsRaw:  "SECRET1={some value}",
			expected:    map[string]string{"SECRET1": "{some value}"},
			expectError: false,
		},
		{
			name:        "Multiple key-values with one having JSON value",
			secretsRaw:  "SECRET1={json}\nSECRET2=normal",
			expected:    map[string]string{"SECRET1": "{json}", "SECRET2": "normal"},
			expectError: false,
		},
		{
			name:        "Key-value starting with letter not brace",
			secretsRaw:  "SECRET1=value1",
			expected:    map[string]string{"SECRET1": "value1"},
			expectError: false,
		},
		{
			// Edge case: input starts with { but doesn't end with }
			// This is NOT detected as JSON and falls through to key-value parsing
			name:        "Starts with brace but no closing brace - parsed as key-value",
			secretsRaw:  "{SECRET=value",
			expected:    map[string]string{"{SECRET": "value"},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseKeyValuePairs(tc.secretsRaw)
			if (err != nil) != tc.expectError {
				t.Fatalf("Expected error: %v, got: %v (error: %v)", tc.expectError, err != nil, err)
			}
			if err == nil && !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected result: %v, got: %v", tc.expected, result)
			}
		})
	}
}

func TestParseSecretsEmptyInput(t *testing.T) {
	testCases := []struct {
		name        string
		secretsRaw  string
		expected    map[string]string
		expectError bool
	}{
		{
			name:        "Empty string",
			secretsRaw:  "",
			expected:    map[string]string{},
			expectError: false,
		},
		{
			name:        "Only whitespace",
			secretsRaw:  "   \n\n\t\t  ",
			expected:    map[string]string{},
			expectError: false,
		},
		{
			name:        "Only newlines",
			secretsRaw:  "\n\n\n",
			expected:    map[string]string{},
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

func TestParseJSONKeyValuePairs(t *testing.T) {
	// Direct tests for parseJSONKeyValuePairs function
	testCases := []struct {
		name        string
		jsonStr     string
		expected    map[string]string
		expectError bool
	}{
		{
			name:        "Valid JSON",
			jsonStr:     `{"KEY1": "value1"}`,
			expected:    map[string]string{"KEY1": "value1"},
			expectError: false,
		},
		{
			name:        "Invalid JSON syntax",
			jsonStr:     `{invalid}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "JSON with non-string value",
			jsonStr:     `{"KEY1": 123}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "JSON with null value",
			jsonStr:     `{"KEY1": null}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "JSON with array value",
			jsonStr:     `{"KEY1": ["a", "b"]}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "JSON with nested object value",
			jsonStr:     `{"KEY1": {"nested": "value"}}`,
			expected:    nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseJSONKeyValuePairs(tc.jsonStr)
			if (err != nil) != tc.expectError {
				t.Fatalf("Expected error: %v, got: %v (error: %v)", tc.expectError, err != nil, err)
			}
			if err == nil && !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected result: %v, got: %v", tc.expected, result)
			}
		})
	}
}

// TestRealWorldMultiLineSecrets tests realistic scenarios with actual certificate and key formats.
// These tests verify that the JSON format works correctly with real-world multi-line secrets.
func TestRealWorldMultiLineSecrets(t *testing.T) {
	// Realistic PEM certificate (simplified but structurally accurate)
	pemCert := `-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAJC1HiIAZAiUMA0GCSqGSIb3Qasdfasdfasd
Ef8xQzBBBgNVBAcTOkZvdW5kYXRpb24gZm9yIEludGVybmV0IFJlc2Vh
cmNoIGFuZCBEZXZlbG9wbWVudCwgSW5jLjEQMA4GA1UEAxMHdGVzdC5j
YTAeFw0xNzA0MTcxNjQwMDlaFw0yNzA0MTUxNjQwMDlaMFcxCzAJBgNV
-----END CERTIFICATE-----`

	// Realistic RSA private key (simplified but structurally accurate)
	rsaKey := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF8PbnGy0AHB7MJvj1Kuq
ZwlGa0Ii1eFEQ3MmCUOLkk/qKdT3g4lGfZ0jxuG8JzYA8MWrkJBLhLmG
V/OiN8Z7V7/V1l7LvZ2V8KVOoV2V8KVOoV2V8KVOoV2V8KVOoV2V8KVO
base64encodedkeycontenthere1234567890abcdefghijklmnopqrstuv
-----END RSA PRIVATE KEY-----`

	// SSH private key format
	sshKey := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtz
c2gtZWQyNTUxOQAAACBHK2Ow5IDLrtuRXbiuRZiN/SAwucxRBvRPMD4emuEMzgAA
AIhBGQ+kQRkPpAAAAAtzc2gtZWQyNTUxOQAAACBHK2Ow5IDLrtuRXbiuRZiN/SAw
ucxRBvRPMD4emuEMzgAAAEBhw2EFv/NfMUDg1srH1Hc1YGQ/a2kKgKr7K4p4YAAG
-----END OPENSSH PRIVATE KEY-----`

	testCases := []struct {
		name        string
		secretsRaw  string
		checkFunc   func(t *testing.T, result map[string]string)
		expectError bool
	}{
		{
			name:       "Realistic PEM certificate via JSON",
			secretsRaw: `{"TLS_CERT": "-----BEGIN CERTIFICATE-----\nMIIDXTCCAkWgAwIBAgIJAJC1HiIAZAiUMA0GCSqGSIb3Qa sdfasdfasd\nEf8xQzBBBgNVBAcTOkZvdW5kYXRpb24gZm9yIEludGVybmV0IFJlc2Vh\nLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==\n-----END CERTIFICATE-----"}`,
			checkFunc: func(t *testing.T, result map[string]string) {
				cert, ok := result["TLS_CERT"]
				if !ok {
					t.Fatal("TLS_CERT not found in result")
				}
				if !containsActualNewline(cert) {
					t.Error("Certificate should contain actual newlines, but doesn't")
				}
				if !hasPrefix(cert, "-----BEGIN CERTIFICATE-----") {
					t.Error("Certificate should start with BEGIN CERTIFICATE marker")
				}
				if !hasSuffix(cert, "-----END CERTIFICATE-----") {
					t.Error("Certificate should end with END CERTIFICATE marker")
				}
			},
			expectError: false,
		},
		{
			name:       "Multiple multi-line secrets in single JSON",
			secretsRaw: `{"CERT": "-----BEGIN CERTIFICATE-----\nline1\nline2\n-----END CERTIFICATE-----", "KEY": "-----BEGIN RSA PRIVATE KEY-----\nkeydata\n-----END RSA PRIVATE KEY-----"}`,
			checkFunc: func(t *testing.T, result map[string]string) {
				if len(result) != 2 {
					t.Fatalf("Expected 2 secrets, got %d", len(result))
				}
				cert := result["CERT"]
				key := result["KEY"]
				if !containsActualNewline(cert) || !containsActualNewline(key) {
					t.Error("Both secrets should contain actual newlines")
				}
			},
			expectError: false,
		},
		{
			name:       "JSON with escaped backslashes in value",
			secretsRaw: `{"PATH": "C:\\Users\\test\\file.txt"}`,
			checkFunc: func(t *testing.T, result map[string]string) {
				path := result["PATH"]
				expected := `C:\Users\test\file.txt`
				if path != expected {
					t.Errorf("Expected %q, got %q", expected, path)
				}
			},
			expectError: false,
		},
		{
			name:       "JSON with mixed escape sequences",
			secretsRaw: `{"CONFIG": "line1\nline2\ttabbed\n\"quoted\""}`,
			checkFunc: func(t *testing.T, result map[string]string) {
				config := result["CONFIG"]
				if !containsActualNewline(config) {
					t.Error("Config should contain actual newlines")
				}
				if !containsActualTab(config) {
					t.Error("Config should contain actual tabs")
				}
				if !containsSubstring(config, `"quoted"`) {
					t.Error("Config should contain quotes")
				}
			},
			expectError: false,
		},
		{
			name:       "JSON with Unicode characters",
			secretsRaw: `{"MESSAGE": "Hello 世界! 🔐 Ключ"}`,
			checkFunc: func(t *testing.T, result map[string]string) {
				msg := result["MESSAGE"]
				if msg != "Hello 世界! 🔐 Ключ" {
					t.Errorf("Unicode not preserved correctly: %q", msg)
				}
			},
			expectError: false,
		},
		{
			name:       "Verify newline conversion - critical test",
			secretsRaw: `{"SECRET": "A\nB\nC"}`,
			checkFunc: func(t *testing.T, result map[string]string) {
				secret := result["SECRET"]
				// The JSON \n should become actual newline character (byte 0x0A)
				lines := splitByNewline(secret)
				if len(lines) != 3 {
					t.Errorf("Expected 3 lines after splitting by newline, got %d. Value: %q", len(lines), secret)
				}
				if lines[0] != "A" || lines[1] != "B" || lines[2] != "C" {
					t.Errorf("Lines not as expected: %v", lines)
				}
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseKeyValuePairs(tc.secretsRaw)
			if (err != nil) != tc.expectError {
				t.Fatalf("Expected error: %v, got: %v (error: %v)", tc.expectError, err != nil, err)
			}
			if err == nil && tc.checkFunc != nil {
				tc.checkFunc(t, result)
			}
		})
	}

	// Additional test: verify that we can parse what toJSON would produce
	// This simulates what GitHub Actions toJSON() function outputs
	t.Run("Simulated toJSON output for certificate", func(t *testing.T) {
		// When GitHub Actions runs toJSON(secrets.CERT) on a multi-line certificate,
		// it produces a JSON string with escaped newlines
		simulatedToJSONOutput := `"-----BEGIN CERTIFICATE-----\nMIIDXTCCAkWgAwIBAgIJAJC1HiIAZAiUMA0GCSqGSIb3Qa\nEf8xQzBBBgNVBAcTOkZvdW5kYXRpb24gZm9yIEludGVybmV0\n-----END CERTIFICATE-----"`

		// The full secrets input would look like this:
		secretsInput := `{"CERT": ` + simulatedToJSONOutput + `}`

		result, err := parseKeyValuePairs(secretsInput)
		if err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		cert := result["CERT"]
		if !containsActualNewline(cert) {
			t.Error("Certificate should contain actual newlines after parsing")
		}

		// Count newlines - should match the \n in the JSON
		newlineCount := countNewlines(cert)
		if newlineCount != 3 {
			t.Errorf("Expected 3 newlines in certificate, got %d", newlineCount)
		}
	})

	// Test that original key-value format still works (backward compatibility)
	t.Run("Backward compatibility - key-value format unchanged", func(t *testing.T) {
		input := "SECRET1=value1\nSECRET2=value2\nSECRET3=value=with=equals"
		result, err := parseKeyValuePairs(input)
		if err != nil {
			t.Fatalf("Key-value parsing failed: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("Expected 3 secrets, got %d", len(result))
		}
		if result["SECRET1"] != "value1" {
			t.Errorf("SECRET1 mismatch: %q", result["SECRET1"])
		}
		if result["SECRET3"] != "value=with=equals" {
			t.Errorf("SECRET3 should preserve multiple equals: %q", result["SECRET3"])
		}
	})

	// Verify that using the actual multi-line strings works
	t.Run("Actual multi-line strings via JSON", func(t *testing.T) {
		// This is what the actual parsed result should look like
		input := `{"CERT": ` + escapeForJSON(pemCert) + `, "KEY": ` + escapeForJSON(rsaKey) + `}`
		result, err := parseKeyValuePairs(input)
		if err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}
		if result["CERT"] != pemCert {
			t.Errorf("Certificate not preserved correctly.\nExpected:\n%s\n\nGot:\n%s", pemCert, result["CERT"])
		}
		if result["KEY"] != rsaKey {
			t.Errorf("Key not preserved correctly.\nExpected:\n%s\n\nGot:\n%s", rsaKey, result["KEY"])
		}
	})

	_ = sshKey // Used in other tests if needed
}

// Helper functions for tests
func containsActualNewline(s string) bool {
	return containsSubstring(s, "\n")
}

func containsActualTab(s string) bool {
	return containsSubstring(s, "\t")
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func splitByNewline(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == '\n' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func countNewlines(s string) int {
	count := 0
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	return count
}

// escapeForJSON converts a string to a JSON string literal (with quotes)
func escapeForJSON(s string) string {
	result := `"`
	for _, c := range s {
		switch c {
		case '"':
			result += `\"`
		case '\\':
			result += `\\`
		case '\n':
			result += `\n`
		case '\r':
			result += `\r`
		case '\t':
			result += `\t`
		default:
			result += string(c)
		}
	}
	result += `"`
	return result
}
