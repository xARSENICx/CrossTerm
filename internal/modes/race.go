package modes

import "crossterm/internal/engine"

type RaceMode struct {
	scores map[string]int
}

func NewRaceMode() *RaceMode {
	return &RaceMode{
		scores: make(map[string]int),
	}
}

func (m *RaceMode) Name() string {
	return "Race"
}

func (m *RaceMode) ProcessEvent(eb *engine.EventBus, state *engine.GameState, evt engine.Event) {
	// e.g. Handle CLUE_SOLVED event and increment score.
}

func (m *RaceMode) IsGameOver(state *engine.GameState) bool {
	// Game ends when all clues are solved
	return false
}
