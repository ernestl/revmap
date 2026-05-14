package cmd

import (
	"testing"
)

func TestParseFieldList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"single field", "version", []string{"version"}},
		{"multiple fields", "version,status,architectures", []string{"version", "status", "architectures"}},
		{"with spaces", "version , status , architectures", []string{"version", "status", "architectures"}},
		{"empty parts", "version,,status", []string{"version", "status"}},
		{"empty string", "", []string(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFieldList(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseFieldList(%q) returned %d items, want %d", tt.input, len(got), len(tt.want))
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("parseFieldList(%q)[%d] = %q, want %q", tt.input, i, v, tt.want[i])
				}
			}
		})
	}
}

func TestFilterFields(t *testing.T) {
	raw := map[string]interface{}{
		"revision": map[string]interface{}{
			"version":       "2.75.2",
			"status":        "Published",
			"architectures": []string{"amd64"},
			"confinement":   "strict",
		},
	}

	// No filter: return full response.
	result := filterFields(raw, "")
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map[string]interface{}")
	}
	if _, ok := resultMap["revision"]; !ok {
		t.Error("full response should contain 'revision' key")
	}

	// Filter specific fields (from nested revision object).
	result = filterFields(raw, "version,status")
	filtered, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map[string]interface{}")
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 fields, got %d", len(filtered))
	}
	if filtered["version"] != "2.75.2" {
		t.Errorf("expected version=2.75.2, got %v", filtered["version"])
	}
	if filtered["status"] != "Published" {
		t.Errorf("expected status=Published, got %v", filtered["status"])
	}

	// Non-existent field returns empty result.
	result = filterFields(raw, "nonexistent")
	filtered, ok = result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map[string]interface{}")
	}
	if len(filtered) != 0 {
		t.Errorf("expected 0 fields for nonexistent key, got %d", len(filtered))
	}

	// Flat structure (no nested "revision" key).
	flat := map[string]interface{}{
		"version": "2.75.2",
		"status":  "Published",
	}
	result = filterFields(flat, "version")
	filtered, ok = result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map[string]interface{}")
	}
	if filtered["version"] != "2.75.2" {
		t.Errorf("expected version=2.75.2, got %v", filtered["version"])
	}
}
