package ui

import (
	"fmt"
	"time"

	"github.com/csams/podcast-tui/internal/download"
	"github.com/csams/podcast-tui/internal/models"
	"github.com/csams/podcast-tui/internal/player"
	"github.com/gdamore/tcell/v2"
)

type QueueView struct {
	table           *Table
	subscriptions   *models.Subscriptions
	episodes        []*models.Episode
	downloadManager *download.Manager
	player          *player.Player
	currentEpisode  *models.Episode
}

type QueueTableRow struct {
	episode         *models.Episode
	podcast         *models.Podcast
	queuePosition   int
	currentEpisode  *models.Episode
	player          *player.Player
	downloadManager *download.Manager
}

func NewQueueView() *QueueView {
	v := &QueueView{
		table: NewTable(),
	}

	// Configure table columns with podcast title
	columns := []TableColumn{
		{Title: "Local", Width: 16, Align: AlignLeft},                       // Status/Download
		{Title: "Podcast", MinWidth: 15, FlexWeight: 0.4, Align: AlignLeft}, // Podcast title
		{Title: "Episode", MinWidth: 20, FlexWeight: 0.6, Align: AlignLeft}, // Episode title
		{Title: "Date", Width: 10, Align: AlignLeft},                        // Date
		{Title: "Position", Width: 17, Align: AlignLeft},                    // Position
	}
	v.table.SetColumns(columns)

	return v
}

func (v *QueueView) SetSubscriptions(subs *models.Subscriptions) {
	v.subscriptions = subs
	v.refresh()
}

func (v *QueueView) SetDownloadManager(dm *download.Manager) {
	v.downloadManager = dm
}

func (v *QueueView) SetPlayer(p *player.Player) {
	v.player = p
}

func (v *QueueView) SetCurrentEpisode(episode *models.Episode) {
	v.currentEpisode = episode
	v.refresh()
}

func (v *QueueView) refresh() {
	if v.subscriptions == nil {
		return
	}

	// Get episodes from queue
	v.episodes = v.subscriptions.GetQueueEpisodes()

	// Convert to table rows
	rows := make([]TableRow, len(v.episodes))
	for i, episode := range v.episodes {
		// Get the podcast for this episode
		podcast := v.subscriptions.GetPodcastForEpisode(episode.ID)

		row := &QueueTableRow{
			episode:         episode,
			podcast:         podcast,
			queuePosition:   i + 1,
			currentEpisode:  v.currentEpisode,
			player:          v.player,
			downloadManager: v.downloadManager,
		}

		rows[i] = row
	}

	v.table.SetRows(rows)
}

func (v *QueueView) isCurrentlyPlaying(episode *models.Episode) bool {
	if v.player == nil {
		return false
	}
	// Check if currently playing by comparing episode ID
	return v.currentEpisode != nil && v.currentEpisode.ID == episode.ID && v.player.GetState() == player.StatePlaying
}

func (v *QueueView) isCurrentlyPaused(episode *models.Episode) bool {
	if v.player == nil {
		return false
	}
	// Check if currently paused by comparing episode ID
	return v.currentEpisode != nil && v.currentEpisode.ID == episode.ID && v.player.GetState() == player.StatePaused
}


func (v *QueueView) Draw(s tcell.Screen) {
	width, height := s.Size()

	// Draw header
	headerText := "Episode Queue"
	if len(v.episodes) > 0 {
		headerText = fmt.Sprintf("Episode Queue (%d)", len(v.episodes))
	}
	drawText(s, 0, 0, tcell.StyleDefault.Bold(true), headerText)
	for x := 0; x < width; x++ {
		s.SetContent(x, 1, '─', nil, tcell.StyleDefault)
	}

	// Table starts below header
	v.table.SetPosition(0, 2)
	v.table.SetSize(width, height-3) // Leave room for header and status bar
	v.table.Draw(s)
}

func (v *QueueView) HandleKey(ev *tcell.EventKey) bool {
	// Normal mode key handling
	switch ev.Key() {
	case tcell.KeyRune:
		// Check for Alt+j and Alt+k
		if ev.Modifiers()&tcell.ModAlt != 0 {
			switch ev.Rune() {
			case 'j':
				// Move episode down in queue
				return true
			case 'k':
				// Move episode up in queue
				return true
			}
		}

		switch ev.Rune() {
		case 'j':
			return v.table.SelectNext()
		case 'k':
			return v.table.SelectPrevious()
		case 'g':
			v.table.SelectFirst()
			return true
		case 'G':
			v.table.SelectLast()
			return true
		case 'u':
			// Remove selected episode from queue - handled by app
			return true
		}
	case tcell.KeyCtrlD:
		return v.table.PageDown()
	case tcell.KeyCtrlU:
		return v.table.PageUp()
	case tcell.KeyEnter:
		// Play selected episode immediately
		if row := v.table.GetSelectedRow(); row != nil {
			// This will be handled by the app
			return true
		}
	}

	return false
}

func (v *QueueView) GetSelected() *models.Episode {
	row := v.table.GetSelectedRow()
	if row != nil {
		if qRow, ok := row.(*QueueTableRow); ok {
			return qRow.episode
		}
	}
	return nil
}

func (v *QueueView) GetSelectedIndex() int {
	return v.table.selectedIdx
}

func (v *QueueView) UpdateCurrentEpisodePosition(s tcell.Screen) {
	// Update the position display for the currently playing episode
	if v.player == nil {
		return
	}

	// Get current position and duration
	position, _ := v.player.GetPosition()
	duration, _ := v.player.GetDuration()
	for i, row := range v.table.rows {
		if qRow, ok := row.(*QueueTableRow); ok {
			// Check if this is the currently playing episode
			if qRow.currentEpisode != nil && qRow.episode.ID == qRow.currentEpisode.ID &&
				(v.player.GetState() == player.StatePlaying || v.player.GetState() == player.StatePaused) {
				// Update the episode position
				qRow.episode.Position = position
				qRow.episode.Duration = duration

				// Redraw just this row
				v.table.drawRow(s, v.table.y+v.table.headerHeight+i-v.table.scrollOffset, row, i == v.table.selectedIdx)
				return
			}
		}
	}
}

// QueueTableRow implementation

func (r *QueueTableRow) GetCell(columnIndex int) string {
	switch columnIndex {
	case 0: // Status column
		status := fmt.Sprintf("%d", r.queuePosition)

		// Add download/playback indicators after position
		if r.currentEpisode != nil && r.episode.ID == r.currentEpisode.ID && r.player != nil {
			if r.player.GetState() == player.StatePlaying {
				status += " ▶"
			} else if r.player.GetState() == player.StatePaused {
				status += " ⏸"
			}
		}

		// Check download status dynamically
		if r.downloadManager != nil {
			// First check if it's downloaded
			podcastTitle := ""
			if r.podcast != nil {
				podcastTitle = r.podcast.Title
			}
			if r.downloadManager.IsEpisodeDownloaded(r.episode, podcastTitle) {
				status += " ✔"
			} else if r.downloadManager.IsDownloading(r.episode.ID) {
				// Check download progress
				if progress, exists := r.downloadManager.GetDownloadProgress(r.episode.ID); exists {
					switch progress.Status {
					case download.StatusDownloading:
						progressPct := int(progress.Progress * 100)
						status += fmt.Sprintf(" [⬇%d%%]", progressPct)
					case download.StatusQueued:
						status += " [⏸]"
					case download.StatusFailed:
						status += " [⚠]"
					default:
						status += " [⬇]"
					}
				} else {
					status += " [⬇]"
				}
			} else {
				// Check for failed downloads
				if progress, exists := r.downloadManager.GetDownloadProgress(r.episode.ID); exists {
					if progress.Status == download.StatusFailed {
						status += " [⚠]"
					}
				}
			}
		}

		// Check for notes
		if r.noteExists() {
			status += " ✎"
		}

		return status
	case 1: // Podcast
		if r.podcast != nil {
			return r.podcast.Title
		}
		return "Unknown"
	case 2: // Episode Title
		return r.episode.Title
	case 3: // Date
		return r.episode.PublishDate.Format("2006-01-02")
	case 4: // Position
		if r.episode.Duration > 0 {
			position := r.episode.Position
			if position < 0 {
				position = 0
			}
			return fmt.Sprintf("%s/%s", formatQueueDuration(position), formatQueueDuration(r.episode.Duration))
		}
		return ""
	default:
		return ""
	}
}

func (r *QueueTableRow) GetCellStyle(columnIndex int, selected bool) *tcell.Style {
	// Style the status column if episode is downloaded
	if columnIndex == 0 && r.downloadManager != nil {
		podcastTitle := ""
		if r.podcast != nil {
			podcastTitle = r.podcast.Title
		}
		if r.downloadManager.IsEpisodeDownloaded(r.episode, podcastTitle) {
			style := tcell.StyleDefault.Foreground(tcell.ColorGreen)
			if selected {
				style = style.Background(ColorSelection)
			}
			// Check if this is the currently playing/paused episode
			if r.currentEpisode != nil && r.episode.ID == r.currentEpisode.ID && r.player != nil {
				if !selected {
					if r.player.GetState() == player.StatePlaying {
						style = style.Background(ColorBlue7)
					} else if r.player.GetState() == player.StatePaused {
						style = style.Background(ColorBlue7)
					}
				}
			}
			return &style
		}
	}
	
	// Check if this is the currently playing/paused episode
	if r.currentEpisode != nil && r.episode.ID == r.currentEpisode.ID && r.player != nil {
		if !selected {
			if r.player.GetState() == player.StatePlaying {
				style := tcell.StyleDefault.Background(ColorBlue7).Foreground(ColorGreen)
				return &style
			} else if r.player.GetState() == player.StatePaused {
				style := tcell.StyleDefault.Background(ColorBlue7).Foreground(ColorYellow)
				return &style
			}
		}
	}
	return nil
}

func (r *QueueTableRow) GetHighlightPositions(columnIndex int) []int {
	// Search is disabled in queue view
	return nil
}

func formatQueueDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

func (r *QueueTableRow) noteExists() bool {
	// Use the shared NoteExists function
	if r.podcast != nil {
		return NoteExists(r.episode, r.podcast.Title, r.downloadManager)
	}
	return false
}
