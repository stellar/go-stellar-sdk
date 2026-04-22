package main

// testdata/mini.json is a committed IR fixture derived from testdata/mini.x.
// To regenerate (requires the Rust generator-definitions-json binary on PATH):
//   generator-definitions-json --input testdata/mini.x --output testdata/mini.json

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestGoldenViews(t *testing.T) {
	data, err := os.ReadFile("testdata/mini.json")
	if err != nil {
		t.Fatalf("reading IR: %v", err)
	}

	var ir IR
	if err = json.Unmarshal(data, &ir); err != nil {
		t.Fatalf("parsing IR: %v", err)
	}

	gen, err := NewGenerator(&ir)
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}

	got, err := gen.GenerateViews()
	if err != nil {
		t.Fatalf("GenerateViews: %v", err)
	}

	goldenPath := "testdata/mini_views.golden"
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		// First run — create the golden file.
		if err = os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("Created golden file: %s", goldenPath)
		return
	}

	if !bytes.Equal(got, golden) {
		if os.Getenv("UPDATE_GOLDEN") != "" {
			if err := os.WriteFile(goldenPath, got, 0644); err != nil {
				t.Fatalf("updating golden file: %v", err)
			}
			t.Logf("Updated golden file: %s", goldenPath)
			return
		}
		// Find first difference for a useful error message.
		line := 1
		for i := 0; i < len(got) && i < len(golden); i++ {
			if got[i] != golden[i] {
				t.Fatalf("generated output differs from golden file at byte %d (line ~%d). Set UPDATE_GOLDEN=1 to update.", i, line)
			}
			if got[i] == '\n' {
				line++
			}
		}
		t.Fatalf("generated output differs from golden file (length: got %d, want %d). Set UPDATE_GOLDEN=1 to update.", len(got), len(golden))
	}
}
