package download

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProgressReader(t *testing.T) {
	// Test data
	testData := "Hello, World! This is a test string for progress reading."
	reader := strings.NewReader(testData)
	totalSize := int64(len(testData))

	// Track progress calls
	var progressCalls []struct {
		current, total, speed int64
	}

	callback := func(current, total, speed int64) {
		progressCalls = append(progressCalls, struct {
			current, total, speed int64
		}{current, total, speed})
	}

	progressReader := NewProgressReader(reader, totalSize, callback)

	// Read data in chunks
	buffer := make([]byte, 10)
	var totalRead int64

	for {
		n, err := progressReader.Read(buffer)
		totalRead += int64(n)

		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error reading: %v", err)
		}

		// Simulate some delay to allow progress callbacks
		time.Sleep(10 * time.Millisecond)
	}

	// Verify total bytes read
	if totalRead != totalSize {
		t.Errorf("Expected to read %d bytes, got %d", totalSize, totalRead)
	}

	// Verify progress reader properties
	if progressReader.current != totalSize {
		t.Errorf("Expected current %d, got %d", totalSize, progressReader.current)
	}

	if progressReader.total != totalSize {
		t.Errorf("Expected total %d, got %d", totalSize, progressReader.total)
	}
}

func TestProgressReader_NoCallback(t *testing.T) {
	testData := "Test data"
	reader := strings.NewReader(testData)

	// Create progress reader without callback
	progressReader := NewProgressReader(reader, int64(len(testData)), nil)

	// Should not panic when reading
	buffer := make([]byte, len(testData))
	n, err := progressReader.Read(buffer)

	if err != nil && err != io.EOF {
		t.Fatalf("Unexpected error: %v", err)
	}

	if n != len(testData) {
		t.Errorf("Expected to read %d bytes, got %d", len(testData), n)
	}
}

func TestNewDownloader(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target")

	downloader := NewDownloader(tempDir, targetDir)

	if downloader.tempDir != tempDir {
		t.Errorf("Expected temp dir '%s', got '%s'", tempDir, downloader.tempDir)
	}

	if downloader.targetDir != targetDir {
		t.Errorf("Expected target dir '%s', got '%s'", targetDir, downloader.targetDir)
	}

	if downloader.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if downloader.userAgent != "podcast-tui/1.0" {
		t.Errorf("Expected user agent 'podcast-tui/1.0', got '%s'", downloader.userAgent)
	}

	if downloader.maxRetries != 5 {
		t.Errorf("Expected max retries 5, got %d", downloader.maxRetries)
	}
}

func TestDownloader_GetFileSize(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("Expected HEAD request, got %s", r.Method)
		}

		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	downloader := NewDownloader(tempDir, tempDir)

	ctx := context.Background()
	size, err := downloader.GetFileSize(ctx, server.URL)

	if err != nil {
		t.Fatalf("Failed to get file size: %v", err)
	}

	if size != 1024 {
		t.Errorf("Expected file size 1024, got %d", size)
	}
}

func TestDownloader_GetFileSize_NoContentLength(t *testing.T) {
	// Create test server without Content-Length header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	downloader := NewDownloader(tempDir, tempDir)

	ctx := context.Background()
	_, err := downloader.GetFileSize(ctx, server.URL)

	if err == nil {
		t.Error("Expected error when Content-Length header is missing")
	}

	if !strings.Contains(err.Error(), "content-length header not found") {
		t.Errorf("Expected 'content-length header not found' error, got: %v", err)
	}
}

func TestDownloader_GetFileSize_InvalidContentLength(t *testing.T) {
	// Create test server with invalid Content-Length
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "invalid")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	downloader := NewDownloader(tempDir, tempDir)

	ctx := context.Background()
	_, err := downloader.GetFileSize(ctx, server.URL)

	if err == nil {
		t.Error("Expected error when Content-Length is invalid")
	}

	if !strings.Contains(err.Error(), "invalid content-length") {
		t.Errorf("Expected 'invalid content-length' error, got: %v", err)
	}
}

func TestDownloader_DownloadFile_Success(t *testing.T) {
	// Test data
	testData := "This is test audio file content for download testing."

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "podcast-tui/1.0" {
			t.Errorf("Expected User-Agent 'podcast-tui/1.0', got '%s'", r.Header.Get("User-Agent"))
		}

		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testData))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target")
	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	downloader := NewDownloader(tempDir, targetDir)

	ctx := context.Background()
	filename := "test-episode.mp3"

	// Track progress
	var progressUpdates []struct {
		current, total, speed int64
	}

	progressCallback := func(current, total, speed int64) {
		progressUpdates = append(progressUpdates, struct {
			current, total, speed int64
		}{current, total, speed})
	}

	err = downloader.DownloadFile(ctx, server.URL, filename, progressCallback)
	if err != nil {
		t.Fatalf("Failed to download file: %v", err)
	}

	// Verify file was created
	targetPath := filepath.Join(targetDir, filename)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Error("Downloaded file was not created")
	}

	// Verify file content
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != testData {
		t.Errorf("Expected file content '%s', got '%s'", testData, string(content))
	}

	// Verify progress was tracked
	if len(progressUpdates) == 0 {
		t.Error("Expected progress updates, got none")
	}

	// Verify temp file was cleaned up
	tempPath := filepath.Join(tempDir, filename+".tmp")
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("Temp file should have been cleaned up")
	}
}

func TestDownloader_DownloadFile_FileExists(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target")
	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	filename := "existing-file.mp3"
	targetPath := filepath.Join(targetDir, filename)

	// Create existing file
	err = os.WriteFile(targetPath, []byte("existing content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	downloader := NewDownloader(tempDir, targetDir)

	ctx := context.Background()
	err = downloader.DownloadFile(ctx, "http://example.com/test.mp3", filename, nil)

	if err == nil {
		t.Error("Expected error when file already exists")
	}

	if !strings.Contains(err.Error(), "file already exists") {
		t.Errorf("Expected 'file already exists' error, got: %v", err)
	}
}

func TestDownloader_DownloadFile_Resume(t *testing.T) {
	// Test data
	fullData := "This is the complete file content for resume testing."
	partialData := fullData[:20]   // First 20 characters
	remainingData := fullData[20:] // Rest of the data

	// Create test server that supports range requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")

		if rangeHeader != "" {
			// Handle range request (resume)
			if rangeHeader != "bytes=20-" {
				t.Errorf("Expected Range header 'bytes=20-', got '%s'", rangeHeader)
			}

			w.Header().Set("Content-Range", fmt.Sprintf("bytes 20-%d/%d", len(fullData)-1, len(fullData)))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(remainingData)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write([]byte(remainingData))
		} else {
			// Handle full request
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullData)))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fullData))
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target")
	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	downloader := NewDownloader(tempDir, targetDir)
	filename := "resume-test.mp3"

	// Create partial temp file
	tempPath := filepath.Join(tempDir, filename+".tmp")
	err = os.WriteFile(tempPath, []byte(partialData), 0644)
	if err != nil {
		t.Fatalf("Failed to create partial temp file: %v", err)
	}

	ctx := context.Background()
	err = downloader.DownloadFile(ctx, server.URL, filename, nil)
	if err != nil {
		t.Fatalf("Failed to resume download: %v", err)
	}

	// Verify complete file
	targetPath := filepath.Join(targetDir, filename)
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read completed file: %v", err)
	}

	if string(content) != fullData {
		t.Errorf("Expected complete content '%s', got '%s'", fullData, string(content))
	}
}

func TestDownloader_DownloadFile_Cancelled(t *testing.T) {
	// Create test server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes.Repeat([]byte("a"), 1000))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	downloader := NewDownloader(tempDir, tempDir)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := downloader.DownloadFile(ctx, server.URL, "test.mp3", nil)

	if err == nil {
		t.Error("Expected error due to context timeout")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "failed to download file") {
		t.Errorf("Expected context timeout error, got: %v", err)
	}
}

func TestDownloader_DownloadFile_ServerError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	downloader := NewDownloader(tempDir, tempDir)

	ctx := context.Background()
	err := downloader.DownloadFile(ctx, server.URL, "test.mp3", nil)

	if err == nil {
		t.Error("Expected error due to server error")
	}

	if !strings.Contains(err.Error(), "unexpected status code: 500") {
		t.Errorf("Expected status code error, got: %v", err)
	}
}

func TestDownloader_CleanupTempFile(t *testing.T) {
	tempDir := t.TempDir()
	downloader := NewDownloader(tempDir, tempDir)

	filename := "test-cleanup"
	tempPath := filepath.Join(tempDir, filename+".tmp")

	// Create temp file
	err := os.WriteFile(tempPath, []byte("temp content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		t.Fatal("Temp file was not created")
	}

	// Cleanup temp file
	err = downloader.CleanupTempFile(filename)
	if err != nil {
		t.Fatalf("Failed to cleanup temp file: %v", err)
	}

	// Verify file was removed
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("Temp file should have been removed")
	}
}

func TestDownloader_CleanupTempFile_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	downloader := NewDownloader(tempDir, tempDir)

	// Cleanup non-existent file should not error
	err := downloader.CleanupTempFile("nonexistent")
	if err != nil {
		t.Errorf("Cleanup of non-existent file should not error: %v", err)
	}
}

func TestDownloader_InvalidURL(t *testing.T) {
	tempDir := t.TempDir()
	downloader := NewDownloader(tempDir, tempDir)

	ctx := context.Background()

	// Test invalid URL
	err := downloader.DownloadFile(ctx, "not-a-valid-url", "test.mp3", nil)
	if err == nil {
		t.Error("Expected error for invalid URL")
	}

	// Test GetFileSize with invalid URL
	_, err = downloader.GetFileSize(ctx, "not-a-valid-url")
	if err == nil {
		t.Error("Expected error for invalid URL in GetFileSize")
	}
}

func TestDownloader_LargeFileSimulation(t *testing.T) {
	// Simulate large file with chunked response
	chunkSize := 1024
	totalChunks := 10
	totalSize := chunkSize * totalChunks

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", totalSize))
		w.WriteHeader(http.StatusOK)

		// Send data in chunks with small delays
		for i := 0; i < totalChunks; i++ {
			chunk := bytes.Repeat([]byte("x"), chunkSize)
			w.Write(chunk)

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			time.Sleep(1 * time.Millisecond) // Small delay
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target")
	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	downloader := NewDownloader(tempDir, targetDir)

	ctx := context.Background()
	filename := "large-file.mp3"

	var progressUpdates []struct {
		current, total, speed int64
	}

	progressCallback := func(current, total, speed int64) {
		progressUpdates = append(progressUpdates, struct {
			current, total, speed int64
		}{current, total, speed})
	}

	err = downloader.DownloadFile(ctx, server.URL, filename, progressCallback)
	if err != nil {
		t.Fatalf("Failed to download large file: %v", err)
	}

	// Verify file size
	targetPath := filepath.Join(targetDir, filename)
	stat, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("Failed to stat downloaded file: %v", err)
	}

	if stat.Size() != int64(totalSize) {
		t.Errorf("Expected file size %d, got %d", totalSize, stat.Size())
	}

	// Verify progress was tracked multiple times
	if len(progressUpdates) < 2 {
		t.Errorf("Expected multiple progress updates, got %d", len(progressUpdates))
	}
}
