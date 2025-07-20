# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Building the application
```bash
go build -o podcast-tui ./cmd/podcast-tui
```

### Running the application
```bash
./podcast-tui
```

### Testing
```bash
go test ./...
```

### Code formatting
```bash
go fmt ./...
```

### Code linting/vetting
```bash
go vet ./...
```

### Module management
```bash
go mod tidy    # Clean up dependencies
go mod vendor  # Vendor dependencies (if needed)
```

## Architecture Overview

This is a terminal-based podcast manager built with Go using the tcell library for the TUI. The application follows a clean modular architecture:

### Core Components

- **UI Layer** (`internal/ui/`): Terminal user interface using tcell
  - `app.go`: Main application controller with event handling and view management
  - `podcast_list.go`: Podcast subscription list view
  - `episode_list.go`: Episode list view for selected podcasts
  - Implements vim-style keybindings (j/k navigation, h/l view switching)

- **Models** (`internal/models/`): Data structures and persistence
  - `podcast.go`: Podcast and episode data structures
  - `subscription.go`: Subscription management with JSON persistence to `~/.config/podcast-tui/subscriptions.json`

- **Player** (`internal/player/`): Audio playback using mpv backend
  - `player.go`: Wraps mpv with IPC socket communication for playback control
  - Supports play/pause, seeking, volume control, speed adjustment, mute
  - Automatic position saving and resume functionality

- **Feed Parser** (`internal/feed/`): RSS feed handling
  - `parser.go`: RSS feed parsing for podcast subscriptions

### Key Design Patterns

- **MVC Architecture**: Clear separation between UI (views), models (data), and controller (app)
- **Event-driven UI**: Goroutine-based event handling for responsive terminal interface
- **IPC Communication**: Uses Unix socket to communicate with mpv player process
- **State Management**: Persistent storage of subscriptions and playback positions

### External Dependencies

- **tcell**: Terminal UI framework for cross-platform terminal handling
- **mpv**: External media player process controlled via IPC socket
- Standard Go libraries for JSON, HTTP, XML parsing

### Application Modes

- **Normal Mode**: Default navigation and playback control
- **Command Mode**: Triggered by `:` for adding podcasts (`:add <url>`) 
- **Search Mode**: Triggered by `/` (implementation varies by view)
- **Help Dialog**: Triggered by `?` to show comprehensive keybinding reference

The application maintains persistent state including subscription list and episode playback positions, automatically resuming from the last played position when episodes are restarted.