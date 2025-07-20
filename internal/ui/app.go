package ui

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/csams/podcast-tui/internal/download"
	"github.com/csams/podcast-tui/internal/feed"
	"github.com/csams/podcast-tui/internal/models"
	"github.com/csams/podcast-tui/internal/player"
	"github.com/gdamore/tcell/v2"
)

type App struct {
	screen          tcell.Screen
	quit            chan struct{}
	mode            Mode
	currentView     View
	podcasts        *PodcastListView
	episodes        *EpisodeListView
	player          *player.Player
	downloadManager *download.Manager
	subscriptions   *models.Subscriptions
	commandLine     string
	statusMessage   string
	currentEpisode  *models.Episode
	currentPodcast  *models.Podcast
	helpDialog      *HelpDialog
	confirmDialog   *ConfirmationDialog
	configDir       string
	shutdownOnce    sync.Once
}

type Mode int

const (
	ModeNormal Mode = iota
	ModeCommand
	ModeSearch
)

type View interface {
	Draw(s tcell.Screen)
	HandleKey(ev *tcell.EventKey) bool
}

func NewApp() *App {
	// Get config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Printf("Failed to get config directory: %v", err)
		configDir = "."
	}
	configDir = filepath.Join(configDir, "podcast-tui")

	app := &App{
		quit:          make(chan struct{}),
		mode:          ModeNormal,
		player:        player.New(),
		helpDialog:    NewHelpDialog(),
		confirmDialog: NewConfirmationDialog(),
		configDir:     configDir,
	}

	// Initialize download manager
	app.downloadManager = download.NewManager(configDir)

	return app
}

func (a *App) Run() error {
	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	a.screen = s

	if err := s.Init(); err != nil {
		return err
	}
	
	// Ensure cleanup happens in the correct order
	defer func() {
		// First stop the event loop by closing quit channel if not already closed
		select {
		case <-a.quit:
			// Already closed
		default:
			close(a.quit)
		}
		
		// Perform shutdown
		a.shutdown()
		
		// Finally clean up the screen
		s.Fini()
		
		// Handle any panic
		if r := recover(); r != nil {
			log.Printf("Panic during shutdown: %v", r)
		}
	}()

	s.SetStyle(tcell.StyleDefault.Background(ColorBg).Foreground(ColorFg))
	s.Clear()

	// Load subscriptions
	subs, err := models.LoadSubscriptions()
	if err != nil {
		log.Printf("Failed to load subscriptions: %v", err)
		subs = &models.Subscriptions{}
	}
	a.subscriptions = subs

	// Start download manager
	if err := a.downloadManager.Start(); err != nil {
		log.Printf("Failed to start download manager: %v", err)
	}

	// Start mpv in idle mode for instant playback
	if err := a.player.StartIdle(); err != nil {
		log.Printf("Failed to start mpv in idle mode: %v", err)
		// Not fatal - playback will still work, just slightly slower on first play
	}

	a.podcasts = NewPodcastListView()
	a.podcasts.SetSubscriptions(subs)
	a.episodes = NewEpisodeListView()
	a.episodes.SetDownloadManager(a.downloadManager) // Pass download manager to episode list
	a.currentView = a.podcasts

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received interrupt signal, shutting down...")
		// Post an interrupt event to ensure event loop exits
		if a.screen != nil {
			a.screen.PostEvent(tcell.NewEventInterrupt(nil))
		}
		close(a.quit)
	}()

	go a.handleEvents()
	go a.handleProgress()
	go a.handleDownloadProgress()
	a.draw()

	<-a.quit

	// Cleanup has already been initiated by quit handler
	log.Println("Shutdown complete")

	return nil
}

// shutdown performs all cleanup operations
func (a *App) shutdown() {
	// Prevent multiple shutdowns
	a.shutdownOnce.Do(func() {
		log.Println("Shutting down podcast-tui...")
		
		// Save current episode position one final time
		a.saveEpisodePosition()
		
		// Stop the player and ensure mpv process is terminated
		if a.player != nil {
			log.Println("Stopping player...")
			if err := a.player.Stop(); err != nil {
				log.Printf("Error stopping player: %v", err)
			}
			// Ensure complete cleanup
			a.player.Cleanup()
		}
		
		// Stop download manager
		if a.downloadManager != nil {
			log.Println("Stopping download manager...")
			a.downloadManager.Stop()
		}
	})
}

func (a *App) handleEvents() {
	for {
		select {
		case <-a.quit:
			return
		default:
			ev := a.screen.PollEvent()
			if ev == nil {
				// Screen might be finalized
				return
			}
			switch ev := ev.(type) {
			case *tcell.EventResize:
				a.screen.Sync()
				a.draw()
			case *tcell.EventKey:
				if a.handleKey(ev) {
					a.draw()
				}
			case *tcell.EventInterrupt:
				// Exit the event loop
				return
			}
		}
	}
}

func (a *App) handleKey(ev *tcell.EventKey) bool {
	// Help dialog takes precedence over all other input
	if a.helpDialog.IsVisible() {
		return a.helpDialog.HandleKey(ev)
	}

	// Confirmation dialog takes precedence over normal input
	if a.confirmDialog.IsVisible() {
		return a.confirmDialog.HandleKey(ev)
	}

	if a.mode == ModeNormal {
		switch ev.Key() {
		case tcell.KeyRune:
			switch ev.Rune() {
			case 'q':
				// Post an interrupt event to ensure event loop exits
				if a.screen != nil {
					a.screen.PostEvent(tcell.NewEventInterrupt(nil))
				}
				close(a.quit)
				a.shutdown()
				return false
			case 'j':
				// Clear status message when navigating
				a.clearStatusMessage()
				return a.currentView.HandleKey(ev)
			case 'k':
				// Clear status message when navigating
				a.clearStatusMessage()
				return a.currentView.HandleKey(ev)
			case 'h':
				if a.currentView == a.episodes {
					// Clear status message when switching views
					a.clearStatusMessage()
					a.currentView = a.podcasts
					return true
				}
			case 'l':
				if a.currentView == a.podcasts {
					if selected := a.podcasts.GetSelected(); selected != nil {
						// Clear status message when switching views
						a.clearStatusMessage()
						a.episodes.SetPodcast(selected)
						a.currentView = a.episodes
						return true
					}
				} else if a.currentView == a.episodes {
					if episode := a.episodes.GetSelected(); episode != nil {
						log.Printf("User pressed 'l' - Episode: %s, Position: %v", episode.Title, episode.Position)
						// Run in goroutine to avoid blocking UI
						go a.playEpisode(episode)
						return true
					}
				}
			case 'g':
				// Clear status message when navigating
				a.clearStatusMessage()
				return a.currentView.HandleKey(ev)
			case 'G':
				// Clear status message when navigating
				a.clearStatusMessage()
				return a.currentView.HandleKey(ev)
			case ' ':
				// Space bar for play/pause ONLY (not for starting new episodes)
				playerState := a.player.GetState()
				
				if playerState != player.StateStopped {
					// Player is active, toggle pause
					go func() {
						// Save position before pausing
						if playerState == player.StatePlaying {
							a.saveEpisodePosition()
						}
						if err := a.player.TogglePause(); err != nil {
							a.statusMessage = "Playback error: " + err.Error()
						} else {
							if a.player.IsPaused() {
								a.statusMessage = "Paused"
							} else {
								a.statusMessage = "Resumed"
							}
						}
					}()
				} else {
					// No active playback - inform user to use Enter or 'l'
					a.statusMessage = "No episode playing. Press Enter or 'l' to play"
				}
				return true
			case 'f':
				// Seek forward 30 seconds
				if a.player.GetState() != player.StateStopped {
					go func() {
						if err := a.player.Seek(30); err != nil {
							a.statusMessage = "Seek error: " + err.Error()
						}
					}()
				}
				return true
			case 'b':
				// Seek backward 30 seconds
				if a.player.GetState() != player.StateStopped {
					go func() {
						if err := a.player.Seek(-30); err != nil {
							a.statusMessage = "Seek error: " + err.Error()
						}
					}()
				}
				return true
			case 'm':
				// Mute/unmute
				if a.player.GetState() != player.StateStopped {
					go func() {
						if err := a.player.ToggleMute(); err != nil {
							a.statusMessage = "Mute error: " + err.Error()
						} else {
							if a.player.IsMuted() {
								a.statusMessage = "Muted"
							} else {
								a.statusMessage = "Unmuted"
							}
						}
					}()
				}
				return true
			case '<':
				// Decrease speed
				if a.player.GetState() != player.StateStopped {
					go func() {
						currentSpeed, _ := a.player.GetSpeed()
						var newSpeed float64
						if currentSpeed > 1.0 {
							newSpeed = 1.0
						} else if currentSpeed > 0.75 {
							newSpeed = 0.75
						} else {
							newSpeed = 0.5
						}
						if err := a.player.SetSpeed(newSpeed); err != nil {
							a.statusMessage = "Speed error: " + err.Error()
						} else {
							a.statusMessage = fmt.Sprintf("Speed: %.2fx", newSpeed)
						}
					}()
				}
				return true
			case '>':
				// Increase speed
				if a.player.GetState() != player.StateStopped {
					go func() {
						currentSpeed, _ := a.player.GetSpeed()
						var newSpeed float64
						if currentSpeed < 1.0 {
							newSpeed = 1.0
						} else if currentSpeed < 1.25 {
							newSpeed = 1.25
						} else if currentSpeed < 1.5 {
							newSpeed = 1.5
						} else {
							newSpeed = 2.0
						}
						if err := a.player.SetSpeed(newSpeed); err != nil {
							a.statusMessage = "Speed error: " + err.Error()
						} else {
							a.statusMessage = fmt.Sprintf("Speed: %.2fx", newSpeed)
						}
					}()
				}
				return true
			case '=':
				// Reset to normal speed
				if a.player.IsPlaying() {
					go func() {
						if err := a.player.SetSpeed(1.0); err != nil {
							a.statusMessage = "Speed error: " + err.Error()
						}
					}()
				}
				return true
			case 'a':
				// Add podcast
				a.mode = ModeCommand
				a.commandLine = "add "
				return true
			case 'd':
				// Download selected episode
				if a.currentView == a.episodes {
					if episode := a.episodes.GetSelected(); episode != nil {
						go a.downloadEpisode(episode)
						return true
					}
				}
			case 'r':
				// Refresh feeds
				totalPodcasts := len(a.subscriptions.Podcasts)
				if totalPodcasts == 0 {
					a.statusMessage = "No podcasts to refresh"
				} else {
					a.statusMessage = fmt.Sprintf("Starting refresh of %d podcasts...", totalPodcasts)
					go a.refreshFeeds()
				}
				return true
			case 's':
				// Stop playback
				if a.player.GetState() != player.StateStopped {
					go a.stopCurrentEpisode()
				}
				return true
			case 'R':
				// Restart episode from beginning
				if a.currentView == a.episodes {
					if episode := a.episodes.GetSelected(); episode != nil {
						go a.restartEpisode(episode)
						return true
					}
				}
			case 'x':
				// Delete podcast or cancel download/delete downloaded episode
				if a.currentView == a.podcasts {
					if selected := a.podcasts.GetSelected(); selected != nil {
						a.confirmPodcastDeletion(selected)
						return true
					}
				} else if a.currentView == a.episodes {
					if episode := a.episodes.GetSelected(); episode != nil {
						a.confirmEpisodeDeletion(episode)
						return true
					}
				}
			case '/':
				a.mode = ModeSearch
				// Preserve existing search query when re-entering search mode
				if a.currentView == a.episodes {
					searchState := a.episodes.GetSearchState()
					a.commandLine = searchState.query
				} else if a.currentView == a.podcasts {
					searchState := a.podcasts.GetSearchState()
					a.commandLine = searchState.query
				} else {
					a.commandLine = ""
				}
				return true
			case ':':
				a.mode = ModeCommand
				a.commandLine = ""
				return true
			case '?':
				a.helpDialog.Show()
				return true
			}
		case tcell.KeyEnter:
			// Handle Enter key same as 'l' for playing episodes
			if a.currentView == a.podcasts {
				if selected := a.podcasts.GetSelected(); selected != nil {
					a.clearStatusMessage()
					a.episodes.SetPodcast(selected)
					a.currentView = a.episodes
					return true
				}
			} else if a.currentView == a.episodes {
				if episode := a.episodes.GetSelected(); episode != nil {
					log.Printf("User pressed Enter - Episode: %s, Position: %v", episode.Title, episode.Position)
					// Run in goroutine to avoid blocking UI
					go a.playEpisode(episode)
					return true
				}
			}
		case tcell.KeyEscape:
			a.mode = ModeNormal
			return true
		case tcell.KeyRight:
			// Seek forward 10 seconds (only when player has content)
			if a.player.GetState() != player.StateStopped {
				go func() {
					if err := a.player.Seek(10); err != nil {
						a.statusMessage = "Seek error: " + err.Error()
					}
				}()
				return true
			}
		case tcell.KeyLeft:
			// Seek backward 10 seconds (only when player has content)
			if a.player.GetState() != player.StateStopped {
				go func() {
					if err := a.player.Seek(-10); err != nil {
						a.statusMessage = "Seek error: " + err.Error()
					}
				}()
				return true
			}
		case tcell.KeyUp:
			// Increase volume by 5% (only when player has content)
			if a.player.GetState() != player.StateStopped {
				go func() {
					currentVol, _ := a.player.GetVolume()
					newVol := currentVol + 5
					if newVol > 100 {
						newVol = 100
					}
					if err := a.player.SetVolume(newVol); err != nil {
						a.statusMessage = "Volume error: " + err.Error()
					} else {
						a.statusMessage = fmt.Sprintf("Volume: %d%%", newVol)
					}
				}()
				return true
			}
		case tcell.KeyDown:
			// Decrease volume by 5% (only when player has content)
			if a.player.GetState() != player.StateStopped {
				go func() {
					currentVol, _ := a.player.GetVolume()
					newVol := currentVol - 5
					if newVol < 0 {
						newVol = 0
					}
					if err := a.player.SetVolume(newVol); err != nil {
						a.statusMessage = "Volume error: " + err.Error()
					} else {
						a.statusMessage = fmt.Sprintf("Volume: %d%%", newVol)
					}
				}()
				return true
			}
		case tcell.KeyCtrlF:
			// Page forward (vim-style)
			// Clear status message when navigating
			a.clearStatusMessage()
			if a.currentView == a.podcasts {
				return a.podcasts.HandlePageDown()
			} else if a.currentView == a.episodes {
				return a.episodes.HandlePageDown()
			}
			return false
		case tcell.KeyCtrlB:
			// Page backward (vim-style)
			// Clear status message when navigating
			a.clearStatusMessage()
			if a.currentView == a.podcasts {
				return a.podcasts.HandlePageUp()
			} else if a.currentView == a.episodes {
				return a.episodes.HandlePageUp()
			}
			return false
		}
	} else if a.mode == ModeCommand {
		switch ev.Key() {
		case tcell.KeyEscape:
			a.mode = ModeNormal
			a.commandLine = ""
			return true
		case tcell.KeyEnter:
			a.executeCommand()
			a.mode = ModeNormal
			a.commandLine = ""
			return true
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(a.commandLine) > 0 {
				a.commandLine = a.commandLine[:len(a.commandLine)-1]
				return true
			}
		case tcell.KeyRune:
			a.commandLine += string(ev.Rune())
			return true
		}
	} else if a.mode == ModeSearch {
		// Handle search mode with emacs-style editing
		var searchState *SearchState
		var updateFunc func()
		
		if a.currentView == a.episodes {
			searchState = a.episodes.GetSearchState()
			updateFunc = a.episodes.UpdateSearch
		} else if a.currentView == a.podcasts {
			searchState = a.podcasts.GetSearchState()
			updateFunc = a.podcasts.UpdateSearch
		} else {
			// No search support for this view
			a.mode = ModeNormal
			return true
		}
		
		prevQuery := searchState.query
		
		switch ev.Key() {
		case tcell.KeyEscape, tcell.KeyEnter:
			// Exit search mode but keep filter active
			a.mode = ModeNormal
			a.commandLine = ""
			return true
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			searchState.DeleteChar()
		case tcell.KeyDelete:
			searchState.DeleteCharForward()
		case tcell.KeyCtrlD:
			// Delete character under cursor (same as Delete key)
			searchState.DeleteCharForward()
		case tcell.KeyLeft:
			searchState.MoveCursorLeft()
		case tcell.KeyRight:
			searchState.MoveCursorRight()
		case tcell.KeyHome, tcell.KeyCtrlA:
			searchState.MoveCursorStart()
		case tcell.KeyEnd, tcell.KeyCtrlE:
			searchState.MoveCursorEnd()
		case tcell.KeyCtrlF:
			// Move forward one character (same as Right)
			searchState.MoveCursorRight()
		case tcell.KeyCtrlB:
			// Move backward one character (same as Left)
			searchState.MoveCursorLeft()
		case tcell.KeyCtrlK:
			searchState.DeleteToEnd()
		case tcell.KeyCtrlW:
			searchState.DeleteWord()
		case tcell.KeyCtrlU:
			// Delete to beginning of line
			searchState.MoveCursorStart()
			searchState.DeleteToEnd()
		case tcell.KeyCtrlT:
			// Toggle search quality filter
			currentScore := searchState.GetMinScore()
			var newScore int
			var message string
			switch currentScore {
			case ScoreThresholdNone:
				newScore = ScoreThresholdPermissive
				message = "Search: Permissive mode (include marginal matches)"
			case ScoreThresholdPermissive:
				newScore = ScoreThresholdNormal
				message = "Search: Normal mode (balanced)"
			case ScoreThresholdNormal:
				newScore = ScoreThresholdStrict
				message = "Search: Strict mode (high quality matches only)"
			case ScoreThresholdStrict:
				newScore = ScoreThresholdNone
				message = "Search: No filtering (all matches)"
			default:
				newScore = ScoreThresholdNormal
				message = "Search: Normal mode (balanced)"
			}
			searchState.SetMinScore(newScore)
			a.statusMessage = message
			// Re-apply filter with new threshold
			updateFunc()
		case tcell.KeyRune:
			// Check for Alt key combinations
			if ev.Modifiers()&tcell.ModAlt != 0 {
				switch ev.Rune() {
				case 'f', 'F':
					searchState.MoveCursorWordForward()
				case 'b', 'B':
					searchState.MoveCursorWordBackward()
				case 'd', 'D':
					searchState.DeleteWordForward()
				}
			} else {
				searchState.InsertChar(ev.Rune())
			}
		}
		
		// Update command line display and apply filter if query changed
		a.commandLine = searchState.query
		if searchState.query != prevQuery {
			updateFunc()
		}
		return true
	}

	return false
}

func (a *App) draw() {
	// Try using Fill instead of Clear to force all cells to update
	w, h := a.screen.Size()
	style := tcell.StyleDefault.Background(ColorBg).Foreground(ColorFg)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a.screen.SetContent(x, y, ' ', nil, style)
		}
	}
	
	a.currentView.Draw(a.screen)
	a.drawStatusBar()

	// Draw help dialog on top of everything if visible
	a.helpDialog.Draw(a.screen)

	// Draw confirmation dialog on top of everything if visible
	a.confirmDialog.Draw(a.screen)

	a.screen.Show()
}

func (a *App) handleProgress() {
	saveCounter := 0
	for range a.player.Progress() {
		a.draw()

		// Save position every 30 seconds
		saveCounter++
		if saveCounter >= 30 {
			saveCounter = 0
			go a.saveEpisodePosition()
		}
	}
}

func (a *App) formatTime(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

func (a *App) formatPlayerStatus(width int) string {
	// Show status when playing or paused (not just when playing)
	playerState := a.player.GetState()
	if playerState == player.StateStopped || a.currentEpisode == nil {
		return ""
	}

	status := "▶"
	if a.player.IsPaused() {
		status = "⏸"
	}

	// Get current progress
	position, _ := a.player.GetPosition()
	duration, _ := a.player.GetDuration()

	// Build status with podcast and episode info
	var statusParts []string
	
	// Add podcast and episode titles if available
	if a.currentPodcast != nil && a.currentEpisode != nil {
		podcastTitle := a.currentPodcast.Title
		episodeTitle := a.currentEpisode.Title
		
		// Truncate titles based on available width
		maxTitleWidth := width / 3 // Reserve about 1/3 of width for titles
		if maxTitleWidth < 20 {
			maxTitleWidth = 20
		}
		
		if len(podcastTitle) > maxTitleWidth {
			podcastTitle = podcastTitle[:maxTitleWidth-3] + "..."
		}
		if len(episodeTitle) > maxTitleWidth {
			episodeTitle = episodeTitle[:maxTitleWidth-3] + "..."
		}
		
		statusParts = append(statusParts, fmt.Sprintf("%s: %s", podcastTitle, episodeTitle))
	}

	// Add playback progress
	progressStr := fmt.Sprintf("[%s %s/%s]",
		status,
		a.formatTime(position),
		a.formatTime(duration))
	statusParts = append(statusParts, progressStr)

	// If we have enough width, add speed and volume
	if width > 120 {
		speed, _ := a.player.GetSpeed()
		volume, _ := a.player.GetVolume()

		if speed != 1.0 {
			statusParts = append(statusParts, fmt.Sprintf("[%.1fx]", speed))
		}

		if a.player.IsMuted() {
			statusParts = append(statusParts, "[🔇]")
		} else if volume != 100 {
			statusParts = append(statusParts, fmt.Sprintf("[Vol:%d%%]", volume))
		}
	}

	return strings.Join(statusParts, " ")
}

func (a *App) drawStatusBar() {
	w, h := a.screen.Size()
	style := tcell.StyleDefault.Background(ColorBgHighlight).Foreground(ColorFg)

	for x := 0; x < w; x++ {
		a.screen.SetContent(x, h-1, ' ', nil, style)
	}

	modeStr := ""
	switch a.mode {
	case ModeNormal:
		modeStr = "NORMAL"
	case ModeCommand:
		modeStr = ":" + a.commandLine
	case ModeSearch:
		// Show search with cursor position for episode view
		if a.currentView == a.episodes {
			searchState := a.episodes.GetSearchState()
			modeStr = "/" + searchState.query
		} else {
			modeStr = "/" + a.commandLine
		}
	}

	drawText(a.screen, 0, h-1, style, modeStr)
	
	// Draw cursor for search mode
	if a.mode == ModeSearch {
		var searchState *SearchState
		if a.currentView == a.episodes {
			searchState = a.episodes.GetSearchState()
		} else if a.currentView == a.podcasts {
			searchState = a.podcasts.GetSearchState()
		}
		
		if searchState != nil {
			cursorX := 1 + searchState.cursorPos // 1 for the "/" prefix
			cursorStyle := style.Reverse(true)
			if searchState.cursorPos < len(searchState.query) {
				// Highlight the character at cursor position
				a.screen.SetContent(cursorX, h-1, rune(searchState.query[searchState.cursorPos]), nil, cursorStyle)
			} else {
				// Cursor at end of line - show a space with reverse video
				a.screen.SetContent(cursorX, h-1, ' ', nil, cursorStyle)
			}
		}
	}

	// Show player status with progress
	playerStatus := a.formatPlayerStatus(w)
	if playerStatus != "" {
		// Calculate position to right-align the player status
		x := w - len(playerStatus) - 1
		drawText(a.screen, x, h-1, style, playerStatus)
	}

	// Show status message if any (but leave space for player status)
	if a.statusMessage != "" {
		msgStyle := tcell.StyleDefault.Background(ColorBgHighlight).Foreground(ColorYellow)
		maxMsgWidth := w - len(modeStr) - len(playerStatus) - 4
		if len(a.statusMessage) > maxMsgWidth && maxMsgWidth > 0 {
			a.statusMessage = a.statusMessage[:maxMsgWidth-3] + "..."
		}
		if maxMsgWidth > 0 {
			drawText(a.screen, len(modeStr)+2, h-1, msgStyle, a.statusMessage)
		}
	}
}

func drawText(s tcell.Screen, x, y int, style tcell.Style, text string) {
	pos := 0
	for _, r := range text {
		s.SetContent(x+pos, y, r, nil, style)
		pos++
	}
}

// drawTextWithHighlight draws text with specified positions highlighted
func drawTextWithHighlight(s tcell.Screen, x, y, maxWidth int, style tcell.Style, text string, highlightPositions []int) {
	// Create a map for quick position lookup
	highlightMap := make(map[int]bool)
	for _, pos := range highlightPositions {
		highlightMap[pos] = true
	}
	
	highlightStyle := style.Foreground(ColorHighlight).Bold(true)
	
	// Convert text to runes for proper Unicode handling
	runes := []rune(text)
	
	// Draw the text with highlights
	screenPos := 0
	for runeIdx, r := range runes {
		if screenPos >= maxWidth {
			break
		}
		
		charStyle := style
		if highlightMap[runeIdx] {
			charStyle = highlightStyle
		}
		
		s.SetContent(x+screenPos, y, r, nil, charStyle)
		screenPos++
	}
}

func (a *App) executeCommand() {
	parts := strings.Fields(a.commandLine)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "add":
		if len(parts) < 2 {
			a.statusMessage = "Usage: add <feed-url>"
			return
		}
		go a.addPodcast(parts[1])
	case "q", "quit":
		// Post an interrupt event to ensure event loop exits
		if a.screen != nil {
			a.screen.PostEvent(tcell.NewEventInterrupt(nil))
		}
		close(a.quit)
	}
}

func (a *App) addPodcast(url string) {
	a.statusMessage = "Adding podcast..."
	a.draw() // Show status immediately
	
	podcast, err := feed.ParseFeed(url)
	if err != nil {
		a.statusMessage = "Error: " + err.Error()
		log.Printf("Failed to add podcast: %v", err)
		a.draw() // Show error
		return
	}

	// Check if already subscribed
	for _, p := range a.subscriptions.Podcasts {
		if p.URL == url {
			a.statusMessage = "Already subscribed to: " + p.Title
			a.draw()
			return
		}
	}

	a.subscriptions.Add(podcast)
	if err := a.subscriptions.Save(); err != nil {
		a.statusMessage = "Error saving: " + err.Error()
		log.Printf("Failed to save subscriptions: %v", err)
		a.draw() // Show error
		return
	}

	a.podcasts.SetSubscriptions(a.subscriptions)
	a.statusMessage = "Added: " + podcast.Title + fmt.Sprintf(" (%d episodes)", len(podcast.Episodes))
	a.draw() // Update UI to show new podcast
}

func (a *App) refreshFeeds() {
	totalPodcasts := len(a.subscriptions.Podcasts)

	if totalPodcasts == 0 {
		a.statusMessage = "No podcasts to refresh"
		return
	}

	successCount := 0
	for i, podcast := range a.subscriptions.Podcasts {
		// Calculate percentage
		percentage := int(float64(i+1) / float64(totalPodcasts) * 100)

		// Update progress indicator with percentage and current podcast
		a.statusMessage = fmt.Sprintf("Refreshing feeds... %d%% (%d/%d) %s",
			percentage, i+1, totalPodcasts, podcast.Title)
		a.draw() // Force redraw to show progress immediately

		updated, err := feed.ParseFeed(podcast.URL)
		if err != nil {
			log.Printf("Failed to refresh %s: %v", podcast.Title, err)
			a.statusMessage = fmt.Sprintf("Refreshing feeds... %d%% (%d/%d) Failed: %s",
				percentage, i+1, totalPodcasts, podcast.Title)
			a.draw()                           // Show error briefly
			time.Sleep(500 * time.Millisecond) // Brief pause to show error
			continue
		}
		a.mergePodcastData(podcast, updated)
		successCount++

		// Brief pause between refreshes to show progress
		if i < totalPodcasts-1 { // Don't pause after the last one
			time.Sleep(100 * time.Millisecond)
		}
	}

	if err := a.subscriptions.Save(); err != nil {
		log.Printf("Failed to save subscriptions: %v", err)
	}

	a.podcasts.SetSubscriptions(a.subscriptions)

	// Final status message with summary
	if successCount == totalPodcasts {
		a.statusMessage = fmt.Sprintf("All %d feeds refreshed successfully", totalPodcasts)
	} else {
		failedCount := totalPodcasts - successCount
		a.statusMessage = fmt.Sprintf("Refreshed %d/%d feeds (%d failed)",
			successCount, totalPodcasts, failedCount)
	}
	
	// Update UI to show refreshed episode counts
	a.draw()
}

func (a *App) saveEpisodePosition() {
	if a.currentEpisode != nil && a.player.GetState() != player.StateStopped {
		if position, err := a.player.GetPosition(); err == nil {
			log.Printf("Saving position for episode '%s': %v", a.currentEpisode.Title, position)
			a.currentEpisode.Position = position

			// Mark as played if >95% complete
			if duration, err := a.player.GetDuration(); err == nil && duration > 0 {
				progressPercent := float64(position) / float64(duration)
				if progressPercent > 0.95 {
					a.currentEpisode.Played = true
				}
			}

			// Save to disk
			go func() {
				if err := a.subscriptions.Save(); err != nil {
					log.Printf("Failed to save episode position: %v", err)
				}
			}()
		}
	}
}

// stopCurrentEpisode stops the current episode synchronously and updates status
func (a *App) stopCurrentEpisode() {
	if a.player.GetState() == player.StateStopped {
		return // Already stopped
	}
	
	a.statusMessage = "Stopping..."
	a.draw() // Show status immediately
	
	// Save position before stopping
	a.saveEpisodePosition()
	
	// Stop playback but keep mpv idle for instant resume
	if err := a.player.StopKeepIdle(); err != nil {
		log.Printf("Error stopping player: %v", err)
		a.statusMessage = "Stop error: " + err.Error()
	} else {
		a.statusMessage = "Stopped"
	}
	
	// Clear current episode state
	a.currentEpisode = nil
	a.currentPodcast = nil
	a.episodes.SetCurrentEpisode(nil)
}

func (a *App) playEpisode(episode *models.Episode) {
	log.Printf("playEpisode called for: %s", episode.Title)
	log.Printf("Episode position at start of playEpisode: %v", episode.Position)
	
	// Save position of current episode if switching
	if a.currentEpisode != nil && a.currentEpisode.ID != episode.ID {
		a.saveEpisodePosition()
	}

	a.currentEpisode = episode
	// Set current podcast for status bar display
	if a.currentView == a.episodes {
		a.currentPodcast = a.episodes.GetCurrentPodcast()
	}
	
	// Update episode list to highlight the current episode
	a.episodes.SetCurrentEpisode(episode)

	// Get podcast title for download checking
	podcastTitle := ""
	if a.currentView == a.episodes {
		if podcast := a.episodes.GetCurrentPodcast(); podcast != nil {
			podcastTitle = podcast.Title
		}
	}

	// Show switching/starting status
	if a.player.GetState() != player.StateStopped {
		// Switching from another episode
		if episode.Position > 0 && episode.Position < time.Hour*24 {
			a.statusMessage = fmt.Sprintf("Switching to: %s (resuming from %s)", episode.Title, a.formatTime(episode.Position))
		} else {
			a.statusMessage = "Switching to: " + episode.Title
		}
	} else {
		// Starting fresh
		if episode.Position > 0 && episode.Position < time.Hour*24 {
			a.statusMessage = fmt.Sprintf("Starting: %s (resuming from %s)", episode.Title, a.formatTime(episode.Position))
		} else {
			a.statusMessage = "Starting: " + episode.Title
		}
	}
	a.draw() // Show status immediately
	
	// Use comprehensive download check to get latest state
	var playURL string
	log.Printf("Checking download status for episode: %s", episode.Title)
	log.Printf("Episode.Downloaded: %v, Episode.DownloadPath: %s", episode.Downloaded, episode.DownloadPath)
	
	var isLocal bool
	if a.downloadManager.IsEpisodeDownloaded(episode, podcastTitle) && episode.DownloadPath != "" {
		// Verify file exists and is valid
		if _, err := os.Stat(episode.DownloadPath); err == nil {
			playURL = episode.DownloadPath
			isLocal = true
			log.Printf("Playing from local file: %s", playURL)
		} else {
			// File missing, fallback to streaming
			log.Printf("Downloaded file missing for %s, streaming instead", episode.Title)
			playURL = episode.URL
			isLocal = false
			// Reset download status if file is missing
			episode.Downloaded = false
			episode.DownloadPath = ""
		}
	} else {
		playURL = episode.URL
		isLocal = false
		log.Printf("Playing from URL: %s", playURL)
	}

	// Use SwitchTrack for seamless switching between episodes
	if err := a.player.SwitchTrack(playURL); err != nil {
		a.statusMessage = "Error: " + err.Error()
		log.Printf("Failed to play episode: %v", err)
		return
	}

	// Update status to show playing
	playingStatus := "Playing: " + episode.Title
	if isLocal {
		playingStatus += " (local)"
	}
	a.statusMessage = playingStatus

	// Update last played timestamp
	episode.LastPlayed = time.Now()

	// Resume from saved position if available
	log.Printf("Episode position check - Position: %v, Title: %s", episode.Position, episode.Title)
	if episode.Position > 0 && episode.Position < time.Hour*24 {
		go func() {
			// Give mpv more time to start and be ready for seek operations
			time.Sleep(2 * time.Second)

			// Seek to saved position
			seekSeconds := int(episode.Position.Seconds())
			log.Printf("Attempting to seek to position: %d seconds (%v)", seekSeconds, episode.Position)
			
			// Try seeking multiple times if it fails initially
			var err error
			for attempts := 0; attempts < 3; attempts++ {
				err = a.player.Seek(seekSeconds)
				if err == nil {
					log.Printf("Successfully resumed from position: %v on attempt %d", episode.Position, attempts+1)
					a.statusMessage = fmt.Sprintf("Resumed: %s at %s",
						episode.Title, a.formatTime(episode.Position))
					break
				}
				log.Printf("Seek attempt %d failed: %v", attempts+1, err)
				time.Sleep(500 * time.Millisecond)
			}
			
			if err != nil {
				log.Printf("Failed to resume from position after 3 attempts: %v", err)
				a.statusMessage = "Failed to resume from saved position"
			}
		}()
	} else {
		log.Printf("Not resuming - Position: %v (either 0 or > 24h)", episode.Position)
	}
}

// restartEpisode starts playing an episode from the beginning, ignoring saved position
func (a *App) restartEpisode(episode *models.Episode) {
	// Save position of current episode if it's different
	if a.currentEpisode != nil && a.currentEpisode.ID != episode.ID {
		a.saveEpisodePosition()
	}

	// Show restarting status
	a.statusMessage = "Restarting: " + episode.Title
	a.draw() // Show status immediately

	a.currentEpisode = episode
	// Set current podcast for status bar display
	if a.currentView == a.episodes {
		a.currentPodcast = a.episodes.GetCurrentPodcast()
	}
	
	// Update episode list to highlight the current episode
	a.episodes.SetCurrentEpisode(episode)

	// Reset position to beginning
	episode.Position = 0

	// Get podcast title for download checking
	podcastTitle := ""
	if a.currentView == a.episodes {
		if podcast := a.episodes.GetCurrentPodcast(); podcast != nil {
			podcastTitle = podcast.Title
		}
	}

	// Use comprehensive download check to get latest state
	var playURL string
	var isLocal bool
	if a.downloadManager.IsEpisodeDownloaded(episode, podcastTitle) && episode.DownloadPath != "" {
		// Verify file exists and is valid
		if _, err := os.Stat(episode.DownloadPath); err == nil {
			playURL = episode.DownloadPath
			isLocal = true
		} else {
			// File missing, fallback to streaming
			log.Printf("Downloaded file missing for %s, streaming instead", episode.Title)
			playURL = episode.URL
			isLocal = false
			// Reset download status if file is missing
			episode.Downloaded = false
			episode.DownloadPath = ""
		}
	} else {
		playURL = episode.URL
		isLocal = false
	}

	// Use SwitchTrack for seamless switching
	if err := a.player.SwitchTrack(playURL); err != nil {
		a.statusMessage = "Error: " + err.Error()
		log.Printf("Failed to restart episode: %v", err)
		return
	}

	// Update status to show playing
	playingStatus := "Playing: " + episode.Title
	if isLocal {
		playingStatus += " (local)"
	}
	a.statusMessage = playingStatus

	// Update last played timestamp
	episode.LastPlayed = time.Now()

	// Save the reset position immediately
	if err := a.subscriptions.Save(); err != nil {
		log.Printf("Failed to save position reset: %v", err)
	}
}

// handleDownloadProgress handles download progress updates
func (a *App) handleDownloadProgress() {
	for progress := range a.downloadManager.GetProgressChannel() {
		// Update status message for active downloads
		switch progress.Status {
		case download.StatusDownloading:
			if progress.Speed > 0 {
				speedMBps := float64(progress.Speed) / (1024 * 1024)
				eta := ""
				if progress.ETA > 0 {
					eta = fmt.Sprintf(" ETA: %v", progress.ETA.Round(time.Second))
				}
				a.statusMessage = fmt.Sprintf("⬇ %.1f%% %.1fMB/s%s",
					progress.Progress*100, speedMBps, eta)
			}
		case download.StatusCompleted:
			a.statusMessage = "✓ Download completed"
			// Save subscriptions to persist the download state
			go func() {
				if err := a.subscriptions.Save(); err != nil {
					log.Printf("Failed to save subscriptions after download completion: %v", err)
				}
			}()
		case download.StatusFailed:
			a.statusMessage = "✗ Download failed: " + progress.LastError
		case download.StatusCancelled:
			a.statusMessage = "Download cancelled"
		}

		// Trigger redraw to update episode list indicators
		a.draw()
	}
}

// downloadEpisode starts downloading an episode
func (a *App) downloadEpisode(episode *models.Episode) {
	// Get podcast title for comprehensive download check
	podcastTitle := ""
	if a.currentView == a.episodes {
		if podcast := a.episodes.GetCurrentPodcast(); podcast != nil {
			podcastTitle = podcast.Title
		}
	}
	
	// Check if episode is already downloaded (comprehensive check: filesystem + registry)
	if a.downloadManager.IsEpisodeDownloaded(episode, podcastTitle) {
		// Episode already downloaded - ask user if they want to re-download
		a.confirmEpisodeRedownload(episode)
		return
	}

	if a.downloadManager.IsDownloading(episode.ID) {
		a.statusMessage = "Episode already downloading"
		return
	}

	// Proceed with normal download
	a.startEpisodeDownload(episode)
}

// confirmEpisodeRedownload shows a confirmation dialog for re-downloading an already downloaded episode
func (a *App) confirmEpisodeRedownload(episode *models.Episode) {
	message := fmt.Sprintf("Episode '%s' is already downloaded. Delete and re-download?", episode.Title)
	a.confirmDialog.Show("Re-download Episode", message,
		func() {
			// On Yes - delete existing file and re-download
			go func() {
				if err := a.deleteDownloadedEpisode(episode); err != nil {
					a.statusMessage = "Delete error: " + err.Error()
					log.Printf("Failed to delete episode for re-download: %v", err)
					a.draw() // Refresh UI to show error
					return
				}
				// Remove from registry so it can be downloaded again
				a.downloadManager.RemoveFromRegistry(episode.ID)
				a.draw() // Refresh UI to update download indicators before starting new download
				// Start new download
				a.startEpisodeDownload(episode)
			}()
		},
		func() {
			// On No - keep existing file
			a.statusMessage = "Keeping existing download"
		})
}

// startEpisodeDownload handles the actual download process
func (a *App) startEpisodeDownload(episode *models.Episode) {
	// Get podcast title for directory structure
	podcastTitle := ""
	if a.currentView == a.episodes {
		if podcast := a.episodes.GetCurrentPodcast(); podcast != nil {
			podcastTitle = podcast.Title
		}
	}

	if err := a.downloadManager.QueueDownload(episode, podcastTitle); err != nil {
		a.statusMessage = "Download error: " + err.Error()
		log.Printf("Failed to queue download: %v", err)
		return
	}

	a.statusMessage = "Queued for download: " + episode.Title
}

// cancelOrDeleteEpisode cancels download or deletes downloaded episode
func (a *App) cancelOrDeleteEpisode(episode *models.Episode) {
	if a.downloadManager.IsDownloading(episode.ID) {
		// Cancel active download
		if err := a.downloadManager.CancelDownload(episode.ID); err != nil {
			a.statusMessage = "Cancel error: " + err.Error()
			log.Printf("Failed to cancel download: %v", err)
			return
		}
		a.statusMessage = "Download cancelled: " + episode.Title
	} else if a.downloadManager.IsDownloaded(episode.ID) {
		// Delete downloaded file
		if err := a.deleteDownloadedEpisode(episode); err != nil {
			a.statusMessage = "Delete error: " + err.Error()
			log.Printf("Failed to delete episode: %v", err)
			return
		}
		a.statusMessage = "Deleted: " + episode.Title
	} else {
		a.statusMessage = "Episode not downloaded"
	}
}

// deleteDownloadedEpisode removes a downloaded episode file
func (a *App) deleteDownloadedEpisode(episode *models.Episode) error {
	// Delete the file from disk if it exists
	if episode.DownloadPath != "" {
		if err := os.Remove(episode.DownloadPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete file: %w", err)
		}
		log.Printf("Deleted file: %s", episode.DownloadPath)
	}

	// Remove from download registry to ensure consistent state
	a.downloadManager.RemoveFromRegistry(episode.ID)
	log.Printf("Removed episode %s from download registry", episode.ID)

	// Reset episode download fields
	episode.Downloaded = false
	episode.DownloadPath = ""
	episode.DownloadSize = 0
	episode.DownloadDate = time.Time{}

	// Save subscriptions to persist the updated episode state
	if err := a.subscriptions.Save(); err != nil {
		log.Printf("Failed to save subscriptions after delete: %v", err)
	}

	return nil
}

// confirmPodcastDeletion shows a confirmation dialog for podcast deletion
func (a *App) confirmPodcastDeletion(podcast *models.Podcast) {
	message := fmt.Sprintf("Delete podcast '%s'?", podcast.Title)
	a.confirmDialog.Show("Confirm Deletion", message,
		func() {
			// On Yes
			a.statusMessage = "Deleting: " + podcast.Title
			a.draw() // Show status immediately
			
			a.subscriptions.Remove(podcast.URL)
			if err := a.subscriptions.Save(); err != nil {
				a.statusMessage = "Error saving: " + err.Error()
				log.Printf("Failed to save subscriptions after deletion: %v", err)
			} else {
				a.statusMessage = "Deleted: " + podcast.Title
			}
			
			a.podcasts.SetSubscriptions(a.subscriptions)
			
			// If we deleted the last podcast, reset view
			if len(a.subscriptions.Podcasts) == 0 {
				a.statusMessage = "No podcasts. Press 'a' to add one."
			}
			
			a.draw() // Update UI to reflect deletion
		},
		func() {
			// On No
			a.statusMessage = "Deletion cancelled"
			a.draw() // Update status
		})
}

// confirmEpisodeDeletion shows a confirmation dialog for episode deletion
func (a *App) confirmEpisodeDeletion(episode *models.Episode) {
	if a.downloadManager.IsDownloading(episode.ID) {
		// For active downloads, just ask for cancellation
		message := fmt.Sprintf("Cancel download of '%s'?", episode.Title)
		a.confirmDialog.Show("Confirm Cancellation", message,
			func() {
				// On Yes
				go func() {
					if err := a.downloadManager.CancelDownload(episode.ID); err != nil {
						a.statusMessage = "Cancel error: " + err.Error()
						log.Printf("Failed to cancel download: %v", err)
						a.draw() // Refresh UI to show error
						return
					}
					a.statusMessage = "Download cancelled: " + episode.Title
					a.draw() // Refresh UI to update download indicators
				}()
			},
			func() {
				// On No
				a.statusMessage = "Cancellation aborted"
			})
	} else if a.downloadManager.IsDownloaded(episode.ID) {
		// For downloaded episodes, ask for deletion
		message := fmt.Sprintf("Delete downloaded file for '%s'?", episode.Title)
		a.confirmDialog.Show("Confirm Deletion", message,
			func() {
				// On Yes
				go func() {
					if err := a.deleteDownloadedEpisode(episode); err != nil {
						a.statusMessage = "Delete error: " + err.Error()
						log.Printf("Failed to delete episode: %v", err)
						a.draw() // Refresh UI to show error
						return
					}
					a.statusMessage = "Deleted: " + episode.Title
					a.draw() // Refresh UI to update download indicators
				}()
			},
			func() {
				// On No
				a.statusMessage = "Deletion cancelled"
			})
	} else {
		a.statusMessage = "Episode not downloaded"
	}
}

// clearStatusMessage clears status messages except for active downloads/playback
func (a *App) clearStatusMessage() {
	// Don't clear status if there are active downloads
	allDownloads := a.downloadManager.GetAllDownloads()
	for _, progress := range allDownloads {
		if progress.Status == download.StatusDownloading {
			return // Keep download status visible
		}
	}
	
	// Don't clear status if showing current playback information
	if strings.Contains(a.statusMessage, "Playing") || 
	   strings.Contains(a.statusMessage, "Resumed") || 
	   strings.Contains(a.statusMessage, "Restarting") {
		return // Keep playback status visible
	}
	
	// Don't clear recent refresh status
	if strings.Contains(a.statusMessage, "Refreshing feeds") || 
	   strings.Contains(a.statusMessage, "feeds refreshed") {
		return // Keep refresh status visible
	}
	
	// Clear other status messages (completed downloads, errors, etc.)
	a.statusMessage = ""
}

// mergePodcastData merges updated podcast data with existing data, preserving user state
func (a *App) mergePodcastData(existing *models.Podcast, updated *models.Podcast) {
	// Update podcast metadata
	existing.Title = updated.Title
	existing.Description = updated.Description
	existing.ImageURL = updated.ImageURL
	existing.Author = updated.Author
	existing.LastUpdated = updated.LastUpdated

	// Create maps for existing episodes - by ID and by URL+date for fallback
	existingEpisodesById := make(map[string]*models.Episode)
	existingEpisodesByKey := make(map[string]*models.Episode)
	
	for _, episode := range existing.Episodes {
		if episode.ID != "" {
			existingEpisodesById[episode.ID] = episode
		}
		// Create fallback key using URL and publish date
		key := episode.URL + "|" + episode.PublishDate.Format("2006-01-02T15:04:05Z")
		existingEpisodesByKey[key] = episode
	}

	// Process updated episodes
	var mergedEpisodes []*models.Episode
	for _, newEpisode := range updated.Episodes {
		var existingEp *models.Episode
		var found bool
		
		// Try to find by ID first
		if newEpisode.ID != "" {
			existingEp, found = existingEpisodesById[newEpisode.ID]
		}
		
		// If not found by ID, try to find by URL+date
		if !found {
			key := newEpisode.URL + "|" + newEpisode.PublishDate.Format("2006-01-02T15:04:05Z")
			existingEp, found = existingEpisodesByKey[key]
		}
		
		if found {
			// Episode already exists - merge data, preserving user state
			existingEp.ID = newEpisode.ID // Update ID if it was empty
			existingEp.Title = newEpisode.Title
			existingEp.Description = newEpisode.Description
			existingEp.URL = newEpisode.URL
			existingEp.PublishDate = newEpisode.PublishDate
			
			// Update duration if the new episode has duration data and existing doesn't
			if newEpisode.Duration > 0 && existingEp.Duration == 0 {
				existingEp.Duration = newEpisode.Duration
			}
			
			// Keep existing user state: Position, Played, Downloaded, etc.
			mergedEpisodes = append(mergedEpisodes, existingEp)
		} else {
			// New episode - add it as-is
			mergedEpisodes = append(mergedEpisodes, newEpisode)
		}
	}

	// Replace episodes with merged list
	existing.Episodes = mergedEpisodes
}
