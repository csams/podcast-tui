# Podcast TUI

A terminal-based podcast manager with vim-style keybindings, built with Go and tcell.

## Features

- Vim-style navigation with familiar keybindings
- RSS feed parsing and subscription management
- Audio playback with mpv backend
- Episode download management with progress tracking
- Persistent storage of subscriptions and playback positions
- Terminal-based UI using tcell library
- Organized download storage with podcast subdirectories
- Enhanced episode display with publication dates, duration, and listening progress
- Episode count display for each podcast
- Ability to restart episodes from the beginning or resume from saved position
- Real-time progress indicator when refreshing podcast feeds
- Episode description window showing details of the currently selected episode
- Markdown/HTML to terminal text conversion for better description readability
- Fuzzy search with highlighting for both podcasts and episodes
- Real-time playback position updates in the episode list
- Automatic position saving and resume functionality
- Episode queue management with auto-advance playback
- Queue view showing podcast titles and playback controls
- Flexible navigation between podcast, episode, and queue views

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
- `Ctrl+F` / `Ctrl+B` - Page down/up in lists
- `h` / `l` - Switch between podcast and episode views
- `p` - Go to podcast view (from episode/queue view)
- `e` - Go to episode view (from podcast/queue view)
- `q` - Go to queue view (from podcast/episode view)
- `Tab` - Toggle queue view / return to previous view
- `Enter` - Select item (same as `l`; adds to queue in episode view)
- `g` - Go to top of list
- `G` - Go to bottom of list
- `Alt+j` / `Alt+k` - Scroll down/up in the description window (episode view)

**Episode View Layout**: When viewing episodes, the screen is split with the episode list on top and a description window at the bottom showing details of the currently selected episode. The description window automatically converts markdown/HTML to readable terminal text.

**Episode Status Indicators**:
- `Q:1`, `Q:2` - Queue position
- `▶` - Currently playing episode (highlighted with green text)
- `⏸` - Currently paused episode (highlighted with yellow text)
- `✔` - Downloaded episode
- `[⬇50%]` - Downloading (with progress percentage)
- `[⏸]` - Download queued
- `[⚠]` - Download failed
- Position format: `15:30/45:00` (current position/total duration)

### Playback Control
- `Space` - Play/pause current episode
- `s` - Stop playback
- `R` - Restart episode from beginning (reset position to 0:00)
- `f` - Seek forward 30 seconds
- `b` - Seek backward 30 seconds
- `Left` / `Right` - Seek backward/forward 10 seconds
- `0`-`9` - Seek to 0%-90% of episode duration
- `m` - Mute/unmute
- `<` / `>` - Decrease/increase playback speed
- `=` - Reset to normal speed (1.0x)
- `Up` / `Down` - Increase/decrease volume by 5%

**Note**: Playback positions are automatically saved and updated in real-time. When you play an episode, it will resume from where you left off.

### Episode Downloads
- `d` - Download selected episode
- `x` - Cancel download or delete downloaded episode

### Podcast Management
- `a` - Add new podcast (enters command mode)
- `x` - Delete selected podcast
- `r` - Refresh feeds (all feeds in podcast list, current podcast in episode list)

### Search
- `/` - Enter search mode (fuzzy search with highlighting)
- `Ctrl+T` - Toggle search quality filter (Normal/Strict/Permissive/All)
- `Enter` / `Esc` - Exit search mode (filter stays active)
- Empty search clears filter and shows all items

**Search Mode Editing** (Emacs-style keybindings):
- `Ctrl+A` / `Home` - Move cursor to beginning
- `Ctrl+E` / `End` - Move cursor to end
- `Ctrl+F` / `Right` - Move cursor forward one character
- `Ctrl+B` / `Left` - Move cursor backward one character
- `Alt+F` - Move cursor forward one word
- `Alt+B` - Move cursor backward one word
- `Ctrl+K` - Delete from cursor to end
- `Ctrl+U` - Delete entire search query
- `Ctrl+W` - Delete word before cursor
- `Alt+D` - Delete word after cursor
- `Ctrl+D` / `Delete` - Delete character at cursor

### Queue Management
- `Enter` / `l` - Add episode to queue (from episode list)
- `u` - Remove episode from queue (from episode/queue view)
- `Enter` - Play episode immediately (from queue view)
- `g` - Go to episode in episode list (from queue view)
- `Alt+j` - Move episode down in queue (from queue view)
- `Alt+k` - Move episode up in queue (from queue view)
- `R` - Restart episode from beginning (from queue view)
- `0`-`9` - Seek to 0%-90% of episode (from queue view)

**Note**: First episode added to empty queue starts playing automatically. Episodes play sequentially; completed episodes are removed from queue. Auto-advances to next episode when one completes.

### Other
- `:` - Enter command mode
- `?` - Show help dialog
- `Esc` - Return to normal mode / close dialogs
- `Q` - Quit application (uppercase Q required)

### Command Mode
- `:add <feed-url>` - Add a new podcast subscription
- `:q` - Go to queue view (from podcast/episode view)
- `:Q` or `:quit` - Quit the application

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
│   ├── download/        # Episode download management with progress tracking
│   └── markdown/        # Markdown/HTML to terminal text conversion
└── go.mod
```