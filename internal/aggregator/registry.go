package aggregator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Aggregator defines a vendored puzzle source.
type Aggregator struct {
	Name       string                          // Display name, e.g., "The Hindu Crossword"
	ScriptDir  string                          // Relative path to vendored repo
	EntryPoint string                          // Main script filename
	OutputDir  string                          // Where .puz files are copied to
	InputLabel string                          // Prompt label for user input, e.g., "Enter date (DD/MM/YYYY):"
	Args       func(userInput string) []string // Build CLI args from user input
}

var registry []Aggregator

// Register adds an aggregator to the global registry.
func Register(agg Aggregator) {
	registry = append(registry, agg)
}

// GetAll returns all registered aggregators.
func GetAll() []Aggregator {
	return registry
}

// Run executes an aggregator script and copies the output .puz to the target directory.
// Returns the path to the copied .puz file, or an error.
func Run(agg Aggregator, userInput string) (string, error) {
	args := agg.Args(userInput)
	cmdArgs := append([]string{agg.EntryPoint}, args...)

	cmd := exec.Command("python3", cmdArgs...)
	cmd.Dir = agg.ScriptDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("aggregator failed: %s\n%s", err, string(output))
	}

	// Ensure output directory exists
	if err := os.MkdirAll(agg.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output dir: %w", err)
	}

	// Find generated .puz files in the aggregator's output folder
	aggOutputDir := filepath.Join(agg.ScriptDir, "Puzzles")
	entries, err := os.ReadDir(aggOutputDir)
	if err != nil {
		return "", fmt.Errorf("no output from aggregator: %w", err)
	}

	var copiedPath string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".puz") {
			src := filepath.Join(aggOutputDir, entry.Name())
			dst := filepath.Join(agg.OutputDir, entry.Name())

			data, err := os.ReadFile(src)
			if err != nil {
				return "", fmt.Errorf("failed to read generated file: %w", err)
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				return "", fmt.Errorf("failed to copy to puzzle dir: %w", err)
			}

			copiedPath = dst
			// Remove from aggregator's output to avoid stale copies
			os.Remove(src)
		}
	}

	if copiedPath == "" {
		return "", fmt.Errorf("aggregator ran but produced no .puz file.\nOutput: %s", string(output))
	}

	return copiedPath, nil
}

// EnsureDeps installs Python dependencies for an aggregator if requirements.txt exists.
func EnsureDeps(agg Aggregator) error {
	reqPath := filepath.Join(agg.ScriptDir, "requirements.txt")
	if _, err := os.Stat(reqPath); os.IsNotExist(err) {
		return nil // No requirements file, nothing to do
	}

	cmd := exec.Command("pip3", "install", "-r", "requirements.txt", "--quiet", "--break-system-packages")
	cmd.Dir = agg.ScriptDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install deps: %s\n%s", err, string(output))
	}
	return nil
}
