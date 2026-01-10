package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerationFromSegmentsFileName tests the generationFromSegmentsFileName function
func TestGenerationFromSegmentsFileName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     int64
		wantErr  bool
	}{
		{
			name:     "segments_1",
			filename: "segments_1",
			want:     1,
			wantErr:  false,
		},
		{
			name:     "segments_a",
			filename: "segments_a",
			want:     10,
			wantErr:  false,
		},
		{
			name:     "segments",
			filename: "segments",
			want:     -1,
			wantErr:  true,
		},
		{
			name:     "segments.gen",
			filename: "segments.gen",
			want:     -1,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generationFromSegmentsFileName(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("generationFromSegmentsFileName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("generationFromSegmentsFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFindLatestSegmentsFile tests the findLatestSegmentsFile function
func TestFindLatestSegmentsFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-segments-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	files := []string{
		"segments_1",
		"segments_2",
		"segments_a", // This should be latest (a=10 in base36)
		"segments.gen",
	}

	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tempDir, f), []byte(""), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", f, err)
		}
	}

	// Test finding latest segments file
	latest, err := findLatestSegmentsFile(tempDir)
	if err != nil {
		t.Fatalf("findLatestSegmentsFile() error = %v", err)
	}

	expected := "segments_a"
	if latest != expected {
		t.Errorf("findLatestSegmentsFile() = %v, want %v", latest, expected)
	}
}

// TestParseSegmentSI tests the parseSegmentSI function
// This is a more complex test that would require a real .si file
// For now, we'll test the error handling
func TestParseSegmentSI(t *testing.T) {
	// Test with non-existent file
	docCount, isCompound, diag, err := parseSegmentSI(".", "non_existent_segment")
	if err == nil {
		t.Errorf("parseSegmentSI() should return error for non-existent file")
	}

	// Verify default values
	if docCount != 0 {
		t.Errorf("parseSegmentSI() docCount = %v, want 0 for error case", docCount)
	}
	if isCompound != false {
		t.Errorf("parseSegmentSI() isCompound = %v, want false for error case", isCompound)
	}
	if diag != nil {
		t.Errorf("parseSegmentSI() diag = %v, want nil for error case", diag)
	}
}
