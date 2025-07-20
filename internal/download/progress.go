package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ProgressReader wraps an io.Reader to track download progress
type ProgressReader struct {
	reader    io.Reader
	total     int64
	current   int64
	callback  func(current, total, speed int64)
	lastTime  time.Time
	lastBytes int64
}

// NewProgressReader creates a new progress tracking reader
func NewProgressReader(reader io.Reader, total int64, callback func(current, total, speed int64)) *ProgressReader {
	return &ProgressReader{
		reader:    reader,
		total:     total,
		callback:  callback,
		lastTime:  time.Now(),
		lastBytes: 0,
	}
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)

	// Calculate speed and call callback every second
	now := time.Now()
	if now.Sub(pr.lastTime) >= time.Second {
		elapsed := now.Sub(pr.lastTime)
		bytesDiff := pr.current - pr.lastBytes
		speed := int64(float64(bytesDiff) / elapsed.Seconds())

		if pr.callback != nil {
			pr.callback(pr.current, pr.total, speed)
		}

		pr.lastTime = now
		pr.lastBytes = pr.current
	}

	return n, err
}

// Downloader handles HTTP downloads with progress tracking
type Downloader struct {
	client     *http.Client
	tempDir    string
	targetDir  string
	userAgent  string
	maxRetries int
}

// NewDownloader creates a new downloader
func NewDownloader(tempDir, targetDir string) *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 30 * time.Minute, // Long timeout for large files
		},
		tempDir:    tempDir,
		targetDir:  targetDir,
		userAgent:  "podcast-tui/1.0",
		maxRetries: 5,
	}
}

// DownloadFile downloads a file with progress tracking and resume support
func (d *Downloader) DownloadFile(ctx context.Context, url, filename string, progressCallback func(current, total, speed int64)) error {
	tempPath := filepath.Join(d.tempDir, filename+".tmp")
	targetPath := filepath.Join(d.targetDir, filename)

	// Check if file already exists
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("file already exists: %s", targetPath)
	}

	// Check for partial download
	var resumeBytes int64 = 0
	if stat, err := os.Stat(tempPath); err == nil {
		resumeBytes = stat.Size()
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", d.userAgent)
	if resumeBytes > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeBytes))
	}

	// Execute request
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Get total size
	var totalSize int64
	if resumeBytes > 0 && resp.StatusCode == http.StatusPartialContent {
		// Parse Content-Range header for partial content
		contentRange := resp.Header.Get("Content-Range")
		if contentRange != "" {
			// Format: "bytes 200-1023/1024"
			var start, end, total int64
			if n, err := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &total); n == 3 && err == nil {
				totalSize = total
			}
		}
	} else {
		// Full download
		resumeBytes = 0 // Reset resume bytes for full download
		if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
			if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
				totalSize = size
			}
		}
	}

	// Open/create temp file
	var file *os.File
	if resumeBytes > 0 {
		file, err = os.OpenFile(tempPath, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		file, err = os.Create(tempPath)
	}
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	// Create progress reader
	progressReader := NewProgressReader(resp.Body, totalSize, func(current, total, speed int64) {
		// Add resume bytes to current for accurate progress
		adjustedCurrent := resumeBytes + current
		if progressCallback != nil {
			progressCallback(adjustedCurrent, total, speed)
		}
	})

	// Copy data
	_, err = io.Copy(file, progressReader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Ensure directory exists for target
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Move temp file to final location (atomic operation)
	if err := os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("failed to move file to final location: %w", err)
	}

	return nil
}

// GetFileSize returns the size of a remote file without downloading it
func (d *Downloader) GetFileSize(ctx context.Context, url string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, fmt.Errorf("content-length header not found")
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid content-length: %w", err)
	}

	return size, nil
}

// CleanupTempFile removes a temporary download file
func (d *Downloader) CleanupTempFile(filename string) error {
	tempPath := filepath.Join(d.tempDir, filename+".tmp")
	if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cleanup temp file: %w", err)
	}
	return nil
}
