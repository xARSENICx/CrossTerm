package puzzle

// Direction defines word orientation (Across or Down).
type Direction string

const (
	DirAcross Direction = "ACROSS"
	DirDown   Direction = "DOWN"
)

// Clue represents a single crossword clue.
type Clue struct {
	Number    int       // Clue number displayed in UI
	Direction Direction // Whether the clue is Across or Down
	Text      string    // The actual text of the clue
	Length    int       // Number of characters in the word
	StartX    int       // Starting X coordinate in the Grid
	StartY    int       // Starting Y coordinate in the Grid
}

// Puzzle integrates the full crossword specification, state, and metadata.
type Puzzle struct {
	Title     string
	Author    string
	Copyright string
	Notes     string

	Grid        *Grid
	Clues       []Clue
	HasSolution bool
}
