# Episode Download Requirements Specification

## Problem Statement and Solution Overview

The podcast-tui application currently operates as a streaming-only podcast player, requiring network connectivity for all episode playback. Users need the ability to download episodes locally for offline listening, manage storage usage, and seamlessly switch between streaming and local playback based on availability.

This feature will add a comprehensive episode download system with background downloading, storage management, detailed progress tracking, and automatic retry capabilities. The implementation will maintain the existing clean architecture while adding a dedicated download management layer that operates independently of the subscription system.

## Functional Requirements

### 1. Manual Episode Downloads

- **Download Trigger**: Press `D` on selected episode to start download
- **Background Processing**: Downloads continue while user navigates the application
- **Concurrent Downloads**: Support up to 3 simultaneous downloads for optimal bandwidth usage
- **Download Queue**: Episodes queue automatically when max concurrent downloads reached
- **Cancel Support**: Press `x` on downloading episode to cancel and remove partial files

### 2. Download Progress and Status Display

- **Episode List Indicators**:
  - `[D]`: Episode fully downloaded and available offline
  - `[⬇ 45%]`: Episode currently downloading with percentage
  - `[⚠]`: Download failed (retry available)
  - `[⏸]`: Download paused or queued

- **Detailed Progress View**: Press `v` to open download manager showing:
  - Episode title and podcast name
  - Download speed (KB/s, MB/s)
  - Time remaining (ETA)
  - Bytes transferred / Total size
  - Progress bar visualization
  - Current status (downloading, queued, failed, completed)

- **Status Bar Integration**:
  - Active downloads: `"⬇ 2 active downloads (15.2 MB/s)"`
  - Queue status: `"⬇ 3 downloading, 2 queued"`
  - Storage warning: `"⚠ Storage 90% full"`

### 3. Automatic Retry and Error Handling

- **Retry Strategy**: Exponential backoff (1s, 2s, 4s, 8s, 16s max)
- **Max Retries**: 5 attempts per episode
- **Retry Triggers**: Network timeouts, HTTP 5xx errors, connection failures
- **Manual Retry**: Press `r` on failed episodes to restart download
- **Failure Persistence**: Failed downloads survive app restarts for manual retry

### 4. Offline Playback Integration

- **Player Priority**: Automatically prefer local files over streaming when available
- **Seamless Fallback**: If local file is corrupted/missing, automatically stream from URL
- **Position Sync**: Playback positions sync between local and streamed versions
- **File Validation**: Verify file integrity before playback (basic size/format checks)

### 5. Storage Management and Limits

- **Storage Configuration**:
  - Maximum total storage (default: 5GB)
  - Maximum episodes per podcast (default: 10)
  - Auto-cleanup enabled/disabled (default: enabled)
  - Cleanup threshold (default: 30 days unused)

- **Storage Monitoring**:
  - Real-time storage usage display
  - Warning at 90% capacity
  - Auto-cleanup when approaching limits
  - Manual cleanup: Press `c` in download manager

- **Cleanup Strategies**:
  - Least Recently Played (LRP) algorithm
  - Age-based removal (configurable days)
  - Manual deletion with confirmation
  - Respect user-favorited episodes (future enhancement)

## Technical Requirements

### 1. New Module Structure

**Create `internal/download/` package with:**

- `manager.go`: Core download orchestration and queue management
- `config.go`: Storage limits, paths, and cleanup configuration  
- `progress.go`: Progress tracking and UI update coordination
- `registry.go`: Download state persistence (separate from subscriptions)
- `cleanup.go`: Storage management and file cleanup logic

### 2. Data Model Extensions

**Extend Episode model in `internal/models/podcast.go`:**

```go
type Episode struct {
    // Existing fields...
    ID           string    `json:"id"`                    // SHA-256 hash generation needed
    Downloaded   bool      `json:"downloaded,omitempty"`  // Local availability flag
    DownloadPath string    `json:"downloadPath,omitempty"` // Absolute file path
    DownloadSize int64     `json:"downloadSize,omitempty"` // File size in bytes
    DownloadDate time.Time `json:"downloadDate,omitempty"` // When download completed
    LastPlayed   time.Time `json:"lastPlayed,omitempty"`   // For cleanup LRP algorithm
}
```

**Episode ID Generation (critical missing functionality):**
```go
func GenerateEpisodeID(podcastURL, episodeURL string, publishDate time.Time) string {
    h := sha256.New()
    h.Write([]byte(podcastURL + episodeURL + publishDate.Format(time.RFC3339)))
    return fmt.Sprintf("%x", h.Sum(nil))[:16] // First 16 chars for filename safety
}
```

### 3. Download Registry System (Separate from Subscriptions)

**Location**: `~/.config/podcast-tui/downloads/registry.json`

**Structure**:
```json
{
  "downloads": {
    "episode-id-1": {
      "status": "completed|downloading|queued|failed",
      "progress": 0.75,
      "speed": 1024000,
      "bytesDownloaded": 15728640,
      "totalBytes": 20971520,
      "retryCount": 0,
      "lastError": "",
      "startTime": "2025-07-19T08:30:00Z",
      "estimatedTime": 120
    }
  },
  "config": {
    "maxSizeGB": 5,
    "maxEpisodesPerPodcast": 10,
    "autoCleanup": true,
    "cleanupDays": 30,
    "maxConcurrentDownloads": 3
  }
}
```

### 4. File Storage Structure

**Download Directory Layout**:
```
~/.config/podcast-tui/downloads/
├── registry.json                    # Download tracking
├── config.json                      # Storage settings
├── [podcast-hash]/                  # One dir per podcast
│   ├── [episode-id].mp3            # Audio file
│   ├── [episode-id].json           # Episode metadata cache
│   └── [episode-id].progress       # Resume data for partial downloads
└── temp/                           # Temporary download files
    └── [episode-id].tmp            # In-progress downloads
```

### 5. Download Manager Implementation

**Core Manager Interface**:
```go
type DownloadManager struct {
    registry     *Registry
    config       *Config
    queue        chan *DownloadTask
    active       map[string]*Download
    progressCh   chan ProgressUpdate
    stopCh       chan struct{}
    maxConcurrent int
}

type DownloadTask struct {
    Episode     *models.Episode
    PodcastHash string
    Priority    int
}

type ProgressUpdate struct {
    EpisodeID   string
    Progress    float64
    Speed       int64
    ETA         time.Duration
    Status      DownloadStatus
}
```

**Background Worker Pattern**:
```go
func (dm *DownloadManager) Start() {
    for i := 0; i < dm.maxConcurrent; i++ {
        go dm.downloadWorker()
    }
    go dm.progressReporter()
}

func (dm *DownloadManager) downloadWorker() {
    for task := range dm.queue {
        dm.processDownload(task)
    }
}
```

### 6. HTTP Download Implementation

**Resumable Download with Progress**:
```go
type ProgressReader struct {
    io.Reader
    total    int64
    current  int64
    callback func(current, total int64, speed int64)
    lastTime time.Time
}

func (dm *DownloadManager) downloadEpisode(episode *models.Episode) error {
    // Support HTTP Range requests for resume
    req, _ := http.NewRequest("GET", episode.URL, nil)
    if resumeBytes > 0 {
        req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeBytes))
    }
    
    // Stream to temporary file, then atomic move
    tempPath := filepath.Join(dm.tempDir, episode.ID+".tmp")
    finalPath := dm.getEpisodePath(episode)
    
    // Progress tracking with speed calculation
    reader := &ProgressReader{
        Reader:   resp.Body,
        total:    resp.ContentLength,
        callback: dm.updateProgress,
    }
    
    // Atomic completion
    if err := os.Rename(tempPath, finalPath); err == nil {
        episode.Downloaded = true
        episode.DownloadPath = finalPath
        episode.DownloadDate = time.Now()
    }
}
```

### 7. UI Integration Points

**Extend `internal/ui/app.go` HandleKey method**:
```go
case tcell.KeyRune:
    switch ev.Rune() {
    case 'D':
        if a.currentMode == ModeNormal {
            go a.downloadSelectedEpisode()
        }
    case 'v':
        a.showDownloadManager()
    case 'x':
        if a.currentMode == ModeDownloads {
            go a.cancelSelectedDownload()
        }
    }
```

**Download Manager View (`internal/ui/downloads.go`)**:
- Table view with episode title, podcast, progress, speed, ETA
- Sortable by status, progress, time remaining
- Keybindings: `j/k` navigate, `x` cancel, `r` retry, `c` cleanup, `q` close

### 8. Player Integration

**Modify `internal/player/player.go`**:
```go
func (p *Player) Play(episode *models.Episode) error {
    // Priority: local file > streaming
    if episode.Downloaded && fileExists(episode.DownloadPath) {
        if err := p.validateLocalFile(episode.DownloadPath); err == nil {
            log.Printf("Playing local: %s", episode.DownloadPath)
            return p.playFile(episode.DownloadPath)
        }
        log.Printf("Local file invalid, streaming: %v", err)
    }
    
    log.Printf("Streaming: %s", episode.URL)
    return p.playURL(episode.URL)
}

func (p *Player) validateLocalFile(path string) error {
    info, err := os.Stat(path)
    if err != nil {
        return err
    }
    if info.Size() < 1024 { // Minimum reasonable file size
        return fmt.Errorf("file too small: %d bytes", info.Size())
    }
    return nil
}
```

## Implementation Hints and Patterns

### 1. Download Queue Management
```go
func (dm *DownloadManager) QueueDownload(episode *models.Episode) error {
    if dm.isDownloaded(episode.ID) {
        return fmt.Errorf("episode already downloaded")
    }
    
    task := &DownloadTask{
        Episode:     episode,
        PodcastHash: dm.getPodcastHash(episode),
        Priority:    0, // Normal priority
    }
    
    select {
    case dm.queue <- task:
        dm.registry.SetStatus(episode.ID, StatusQueued)
        return nil
    default:
        return fmt.Errorf("download queue full")
    }
}
```

### 2. Storage Cleanup Algorithm
```go
func (dm *DownloadManager) cleanupStorage() error {
    usage := dm.calculateStorageUsage()
    if usage < dm.config.MaxSizeGB*1024*1024*1024 {
        return nil // Under limit
    }
    
    // Get candidates sorted by LRP (Least Recently Played)
    candidates := dm.getLRPCandidates()
    
    for _, episode := range candidates {
        if err := dm.deleteEpisode(episode.ID); err != nil {
            continue
        }
        usage -= episode.DownloadSize
        if usage < dm.config.MaxSizeGB*1024*1024*1024*0.8 { // 80% threshold
            break
        }
    }
    return nil
}
```

### 3. Progress Update Pattern
```go
func (dm *DownloadManager) updateProgress(episodeID string, current, total, speed int64) {
    update := ProgressUpdate{
        EpisodeID: episodeID,
        Progress:  float64(current) / float64(total),
        Speed:     speed,
        ETA:       time.Duration((total-current)/speed) * time.Second,
        Status:    StatusDownloading,
    }
    
    select {
    case dm.progressCh <- update:
    default:
        // Don't block if UI isn't consuming updates
    }
}
```

### 4. Error Handling with Exponential Backoff
```go
func (dm *DownloadManager) retryDownload(episode *models.Episode, retryCount int) {
    if retryCount >= 5 {
        dm.registry.SetStatus(episode.ID, StatusFailed)
        return
    }
    
    delay := time.Duration(math.Pow(2, float64(retryCount))) * time.Second
    if delay > 16*time.Second {
        delay = 16 * time.Second
    }
    
    time.Sleep(delay)
    go dm.processDownload(&DownloadTask{Episode: episode})
}
```

## Acceptance Criteria

1. ✓ Episodes can be downloaded with `D` key in episode list
2. ✓ Downloads continue in background while using other app features
3. ✓ Up to 3 concurrent downloads with automatic queuing
4. ✓ Download progress shown with percentage, speed, and ETA
5. ✓ Failed downloads retry automatically with exponential backoff
6. ✓ Downloaded episodes play locally, fallback to streaming if needed
7. ✓ Storage limits enforced with automatic cleanup
8. ✓ Download manager view accessible with `v` key
9. ✓ Downloads can be cancelled with `x` key
10. ✓ Download state persists across app restarts
11. ✓ Manual retry available for failed downloads
12. ✓ Storage usage displayed with warnings at 90% capacity
13. ✓ Downloaded episodes preserve playback positions
14. ✓ Episode list shows clear download status indicators
15. ✓ No regression in existing streaming and playback functionality

## Assumptions

1. **Network Conditions**: Users may have intermittent connectivity requiring robust retry logic
2. **Storage Constraints**: Users on limited storage devices need proactive space management  
3. **Audio Formats**: Standard podcast formats (MP3, M4A) supported by mpv player
4. **File Permissions**: Download directory writable with standard user permissions
5. **Performance**: Progress updates every 1-2 seconds acceptable for UI responsiveness
6. **Episode Persistence**: RSS feeds may remove old episodes but downloads should persist
7. **Platform Support**: Works on Linux, macOS, Windows where mpv and Go are supported
8. **Configuration**: Default settings work for most users with customization available

## Migration and Compatibility

- **Existing Data**: No changes to subscription.json format ensures backward compatibility
- **New Installations**: Download functionality optional, app works without any downloads
- **Config Migration**: New download config created with sensible defaults on first use
- **File Cleanup**: Orphaned downloads handled gracefully if episode metadata changes