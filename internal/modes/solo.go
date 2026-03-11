package modes

import (
	"crossterm/internal/engine"
	"fmt"
	"time"
)

type SoloMode struct{}

func NewSoloMode() *SoloMode {
	return &SoloMode{}
}

func init() {
	// Register variations of solo modes
	s := NewSoloMode()
	Register("not_timed_standard", s)
	Register("not_timed_checks", s)
	Register("not_timed_tools", s)
	Register("timed_standard", s)
	Register("timed_checks", s)
	Register("timed_tools", s)
}

func (m *SoloMode) Name() string {
	return "Solo"
}

func (m *SoloMode) ProcessEvent(eb *engine.EventBus, state *engine.GameState, evt engine.Event) {
	if state.IsFinished {
		return
	}

	switch evt.Type {
	case engine.EventPuzzleSubmit:
		m.HandleSubmit(eb, state)
	case engine.EventCellTyped:
		// We could auto-check for perfection here if we wanted auto-win,
		// but user requested Ctrl+S submit logic.
	}
}

func (m *SoloMode) HandleSubmit(eb *engine.EventBus, state *engine.GameState) {
	if state.Puzzle == nil || state.Puzzle.Grid == nil {
		return
	}

	grid := state.Puzzle.Grid
	isFull := true
	isPerfect := true

	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			cell := grid.Cells[y][x]
			if !cell.IsBlack {
				if cell.Value == 0 {
					isFull = false
				}
				if cell.Value != cell.Solution {
					isPerfect = false
				}
			}
		}
	}

	if !isFull {
		state.StatusMsg = "Slow down there. Finish the puzzle before submitting."
		state.StatusLevel = "warn"
		state.StatusExp = time.Now().Add(5 * time.Second)
		eb.Publish(engine.Event{Type: engine.EventStateUpdate})
		return
	}

	if isPerfect {
		elapsed := time.Since(state.StartTime)
		h := int(elapsed.Hours())
		m := int(elapsed.Minutes()) % 60
		s := int(elapsed.Seconds()) % 60
		
		state.Status = "WON"
		state.IsFinished = true
		state.FinalTime = elapsed
		state.StatusMsg = fmt.Sprintf("Correct! You took %02d:%02d:%02d time to solve this puzzle.", h, m, s)
		state.StatusLevel = "info" // Render system will map this to Green
		state.StatusExp = time.Now().Add(1 * time.Hour) // Keep it there
	} else {
		state.StatusMsg = "Incorrect! The puzzle hasn't been solved yet."
		state.StatusLevel = "error" // Red
		state.StatusExp = time.Now().Add(5 * time.Second)
	}

	eb.Publish(engine.Event{Type: engine.EventStateUpdate})
}

func (m *SoloMode) IsGameOver(state *engine.GameState) bool {
	return state.IsFinished
}
