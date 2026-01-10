package main

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// TestFindLuceneIndexDir tests the findLuceneIndexDir function
func TestFindLuceneIndexDir(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "test-lucene-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a nested directory structure with a segments file
	indexPath := filepath.Join(tempDir, "nested", "index")
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	// Create a segments file
	segmentsFile := filepath.Join(indexPath, "segments_1")
	if err := os.WriteFile(segmentsFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create segments file: %v", err)
	}

	// Test finding the index directory
	foundDir, err := findLuceneIndexDir(tempDir)
	if err != nil {
		t.Fatalf("findLuceneIndexDir() error = %v", err)
	}

	if foundDir != indexPath {
		t.Errorf("findLuceneIndexDir() = %v, want %v", foundDir, indexPath)
	}
}

// TestFindLuceneIndexDirNoSegments tests findLuceneIndexDir with no segments file
func TestFindLuceneIndexDirNoSegments(t *testing.T) {
	// Create a temporary directory structure without segments file
	tempDir, err := os.MkdirTemp("", "test-lucene-no-segments-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a nested directory structure
	if err := os.MkdirAll(filepath.Join(tempDir, "nested", "index"), 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	// Test finding the index directory (should fail)
	_, err = findLuceneIndexDir(tempDir)
	if err == nil {
		t.Errorf("findLuceneIndexDir() should return error when no segments file found")
	}
}

// TestBuildReport tests the buildReport function with a simple segments file
func TestBuildReport(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "test-report-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a minimal valid segments file
	// This is a simplified version for testing purposes
	segmentsFile := filepath.Join(tempDir, "segments_1")
	
	// Create a simple segments file with the expected magic number
	var buf bytes.Buffer
	
	// Write magic number (CODEC_MAGIC)
	binary.Write(&buf, binary.BigEndian, int32(CODEC_MAGIC))
	
	// Write "segments" string
	writeVIntBytes(&buf, 8) // Length of "segments"
	buf.Write([]byte("segments"))
	
	// Write version (4 bytes)
	binary.Write(&buf, binary.BigEndian, int32(9))
	
	// Write ID (16 bytes)
	buf.Write(make([]byte, 16))
	
	// Write suffix length (1 byte)
	buf.Write([]byte{0})
	
	// Write version triple (3 bytes)
	buf.Write([]byte{9, 0, 0})
	
	// Write index created version (1 byte)
	buf.Write([]byte{9})
	
	// Write SegInfo version (8 bytes)
	binary.Write(&buf, binary.BigEndian, int64(1))
	
	// Write counter (vLong)
	writeVLongBytes(&buf, 1)
	
	// Write number of segments (4 bytes)
	binary.Write(&buf, binary.BigEndian, int32(0))
	
	// Write user data (empty map)
	writeVIntBytes(&buf, 0)
	
	if err := ioutil.WriteFile(segmentsFile, buf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to create segments file: %v", err)
	}

	// Test building the report
	report, err := buildReport(tempDir)
	if err != nil {
		t.Fatalf("buildReport() error = %v", err)
	}

	// Verify report contents
	if report.IndexPath != tempDir {
		t.Errorf("report.IndexPath = %v, want %v", report.IndexPath, tempDir)
	}

	if report.SegmentsFile != "segments_1" {
		t.Errorf("report.SegmentsFile = %v, want segments_1", report.SegmentsFile)
	}

	if report.TotalSegments != 0 {
		t.Errorf("report.TotalSegments = %v, want 0", report.TotalSegments)
	}
}

// Helper functions for writing vInt and vLong to bytes buffer
func writeVIntBytes(buf *bytes.Buffer, i int) {
	for {
		b := byte(i & 0x7F)
		i >>= 7
		if i == 0 {
			buf.Write([]byte{b})
			return
		}
		buf.Write([]byte{b | 0x80})
	}
}

func writeVLongBytes(buf *bytes.Buffer, i int64) {
	for {
		b := byte(i & 0x7F)
		i >>= 7
		if i == 0 {
			buf.Write([]byte{b})
			return
		}
		buf.Write([]byte{b | 0x80})
	}
}
