package modes

import (
	"crossterm/internal/engine"
	"crossterm/internal/puzzle"
	"fmt"
	"time"
)

type CollabMode struct{}

func NewCollabMode() *CollabMode {
	return &CollabMode{}
}

func init() {
	c := NewCollabMode()
	Register("collab_not_timed_standard", c)
	Register("collab_not_timed_checks", c)
	Register("collab_not_timed_tools", c)
	Register("collab_timed_standard", c)
	Register("collab_timed_checks", c)
	Register("collab_timed_tools", c)
}

func (m *CollabMode) Name() string {
	return "Collaborative Mode"
}

func (m *CollabMode) ProcessEvent(eb *engine.EventBus, state *engine.GameState, evt engine.Event) {
	if state.IsFinished {
		return
	}

	switch evt.Type {
	case engine.EventPuzzleSubmit:
		m.HandleSubmit(eb, state)
	case engine.EventCellTyped, engine.EventRemoteCellTyped:
		m.CheckSolvedClues(eb, state)
	}
}

func (m *CollabMode) HandleSubmit(eb *engine.EventBus, state *engine.GameState) {
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
		state.StatusMsg = "The puzzle is not fully framed yet."
		state.StatusLevel = "warn"
		state.StatusExp = time.Now().Add(5 * time.Second)
		eb.Publish(engine.Event{Type: engine.EventStateUpdate})
		return
	}

	if isPerfect {
		elapsed := time.Since(state.StartTime)
		state.Status = "WON"
		state.IsFinished = true
		state.FinalTime = elapsed
		
		totalStr := fmt.Sprintf("Great teamwork! %s: %d clues, %s: %d clues.", 
			state.LocalUsername, state.LocalSolvedClues, state.PeerUsername, state.PeerSolvedClues)
		state.StatusMsg = totalStr
		state.StatusLevel = "info"
		state.StatusExp = time.Now().Add(1 * time.Hour)
	} else {
		state.StatusMsg = "Some cells aren't quite right..."
		state.StatusLevel = "error"
		state.StatusExp = time.Now().Add(5 * time.Second)
	}

	eb.Publish(engine.Event{Type: engine.EventStateUpdate})
}

func (m *CollabMode) CheckSolvedClues(eb *engine.EventBus, state *engine.GameState) {
	if state.Puzzle == nil || state.Puzzle.Grid == nil || state.SolvedClues == nil {
		return
	}

	grid := state.Puzzle.Grid
	newlySolved := false

	for _, clue := range state.Puzzle.Clues {
		clueKey := fmt.Sprintf("%v-%d", clue.Direction, clue.Number)

		if state.SolvedClues[clueKey] {
			continue // Already solved
		}

		// Check if this clue is now completely filled and correct
		isSolved := true
		lastTypedBy := 0

		cx, cy := clue.StartX, clue.StartY
		
		for {
			cell := grid.GetCell(cx, cy)
			if cell == nil || cell.IsBlack {
				break
			}
			
			if cell.Value != cell.Solution {
				isSolved = false
				break
			}
			
			if cell.TypedBy != 0 {
				lastTypedBy = cell.TypedBy
			}

			if clue.Direction == puzzle.DirAcross {
				cx++
			} else {
				cy++
			}
		}

		if isSolved && lastTypedBy != 0 {
			state.SolvedClues[clueKey] = true
			if lastTypedBy == 1 {
				state.LocalSolvedClues++
			} else if lastTypedBy == 2 {
				state.PeerSolvedClues++
			}
			newlySolved = true
		}
	}

	if newlySolved {
		eb.Publish(engine.Event{Type: engine.EventStateUpdate})
	}
}

func (m *CollabMode) IsGameOver(state *engine.GameState) bool {
	return state.IsFinished
}
