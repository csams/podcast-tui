package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

type PlayerState int

const (
	StateStopped PlayerState = iota
	StatePlaying
	StatePaused
)

type Player struct {
	cmd        *exec.Cmd
	url        string
	state      PlayerState
	position   time.Duration
	duration   time.Duration
	volume     int
	speed      float64
	isMuted    bool
	mu         sync.Mutex
	stopCh     chan struct{}
	progressCh chan Progress
	socketPath string
	watchOnce  sync.Once
	eventConn  net.Conn
	eventStop  chan struct{}
}

type Progress struct {
	Position time.Duration
	Duration time.Duration
}

type mpvCommand struct {
	Command   []interface{} `json:"command"`
	RequestID int           `json:"request_id,omitempty"`
}

type mpvResponse struct {
	Data      interface{} `json:"data"`
	RequestID int         `json:"request_id"`
	Error     string      `json:"error"`
}

type mpvEvent struct {
	Event string      `json:"event"`
	ID    int         `json:"id,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

func New() *Player {
	p := &Player{
		progressCh: make(chan Progress, 1),
		socketPath: fmt.Sprintf("/tmp/mpv-socket-%d", os.Getpid()),
		volume:     100,
		speed:      1.0,
		state:      StateStopped,
	}
	
	// Clean up any stale socket from previous run
	os.Remove(p.socketPath)
	
	return p
}

// StartIdle starts mpv in idle mode, ready to play tracks instantly
func (p *Player) StartIdle() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Already running
	if p.cmd != nil && p.state != StateStopped {
		return nil
	}

	// Clean up any existing socket file
	os.Remove(p.socketPath)

	// Start mpv in idle mode
	p.cmd = exec.Command("mpv",
		"--no-video",
		"--really-quiet",
		"--no-terminal",
		fmt.Sprintf("--input-ipc-server=%s", p.socketPath),
		"--idle",
		"--force-window=no",
		"--keep-open=no",
	)

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mpv in idle mode: %w", err)
	}

	// Wait for mpv to create the socket with timeout
	socketReady := false
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(p.socketPath); err == nil {
			socketReady = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !socketReady {
		p.cmd.Process.Kill()
		p.cmd.Wait()
		p.cmd = nil
		return fmt.Errorf("mpv socket not created after timeout")
	}

	// mpv is now running in idle mode, ready to accept commands
	log.Println("mpv started in idle mode, ready for instant playback")
	return nil
}

// SwitchTrack switches to a new track without stopping mpv
func (p *Player) SwitchTrack(url string) error {
	p.mu.Lock()
	
	log.Printf("Player: SwitchTrack called - current state: %v, cmd: %v, url: %s", p.state, p.cmd != nil, url)
	
	if p.state == StateStopped || p.cmd == nil {
		// No player running, use regular Play
		log.Printf("Player: No active player, using regular Play")
		p.mu.Unlock()
		return p.Play(url)
	}

	// Update URL
	p.url = url
	p.position = 0
	p.duration = 0

	// Load the new file
	loadCmd := mpvCommand{
		Command: []interface{}{"loadfile", url},
	}

	resp, err := p.sendCommand(loadCmd)
	if err != nil {
		// If command fails, fallback to regular play
		log.Printf("Player: loadfile command failed: %v, falling back to Play", err)
		p.mu.Unlock()
		return p.Play(url)
	}
	
	// Check if loadfile succeeded
	if resp != nil && resp.Error != "success" && resp.Error != "" {
		log.Printf("Player: loadfile returned error: %s, falling back to Play", resp.Error)
		p.mu.Unlock()
		return p.Play(url)
	}

	// Ensure playback is not paused
	unpauseCmd := mpvCommand{
		Command: []interface{}{"set_property", "pause", false},
	}
	if _, err := p.sendCommand(unpauseCmd); err != nil {
		log.Printf("Warning: failed to unpause after track switch: %v", err)
	}
	
	// Reset state to playing (not paused)
	p.state = StatePlaying
	
	// Reset watch once to allow new progress watcher
	p.watchOnce = sync.Once{}
	
	// Start progress watcher for new track
	go p.watchProgress()
	
	log.Printf("Player: SwitchTrack completed successfully, state: %v", p.state)
	p.mu.Unlock()
	return nil
}

func (p *Player) Play(url string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If we have an idle mpv running, just load the file
	if p.cmd != nil && p.state == StateStopped {
		p.url = url
		p.position = 0
		p.duration = 0
		
		// Load the file
		loadCmd := mpvCommand{
			Command: []interface{}{"loadfile", url},
		}
		
		if _, err := p.sendCommand(loadCmd); err == nil {
			// Ensure playback is not paused
			unpauseCmd := mpvCommand{
				Command: []interface{}{"set_property", "pause", false},
			}
			if _, err := p.sendCommand(unpauseCmd); err != nil {
				log.Printf("Warning: failed to unpause after loading file: %v", err)
			}
			
			// Start event listener if not already running
			if p.eventConn == nil {
				if err := p.startEventListener(); err != nil {
					log.Printf("Warning: failed to start event listener: %v", err)
				}
			}
			
			p.state = StatePlaying
			go p.watchProgress()
			return nil
		}
		// If loading failed, fall through to start new mpv
	}

	if p.state != StateStopped {
		p.stop()
	}

	// Clean up any existing socket file
	os.Remove(p.socketPath)

	p.url = url
	p.position = 0
	p.duration = 0
	p.stopCh = make(chan struct{})
	p.watchOnce = sync.Once{}

	// Start mpv in idle mode first, then load the URL
	p.cmd = exec.Command("mpv",
		"--no-video",
		"--really-quiet",
		"--no-terminal",
		fmt.Sprintf("--input-ipc-server=%s", p.socketPath),
		"--idle",
		"--force-window=no",
		"--keep-open=no",  // Ensure mpv goes idle when file ends
	)

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start player: %w", err)
	}

	// Wait for mpv to create the socket with timeout
	socketReady := false
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(p.socketPath); err == nil {
			socketReady = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !socketReady {
		p.cmd.Process.Kill()
		p.cmd.Wait()
		return fmt.Errorf("mpv socket not created after timeout")
	}

	// Load the URL via IPC command
	loadCmd := mpvCommand{
		Command: []interface{}{"loadfile", url},
	}

	if _, err := p.sendCommand(loadCmd); err != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
		return fmt.Errorf("failed to load file: %w", err)
	}

	// Start event listener BEFORE verifying playback
	if err := p.startEventListener(); err != nil {
		log.Printf("Warning: failed to start event listener: %v", err)
	}

	// Verify playback actually started
	verifyCmd := mpvCommand{
		Command: []interface{}{"get_property", "idle-active"},
	}
	resp, err := p.sendCommand(verifyCmd)
	if err == nil && resp.Data == false {
		p.state = StatePlaying
		go p.watchProgress()
		return nil
	}

	// Playback failed to start
	p.cmd.Process.Kill()
	p.cmd.Wait()
	return fmt.Errorf("failed to start playback")
}

// sendCommand sends a command to mpv via IPC socket
func (p *Player) sendCommand(cmd mpvCommand) (*mpvResponse, error) {
	conn, err := net.Dial("unix", p.socketPath)
	if err != nil {
		// If we can't connect, the player process likely died
		p.mu.Lock()
		p.state = StateStopped
		p.mu.Unlock()
		return nil, fmt.Errorf("failed to connect to mpv socket: %w", err)
	}
	defer conn.Close()

	// Set a timeout for the connection
	conn.SetDeadline(time.Now().Add(2 * time.Second))

	// Send command
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}
	data = append(data, '\n')

	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write command: %w", err)
	}

	// Read response
	reader := bufio.NewReader(conn)
	responseData, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response mpvResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if response.Error != "" && response.Error != "success" {
		return &response, fmt.Errorf("mpv error: %s", response.Error)
	}

	return &response, nil
}

func (p *Player) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != StatePlaying {
		return nil
	}

	cmd := mpvCommand{
		Command: []interface{}{"set_property", "pause", true},
	}

	if _, err := p.sendCommand(cmd); err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}

	p.state = StatePaused
	return nil
}

func (p *Player) Resume() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != StatePaused {
		return nil
	}

	cmd := mpvCommand{
		Command: []interface{}{"set_property", "pause", false},
	}

	if _, err := p.sendCommand(cmd); err != nil {
		return fmt.Errorf("failed to resume: %w", err)
	}

	p.state = StatePlaying
	return nil
}

func (p *Player) TogglePause() error {
	if p.state == StatePaused {
		return p.Resume()
	}
	return p.Pause()
}

func (p *Player) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.stop()
}

// Cleanup ensures all resources are properly released
// This should be called when the application is shutting down
func (p *Player) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Force stop if still running
	if p.cmd != nil {
		p.stop()
	}
	
	// Final cleanup of socket file
	os.Remove(p.socketPath)
}

// StopKeepIdle stops playback but keeps mpv running in idle mode
func (p *Player) StopKeepIdle() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopped {
		return nil
	}

	// Stop playback but keep mpv running
	stopCmd := mpvCommand{
		Command: []interface{}{"stop"},
	}
	
	if _, err := p.sendCommand(stopCmd); err != nil {
		// If command fails, fallback to full stop
		return p.stop()
	}

	// Mark as stopped
	p.state = StateStopped
	
	// Signal watch goroutine to stop
	if p.stopCh != nil {
		select {
		case <-p.stopCh:
			// Already closed
		default:
			close(p.stopCh)
		}
	}

	// Stop event listener
	if p.eventStop != nil {
		select {
		case <-p.eventStop:
			// Already closed
		default:
			close(p.eventStop)
		}
	}
	if p.eventConn != nil {
		p.eventConn.Close()
		p.eventConn = nil
	}

	// Reset playback state
	p.position = 0
	p.duration = 0
	p.url = ""

	return nil
}

func (p *Player) stop() error {
	if p.state == StateStopped && p.cmd == nil {
		return nil
	}

	// Mark as stopped first to prevent new operations
	p.state = StateStopped

	// Signal watch goroutine to stop
	if p.stopCh != nil {
		select {
		case <-p.stopCh:
			// Already closed
		default:
			close(p.stopCh)
		}
	}

	// Stop event listener
	if p.eventStop != nil {
		select {
		case <-p.eventStop:
			// Already closed
		default:
			close(p.eventStop)
		}
	}
	if p.eventConn != nil {
		p.eventConn.Close()
		p.eventConn = nil
	}

	if p.cmd != nil && p.cmd.Process != nil {
		// Try graceful quit first
		quitCmd := mpvCommand{
			Command: []interface{}{"quit"},
		}
		p.sendCommand(quitCmd)
		
		// Give it a moment to quit gracefully
		done := make(chan error, 1)
		go func() {
			done <- p.cmd.Wait()
		}()
		
		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(500 * time.Millisecond):
			// Force kill if not exited
			log.Printf("Force killing mpv process (pid: %d)", p.cmd.Process.Pid)
			if err := p.cmd.Process.Kill(); err != nil {
				log.Printf("Error killing mpv process: %v", err)
			}
			<-done // Wait for process to exit
		}
		
		// Ensure process is really dead by checking ProcessState
		if p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited() {
			log.Printf("Warning: mpv process may not have exited cleanly")
		}
	}

	// Clean up socket file - try multiple times in case it's still in use
	for i := 0; i < 3; i++ {
		if err := os.Remove(p.socketPath); err == nil || os.IsNotExist(err) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	// Reset state
	p.cmd = nil
	p.url = ""
	p.position = 0
	p.duration = 0

	log.Printf("Player stopped and cleaned up")
	return nil
}

func (p *Player) Seek(seconds int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopped {
		return nil
	}

	// For resuming playback, use absolute seek if the value is large
	// This assumes values > 300 (5 minutes) are absolute positions for resuming
	seekType := "relative"
	if seconds > 300 {
		seekType = "absolute"
	}
	
	// For relative seeks, check bounds
	if seekType == "relative" {
		// Get current position
		newPosition := p.position + time.Duration(seconds)*time.Second
		
		// Check if seek would go before start
		if newPosition < 0 {
			// Seek to beginning instead
			cmd := mpvCommand{
				Command: []interface{}{"seek", 0, "absolute"},
			}
			if _, err := p.sendCommand(cmd); err != nil {
				return fmt.Errorf("failed to seek to beginning: %w", err)
			}
			return nil
		}
		
		// Check if seek would go past end
		if p.duration > 0 && newPosition > p.duration {
			// Seek to near end instead (1 second before end)
			targetSeconds := int(p.duration.Seconds()) - 1
			if targetSeconds < 0 {
				targetSeconds = 0
			}
			cmd := mpvCommand{
				Command: []interface{}{"seek", targetSeconds, "absolute"},
			}
			if _, err := p.sendCommand(cmd); err != nil {
				return fmt.Errorf("failed to seek to end: %w", err)
			}
			return nil
		}
	}

	cmd := mpvCommand{
		Command: []interface{}{"seek", seconds, seekType},
	}

	if _, err := p.sendCommand(cmd); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	return nil
}

// SeekAbsolute seeks to an absolute position in seconds
func (p *Player) SeekAbsolute(seconds int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopped {
		return nil
	}

	// Bounds checking
	if seconds < 0 {
		seconds = 0
	} else if p.duration > 0 {
		maxSeconds := int(p.duration.Seconds())
		if seconds >= maxSeconds {
			// Don't seek past the end - stop 1 second before
			seconds = maxSeconds - 1
			if seconds < 0 {
				seconds = 0
			}
		}
	}

	cmd := mpvCommand{
		Command: []interface{}{"seek", seconds, "absolute"},
	}

	if _, err := p.sendCommand(cmd); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	return nil
}

// Volume control methods
func (p *Player) GetVolume() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopped {
		return p.volume, nil
	}

	cmd := mpvCommand{
		Command: []interface{}{"get_property", "volume"},
	}

	resp, err := p.sendCommand(cmd)
	if err != nil {
		return p.volume, err
	}

	if vol, ok := resp.Data.(float64); ok {
		p.volume = int(vol)
	}

	return p.volume, nil
}

func (p *Player) SetVolume(volume int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if volume < 0 {
		volume = 0
	} else if volume > 100 {
		volume = 100
	}

	p.volume = volume

	if p.state == StateStopped {
		return nil
	}

	cmd := mpvCommand{
		Command: []interface{}{"set_property", "volume", volume},
	}

	if _, err := p.sendCommand(cmd); err != nil {
		return fmt.Errorf("failed to set volume: %w", err)
	}

	return nil
}

// Speed control methods
func (p *Player) GetSpeed() (float64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopped {
		return p.speed, nil
	}

	cmd := mpvCommand{
		Command: []interface{}{"get_property", "speed"},
	}

	resp, err := p.sendCommand(cmd)
	if err != nil {
		return p.speed, err
	}

	if speed, ok := resp.Data.(float64); ok {
		p.speed = speed
	}

	return p.speed, nil
}

func (p *Player) SetSpeed(speed float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if speed < 0.25 {
		speed = 0.25
	} else if speed > 4.0 {
		speed = 4.0
	}

	p.speed = speed

	if p.state == StateStopped {
		return nil
	}

	cmd := mpvCommand{
		Command: []interface{}{"set_property", "speed", speed},
	}

	if _, err := p.sendCommand(cmd); err != nil {
		return fmt.Errorf("failed to set speed: %w", err)
	}

	return nil
}

// Mute control
func (p *Player) IsMuted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isMuted
}

func (p *Player) ToggleMute() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.isMuted = !p.isMuted

	if p.state == StateStopped {
		return nil
	}

	cmd := mpvCommand{
		Command: []interface{}{"set_property", "mute", p.isMuted},
	}

	if _, err := p.sendCommand(cmd); err != nil {
		return fmt.Errorf("failed to toggle mute: %w", err)
	}

	return nil
}

// Progress tracking methods
func (p *Player) GetPosition() (time.Duration, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopped {
		return p.position, nil
	}

	cmd := mpvCommand{
		Command: []interface{}{"get_property", "time-pos"},
	}

	resp, err := p.sendCommand(cmd)
	if err != nil {
		// If property is unavailable, return cached position
		if resp != nil && resp.Error == "property unavailable" {
			return p.position, nil
		}
		return p.position, err
	}

	if pos, ok := resp.Data.(float64); ok && pos >= 0 {
		p.position = time.Duration(pos * float64(time.Second))
	}

	return p.position, nil
}

func (p *Player) GetDuration() (time.Duration, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopped {
		return p.duration, nil
	}

	cmd := mpvCommand{
		Command: []interface{}{"get_property", "duration"},
	}

	resp, err := p.sendCommand(cmd)
	if err != nil {
		// If property is unavailable, return cached duration
		if resp != nil && resp.Error == "property unavailable" {
			return p.duration, nil
		}
		return p.duration, err
	}

	if dur, ok := resp.Data.(float64); ok && dur > 0 {
		p.duration = time.Duration(dur * float64(time.Second))
	}

	return p.duration, nil
}

func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state == StatePlaying
}

func (p *Player) IsPaused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state == StatePaused
}

func (p *Player) GetState() PlayerState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

func (p *Player) Progress() chan Progress {
	return p.progressCh
}

func (p *Player) watchProgress() {
	// Ensure we only run one watch goroutine
	p.watchOnce.Do(func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				// Check if we should still be watching
				p.mu.Lock()
				currentState := p.state
				p.mu.Unlock()
				
				if currentState == StatePlaying || currentState == StatePaused {
					// Get position
					cmd := mpvCommand{
						Command: []interface{}{"get_property", "time-pos"},
					}

					if resp, err := p.sendCommand(cmd); err == nil {
						if pos, ok := resp.Data.(float64); ok && pos >= 0 {
							p.mu.Lock()
							p.position = time.Duration(pos * float64(time.Second))
							p.mu.Unlock()
						}
					} else if err != nil {
						// Check if mpv process died
						if p.cmd != nil && p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
							p.mu.Lock()
							p.state = StateStopped
							p.mu.Unlock()
							log.Printf("Player: mpv process died unexpectedly")
							return
						}
					}

					// Get duration
					cmd = mpvCommand{
						Command: []interface{}{"get_property", "duration"},
					}

					if resp, err := p.sendCommand(cmd); err == nil {
						if dur, ok := resp.Data.(float64); ok && dur > 0 {
							p.mu.Lock()
							p.duration = time.Duration(dur * float64(time.Second))
							p.mu.Unlock()
						}
					}
				}

			p.mu.Lock()
			progress := Progress{
				Position: p.position,
				Duration: p.duration,
			}
			p.mu.Unlock()

			select {
			case p.progressCh <- progress:
			default:
				}
			}
		}
	})
}

// startEventListener starts listening for mpv events
func (p *Player) startEventListener() error {
	// Connect to mpv socket for events
	conn, err := net.Dial("unix", p.socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect for events: %w", err)
	}

	// Enable event notifications
	enableCmd := mpvCommand{
		Command: []interface{}{"enable_event", "end-file"},
	}
	data, _ := json.Marshal(enableCmd)
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		conn.Close()
		return fmt.Errorf("failed to enable events: %w", err)
	}

	// Store connection and start event handler
	p.eventConn = conn
	p.eventStop = make(chan struct{})
	go p.handleEvents()

	return nil
}

// handleEvents processes mpv events
func (p *Player) handleEvents() {
	if p.eventConn == nil {
		return
	}
	defer p.eventConn.Close()

	reader := bufio.NewReader(p.eventConn)
	for {
		select {
		case <-p.eventStop:
			return
		default:
			// Set read timeout
			p.eventConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			
			line, err := reader.ReadBytes('\n')
			if err != nil {
				// Timeout is normal, continue
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				// Other errors mean connection is broken
				log.Printf("Event reader error: %v", err)
				return
			}

			var event mpvEvent
			if err := json.Unmarshal(line, &event); err != nil {
				continue // Skip malformed events
			}

			// Handle end-file event
			if event.Event == "end-file" {
				log.Printf("Player: Received end-file event")
				
				// Send final progress with position = duration
				p.mu.Lock()
				if p.duration > 0 {
					p.position = p.duration
				}
				finalProgress := Progress{
					Position: p.position,
					Duration: p.duration,
				}
				p.mu.Unlock()

				// Send progress update immediately
				select {
				case p.progressCh <- finalProgress:
					log.Printf("Player: Sent final progress (pos=%v, dur=%v)", finalProgress.Position, finalProgress.Duration)
				default:
					log.Printf("Player: Failed to send final progress (channel full)")
				}

				// Wait a bit for app to process, then update state
				time.Sleep(200 * time.Millisecond)
				
				p.mu.Lock()
				p.state = StateStopped
				p.mu.Unlock()
			}
		}
	}
}
