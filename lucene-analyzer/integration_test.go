package main

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// TestAnalyzeWithRealData tests the full analyze functionality with real test data
func TestAnalyzeWithRealData(t *testing.T) {
	// Use one of the test data files from the test directory
	testDataPath := "../test/test-data/4bMihoe5Q8Ww7MB_n7z-EA.zip"
	
	// Read the test file
	testData, err := ioutil.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	// Create a temporary directory to extract the archive
	tempDir, err := os.MkdirTemp("", "test-analyze-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract the zip file
	reader, err := zip.NewReader(bytes.NewReader(testData), int64(len(testData)))
	if err != nil {
		t.Fatalf("Failed to process zip file: %v", err)
	}

	for _, f := range reader.File {
		path := filepath.Join(tempDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(path), 0755)
		dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		src, err := f.Open()
		if err != nil {
			dst.Close()
			t.Fatalf("Failed to open zip file entry: %v", err)
		}
		_, err = io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			t.Fatalf("Failed to copy file: %v", err)
		}
	}

	// Find the Lucene index directory
	indexDir, err := findLuceneIndexDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to find Lucene index directory: %v", err)
	}

	// Test building the report
	report, err := buildReport(indexDir)
	if err != nil {
		t.Fatalf("buildReport() error = %v", err)
	}

	// Verify the report contains expected information
	if report.IndexPath != indexDir {
		t.Errorf("report.IndexPath = %v, want %v", report.IndexPath, indexDir)
	}

	// The report should have segments information
	if report.TotalSegments < 0 {
		t.Errorf("report.TotalSegments = %v, should be >= 0", report.TotalSegments)
	}

	// The report should have total docs information
	if report.TotalDocs < 0 {
		t.Errorf("report.TotalDocs = %v, should be >= 0", report.TotalDocs)
	}

	// Print the report for debugging (optional)
	// t.Logf("Report: %+v", report)
}

// TestMultipleTestDataFiles tests analyzing multiple test data files
func TestMultipleTestDataFiles(t *testing.T) {
	// Get all test data files
	testDataDir := "../test/test-data/"
	files, err := ioutil.ReadDir(testDataDir)
	if err != nil {
		t.Fatalf("Failed to read test data directory: %v", err)
	}

	// Limit to first 2 files for faster testing
	maxFiles := 2
	processedFiles := 0

	for _, file := range files {
		if processedFiles >= maxFiles {
			break
		}

		if filepath.Ext(file.Name()) != ".zip" {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			testDataPath := filepath.Join(testDataDir, file.Name())
			
			// Read the test file
			testData, err := ioutil.ReadFile(testDataPath)
			if err != nil {
				t.Fatalf("Failed to read test data file: %v", err)
			}

			// Create a temporary directory to extract the archive
			tempDir, err := os.MkdirTemp("", "test-analyze-")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Extract the zip file
			reader, err := zip.NewReader(bytes.NewReader(testData), int64(len(testData)))
			if err != nil {
				t.Fatalf("Failed to process zip file: %v", err)
			}

			for _, f := range reader.File {
				path := filepath.Join(tempDir, f.Name)
				if f.FileInfo().IsDir() {
					os.MkdirAll(path, 0755)
					continue
				}

				os.MkdirAll(filepath.Dir(path), 0755)
				dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
				if err != nil {
					t.Fatalf("Failed to create file: %v", err)
				}
				src, err := f.Open()
				if err != nil {
					dst.Close()
					t.Fatalf("Failed to open zip file entry: %v", err)
				}
				_, err = io.Copy(dst, src)
				src.Close()
				dst.Close()
				if err != nil {
					t.Fatalf("Failed to copy file: %v", err)
				}
			}

			// Find the Lucene index directory
			indexDir, err := findLuceneIndexDir(tempDir)
			if err != nil {
				t.Fatalf("Failed to find Lucene index directory: %v", err)
			}

			// Test building the report
			report, err := buildReport(indexDir)
			if err != nil {
				t.Fatalf("buildReport() error = %v", err)
			}

			// Verify basic report structure
			if report.IndexPath != indexDir {
				t.Errorf("report.IndexPath = %v, want %v", report.IndexPath, indexDir)
			}
			if report.SegmentsFile == "" {
				t.Error("report.SegmentsFile should not be empty")
			}
		})

		processedFiles++
	}
}
