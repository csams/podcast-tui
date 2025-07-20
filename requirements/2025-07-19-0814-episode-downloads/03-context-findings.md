# Context Findings

## File Storage Patterns

**Current Pattern:**
- Location: `~/.config/podcast-tui/` via `os.UserConfigDir()`
- Permissions: `0755` for directories, `0644` for files
- JSON persistence with atomic writes and pretty formatting
- Graceful fallback on missing files

**Proposed Download Structure:**
```
~/.config/podcast-tui/
├── subscriptions.json
└── downloads/
    ├── [podcast-hash]/
    │   ├── [episode-hash].mp3
    │   └── [episode-hash].json (metadata)
    └── registry.json (download tracking)
```

## HTTP Download Implementation

**Go Standard Library Capabilities:**
- `http.Client` with custom timeouts and context cancellation
- Range header support for resumable downloads
- Stream-to-disk patterns for memory efficiency
- Progress tracking via custom `io.Reader` wrapper

**Example Pattern:**
```go
type ProgressReader struct {
    io.Reader
    Total    int64
    Current  int64
    Progress func(current, total int64)
}
```

## Background Task Patterns

**Current App Patterns:**
- Async operations: `go func() { ... }()` throughout `internal/ui/app.go`
- Error reporting: `a.statusMessage` field for user feedback
- Cancellation: `chan struct{}` for graceful shutdown
- UI updates: Direct field assignment with screen refresh

**Download Manager Pattern:**
```go
type DownloadManager struct {
    queue    chan DownloadTask
    active   map[string]*Download
    stop     chan struct{}
    progress chan DownloadProgress
}
```

## Data Model Extensions

**Current Episode Model Gaps:**
- `Episode.ID` field exists but is never populated in `internal/feed/parser.go:109`
- No download-related fields in current structure
- JSON persistence supports new fields via `omitempty` tags

**Required Extensions:**
```go
type Episode struct {
    // Existing fields...
    ID           string `json:"id"`           // Needs generation
    Downloaded   bool   `json:"downloaded,omitempty"`
    DownloadPath string `json:"downloadPath,omitempty"`
    DownloadSize int64  `json:"downloadSize,omitempty"`
    DownloadDate time.Time `json:"downloadDate,omitempty"`
}
```

**ID Generation Strategy:**
- Use SHA-256 hash of `podcast.URL + episode.URL + episode.PublishDate`
- Ensures uniqueness and consistency across feed refreshes

## UI Integration Points

**Available Keybindings:**
- `D` or `d`: Download selected episode
- `x`: Delete downloaded episode  
- `c`: Cancel ongoing download
- `v`: View downloads/queue

**Status Display Integration:**
- Episode list indicators: `[D]` (downloaded), `[⬇]` (downloading), `[⚠]` (failed)
- Status bar: `"Downloading: episode.mp3 (45%)"` during downloads
- Help dialog extension needed for new commands

**Visual Integration Pattern:**
```go
// In episode_list.go
func (el *EpisodeList) formatEpisodeTitle(episode *models.Episode) string {
    status := ""
    if episode.Downloaded {
        status = "[D] "
    } else if el.app.downloads.IsDownloading(episode.ID) {
        status = "[⬇] "
    }
    return status + episode.Title
}
```

## Storage Management Requirements

**Disk Usage Calculation:**
```go
func calculateDirSize(path string) (int64, error) {
    var size int64
    err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
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

**Configuration Needed:**
```go
type DownloadConfig struct {
    MaxSizeGB      int    `json:"maxSizeGB"`      // Total storage limit
    MaxEpisodesPer int    `json:"maxEpisodesPer"` // Per podcast limit
    AutoCleanup    bool   `json:"autoCleanup"`    // Enable automatic cleanup
    CleanupDays    int    `json:"cleanupDays"`    // Days before auto-delete
    DownloadPath   string `json:"downloadPath"`   // Custom download location
}
```

## Architecture Integration

**New Modules Needed:**
- `internal/download/manager.go` - Core download orchestration
- `internal/download/config.go` - Storage management and limits
- `internal/download/progress.go` - Progress tracking and UI updates

**Existing Module Modifications:**
- `internal/models/podcast.go` - Add Episode.ID generation
- `internal/player/player.go` - Prefer local files over streaming
- `internal/ui/app.go` - Add download keybindings and status
- `internal/ui/episode_list.go` - Add download status indicators

**Player Integration:**
```go
func (p *Player) Play(episode *models.Episode) error {
    if episode.Downloaded && fileExists(episode.DownloadPath) {
        return p.playLocal(episode.DownloadPath)
    }
    return p.playStream(episode.URL)
}
```

## Critical Implementation Notes

1. **Episode ID Generation**: Must be implemented first as foundation for all download tracking
2. **Thread Safety**: Download manager must handle concurrent operations safely
3. **Persistence**: Download queue/status must survive app restarts
4. **Error Recovery**: Handle partial downloads, network failures, disk full scenarios
5. **Cleanup Integration**: Hook into existing app shutdown patterns for graceful cleanup

The codebase architecture strongly supports this feature addition with minimal disruption to existing functionality.