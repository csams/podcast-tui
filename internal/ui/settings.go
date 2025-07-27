package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings holds the application UI settings
type Settings struct {
	// Terminal specifies the terminal emulator to use for editing notes
	// Default: "kitty"
	Terminal string `json:"terminal"`
	
	// TerminalArgs specifies the command-line arguments for the terminal
	// Use {file} as a placeholder for the file path
	// Default depends on the terminal
	TerminalArgs []string `json:"terminalArgs,omitempty"`
}

// DefaultSettings returns the default settings
func DefaultSettings() *Settings {
	return &Settings{
		Terminal:     "kitty",
		TerminalArgs: []string{"nvim", "{file}"},
	}
}

// LoadSettings loads the settings from the config directory
func LoadSettings(configDir string) (*Settings, error) {
	settingsPath := filepath.Join(configDir, "settings.json")
	
	// Check if settings file exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		// Return default settings if file doesn't exist
		return DefaultSettings(), nil
	}
	
	// Read settings file
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, err
	}
	
	// Parse settings
	settings := DefaultSettings()
	if err := json.Unmarshal(data, settings); err != nil {
		return nil, err
	}
	
	// Ensure terminal is set
	if settings.Terminal == "" {
		settings.Terminal = "kitty"
	}
	
	// Set default args based on terminal if not specified
	if len(settings.TerminalArgs) == 0 {
		settings.TerminalArgs = getDefaultTerminalArgs(settings.Terminal)
	}
	
	return settings, nil
}

// SaveSettings saves the settings to the config directory
func SaveSettings(configDir string, settings *Settings) error {
	settingsPath := filepath.Join(configDir, "settings.json")
	
	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	
	// Marshal settings with indentation for readability
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	
	// Add a newline at the end
	data = append(data, '\n')
	
	// Write settings file
	return os.WriteFile(settingsPath, data, 0644)
}

// getDefaultTerminalArgs returns the default arguments for a given terminal
func getDefaultTerminalArgs(terminal string) []string {
	switch terminal {
	case "gnome-terminal":
		return []string{"--", "nvim", "{file}"}
	case "konsole":
		return []string{"-e", "nvim", "{file}"}
	case "xfce4-terminal":
		return []string{"-e", "nvim {file}"}
	case "xterm":
		return []string{"-e", "nvim", "{file}"}
	case "alacritty":
		return []string{"-e", "nvim", "{file}"}
	case "kitty":
		return []string{"nvim", "{file}"}
	case "wezterm":
		return []string{"start", "--", "nvim", "{file}"}
	case "foot":
		return []string{"nvim", "{file}"}
	case "terminator":
		return []string{"-e", "nvim", "{file}"}
	case "tilix":
		return []string{"-e", "nvim", "{file}"}
	case "st":
		return []string{"-e", "nvim", "{file}"}
	case "urxvt":
		return []string{"-e", "nvim", "{file}"}
	case "rxvt":
		return []string{"-e", "nvim", "{file}"}
	default:
		// Generic fallback
		return []string{"-e", "nvim", "{file}"}
	}
}