package engine

import "crossterm/internal/puzzle"

type GameState struct {
	Puzzle *puzzle.Puzzle
	Cursor CursorPos
	Mode   string 
	GotoMode   bool
	GotoBuffer string
	Anagram    AnagramState
	// Additional multiplayer stats later
}

type CursorPos struct {
	X         int
	Y         int
	Direction puzzle.Direction
}

// NewGameState creates the initial deterministic state.
func NewGameState(p *puzzle.Puzzle) *GameState {
	// Find the first unfilled, non-black cell to start cursor
	sx, sy := 0, 0
	if p != nil && p.Grid != nil {
		for y := 0; y < p.Grid.Height; y++ {
			for x := 0; x < p.Grid.Width; x++ {
				if !p.Grid.Cells[y][x].IsBlack {
					sx, sy = x, y
					goto FoundStart
				}
			}
		}
	FoundStart:
	}

	return &GameState{
		Puzzle: p,
		Cursor: CursorPos{
			X:         sx,
			Y:         sy,
			Direction: puzzle.DirAcross,
		},
	}
}

type AnagramState struct {
	Active    bool
	StartX    int
	StartY    int
	Direction puzzle.Direction
	Length    int
	Letters   []byte
	Locked    []bool
	CursorIdx int
}
