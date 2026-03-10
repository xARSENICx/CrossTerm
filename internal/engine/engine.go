package engine

import "crossterm/internal/puzzle"

// CoreEngine orchestrates game logic deterministically.
type CoreEngine struct {
	EventBus *EventBus
	State    *GameState
	runState bool
}

// NewCoreEngine configures a deterministic game loop / event processor.
func NewCoreEngine(eventBus *EventBus, p *puzzle.Puzzle) *CoreEngine {
	return &CoreEngine{
		EventBus: eventBus,
		State:    NewGameState(p),
		runState: true,
	}
}

// Run should be called in a goroutine or directly to process central non-system logic
// Or, if fully ECS-driven, the Engine just holds state and runs systems.
func (e *CoreEngine) Run() {
	quitCh := e.EventBus.Subscribe(EventQuit)

	for range quitCh {
		e.runState = false
		return
	}
}

// IsRunning indicates if the engine loop is active.
func (e *CoreEngine) IsRunning() bool {
	return e.runState
}
