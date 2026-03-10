package inputsystem

import (
	"crossterm/internal/engine"
	"github.com/gdamore/tcell/v2"
)

type InputSystem struct {
	Screen   tcell.Screen
	EventBus *engine.EventBus
}

func NewInputSystem(screen tcell.Screen, eb *engine.EventBus) *InputSystem {
	return &InputSystem{
		Screen:   screen,
		EventBus: eb,
	}
}

func (s *InputSystem) Run() {
	for {
		ev := s.Screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			s.EventBus.Publish(engine.Event{
				Type:    engine.EventStateUpdate,
				Payload: nil,
			})
			s.Screen.Sync()
		case *tcell.EventKey:
			s.EventBus.Publish(engine.Event{
				Type: engine.EventKeyPress,
				Payload: engine.KeyEventPayload{
					Key:       ev.Key(),
					Rune:      ev.Rune(),
					Modifiers: ev.Modifiers(),
				},
			})
		case *tcell.EventMouse:
			s.EventBus.Publish(engine.Event{
				Type:    engine.EventMouseScroll,
				Payload: ev.Buttons(),
			})
		}
	}
}
