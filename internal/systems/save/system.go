package savesystem

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"crossterm/internal/engine"
	"crossterm/internal/paths"
)

type SaveSystem struct {
	EventBus *engine.EventBus
	State    *engine.GameState
}

type SavedCellMeta struct {
	CheckedCorrect bool   `json:"cc,omitempty"`
	WasChecked     bool   `json:"wc,omitempty"`
	WrongGuesses   string `json:"wg,omitempty"`
}

type SaveData struct {
	Answers []string          `json:"answers"` // Each row as a string
	Meta    [][]SavedCellMeta `json:"meta,omitempty"` // Parallel grid for metadata
}

func NewSaveSystem(eb *engine.EventBus, state *engine.GameState) *SaveSystem {
	return &SaveSystem{
		EventBus: eb,
		State:    state,
	}
}

func getSaveFileName(title, author string) string {
	hash := sha256.Sum256([]byte(title + author))
	return filepath.Join(paths.SavesDir(), hex.EncodeToString(hash[:16])+".json")
}

func (s *SaveSystem) Run() {
	sub := s.EventBus.Subscribe(engine.EventStateUpdate)
	stop := s.EventBus.Subscribe(engine.EventShutdown)
	
	for {
		select {
		case <-stop:
			return
		case <-sub:
			s.saveState()
		}
	}
}

func (s *SaveSystem) Load() {
	if s.State.Puzzle == nil || s.State.Puzzle.Grid == nil {
		return
	}
	path := getSaveFileName(s.State.Puzzle.Title, s.State.Puzzle.Author)
	data, err := os.ReadFile(path)
	if err != nil {
		return // Save doesn't exist
	}
	
	var save SaveData
	if err := json.Unmarshal(data, &save); err == nil {
		grid := s.State.Puzzle.Grid
		for y := 0; y < grid.Height && y < len(save.Answers); y++ {
			rowStr := save.Answers[y]
			var metaRow []SavedCellMeta
			if y < len(save.Meta) {
				metaRow = save.Meta[y]
			}
			
			for x := 0; x < grid.Width && x < len(rowStr); x++ {
				char := rowStr[x]
				if char != ' ' && char != '_' {
					if !grid.Cells[y][x].IsBlack {
						grid.Cells[y][x].Value = char
					}
				} else if char == ' ' {
					if !grid.Cells[y][x].IsBlack {
						grid.Cells[y][x].Value = 0 // Explicitly empty
					}
				}
				
				if x < len(metaRow) {
					m := metaRow[x]
					grid.Cells[y][x].CheckedCorrect = m.CheckedCorrect
					grid.Cells[y][x].WasChecked = m.WasChecked
					grid.Cells[y][x].WrongGuesses = []byte(m.WrongGuesses)
				}
			}
		}
	}
}

func (s *SaveSystem) saveState() {
	if s.State.Puzzle == nil || s.State.Puzzle.Grid == nil {
		return
	}
	grid := s.State.Puzzle.Grid
	
	save := SaveData{
		Answers: make([]string, grid.Height),
		Meta:    make([][]SavedCellMeta, grid.Height),
	}
	
	for y := 0; y < grid.Height; y++ {
		row := make([]byte, grid.Width)
		metaRow := make([]SavedCellMeta, grid.Width)
		for x := 0; x < grid.Width; x++ {
			c := grid.Cells[y][x]
			if c.IsBlack {
				row[x] = '_'
			} else if c.Value != 0 {
				row[x] = c.Value
			} else {
				row[x] = ' '
			}
			metaRow[x] = SavedCellMeta{
				CheckedCorrect: c.CheckedCorrect,
				WasChecked:     c.WasChecked,
				WrongGuesses:   string(c.WrongGuesses),
			}
		}
		save.Answers[y] = string(row)
		save.Meta[y] = metaRow
	}
	
	path := getSaveFileName(s.State.Puzzle.Title, s.State.Puzzle.Author)
	
	// Ensure directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
	
	data, err := json.MarshalIndent(save, "", "  ")
	if err == nil {
		os.WriteFile(path, data, 0644)
	}
}
