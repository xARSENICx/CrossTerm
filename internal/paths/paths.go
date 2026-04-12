package paths

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// AppDataDir returns the base directory for Crossterm data depending on OS.
func AppDataDir() string {
	// Standardize on using UserConfigDir where possible:
	// Windows: %AppData%\crossterm
	// macOS/Linux CLI standard: ~/.crossterm
	
	home, err := os.UserHomeDir()
	if err == nil {
		if runtime.GOOS == "windows" {
			configDir, err := os.UserConfigDir()
			if err == nil {
				return filepath.Join(configDir, "crossterm")
			}
			return filepath.Join(home, "AppData", "Roaming", "crossterm")
		}
		// Mac/Linux
		return filepath.Join(home, ".crossterm")
	}
	
	// Absolute last resort fallback to local directory
	return "data"
}

// PuzzlesDir returns the absolute path to the puzzles directory, ensuring it exists.
func PuzzlesDir() string {
	path := filepath.Join(AppDataDir(), "puzzles")
	ensureDir(path)
	return path
}

// SavesDir returns the absolute path to the saves directory, ensuring it exists.
func SavesDir() string {
	path := filepath.Join(AppDataDir(), "saves")
	ensureDir(path)
	return path
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		log.Printf("Warning: automatically creating directory %s failed: %v", path, err)
	}
}
