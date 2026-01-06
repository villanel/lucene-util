package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	version  = "dev"
	gitSha   = "unknown"
	hostname = "unknown"

	// Prometheus metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"endpoint", "method", "status"},
	)

	analyzeOperationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "analyze_operations_total",
			Help: "Total number of analyze operations",
		},
	)

	analyzeOperationDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "analyze_operation_duration_seconds",
			Help:    "Analyze operation duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	errorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "error_count",
			Help: "Total number of errors",
		},
		[]string{"type"},
	)
)

// HealthResponse is the response for the /healthz endpoint
type HealthResponse struct {
	Status string `json:"status"`
}

// InfoResponse is the response for the /info endpoint
type InfoResponse struct {
	Version  string `json:"version"`
	GitSha   string `json:"git_sha"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
}

// ---------- HTTP handlers ----------

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	response := InfoResponse{
		Version:  version,
		GitSha:   gitSha,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func analyzeHandler(w http.ResponseWriter, r *http.Request) {
	// Record start time for operation duration metric
	startTime := time.Now()
	defer func() {
		// Record the operation duration
		duration := time.Since(startTime).Seconds()
		analyzeOperationDuration.Observe(duration)
		// Increment the operation counter
		analyzeOperationsTotal.Inc()
	}()

	// Create a temporary directory to extract the archive
	tempDir, err := os.MkdirTemp("", "lucene-shard-")
	if err != nil {
		http.Error(w, "Failed to create temporary directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	// Read the uploaded file
	var fileContent []byte
	var fileExt string

	// Handle both multipart form and direct file upload
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		// Get file from form
		file, header, err := r.FormFile("archive")
		if err != nil {
			http.Error(w, "Failed to get file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Read file content
		fileContent, err = io.ReadAll(file)
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusInternalServerError)
			return
		}

		// Get file extension
		fileExt = strings.ToLower(filepath.Ext(header.Filename))
	} else {
		// Direct file upload
		fileContent, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		// Determine file type from Content-Type or extension
		switch contentType {
		case "application/zip":
			fileExt = ".zip"
		case "application/x-tar", "application/tar":
			fileExt = ".tar"
		case "application/x-gzip", "application/gzip":
			fileExt = ".tar.gz"
		default:
			// Check Content-Disposition for filename
			contentDisposition := r.Header.Get("Content-Disposition")
			if strings.Contains(contentDisposition, "filename=") {
				parts := strings.Split(contentDisposition, "filename=")
				if len(parts) > 1 {
					filename := strings.Trim(parts[1], `"; `)
					fileExt = strings.ToLower(filepath.Ext(filename))
				}
			}
		}
	}

	// Validate file extension
	if fileExt != ".zip" && fileExt != ".tar" && fileExt != ".tar.gz" {
		http.Error(w, "Unsupported file format. Please upload tar, tar.gz, or zip files.", http.StatusBadRequest)
		return
	}

	// Extract the archive
	if fileExt == ".zip" {
		// Extract zip file
		reader, err := zip.NewReader(bytes.NewReader(fileContent), int64(len(fileContent)))
		if err != nil {
			http.Error(w, "Failed to process zip file", http.StatusBadRequest)
			return
		}

		for _, f := range reader.File {
			path := filepath.Join(tempDir, f.Name)
			if f.FileInfo().IsDir() {
				os.MkdirAll(path, 0755)
				continue
			}

			os.MkdirAll(filepath.Dir(path), 0755)
			dst, _ := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if dst == nil {
				continue
			}
			src, _ := f.Open()
			if src == nil {
				dst.Close()
				continue
			}
			io.Copy(dst, src)
			src.Close()
			dst.Close()
		}
	} else {
		// Extract tar file (both regular and gzipped)
		var reader *tar.Reader
		if fileExt == ".tar.gz" {
			// Handle gzipped tar
			gzipReader, err := gzip.NewReader(bytes.NewReader(fileContent))
			if err != nil {
				http.Error(w, "Failed to create gzip reader: "+err.Error(), http.StatusBadRequest)
				errorCount.WithLabelValues("create_gzip_reader").Inc()
				return
			}
			defer gzipReader.Close()
			reader = tar.NewReader(gzipReader)
		} else {
			// Handle regular tar
			reader = tar.NewReader(bytes.NewReader(fileContent))
		}

		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, "Failed to read tar header: "+err.Error(), http.StatusInternalServerError)
				errorCount.WithLabelValues("read_tar_header").Inc()
				return
			}

			path := filepath.Join(tempDir, header.Name)
			switch header.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(path, 0755); err != nil {
					http.Error(w, "Failed to create directory: "+err.Error(), http.StatusInternalServerError)
					errorCount.WithLabelValues("mkdir").Inc()
					return
				}
			case tar.TypeReg:
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					http.Error(w, "Failed to create directory: "+err.Error(), http.StatusInternalServerError)
					errorCount.WithLabelValues("mkdir").Inc()
					return
				}

				dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				if err != nil {
					http.Error(w, "Failed to create file: "+err.Error(), http.StatusInternalServerError)
					errorCount.WithLabelValues("create_file").Inc()
					return
				}

				_, err = io.Copy(dst, reader)
				if err != nil {
					dst.Close() // 关闭文件句柄
					http.Error(w, "Failed to copy file: "+err.Error(), http.StatusInternalServerError)
					errorCount.WithLabelValues("copy_file").Inc()
					return
				}

				dst.Close() // 直接关闭文件，不要使用defer，否则在循环中会导致文件句柄泄漏
			}
		}
	}

	// Find the Lucene index directory
	indexDir, err := findLuceneIndexDir(tempDir)
	if err != nil {
		http.Error(w, "Failed to find Lucene index directory: "+err.Error(), http.StatusBadRequest)
		errorCount.WithLabelValues("find_index_dir").Inc()
		return
	}

	// Build the report
	report, err := buildReport(indexDir)
	if err != nil {
		http.Error(w, "Failed to analyze Lucene shard: "+err.Error(), http.StatusInternalServerError)
		errorCount.WithLabelValues("build_report").Inc()
		return
	}

	// Return the report as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(report); err != nil {
		errorCount.WithLabelValues("encode_json").Inc()
	}
}

func findLuceneIndexDir(rootDir string) (string, error) {
	// Walk through the directory structure to find the Lucene index directory
	var indexDir string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Check if this directory contains segments_N files
			fis, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			for _, fi := range fis {
				if strings.HasPrefix(fi.Name(), SEGMENTS_PREFIX) && fi.Name() != SEGMENTS_GEN_FILE {
					indexDir = path
					return filepath.SkipDir
				}
			}
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	if indexDir == "" {
		return "", errors.New("no Lucene index directory found in the archive")
	}
	return indexDir, nil
}

// ---------- HTTP middleware ----------

func metricsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get endpoint
		endpoint := r.URL.Path
		method := r.Method

		// Create a custom response writer to capture status code
		statusWriter := &statusResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call the next handler
		next(statusWriter, r)

		// Update request counter metric
		httpRequestsTotal.WithLabelValues(endpoint, method, strconv.Itoa(statusWriter.statusCode)).Inc()
	}
}

// Custom response writer to capture status code
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// ---------- main function ----------

func main() {
	// Parse command line flags
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	// Set hostname
	if hn, err := os.Hostname(); err == nil {
		hostname = hn
	}

	// Set git SHA if available
	if cmd := exec.Command("git", "rev-parse", "--short", "HEAD"); cmd.Run() == nil {
		if output, err := cmd.Output(); err == nil {
			gitSha = strings.TrimSpace(string(output))
		}
	}

	// Create a new registry for metrics
	registry := prometheus.NewRegistry()

	// Set as default registerer and gatherer
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	// Register metrics
	registry.MustRegister(
		httpRequestsTotal,
		analyzeOperationsTotal,
		analyzeOperationDuration,
		errorCount,
	)

	// Set up HTTP routes with middleware
	http.HandleFunc("/healthz", metricsMiddleware(healthzHandler))
	http.HandleFunc("/info", metricsMiddleware(infoHandler))
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/analyze", metricsMiddleware(analyzeHandler))

	// Start the server
	log.Printf("Starting Lucene Shard Analyzer Service on port %s", *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
