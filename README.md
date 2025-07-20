# Podcast TUI

A terminal-based podcast manager with vim-style keybindings, built with Go and tcell.

## Features

- Vim-style navigation
- RSS feed parsing and subscription management
- Audio playback with mpv backend
- Episode download management with progress tracking
- Persistent storage of subscriptions and playback positions
- Terminal-based UI using ncurses (via tcell)
- Organized download storage with podcast subdirectories
- Enhanced episode display with publication dates, duration, and listening progress
- Episode count display for each podcast
- Ability to restart episodes from the beginning or resume from saved position
- Real-time progress indicator when refreshing podcast feeds
- Episode description window showing details of the currently selected episode

## Requirements

- Go 1.18+
- mpv (for audio playback)

## Usage

```bash
./podcast-tui
```

## Keybindings

### Navigation
- `j` / `k` - Move down/up in lists
- `h` / `l` - Switch between podcast and episode views
- `Enter` - Select item (same as `l` when on podcast, plays episode when on episode)
- `g` - Go to top of list
- `G` - Go to bottom of list

**Episode View Layout**: When viewing episodes, the screen is split with the episode list on top and a description window at the bottom showing details of the currently selected episode.

### Playback Control
- `Space` - Play/pause current episode
- `Enter` - Play selected episode (resume from saved position)
- `R` - Restart episode from beginning (reset position to 0:00)
- `f` - Seek forward 30 seconds
- `b` - Seek backward 30 seconds
- `Left` / `Right` - Seek backward/forward 10 seconds
- `m` - Mute/unmute
- `<` / `>` - Decrease/increase playback speed
- `=` - Reset to normal speed (1.0x)
- `Up` / `Down` - Increase/decrease volume by 5%

### Episode Downloads
- `d` - Download selected episode
- `x` - Cancel download or delete downloaded episode

### Podcast Management
- `a` - Add new podcast (enters command mode)
- `x` - Delete selected podcast
- `r` - Refresh all podcast feeds (with progress indicator)

### Other
- `/` - Enter search mode
- `:` - Enter command mode
- `?` - Show help dialog
- `Esc` - Return to normal mode / close dialogs
- `q` - Quit application

### Command Mode
- `:add <feed-url>` - Add a new podcast subscription
- `:q` or `:quit` - Quit the application

## Building

```bash
go build -o podcast-tui ./cmd/podcast-tui
```

## Configuration

The application stores all configuration and data in the user's config directory at `~/.config/podcast-tui/`.

### Configuration Files

#### Subscription Data (`subscriptions.json`)
- **Path**: `~/.config/podcast-tui/subscriptions.json`
- **Content**: Podcast subscriptions, episode metadata, and playback positions
- **Format**: JSON with automatic backup and atomic writes

#### Download Configuration (`download-config.json`)
- **Path**: `~/.config/podcast-tui/download-config.json`
- **Auto-created**: Yes, with default values if not present

**Available Options:**
```json
{
  "maxSizeGB": 5,
  "maxEpisodesPerPodcast": 10,
  "autoCleanup": true,
  "cleanupDays": 30,
  "maxConcurrentDownloads": 3,
  "downloadPath": ""
}
```

**Configuration Options:**

- `maxSizeGB` (integer, default: 5)
  - Maximum total storage space for downloads in gigabytes
  - When exceeded, automatic cleanup removes oldest episodes

- `maxEpisodesPerPodcast` (integer, default: 10)
  - Maximum number of episodes to keep downloaded per podcast
  - Oldest episodes are removed when limit is exceeded

- `autoCleanup` (boolean, default: true)
  - Enable automatic cleanup of old downloaded episodes
  - Uses `cleanupDays` and storage limits to determine cleanup

- `cleanupDays` (integer, default: 30)
  - Episodes downloaded more than this many days ago become eligible for automatic cleanup

- `maxConcurrentDownloads` (integer, default: 3)
  - Maximum number of episodes that can download simultaneously
  - Higher values may improve download speed but use more bandwidth/system resources

- `downloadPath` (string, default: "")
  - Custom path for storing downloaded episodes
  - If empty, defaults to `~/Music/Podcasts`
  - Episodes are organized in subdirectories named after each podcast

#### Download Registry (`downloads/registry.json`)
- **Path**: `~/.config/podcast-tui/downloads/registry.json`
- **Content**: Download status, progress, and metadata for all episodes
- **Management**: Automatically managed by the application

### Directory Structure

```
~/.config/podcast-tui/
├── subscriptions.json         # Podcast subscriptions and episode data
├── download-config.json       # Download configuration settings
└── downloads/
    ├── registry.json         # Download status and metadata
    └── temp/                 # Temporary files during downloads

~/Music/Podcasts/              # Default download location (configurable)
├── Podcast_Name_1/           # Sanitized podcast name as directory
│   ├── Episode_Title_1.mp3   # Downloaded episodes with sanitized names
│   └── Episode_Title_2.mp3
├── Podcast_Name_2/
│   └── Another_Episode.mp3
└── ...
```

### File Organization

- **Podcast Directories**: Named after the podcast title with invalid filesystem characters replaced by underscores
- **Episode Files**: Named after the episode title (sanitized) with `.mp3` extension
- **Character Sanitization**: Replaces `/\:*?"<>|` with underscores, limits length to prevent filesystem issues
- **Fallback Naming**: If sanitization results in empty names, uses episode/podcast IDs as fallback

### Environment Variables

Currently, the application does not support environment variable configuration. All settings are file-based in the configuration directory.

### Platform-Specific Paths

The application uses Go's standard user directory functions:
- **Linux/Unix**: `~/.config/podcast-tui/`
- **macOS**: `~/Library/Application Support/podcast-tui/`
- **Windows**: `%APPDATA%\podcast-tui\`

Downloads default to:
- **All Platforms**: `~/Music/Podcasts/` (customizable via `downloadPath`)

## Project Structure

```
podcast-tui/
├── cmd/
│   └── podcast-tui/     # Main application entry point
├── internal/
│   ├── ui/              # UI components and views (help dialogs, confirmation dialogs)
│   ├── models/          # Data structures and subscription management
│   ├── player/          # Audio playback with mpv backend
│   ├── feed/            # RSS feed parsing
│   └── download/        # Episode download management with progress tracking
└── go.mod
```