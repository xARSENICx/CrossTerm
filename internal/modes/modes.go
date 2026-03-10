package modes

import "crossterm/internal/engine"

var registry = make(map[string]engine.GameMode)

func Register(name string, m engine.GameMode) {
	registry[name] = m
}

func GetMode(name string) engine.GameMode {
	if m, ok := registry[name]; ok {
		return m
	}
	// Default or fallback
	return nil
}
