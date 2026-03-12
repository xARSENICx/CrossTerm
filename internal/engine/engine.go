package engine

import "crossterm/internal/puzzle"

// CoreEngine orchestrates game logic deterministically.
type CoreEngine struct {
	EventBus   *EventBus
	State      *GameState
	ActiveMode GameMode
	runState   bool
}

// NewCoreEngine configures a deterministic game loop / event processor.
func NewCoreEngine(eventBus *EventBus, p *puzzle.Puzzle) *CoreEngine {
	return &CoreEngine{
		EventBus: eventBus,
		State:    NewGameState(p),
		runState: true,
	}
}

func (e *CoreEngine) SetMode(m GameMode) {
	e.ActiveMode = m
}

func (e *CoreEngine) Run() bool {
	quitCh := e.EventBus.Subscribe(EventQuit)
	menuCh := e.EventBus.Subscribe(EventReturnToMenu)
	cellCh := e.EventBus.Subscribe(EventCellTyped)
	submitCh := e.EventBus.Subscribe(EventPuzzleSubmit)
	resignCh := e.EventBus.Subscribe(EventResign)
	drawCh := e.EventBus.Subscribe(EventDrawOffer)

	for {
		select {
		case <-quitCh:
			e.runState = false
			return false
		case <-menuCh:
			e.runState = false
			return true
		case evt := <-cellCh:
			if e.ActiveMode != nil {
				e.ActiveMode.ProcessEvent(e.EventBus, e.State, evt)
			}
		case evt := <-submitCh:
			if e.ActiveMode != nil {
				e.ActiveMode.ProcessEvent(e.EventBus, e.State, evt)
			}
		case evt := <-resignCh:
			if e.ActiveMode != nil {
				e.ActiveMode.ProcessEvent(e.EventBus, e.State, evt)
			}
		case evt := <-drawCh:
			if e.ActiveMode != nil {
				e.ActiveMode.ProcessEvent(e.EventBus, e.State, evt)
			}
		}
	}
}

// IsRunning indicates if the engine loop is active.
func (e *CoreEngine) IsRunning() bool {
	return e.runState
}
