# Deep Context Analysis for Episode Download Feature

## Overview
This document analyzes the existing codebase patterns and external research to inform the implementation of episode download functionality. The analysis covers six key areas identified in the discovery phase.

## 1. File Storage Patterns

### Current Implementation Analysis

**Directory Structure:**
- Uses `os.UserConfigDir()` to get platform-appropriate config directory
- Creates `~/.config/podcast-tui/` directory structure  
- Pattern: `filepath.Join(configDir, "podcast-tui", "subscriptions.json")`
- Uses `os.MkdirAll(dir, 0755)` for directory creation with proper permissions

**File Operations:**
- JSON persistence pattern in `/home/csams/projects/personal/podcast-tui/internal/models/subscription.go`
- Error handling: checks `os.IsNotExist(err)` for graceful missing file handling
- File permissions: `0644` for data files, `0755` for directories
- Atomic writes using `os.WriteFile()`

**Storage Patterns to Follow:**
```go
configDir, err := os.UserConfigDir()
dir := filepath.Join(configDir, "podcast-tui")
if err := os.MkdirAll(dir, 0755); err != nil {
    return err
}
```

**Proposed Download Storage Structure:**
```
~/.config/podcast-tui/
├── subscriptions.json
├── downloads/
│   ├── [podcast-id]/
│   │   ├── [episode-id].mp3
│   │   └── [episode-id].json (metadata)
│   └── downloads.json (download registry)
```

## 2. HTTP Download Implementation Research

### Go Standard Library Capabilities

**Core HTTP Features:**
- `net/http` package supports Range headers for resumable downloads
- Built-in support for partial content via `http.ServeContent()` 
- Range requests return HTTP 206 Partial Content status
- Client can specify byte ranges: `Range: bytes=100-199`

**Progress Tracking Pattern:**
```go
type ProgressReader struct {
    Reader io.Reader
    Size   int64
    Pos    int64
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
    n, err := pr.Reader.Read(p)
    if err == nil {
        pr.Pos += int64(n)
        // Report progress: float64(pr.Pos)/float64(pr.Size)*100
    }
    return n, err
}
```

**Resume Download Implementation:**
1. Check if partial file exists
2. Get file size and set Range header: `Range: bytes=[resumePos]-`
3. Append to existing file instead of overwriting
4. Stream directly to disk using `io.Copy()`

**Timeout and Error Handling:**
- Use `http.Client` with custom timeouts
- Handle network interruptions gracefully
- Implement retry logic with exponential backoff

## 3. Background Task Patterns

### Current Async Patterns in Codebase

**Goroutine Usage Analysis:**
- Heavy use of anonymous goroutines for non-blocking operations
- Pattern: `go func() { ... }()` for player operations, feed updates
- Error handling via status messages: `a.statusMessage = "Error: " + err.Error()`
- Long-running tasks in `/home/csams/projects/personal/podcast-tui/internal/ui/app.go`:
  - `go a.handleEvents()` - event loop
  - `go a.handleProgress()` - player progress tracking
  - `go a.refreshFeeds()` - RSS feed updates

**Progress Reporting Mechanism:**
- Player uses `Progress` channel: `chan Progress`
- Status updates via `a.statusMessage` field
- UI redraws triggered by goroutines calling `a.draw()`

**Cancellation Pattern:**
- Uses `chan struct{}` for stop signals (`a.quit`, `p.stopCh`)
- Graceful shutdown with `close(a.quit)`

**Proposed Download Manager Pattern:**
```go
type DownloadManager struct {
    downloads map[string]*Download
    statusCh  chan DownloadStatus
    mu        sync.RWMutex
}

type Download struct {
    Episode   *models.Episode
    Progress  int64
    Size      int64
    Status    DownloadStatus
    CancelCh  chan struct{}
}
```

## 4. Data Model Extensions

### Current Episode Model Analysis

**Existing Fields:**
```go
type Episode struct {
    ID          string        // Currently empty in feed parser
    Title       string
    Description string
    URL         string
    Duration    string
    PublishDate time.Time
    Played      bool
    Position    time.Duration
}
```

**Required Extensions for Downloads:**
```go
type Episode struct {
    // Existing fields...
    
    // Download-related fields
    Downloaded     bool           `json:"downloaded"`
    DownloadPath   string         `json:"download_path,omitempty"`
    DownloadSize   int64          `json:"download_size,omitempty"`
    DownloadedAt   time.Time      `json:"downloaded_at,omitempty"`
    DownloadStatus DownloadStatus `json:"download_status,omitempty"`
}

type DownloadStatus int
const (
    DownloadStatusNone DownloadStatus = iota
    DownloadStatusQueued
    DownloadStatusInProgress
    DownloadStatusCompleted
    DownloadStatusFailed
    DownloadStatusPaused
)
```

**ID Generation Gap:**
- Feed parser currently doesn't set Episode.ID
- Need consistent ID generation for file naming
- Proposed: hash of URL or use episode GUID from RSS

**Backward Compatibility:**
- JSON marshaling will handle new fields gracefully
- Use `omitempty` tags for optional fields
- Existing subscriptions.json will load without downloads data

## 5. UI Integration Points

### Current Keybinding Analysis

**Available Keys in Normal Mode:**
From `/home/csams/projects/personal/podcast-tui/internal/ui/app.go` and help dialog:
- Navigation: `j/k`, `h/l`, `g/G`, Enter
- Playback: Space, `f/b`, `</>/=`, `m`, Arrow keys
- Management: `a`, `d`, `r`
- Modes: `/`, `:`, `?`, Esc
- Quit: `q`

**Available Keys for Download Feature:**
- `D` (uppercase) - Download episode
- `x` - Cancel download  
- `v` - View downloads
- `c` - Clear completed downloads

**Status Display Integration:**
- Current pattern: `a.statusMessage = "message"`
- Download progress: "Downloading: [Episode] 45% (2.1MB/4.7MB)"
- Multiple downloads: "Downloads: 3 active, 1 queued"

**Episode List View Extensions:**
- Download status indicators: `[D]` downloaded, `[⬇]` downloading, `[⚠]` failed
- Progress bar for active downloads
- File size display alongside duration

**Help Dialog Extensions:**
Need to add to help text in `/home/csams/projects/personal/podcast-tui/internal/ui/help_dialog.go`:
```
"Downloads:",
"  D             Download selected episode",
"  x             Cancel active download", 
"  v             View download status",
"  c             Clear completed downloads",
```

## 6. Storage Management Research

### Directory Size Calculation Patterns

**Standard Approach:**
```go
func calculateDirSize(dirPath string) (int64, error) {
    var size int64
    err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() {
            size += info.Size()
        }
        return nil
    })
    return size, err
}
```

**Concurrent Traversal for Performance:**
- Use worker goroutines for large directory trees
- Channel-based size accumulation
- Context cancellation support

**Storage Limits Configuration:**
```go
type StorageConfig struct {
    MaxStorageBytes    int64  // Total storage limit
    MaxEpisodesPerPod  int    // Per-podcast episode limit  
    AutoCleanupEnabled bool   // Enable automatic cleanup
    KeepPlayedDays     int    // Days to keep played episodes
}
```

**Cleanup Strategies:**
1. LRU (Least Recently Used) - remove oldest unplayed episodes
2. Played episodes older than N days
3. Size-based cleanup when approaching limit
4. Manual cleanup via UI command

## Implementation Gaps and Required Components

### Missing Infrastructure:

1. **Episode ID Generation System**
   - Need deterministic ID generation in feed parser
   - Consistent across RSS updates

2. **Download Queue Management**
   - Thread-safe queue implementation
   - Priority handling (if needed later)
   - Persistence across app restarts

3. **Configuration System**
   - Storage limits and paths
   - Download behavior settings
   - File format preferences

4. **File Format Detection**
   - Extract file extension from Content-Type headers
   - Handle various audio formats (mp3, m4a, ogg, etc.)

5. **Download Registry**
   - Track all downloads (active, completed, failed)
   - Separate from episode data for performance
   - Enable bulk operations

### Integration Points:

1. **Player Integration**
   - Modify player to prefer local files when available
   - Fallback to streaming if local file corrupted/missing

2. **Feed Refresh Integration**  
   - Update download registry when episodes removed from feed
   - Handle URL changes for episodes

3. **UI State Management**
   - Download progress in status bar
   - Episode list visual indicators
   - Download manager view

This analysis provides the foundation for implementing a robust, well-integrated episode download feature that follows the existing codebase patterns and Go best practices.