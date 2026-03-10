package modes

import "crossterm/internal/engine"

type BlindDuelMode struct {
	penalties map[string]int
}

func NewBlindDuelMode() *BlindDuelMode {
	return &BlindDuelMode{
		penalties: make(map[string]int),
	}
}

func (m *BlindDuelMode) Name() string {
	return "Blind Duel"
}

func (m *BlindDuelMode) ProcessEvent(eb *engine.EventBus, state *engine.GameState, evt engine.Event) {
	// E.g. penalty logic:
	// Incorrect cell: +10 seconds
}

func (m *BlindDuelMode) IsGameOver(state *engine.GameState) bool {
	return false
}
