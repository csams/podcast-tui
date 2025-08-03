package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
	previousView    View
	podcasts        *PodcastListView
	episodes        *EpisodeListView
	queue           *QueueView
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
	settings        *Settings
	shutdownOnce    sync.Once
	positionTicker  *time.Ticker
	positionUpdate  chan struct{}
	
	// Refresh management
	refreshSemaphore chan struct{}
	activeRefreshes  map[string]bool // Track which podcasts are currently refreshing
	refreshMutex     sync.Mutex      // Protect activeRefreshes map
	
	// Episode transition management
	transitionMutex     sync.Mutex    // Protect episode transitions
	transitionInProgress bool         // Flag to indicate transition is happening
	completionHandled   atomic.Bool   // Atomic flag to prevent double completion handling
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
		quit:             make(chan struct{}),
		mode:             ModeNormal,
		player:           player.New(),
		helpDialog:       NewHelpDialog(),
		confirmDialog:    NewConfirmationDialog(),
		configDir:        configDir,
		positionUpdate:   make(chan struct{}, 1),
		refreshSemaphore: make(chan struct{}, 10), // Allow up to 10 concurrent refreshes
		activeRefreshes:  make(map[string]bool),
	}

	// Initialize download manager
	app.downloadManager = download.NewManager(configDir)

	// Load settings
	settings, err := LoadSettings(configDir)
	if err != nil {
		log.Printf("Failed to load settings, using defaults: %v", err)
		settings = DefaultSettings()
	}
	app.settings = settings

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
	a.episodes.SetSubscriptions(subs)
	a.episodes.SetDownloadManager(a.downloadManager) // Pass download manager to episode list
	a.episodes.SetPlayer(a.player)                   // Pass player to episode list
	a.queue = NewQueueView()
	a.queue.SetSubscriptions(subs)
	a.queue.SetDownloadManager(a.downloadManager)
	a.queue.SetPlayer(a.player)
	a.currentView = a.podcasts
	a.previousView = a.podcasts

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

		// Stop position ticker
		a.stopPositionTicker()

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
	// Create a channel for screen events
	eventChan := make(chan tcell.Event)
	go func() {
		for {
			ev := a.screen.PollEvent()
			if ev == nil {
				close(eventChan)
				return
			}
			eventChan <- ev
		}
	}()

	for {
		select {
		case <-a.quit:
			return
		case <-a.positionUpdate:
			// Update position display
			a.updateCurrentPosition()
			
			// If a modal is visible, we need to do a full redraw to keep it on screen
			if a.helpDialog.IsVisible() || a.confirmDialog.IsVisible() {
				a.draw()
			} else {
				// Update the position column in current view
				if a.currentView == a.episodes {
					a.episodes.UpdateCurrentEpisodePosition(a.screen)
					a.drawStatusBar() // Also update the status bar
					a.screen.Show()
				} else if a.currentView == a.queue {
					a.queue.UpdateCurrentEpisodePosition(a.screen)
					a.drawStatusBar() // Also update the status bar
					a.screen.Show()
				}
			}
		case ev, ok := <-eventChan:
			if !ok {
				// Channel closed, screen might be finalized
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
			// Check for Alt key combinations first
			if ev.Modifiers()&tcell.ModAlt != 0 && a.currentView == a.queue {
				switch ev.Rune() {
				case 'j':
					// Move episode down in queue
					selectedIdx := a.queue.GetSelectedIndex()
					// Get the episode being moved before the operation
					selectedEpisode := a.queue.GetSelected()
					if selectedEpisode != nil && a.subscriptions.MoveQueueItemDown(selectedIdx) {
						if err := a.subscriptions.Save(); err != nil {
							a.statusMessage = "Error saving queue: " + err.Error()
						} else {
							a.statusMessage = "Moved episode down"
							a.queue.refresh()
							// Find and select the moved episode by ID
							found := false
							for i, ep := range a.queue.episodes {
								if ep.ID == selectedEpisode.ID {
									a.queue.table.selectedIdx = i
									a.queue.table.ensureVisible()
									found = true
									break
								}
							}
							// If episode not found, ensure valid selection
							if !found && len(a.queue.episodes) > 0 {
								if a.queue.table.selectedIdx >= len(a.queue.episodes) {
									a.queue.table.selectedIdx = len(a.queue.episodes) - 1
								}
								a.queue.table.ensureVisible()
							}
						}
					}
					return true
				case 'k':
					// Move episode up in queue
					selectedIdx := a.queue.GetSelectedIndex()
					// Get the episode being moved before the operation
					selectedEpisode := a.queue.GetSelected()
					if selectedEpisode != nil && a.subscriptions.MoveQueueItemUp(selectedIdx) {
						if err := a.subscriptions.Save(); err != nil {
							a.statusMessage = "Error saving queue: " + err.Error()
						} else {
							a.statusMessage = "Moved episode up"
							a.queue.refresh()
							// Find and select the moved episode by ID
							found := false
							for i, ep := range a.queue.episodes {
								if ep.ID == selectedEpisode.ID {
									a.queue.table.selectedIdx = i
									a.queue.table.ensureVisible()
									found = true
									break
								}
							}
							// If episode not found, ensure valid selection
							if !found && len(a.queue.episodes) > 0 {
								if a.queue.table.selectedIdx >= len(a.queue.episodes) {
									a.queue.table.selectedIdx = len(a.queue.episodes) - 1
								}
								a.queue.table.ensureVisible()
							}
						}
					}
					return true
				}
			}

			switch ev.Rune() {
			case 'Q':
				// Quit application (capital Q)
				// Post an interrupt event to ensure event loop exits
				if a.screen != nil {
					a.screen.PostEvent(tcell.NewEventInterrupt(nil))
				}
				close(a.quit)
				a.shutdown()
				return false
			case 'q':
				// Switch to queue view from podcast or episode view
				if a.currentView == a.podcasts || a.currentView == a.episodes {
					a.previousView = a.currentView
					a.currentView = a.queue
					a.queue.refresh()
					a.clearStatusMessage()
					return true
				}
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
						// Check if transition is in progress
						a.transitionMutex.Lock()
						if a.transitionInProgress {
							a.transitionMutex.Unlock()
							a.statusMessage = "Please wait..."
							return true
						}
						a.transitionMutex.Unlock()
						
						log.Printf("User pressed 'l' - Episode: %s, Position: %v", episode.Title, episode.Position)
						// Add to queue
						a.addToQueue(episode)
						return true
					}
				} else if a.currentView == a.queue {
					if episode := a.queue.GetSelected(); episode != nil {
						// Check if transition is in progress
						a.transitionMutex.Lock()
						if a.transitionInProgress {
							a.transitionMutex.Unlock()
							a.statusMessage = "Please wait..."
							return true
						}
						a.transitionMutex.Unlock()
						
						// Check if an episode is already playing or paused
						if a.player.GetState() != player.StateStopped {
							// Episode is playing or paused, do nothing
							return true
						}
						log.Printf("User pressed 'l' in queue - Episode: %s", episode.Title)
						// Play immediately from queue
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
							// Redraw to update episode highlighting for pause state
							a.draw()
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
						position, _ := a.player.GetPosition()
						duration, _ := a.player.GetDuration()

						if err := a.player.Seek(30); err != nil {
							a.statusMessage = "Seek error: " + err.Error()
						} else if duration > 0 && position+30*time.Second > duration {
							a.statusMessage = "Seeked to near end"
						}
					}()
				}
				return true
			case 'b':
				// Seek backward 30 seconds
				if a.player.GetState() != player.StateStopped {
					go func() {
						position, _ := a.player.GetPosition()

						if err := a.player.Seek(-30); err != nil {
							a.statusMessage = "Seek error: " + err.Error()
						} else if position < 30*time.Second {
							a.statusMessage = "Seeked to beginning"
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
			case 'p':
				// Switch to podcast view from episode or queue view
				if a.currentView == a.episodes || a.currentView == a.queue {
					a.currentView = a.podcasts
					a.clearStatusMessage()
					return true
				}
			case 'e':
				// Switch to episode view
				if a.currentView == a.podcasts {
					// From podcast view, open episodes for selected podcast
					if selected := a.podcasts.GetSelected(); selected != nil {
						a.clearStatusMessage()
						a.episodes.SetPodcast(selected)
						a.currentView = a.episodes
						return true
					}
				} else if a.currentView == a.queue {
					// From queue view, same as 'g' - go to episode in episode list
					if episode := a.queue.GetSelected(); episode != nil {
						podcast := a.subscriptions.GetPodcastForEpisode(episode.ID)
						if podcast != nil {
							a.episodes.SetPodcast(podcast)
							a.currentView = a.episodes
							a.selectEpisodeInList(episode.ID)
							a.statusMessage = "Navigated to episode in list"
							return true
						}
					}
				}
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
				} else if a.currentView == a.queue {
					if episode := a.queue.GetSelected(); episode != nil {
						go a.downloadEpisode(episode)
						return true
					}
				}
			case 'n':
				// Open note editor for selected episode
				if a.currentView == a.episodes {
					if episode := a.episodes.GetSelected(); episode != nil {
						if podcast := a.episodes.GetCurrentPodcast(); podcast != nil {
							a.openNoteEditor(episode, podcast.Title)
							return true
						}
					}
				} else if a.currentView == a.queue {
					if episode := a.queue.GetSelected(); episode != nil {
						if podcast := a.subscriptions.GetPodcastForEpisode(episode.ID); podcast != nil {
							a.openNoteEditor(episode, podcast.Title)
							return true
						}
					}
				}
			case 'u':
				// Remove from queue
				if a.currentView == a.episodes {
					if episode := a.episodes.GetSelected(); episode != nil {
						// Check if episode is in queue
						if a.subscriptions.GetQueuePosition(episode.ID) > 0 {
							a.removeFromQueue(episode)
							return true
						} else {
							a.statusMessage = "Episode not in queue"
							return true
						}
					}
				} else if a.currentView == a.queue {
					if episode := a.queue.GetSelected(); episode != nil {
						a.removeFromQueue(episode)
						return true
					}
				}
			case 'r':
				// Refresh feeds
				if a.currentView == a.episodes {
					// In episode list view, refresh only the current podcast
					if podcast := a.episodes.GetCurrentPodcast(); podcast != nil {
						a.statusMessage = fmt.Sprintf("Refreshing %s...", podcast.Title)
						go a.refreshSinglePodcast(podcast)
					}
				} else {
					// In podcast list view, refresh all feeds
					totalPodcasts := len(a.subscriptions.Podcasts)
					if totalPodcasts == 0 {
						a.statusMessage = "No podcasts to refresh"
					} else {
						a.statusMessage = fmt.Sprintf("Starting refresh of %d podcasts...", totalPodcasts)
						go a.refreshFeeds()
					}
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
				} else if a.currentView == a.queue {
					if episode := a.queue.GetSelected(); episode != nil {
						go a.restartEpisode(episode)
						return true
					}
				}
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				// Number keys for percentage-based seeking (0=0%, 1=10%, ..., 9=90%)
				if (a.currentView == a.episodes || a.currentView == a.queue) && a.player.GetState() != player.StateStopped {
					duration, err := a.player.GetDuration()
					if err == nil && duration > 0 {
						percentage := float64(ev.Rune()-'0') / 10.0
						targetSeconds := int(duration.Seconds() * percentage)

						go func() {
							// Use absolute seeking for percentage-based positions
							if err := a.player.SeekAbsolute(targetSeconds); err != nil {
								a.statusMessage = fmt.Sprintf("Seek error: %v", err)
							} else {
								// Format time display
								targetTime := time.Duration(targetSeconds) * time.Second
								a.statusMessage = fmt.Sprintf("Seeking to %d%% (%s)",
									int(percentage*100), a.formatTime(targetTime))
							}
						}()
					}
				}
				return true
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
				} else if a.currentView == a.queue {
					if episode := a.queue.GetSelected(); episode != nil {
						a.confirmEpisodeDeletion(episode)
						return true
					}
				}
			case '/':
				// Search is disabled in queue view
				if a.currentView == a.queue {
					return false
				}
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
			// Handle Enter key for different views
			if a.currentView == a.podcasts {
				if selected := a.podcasts.GetSelected(); selected != nil {
					a.clearStatusMessage()
					a.episodes.SetPodcast(selected)
					a.currentView = a.episodes
					return true
				}
			} else if a.currentView == a.episodes {
				if episode := a.episodes.GetSelected(); episode != nil {
					// Check if transition is in progress
					a.transitionMutex.Lock()
					if a.transitionInProgress {
						a.transitionMutex.Unlock()
						a.statusMessage = "Please wait..."
						return true
					}
					a.transitionMutex.Unlock()
					
					log.Printf("User pressed Enter - Episode: %s, Position: %v", episode.Title, episode.Position)
					// Add to queue
					a.addToQueue(episode)
					return true
				}
			} else if a.currentView == a.queue {
				if episode := a.queue.GetSelected(); episode != nil {
					// Check if transition is in progress
					a.transitionMutex.Lock()
					if a.transitionInProgress {
						a.transitionMutex.Unlock()
						a.statusMessage = "Please wait..."
						return true
					}
					a.transitionMutex.Unlock()
					
					// Check if an episode is already playing or paused
					if a.player.GetState() != player.StateStopped {
						// Episode is playing or paused, do nothing
						return true
					}
					log.Printf("User pressed Enter in queue - Episode: %s", episode.Title)
					// Play immediately from queue
					go a.playEpisode(episode)
					return true
				}
			}
		case tcell.KeyEscape:
			a.mode = ModeNormal
			return true
		case tcell.KeyTab:
			// TAB switches to queue view from podcast/episode view, or returns to previous view from queue
			if a.currentView == a.podcasts || a.currentView == a.episodes {
				// Save current view and switch to queue
				a.previousView = a.currentView
				a.currentView = a.queue
				a.queue.refresh()
			} else if a.currentView == a.queue {
				// Return to previous view
				a.currentView = a.previousView
			}
			return true
		case tcell.KeyRight:
			// Seek forward 10 seconds (only when player has content)
			if a.player.GetState() != player.StateStopped {
				go func() {
					position, _ := a.player.GetPosition()
					duration, _ := a.player.GetDuration()

					if err := a.player.Seek(10); err != nil {
						a.statusMessage = "Seek error: " + err.Error()
					} else if duration > 0 && position+10*time.Second > duration {
						a.statusMessage = "Seeked to near end"
					}
				}()
				return true
			}
		case tcell.KeyLeft:
			// Seek backward 10 seconds (only when player has content)
			if a.player.GetState() != player.StateStopped {
				go func() {
					position, _ := a.player.GetPosition()

					if err := a.player.Seek(-10); err != nil {
						a.statusMessage = "Seek error: " + err.Error()
					} else if position < 10*time.Second {
						a.statusMessage = "Seeked to beginning"
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
	var lastEpisodeID string
	var lastPlayerState player.PlayerState = player.StateStopped

	for progress := range a.player.Progress() {
		// Don't redraw the entire screen - just update the status bar
		// The position ticker handles updating the episode list view
		// But if a modal is visible, skip partial updates to avoid overwriting it
		if !a.helpDialog.IsVisible() && !a.confirmDialog.IsVisible() {
			a.drawStatusBar()
			a.screen.Show()
		}

		// Get current player state
		currentPlayerState := a.player.GetState()

		// Reset completion flag when episode changes
		if a.currentEpisode != nil && a.currentEpisode.ID != lastEpisodeID {
			lastEpisodeID = a.currentEpisode.ID
			a.completionHandled.Store(false)
			lastPlayerState = currentPlayerState
		}

		// Check if player just stopped (state changed from playing/paused to stopped)
		if a.currentEpisode != nil && 
			(lastPlayerState == player.StatePlaying || lastPlayerState == player.StatePaused) &&
			currentPlayerState == player.StateStopped {
			// Player just stopped - check if we're at the end
			if progress.Duration > 0 && progress.Position >= progress.Duration {
				// Try to set the completion flag atomically
				if a.completionHandled.CompareAndSwap(false, true) {
					log.Printf("Episode completion detected via state change! Position: %v, Duration: %v",
						progress.Position, progress.Duration)
					
					// Handle completion in a goroutine to avoid blocking progress updates
					go a.handleEpisodeCompletion()
				}
			}
		}
		lastPlayerState = currentPlayerState

		// Log progress for debugging when near the end
		if progress.Duration > 0 {
			percentComplete := float64(progress.Position) / float64(progress.Duration) * 100
			timeRemaining := progress.Duration - progress.Position
			// Log every 10 seconds when near the end
			if timeRemaining < 30*time.Second && saveCounter%10 == 0 {
				log.Printf("Episode progress: %.1f%% complete, %v remaining (pos: %v, dur: %v)",
					percentComplete, timeRemaining, progress.Position, progress.Duration)
			}
		}

		// Save position every 30 seconds
		saveCounter++
		if saveCounter >= 30 {
			saveCounter = 0
			go a.saveEpisodePosition()
		}
	}
}

func (a *App) formatTime(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
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
	case "Q", "quit":
		// Post an interrupt event to ensure event loop exits
		if a.screen != nil {
			a.screen.PostEvent(tcell.NewEventInterrupt(nil))
		}
		close(a.quit)
	case "q":
		// Switch to queue view
		if a.currentView == a.podcasts || a.currentView == a.episodes {
			a.previousView = a.currentView
			a.currentView = a.queue
			a.queue.refresh()
			a.mode = ModeNormal
			a.commandLine = ""
			return
		}
	}
}

func (a *App) addPodcast(url string) {
	a.statusMessage = "Adding podcast..."
	a.draw() // Show status immediately

	podcast, err := feed.ParseFeed(url)
	if err != nil {
		a.statusMessage = "Error: " + err.Error()
		log.Printf("Failed to add podcast from %s: %v", url, err)
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

	// Create wait group for concurrent refreshes
	var wg sync.WaitGroup
	successCount := int32(0)
	failedCount := int32(0)
	processedCount := int32(0)
	
	startTime := time.Now()
	
	// Start concurrent refreshes for all podcasts
	for _, podcast := range a.subscriptions.Podcasts {
		// Skip if already refreshing
		a.refreshMutex.Lock()
		if a.activeRefreshes[podcast.URL] {
			a.refreshMutex.Unlock()
			atomic.AddInt32(&processedCount, 1)
			continue
		}
		a.activeRefreshes[podcast.URL] = true
		a.refreshMutex.Unlock()
		
		wg.Add(1)
		go func(p *models.Podcast) {
			defer wg.Done()
			
			// Acquire semaphore slot
			a.refreshSemaphore <- struct{}{}
			defer func() {
				<-a.refreshSemaphore
				a.refreshMutex.Lock()
				delete(a.activeRefreshes, p.URL)
				a.refreshMutex.Unlock()
			}()
			
			// Update progress
			current := atomic.AddInt32(&processedCount, 1)
			percentage := (current * 100) / int32(totalPodcasts)
			a.statusMessage = fmt.Sprintf("Refreshing feeds... %d%% (%d/%d)",
				percentage, current, totalPodcasts)
			a.draw()
			
			// Parse the feed
			updated, err := feed.ParseFeed(p.URL)
			if err != nil {
				log.Printf("Failed to refresh podcast '%s' from %s: %v", p.Title, p.URL, err)
				atomic.AddInt32(&failedCount, 1)
				return
			}
			
			// Merge the updated data
			a.mergePodcastData(p, updated)
			atomic.AddInt32(&successCount, 1)
		}(podcast)
	}
	
	// Wait for all refreshes to complete
	wg.Wait()
	
	// Log refresh summary
	failed := atomic.LoadInt32(&failedCount)
	success := atomic.LoadInt32(&successCount)
	log.Printf("Feed refresh completed: %d successful, %d failed out of %d total", 
		success, failed, totalPodcasts)
	
	// Save subscriptions
	if err := a.subscriptions.Save(); err != nil {
		log.Printf("Failed to save subscriptions: %v", err)
		a.statusMessage = "Error saving subscriptions"
	} else {
		// Show completion status
		elapsed := time.Since(startTime).Round(time.Second)
		failedCount := totalPodcasts - int(successCount)
		if failedCount > 0 {
			a.statusMessage = fmt.Sprintf("Refresh complete in %v: %d succeeded, %d failed",
				elapsed, successCount, failedCount)
		} else {
			a.statusMessage = fmt.Sprintf("All %d podcasts refreshed successfully in %v",
				successCount, elapsed)
		}
	}

	// Update podcasts view
	a.podcasts.SetSubscriptions(a.subscriptions)

	// Update UI to show refreshed episode counts
	a.draw()
}

// refreshSinglePodcast refreshes just one podcast's feed
func (a *App) refreshSinglePodcast(podcast *models.Podcast) {
	// Check if this podcast is already being refreshed
	a.refreshMutex.Lock()
	if a.activeRefreshes[podcast.URL] {
		a.refreshMutex.Unlock()
		a.statusMessage = fmt.Sprintf("%s is already refreshing", podcast.Title)
		a.draw()
		return
	}
	a.activeRefreshes[podcast.URL] = true
	a.refreshMutex.Unlock()

	// Acquire semaphore slot
	a.refreshSemaphore <- struct{}{}
	
	// Ensure cleanup
	defer func() {
		<-a.refreshSemaphore
		a.refreshMutex.Lock()
		delete(a.activeRefreshes, podcast.URL)
		a.refreshMutex.Unlock()
	}()

	// Parse the feed
	updated, err := feed.ParseFeed(podcast.URL)
	if err != nil {
		log.Printf("Failed to refresh single podcast '%s' from %s: %v", podcast.Title, podcast.URL, err)
		a.statusMessage = fmt.Sprintf("Failed to refresh %s: %v", podcast.Title, err)
		a.draw()
		return
	}

	// Merge the updated data
	a.mergePodcastData(podcast, updated)

	// Save subscriptions
	if err := a.subscriptions.Save(); err != nil {
		log.Printf("Failed to save subscriptions: %v", err)
	}

	// Update the episode list view with the refreshed podcast
	a.episodes.SetPodcast(podcast)

	// Update status and redraw
	a.statusMessage = fmt.Sprintf("%s refreshed successfully", podcast.Title)
	a.draw()
}

func (a *App) saveEpisodePosition() {
	if a.currentEpisode != nil && a.player.GetState() != player.StateStopped {
		if position, err := a.player.GetPosition(); err == nil {
			log.Printf("Saving position for episode '%s': %v", a.currentEpisode.Title, position)

			// Find and update the episode in the actual subscription data using the index
			if episode := a.subscriptions.GetEpisodeByID(a.currentEpisode.ID); episode != nil {
				// Update the canonical episode in subscriptions
				episode.Position = position

				// Update duration if it's unknown or different from actual
				if duration, err := a.player.GetDuration(); err == nil && duration > 0 {
					// Check if duration is significantly different (more than 1 second difference)
					durationDiff := episode.Duration - duration
					if durationDiff < 0 {
						durationDiff = -durationDiff
					}

					if episode.Duration == 0 || durationDiff > time.Second {
						log.Printf("Updating duration for episode '%s': %v -> %v",
							episode.Title, episode.Duration, duration)
						episode.Duration = duration

						// Update the duration in the episode list directly
						a.episodes.UpdateEpisodeDuration(episode.ID, duration)

						// Also update our local reference
						a.currentEpisode.Duration = duration

						// Update the episode list view's reference
						a.episodes.SetCurrentEpisode(a.currentEpisode)
						a.queue.SetCurrentEpisode(a.currentEpisode)

						// Immediately update the UI to show the new duration
						if a.currentView == a.episodes {
							// Check for modals before updating
							if a.helpDialog.IsVisible() || a.confirmDialog.IsVisible() {
								a.draw()
							} else {
								a.episodes.UpdateCurrentEpisodePosition(a.screen)
								a.screen.Show()
							}
						}
					}
				}

				// Mark as played if >95% complete
				if duration, err := a.player.GetDuration(); err == nil && duration > 0 {
					progressPercent := float64(position) / float64(duration)
					if progressPercent > 0.95 {
						episode.Played = true
					}
				}

				// Also update our local reference
				a.currentEpisode.Position = position
				if episode.Played {
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

	// Stop position ticker
	a.stopPositionTicker()

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
	a.queue.SetCurrentEpisode(nil)

	// Redraw to clear episode highlighting
	a.draw()
}

func (a *App) playEpisode(episode *models.Episode) {
	log.Printf("playEpisode called for: %s", episode.Title)
	log.Printf("Episode position at start of playEpisode: %v", episode.Position)

	// Stop position ticker to ensure clean state transition
	a.stopPositionTicker()

	// Save position of current episode if switching
	if a.currentEpisode != nil && a.currentEpisode.ID != episode.ID {
		a.saveEpisodePosition()
	}

	// Find the canonical episode in subscription data using the index
	canonicalEpisode := a.subscriptions.GetEpisodeByID(episode.ID)

	// Use the canonical episode if found, otherwise use the provided one
	if canonicalEpisode != nil {
		a.currentEpisode = canonicalEpisode
		// Update the UI's reference to use the canonical episode
		a.episodes.SetCurrentEpisode(canonicalEpisode)
		a.queue.SetCurrentEpisode(canonicalEpisode)
	} else {
		a.currentEpisode = episode
		a.episodes.SetCurrentEpisode(episode)
		a.queue.SetCurrentEpisode(episode)
	}
	// Set current podcast for status bar display
	if a.currentView == a.episodes {
		a.currentPodcast = a.episodes.GetCurrentPodcast()
	} else {
		// Find the podcast that contains this episode
		a.currentPodcast = a.subscriptions.GetPodcastForEpisode(episode.ID)
	}

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

	// Redraw to update episode highlighting now that player state has changed
	a.draw()

	// Resume from saved position if available
	log.Printf("Episode position check - Position: %v, Title: %s", episode.Position, episode.Title)
	if episode.Position > 0 && episode.Position < time.Hour*24 {
		// Store the position to resume from (in case it gets modified)
		resumePosition := episode.Position
		go func() {
			// Wait for mpv to fully load the file
			maxWaitTime := 5 * time.Second
			startTime := time.Now()

			// Wait until player reports a valid duration (indicates file is loaded)
			for time.Since(startTime) < maxWaitTime {
				if duration, err := a.player.GetDuration(); err == nil && duration > 0 {
					log.Printf("File loaded, duration: %v", duration)

					// Update episode duration if it's unknown or different
					durationDiff := a.currentEpisode.Duration - duration
					if durationDiff < 0 {
						durationDiff = -durationDiff
					}

					if a.currentEpisode.Duration == 0 || durationDiff > time.Second {
						log.Printf("Updating duration for episode '%s': %v -> %v",
							a.currentEpisode.Title, a.currentEpisode.Duration, duration)

						// Update the episode in the actual subscription data using the index
						if ep := a.subscriptions.GetEpisodeByID(a.currentEpisode.ID); ep != nil {
							ep.Duration = duration
						}

						a.currentEpisode.Duration = duration

						// Update the duration in the episode list directly
						a.episodes.UpdateEpisodeDuration(a.currentEpisode.ID, duration)

						// Immediately update the UI to show the new duration
						if a.currentView == a.episodes {
							// Check for modals before updating
							if a.helpDialog.IsVisible() || a.confirmDialog.IsVisible() {
								a.draw()
							} else {
								a.episodes.UpdateCurrentEpisodePosition(a.screen)
								a.screen.Show()
							}
						}

						// Save immediately to persist the duration
						go func() {
							if err := a.subscriptions.Save(); err != nil {
								log.Printf("Failed to save episode duration: %v", err)
							}
						}()
					}
					break
				}
				time.Sleep(100 * time.Millisecond)
			}

			// Additional small delay to ensure mpv is ready for seeking
			time.Sleep(200 * time.Millisecond)

			// Seek to saved position
			seekSeconds := int(resumePosition.Seconds())
			log.Printf("Attempting to seek to position: %d seconds (%v)", seekSeconds, resumePosition)

			// Try seeking multiple times if it fails initially
			var err error
			for attempts := 0; attempts < 3; attempts++ {
				err = a.player.Seek(seekSeconds)
				if err == nil {
					log.Printf("Successfully resumed from position: %v on attempt %d", resumePosition, attempts+1)
					a.statusMessage = fmt.Sprintf("Resumed: %s at %s",
						episode.Title, a.formatTime(resumePosition))
					// Update the episode position to match what we seeked to
					a.currentEpisode.Position = resumePosition
					// Start position ticker after successful seek
					a.startPositionTicker()
					break
				}
				log.Printf("Seek attempt %d failed: %v", attempts+1, err)
				time.Sleep(500 * time.Millisecond)
			}

			if err != nil {
				log.Printf("Failed to resume from position after 3 attempts: %v", err)
				a.statusMessage = "Failed to resume from saved position"
				// Start position ticker even if seek failed
				a.startPositionTicker()
			}
		}()
	} else {
		log.Printf("Not resuming - Position: %v (either 0 or > 24h)", episode.Position)
		// Start position ticker immediately if no resume needed
		a.startPositionTicker()

		// Check and update duration for episodes starting from beginning
		go func() {
			// Wait a bit for the player to load the file
			time.Sleep(500 * time.Millisecond)

			if duration, err := a.player.GetDuration(); err == nil && duration > 0 {
				// Check if duration is significantly different
				durationDiff := a.currentEpisode.Duration - duration
				if durationDiff < 0 {
					durationDiff = -durationDiff
				}

				if a.currentEpisode.Duration == 0 || durationDiff > time.Second {
					log.Printf("Updating duration for episode '%s': %v -> %v",
						a.currentEpisode.Title, a.currentEpisode.Duration, duration)

					// Update the episode in the actual subscription data
					for _, podcast := range a.subscriptions.Podcasts {
						for _, ep := range podcast.Episodes {
							if ep.ID == a.currentEpisode.ID {
								ep.Duration = duration
								break
							}
						}
					}

					a.currentEpisode.Duration = duration

					// Update the duration in the episode list directly
					a.episodes.UpdateEpisodeDuration(a.currentEpisode.ID, duration)

					// Update the episode list view's reference
					a.episodes.SetCurrentEpisode(a.currentEpisode)

					// Immediately update the UI to show the new duration
					if a.currentView == a.episodes {
						// Check for modals before updating
						if a.helpDialog.IsVisible() || a.confirmDialog.IsVisible() {
							a.draw()
						} else {
							a.episodes.UpdateCurrentEpisodePosition(a.screen)
							a.screen.Show()
						}
					}

					// Save immediately to persist the duration
					if err := a.subscriptions.Save(); err != nil {
						log.Printf("Failed to save episode duration: %v", err)
					}
				}
			}
		}()
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

	// Find the canonical episode in subscription data using the index
	canonicalEpisode := a.subscriptions.GetEpisodeByID(episode.ID)

	// Use the canonical episode if found, otherwise use the provided one
	if canonicalEpisode != nil {
		a.currentEpisode = canonicalEpisode
		canonicalEpisode.Position = 0
		// Update the UI's reference to use the canonical episode
		a.episodes.SetCurrentEpisode(canonicalEpisode)
		a.queue.SetCurrentEpisode(canonicalEpisode)
	} else {
		a.currentEpisode = episode
		episode.Position = 0
		a.episodes.SetCurrentEpisode(episode)
		a.queue.SetCurrentEpisode(episode)
	}
	// Set current podcast for status bar display
	if a.currentView == a.episodes {
		a.currentPodcast = a.episodes.GetCurrentPodcast()
	} else {
		// Find the podcast that contains this episode
		a.currentPodcast = a.subscriptions.GetPodcastForEpisode(episode.ID)
	}

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

	// Redraw to update episode highlighting now that player state has changed
	a.draw()

	// Save the reset position immediately
	if err := a.subscriptions.Save(); err != nil {
		log.Printf("Failed to save position reset: %v", err)
	}

	// Start position ticker for restart
	a.startPositionTicker()
}

// addToQueue adds an episode to the playback queue
func (a *App) addToQueue(episode *models.Episode) {
	// Check if episode is already in queue
	if position := a.subscriptions.GetQueuePosition(episode.ID); position > 0 {
		// Episode is already in queue, just notify the user
		a.statusMessage = fmt.Sprintf("Episode already in queue at position %d", position)
		a.draw()
		return
	}

	err := a.subscriptions.AddToQueue(episode.ID)
	if err != nil {
		a.statusMessage = "Failed to add to queue: " + err.Error()
		a.draw()
		return
	}

	// Save subscriptions to persist queue
	if err := a.subscriptions.Save(); err != nil {
		log.Printf("Failed to save queue: %v", err)
	}

	// Update queue view if it's current
	if a.currentView == a.queue {
		a.queue.refresh()
	}

	// Update episode list to show queue indicators
	if a.currentView == a.episodes {
		a.episodes.updateTableRows()
	}

	// If queue was empty, start playing
	if len(a.subscriptions.Queue) == 1 {
		a.statusMessage = "Added to queue and starting playback"
		a.playEpisode(episode)
	} else {
		queuePos := a.subscriptions.GetQueuePosition(episode.ID)
		a.statusMessage = fmt.Sprintf("Added to queue (position %d)", queuePos)
	}

	a.draw()
}

// handleEpisodeCompletion handles the completion of an episode and advances the queue
func (a *App) handleEpisodeCompletion() {
	a.transitionMutex.Lock()
	defer a.transitionMutex.Unlock()
	
	// Double-check we still have a current episode and haven't already transitioned
	if a.currentEpisode == nil || a.transitionInProgress {
		return
	}
	
	// Set transition flag
	a.transitionInProgress = true
	defer func() {
		a.transitionInProgress = false
	}()
	
	// Check if there's a next episode in queue
	nextEpisode := a.subscriptions.GetNextInQueue()
	if nextEpisode != nil {
		log.Printf("Next episode in queue: %s", nextEpisode.Title)
		a.playNextInQueue()
	} else {
		// No more episodes in queue
		log.Printf("Episode ended, no more episodes in queue")
		a.stopCurrentEpisode()
	}
}

// playNextInQueue plays the next episode in the queue
func (a *App) playNextInQueue() {
	// Stop position ticker before transitioning to prevent race conditions
	a.stopPositionTicker()
	
	// Save current episode position before removing from queue
	if a.currentEpisode != nil {
		a.saveEpisodePosition()
	}
	
	// Atomically remove current episode from queue and get next
	var nextEpisode *models.Episode
	if a.currentEpisode != nil {
		// Remove current episode from queue
		a.subscriptions.RemoveFromQueue(a.currentEpisode.ID)
		
		// Get next episode before saving to ensure atomicity
		nextEpisode = a.subscriptions.GetNextInQueue()
		
		// Save subscriptions to persist queue changes
		if err := a.subscriptions.Save(); err != nil {
			log.Printf("Failed to save queue after removing episode: %v", err)
		}
		
		// Update UI only after queue operations are complete
		// Update queue view if visible
		if a.currentView == a.queue {
			a.queue.refresh()
		}
		// Update episode list to remove queue indicators
		if a.currentView == a.episodes {
			a.episodes.updateTableRows()
		}
		// Trigger UI redraw
		a.draw()
	} else {
		// No current episode, just get next
		nextEpisode = a.subscriptions.GetNextInQueue()
	}

	if nextEpisode == nil {
		a.statusMessage = "Queue finished"
		// Clear current episode state
		a.currentEpisode = nil
		a.currentPodcast = nil
		a.episodes.SetCurrentEpisode(nil)
		a.queue.SetCurrentEpisode(nil)
		a.draw()
		return
	}

	// Play next episode
	log.Printf("Auto-advancing to next episode in queue: %s", nextEpisode.Title)
	a.statusMessage = fmt.Sprintf("Playing next: %s", nextEpisode.Title)
	
	// Play the next episode
	a.playEpisode(nextEpisode)
}

// removeFromQueue removes an episode from the queue and handles playback logic
func (a *App) removeFromQueue(episode *models.Episode) {
	if episode == nil {
		return
	}

	// Use transition mutex to prevent concurrent transitions
	a.transitionMutex.Lock()
	defer a.transitionMutex.Unlock()

	// Check if a transition is already in progress
	if a.transitionInProgress {
		log.Printf("removeFromQueue: Transition already in progress, skipping")
		a.statusMessage = "Please wait for current transition to complete"
		a.draw()
		return
	}

	// Check if we're removing the currently playing episode
	isCurrentlyPlaying := a.currentEpisode != nil && a.currentEpisode.ID == episode.ID

	// If removing current episode, set transition flag
	if isCurrentlyPlaying {
		a.transitionInProgress = true
		defer func() {
			a.transitionInProgress = false
		}()
		
		// Reset completion flag to prevent automatic completion handling
		a.completionHandled.Store(false)
		
		// Stop position ticker before transition
		a.stopPositionTicker()
		
		// Save current episode position
		a.saveEpisodePosition()
	}

	// Remove from queue
	a.subscriptions.RemoveFromQueue(episode.ID)

	// Save subscriptions
	if err := a.subscriptions.Save(); err != nil {
		log.Printf("Failed to save queue after removing episode: %v", err)
	}

	// Update views
	if a.currentView == a.queue {
		a.queue.refresh()
	}
	if a.currentView == a.episodes {
		a.episodes.updateTableRows()
	}

	// Handle playback if we removed the currently playing episode
	if isCurrentlyPlaying {
		// Stop playback when manually removing the playing/paused episode
		log.Printf("removeFromQueue: Stopping playback of removed episode")
		a.statusMessage = "Stopped playback"
		a.draw()
		
		// Stop synchronously
		a.stopCurrentEpisode()
	} else {
		a.statusMessage = "Removed from queue"
		a.draw()
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
	} else if a.currentView == a.queue {
		// Get podcast from subscriptions for queue items
		if podcast := a.subscriptions.GetPodcastForEpisode(episode.ID); podcast != nil {
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
	} else if a.currentView == a.queue {
		// Get podcast from subscriptions for queue items
		if podcast := a.subscriptions.GetPodcastForEpisode(episode.ID); podcast != nil {
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

// selectEpisodeInList selects a specific episode in the episode list view
func (a *App) selectEpisodeInList(episodeID string) {
	if !a.episodes.SelectEpisodeByID(episodeID) {
		log.Printf("Episode %s not found in current episode list", episodeID)
	}
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
	existing.ConvertedDescription = updated.ConvertedDescription
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
			existingEp.ConvertedDescription = newEpisode.ConvertedDescription
			existingEp.URL = newEpisode.URL
			existingEp.PublishDate = newEpisode.PublishDate

			// Update duration only if existing is unknown or new duration is more accurate
			// Preserve discovered durations (they're more accurate than RSS feed data)
			if existingEp.Duration == 0 && newEpisode.Duration > 0 {
				// Only update if we don't have a duration yet
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

	// Update the episode index for all new/modified episodes
	for _, episode := range mergedEpisodes {
		a.subscriptions.UpdateEpisodeIndex(episode, existing)
	}
}

// startPositionTicker starts a ticker that updates the UI periodically when playing
func (a *App) startPositionTicker() {
	// Stop any existing ticker
	a.stopPositionTicker()

	// Create new ticker for position updates (every 500ms)
	a.positionTicker = time.NewTicker(500 * time.Millisecond)

	go func() {
		for range a.positionTicker.C {
			// Only update if playing
			if a.player.GetState() == player.StatePlaying {
				// Send non-blocking update signal
				select {
				case a.positionUpdate <- struct{}{}:
				default:
					// Channel full, skip this update
				}
			}
		}
	}()
}

// stopPositionTicker stops the position update ticker
func (a *App) stopPositionTicker() {
	if a.positionTicker != nil {
		a.positionTicker.Stop()
		a.positionTicker = nil
	}
}

// updateCurrentPosition updates the current episode's position from the player
func (a *App) updateCurrentPosition() {
	if a.currentEpisode != nil && a.player.GetState() != player.StateStopped {
		if position, err := a.player.GetPosition(); err == nil {
			// Update position in the actual subscription data using the index
			if episode := a.subscriptions.GetEpisodeByID(a.currentEpisode.ID); episode != nil {
				episode.Position = position

				// Also check and update duration if it has changed
				if duration, err := a.player.GetDuration(); err == nil && duration > 0 {
					if episode.Duration != duration {
						log.Printf("Updating duration in updateCurrentPosition: %v -> %v", episode.Duration, duration)
						episode.Duration = duration
						// Update the duration in the episode list directly
						a.episodes.UpdateEpisodeDuration(episode.ID, duration)
						// Also update our local reference
						a.currentEpisode.Duration = duration
						// Update the episode list view's reference
						a.episodes.SetCurrentEpisode(a.currentEpisode)
						a.queue.SetCurrentEpisode(a.currentEpisode)
					}
				}
			}

			// Also update our local reference
			a.currentEpisode.Position = position
		}
	}
}

// openNoteEditor opens nvim to edit notes for an episode
func (a *App) openNoteEditor(episode *models.Episode, podcastTitle string) {
	// Generate note file path
	notePath, err := a.generateNotePath(episode, podcastTitle)
	if err != nil {
		a.statusMessage = "Failed to generate note path: " + err.Error()
		return
	}
	
	// Prepare terminal arguments, replacing {file} placeholder
	args := make([]string, len(a.settings.TerminalArgs))
	for i, arg := range a.settings.TerminalArgs {
		if arg == "{file}" {
			args[i] = notePath
		} else {
			// Handle case where file path is embedded in argument (e.g., "nvim {file}")
			args[i] = strings.ReplaceAll(arg, "{file}", notePath)
		}
	}
	
	// Try configured terminal first
	var cmd *exec.Cmd
	if _, err := exec.LookPath(a.settings.Terminal); err == nil {
		cmd = exec.Command(a.settings.Terminal, args...)
	} else {
		// Fallback to common terminals if configured one is not found
		terminals := []struct {
			name string
			args []string
		}{
			{"kitty", []string{"nvim", notePath}},
			{"gnome-terminal", []string{"--", "nvim", notePath}},
			{"konsole", []string{"-e", "nvim", notePath}},
			{"xfce4-terminal", []string{"-e", "nvim " + notePath}},
			{"xterm", []string{"-e", "nvim", notePath}},
			{"alacritty", []string{"-e", "nvim", notePath}},
			{"wezterm", []string{"start", "--", "nvim", notePath}},
			{"foot", []string{"nvim", notePath}},
		}
		
		var termFound bool
		for _, term := range terminals {
			if _, err := exec.LookPath(term.name); err == nil {
				cmd = exec.Command(term.name, term.args...)
				termFound = true
				a.statusMessage = fmt.Sprintf("Terminal '%s' not found, using %s", a.settings.Terminal, term.name)
				log.Printf("Configured terminal '%s' not found, falling back to %s", a.settings.Terminal, term.name)
				break
			}
		}
		
		if !termFound {
			// Last resort fallbacks
			if _, err := exec.LookPath("x-terminal-emulator"); err == nil {
				cmd = exec.Command("x-terminal-emulator", "-e", "nvim", notePath)
			} else if _, err := exec.LookPath("open"); err == nil {
				// macOS - use Terminal.app
				cmd = exec.Command("open", "-a", "Terminal", notePath)
			} else {
				a.statusMessage = "No terminal emulator found"
				return
			}
		}
	}
	
	// Start the terminal in the background
	err = cmd.Start()
	
	if err != nil {
		a.statusMessage = "Failed to open editor: " + err.Error()
	} else {
		a.statusMessage = fmt.Sprintf("Editing note: %s", episode.Title)
	}
}

// generateNotePath creates the full path for a note file
func (a *App) generateNotePath(episode *models.Episode, podcastTitle string) (string, error) {
	// Use the same path generation as download manager to ensure consistency
	// Notes are stored alongside downloaded episodes
	downloadDir := a.downloadManager.GetDownloadDir()
	podcastDir := a.downloadManager.GeneratePodcastDirectory(podcastTitle)
	episodeFilename := a.downloadManager.GenerateFilename(episode)
	noteFilename := strings.TrimSuffix(episodeFilename, ".mp3") + ".md"
	noteDir := filepath.Join(downloadDir, podcastDir)
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(noteDir, 0755); err != nil {
		return "", err
	}
	
	return filepath.Join(noteDir, noteFilename), nil
}


// NoteExists checks if a note file exists for an episode
func NoteExists(episode *models.Episode, podcastTitle string, downloadManager *download.Manager) bool {
	if episode == nil || podcastTitle == "" || downloadManager == nil {
		return false
	}
	
	// Generate note path using download manager's methods
	downloadDir := downloadManager.GetDownloadDir()
	podcastDir := downloadManager.GeneratePodcastDirectory(podcastTitle)
	episodeFilename := downloadManager.GenerateFilename(episode)
	noteFilename := strings.TrimSuffix(episodeFilename, ".mp3") + ".md"
	notePath := filepath.Join(downloadDir, podcastDir, noteFilename)
	
	_, err := os.Stat(notePath)
	return err == nil
}
