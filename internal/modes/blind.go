package modes

import (
	"crossterm/internal/engine"
	"fmt"
	"time"
)

type BlindDuelMode struct {
	penalties map[string]int
}

func NewBlindDuelMode() *BlindDuelMode {
	return &BlindDuelMode{
		penalties: make(map[string]int),
	}
}

func init() {
	Register("blind", NewBlindDuelMode())
}

func (m *BlindDuelMode) Name() string {
	return "Blind Duel"
}

func (m *BlindDuelMode) ProcessEvent(eb *engine.EventBus, state *engine.GameState, evt engine.Event) {
	if state.IsFinished {
		return
	}

	switch evt.Type {
	case engine.EventCellTyped:
		if m.IsPerfect(state) {
			state.Status = "WON (Perfect)"
			state.FinalTime = time.Since(state.StartTime) + state.PenaltyTime
			state.IsFinished = true
			eb.Publish(engine.Event{Type: engine.EventStateUpdate})
		}
	case engine.EventPuzzleSubmit:
		if !m.IsFull(state) {
			state.StatusMsg = "Submitting before finising makes no sense in this mode! (Press Enter to proceed)"
			state.StatusExp = time.Now().Add(30 * time.Second)
			state.StatusLevel = "error"
			eb.Publish(engine.Event{Type: engine.EventStateUpdate})
			return
		}

		if m.IsPerfect(state) {
			state.Status = "WON (Perfect)"
			state.FinalTime = time.Since(state.StartTime) + state.PenaltyTime
			state.IsFinished = true
			eb.Publish(engine.Event{Type: engine.EventStateUpdate})
		} else {
			// Find errors, add penalty
			errors := 0
			grid := state.Puzzle.Grid
			for y := 0; y < grid.Height; y++ {
				for x := 0; x < grid.Width; x++ {
					cell := &grid.Cells[y][x]
					if !cell.IsBlack && cell.Value != cell.Solution {
						errors++
					}
				}
			}
			state.PenaltyTime += time.Duration(errors*10) * time.Second
			state.StatusMsg = fmt.Sprintf("The puzzle hasn't been solved yet (penalty +%ds)! (Press Enter to proceed)", errors*10)
			state.StatusLevel = "warn"
			state.StatusExp = time.Now().Add(30 * time.Second)
			eb.Publish(engine.Event{Type: engine.EventStateUpdate})
		}

	case engine.EventResign:
		state.Status = "RESIGNED"
		state.FinalTime = time.Since(state.StartTime) + state.PenaltyTime
		state.IsFinished = true
		eb.Publish(engine.Event{Type: engine.EventStateUpdate})

	case engine.EventDrawOffer:
		state.Status = "DRAW"
		state.FinalTime = time.Since(state.StartTime) + state.PenaltyTime
		state.IsFinished = true
		eb.Publish(engine.Event{Type: engine.EventStateUpdate})
	}
}

func (m *BlindDuelMode) IsFull(state *engine.GameState) bool {
	if state.Puzzle == nil || state.Puzzle.Grid == nil {
		return false
	}
	grid := state.Puzzle.Grid
	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			if !grid.Cells[y][x].IsBlack && grid.Cells[y][x].Value == 0 {
				return false
			}
		}
	}
	return true
}

func (m *BlindDuelMode) IsPerfect(state *engine.GameState) bool {
	if state.Puzzle == nil || state.Puzzle.Grid == nil {
		return false
	}
	grid := state.Puzzle.Grid
	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			cell := grid.Cells[y][x]
			if !cell.IsBlack && cell.Value != cell.Solution {
				return false
			}
			if !cell.IsBlack && cell.Value == 0 {
				return false // Has empty non-black cells
			}
		}
	}
	return true
}

func (m *BlindDuelMode) IsGameOver(state *engine.GameState) bool {
	return m.IsPerfect(state)
}
