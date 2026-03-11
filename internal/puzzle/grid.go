package puzzle

// Cell represents a single square in the crossword grid.
// Black squares usually hold no value but block words.
type Cell struct {
	Solution       byte   // The correct character
	Value          byte   // The currently typed character
	IsBlack        bool   // Whether this cell is a block square
	Number         int    // Clue number if it's the start of a clue (0 if none)
	CheckedCorrect bool   // Whether this letter was checked and is correct
	WrongGuesses   []byte // The letters that have been checked and proven wrong
	WasChecked     bool   // Whether this cell has explicitly been checked before
}

// Grid holds the two-dimensional crossword data.
type Grid struct {
	Width  int
	Height int
	Cells  [][]Cell
}

// NewGrid initializes a crossword grid of a specific width and height.
func NewGrid(width, height int) *Grid {
	cells := make([][]Cell, height)
	for i := range cells {
		cells[i] = make([]Cell, width)
	}
	return &Grid{
		Width:  width,
		Height: height,
		Cells:  cells,
	}
}

// GetCell safely returns the cell at (x, y) if within bounds.
func (g *Grid) GetCell(x, y int) *Cell {
	if x >= 0 && x < g.Width && y >= 0 && y < g.Height {
		return &g.Cells[y][x]
	}
	return nil
}
