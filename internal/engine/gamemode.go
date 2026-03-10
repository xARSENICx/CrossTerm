package engine

// GameMode interface allows different competitive rulesets to hook into the engine.
type GameMode interface {
	// Name returns the identifier for this mode
	Name() string
	
	// ProcessEvent evaluates a game event using the mode's rules.
	// It can modify state or emit new events (e.g. applying a penalty).
	ProcessEvent(eb *EventBus, state *GameState, evt Event)
	
	// IsGameOver returns if the win conditions are met
	IsGameOver(state *GameState) bool
}
