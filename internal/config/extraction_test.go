package config

import (
	"fmt"
	"os"
	"testing"
)

func TestExtractionPatternsLoadedFromJSON(t *testing.T) {
	// Test that extraction patterns are loaded when JSON file doesn't have them
	jsonContent := `{
		"keywords": [{"name": "test", "patterns": ["password"], "description": "test"}]
	}`

	tmpFile, err := os.CreateTemp("", "test_config_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(jsonContent); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	patterns := cfg.GetCompiledPatterns()
	if len(patterns) == 0 {
		t.Error("Expected default extraction patterns to be present when JSON doesn't have them")
	}

	fmt.Printf("Loaded %d extraction patterns from config without extractionPatterns in JSON\n", len(patterns))
	for _, p := range patterns {
		fmt.Printf("  - %s: %s\n", p.Name, p.Regex.String())
	}
}

func TestExtractionPatternsFromJSONOverride(t *testing.T) {
	// Test that extraction patterns from JSON override defaults
	jsonContent := `{
		"extractionPatterns": [
			{"name": "custom", "pattern": "^custom_(.+)=(.+)$", "valueGroup": 2, "description": "Custom"}
		],
		"keywords": [{"name": "test", "patterns": ["password"], "description": "test"}]
	}`

	tmpFile, err := os.CreateTemp("", "test_config_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(jsonContent); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	patterns := cfg.GetCompiledPatterns()
	if len(patterns) != 1 {
		t.Errorf("Expected 1 custom pattern, got %d", len(patterns))
	}

	if len(patterns) > 0 && patterns[0].Name != "custom" {
		t.Errorf("Expected pattern name 'custom', got '%s'", patterns[0].Name)
	}

	fmt.Printf("Loaded %d extraction patterns from JSON with custom extractionPatterns\n", len(patterns))
}

func TestLoadRealPatternsJSON(t *testing.T) {
	// Try to load the real patterns.json if it exists
	cfg, err := Load("../../patterns.json")
	if err != nil {
		t.Skipf("patterns.json not found: %v", err)
	}

	patterns := cfg.GetCompiledPatterns()
	fmt.Printf("Patterns chargés depuis patterns.json: %d\n", len(patterns))

	if len(patterns) == 0 {
		t.Error("No extraction patterns loaded!")
	}

	for _, p := range patterns {
		fmt.Printf("  - %s: %s\n", p.Name, p.Regex.String())
	}

	// Test with a line from exemple.dic
	testLine := "ibp/ate/mailConfig$service.mail.login.password=xK9m$pQ2wR#7vNjL"
	fmt.Printf("\nTest avec ligne de exemple.dic: %s\n", testLine)

	matched := false
	for _, p := range patterns {
		match := p.Regex.FindStringSubmatch(testLine)
		if match != nil && len(match) > p.ValueGroup {
			fmt.Printf("✓ Pattern '%s' match: key='%s', value='%s'\n", p.Name, match[1], match[p.ValueGroup])
			matched = true
			break
		}
	}

	if !matched {
		t.Error("No pattern matched the exemple.dic line!")
	}
}

func TestLooksLikeCode(t *testing.T) {
	cfg := DefaultConfig()

	testCases := []struct {
		value        string
		shouldIgnore bool
		description  string
	}{
		// Should be detected as code (ignored)
		{"entry.Date", true, "struct field access"},
		{"config.Value", true, "struct field access"},
		{"secret.FirstSeen", true, "struct field access"},
		{"foo.Bar", true, "struct field access"},
		{"make([]string, 0)", true, "function call"},
		{"append(slice, item)", true, "function call"},
		{"data[0]", true, "array access"},
		{"{foo: bar}", true, "object literal"},
		{"foo.bar.baz.qux", true, "multiple dots"},
		{"return err", true, "Go keyword"},

		// Should NOT be detected as code (real secrets)
		{"xK9m$pQ2wR#7vNjL", false, "password with special chars"},
		{"e8d7c6b5-a4f3-42e1-9b8a-1c2d3e4f5a6b", false, "UUID"},
		{"ibp/ate/config$value", false, "path with special chars"},
		{"7f8e9d0c-b1a2-43f4-85c6-2d3e4f5a6b7c", false, "UUID from exemple.dic"},

		// These contain keywords but are real secrets (should NOT be ignored)
		{"mysecretpassword", false, "contains 'secret' but is a real password"},
		// These are ignored due to ignoredValues patterns
		{"api.example.com", true, "contains 'example' - ignored by ignoredValues"},
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			result := cfg.ShouldIgnoreValue(tc.value)
			// Note: ShouldIgnoreValue also checks length, so we test looksLikeCode indirectly
			if tc.shouldIgnore && !result {
				t.Errorf("Expected '%s' (%s) to be ignored as code, but it wasn't", tc.value, tc.description)
			}
			if !tc.shouldIgnore && result {
				t.Errorf("Expected '%s' (%s) to NOT be ignored, but it was", tc.value, tc.description)
			}
			status := "✗ NOT ignored"
			if result {
				status = "✓ ignored"
			}
			fmt.Printf("%s: '%s' (%s)\n", status, tc.value, tc.description)
		})
	}
}

func TestExtractionPatterns(t *testing.T) {
	cfg := DefaultConfig()
	patterns := cfg.GetCompiledPatterns()
	
	testCases := []struct {
		line          string
		expectMatch   bool
		expectedKey   string
		expectedValue string
	}{
		{"password=secret1", true, "password", "secret1"},
		{"password: secret2", true, "password", "secret2"},
		{`"password": "secret3"`, true, "password", "secret3"},
		{"export PASSWORD=secret4", true, "PASSWORD", "secret4"},
		{"api_key = myapikey123", true, "api_key", "myapikey123"},
		{`API_KEY: "quotedvalue"`, true, "API_KEY", "quotedvalue"},
		// Formats from exemple.dic (with /, $, . in key names)
		{"ibp/ate/mailConfig$service.mail.login.password=xK9m$pQ2wR#7vNjL", true, "ibp/ate/mailConfig$service.mail.login.password", "xK9m$pQ2wR#7vNjL"},
		{"ibp/ate/oidc$oidc.client_secret=e8d7c6b5-a4f3-42e1-9b8a-1c2d3e4f5a6b", true, "ibp/ate/oidc$oidc.client_secret", "e8d7c6b5-a4f3-42e1-9b8a-1c2d3e4f5a6b"},
		{"ibp/common/providerConfigApp$BCOM.AccessToken.clientSecret=7f8e9d0c-b1a2-43f4-85c6-2d3e4f5a6b7c", true, "ibp/common/providerConfigApp$BCOM.AccessToken.clientSecret", "7f8e9d0c-b1a2-43f4-85c6-2d3e4f5a6b7c"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.line, func(t *testing.T) {
			found := false
			for _, p := range patterns {
				match := p.Regex.FindStringSubmatch(tc.line)
				if match != nil && len(match) > p.ValueGroup {
					key := match[1]
					value := match[p.ValueGroup]
					found = true
					
					if tc.expectMatch {
						if key != tc.expectedKey {
							t.Errorf("Expected key '%s', got '%s'", tc.expectedKey, key)
						}
						if value != tc.expectedValue {
							t.Errorf("Expected value '%s', got '%s'", tc.expectedValue, value)
						}
						fmt.Printf("✓ Line '%s' -> key='%s', value='%s'\n", tc.line, key, value)
					}
					break
				}
			}
			
			if tc.expectMatch && !found {
				t.Errorf("Expected match for '%s' but none found", tc.line)
			}
			if !tc.expectMatch && found {
				t.Errorf("Did not expect match for '%s' but one was found", tc.line)
			}
		})
	}
}
