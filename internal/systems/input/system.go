package inputsystem

import (
	"crossterm/internal/engine"
	"github.com/gdamore/tcell/v2"
)

type InputSystem struct {
	Screen   tcell.Screen
	EventBus *engine.EventBus
	Done     chan struct{}
}

func NewInputSystem(screen tcell.Screen, eb *engine.EventBus) *InputSystem {
	return &InputSystem{
		Screen:   screen,
		EventBus: eb,
		Done:     make(chan struct{}),
	}
}

func (s *InputSystem) Run() {
	for {
		select {
		case <-s.Done:
			return
		default:
		}

		ev := s.Screen.PollEvent()
		if ev == nil {
			continue // Unblocked by Stop()
		}

		switch ev := ev.(type) {
		case *tcell.EventResize:
			// ... (rest of the logic)
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

func (s *InputSystem) Stop() {
	close(s.Done)
	s.Screen.PostEvent(nil)
}
