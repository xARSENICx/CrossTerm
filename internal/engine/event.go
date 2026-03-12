package engine

import (
	"sync"
	"github.com/gdamore/tcell/v2"
)

type EventType string

const (
	EventKeyPress     EventType = "KEY_PRESS"
	EventCursorMove   EventType = "CURSOR_MOVE"
	EventCellTyped    EventType = "CELL_TYPED"
	EventClueSolved   EventType = "CLUE_SOLVED"
	EventPuzzleSubmit EventType = "PUZZLE_SUBMIT"
	EventPlayerJoined EventType = "PLAYER_JOINED"
	EventGameStart    EventType = "GAME_START"
	EventQuit         EventType = "QUIT"
	EventResign       EventType = "RESIGN"
	EventDrawOffer    EventType = "DRAW_OFFER"
	EventReturnToMenu EventType = "RETURN_TO_MENU"
	EventStateUpdate  EventType = "STATE_UPDATE"
	EventMouseScroll  EventType = "MOUSE_SCROLL"
	EventShutdown     EventType = "SHUTDOWN"
)

type KeyEventPayload struct {
	Key       tcell.Key      `json:"key"`
	Rune      rune           `json:"rune"`
	Modifiers tcell.ModMask `json:"modifiers"`
}

type Event struct {
	Type    EventType   `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// EventBus handles pub/sub for all engine events.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]chan Event
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]chan Event),
	}
}

func (eb *EventBus) Subscribe(eventType EventType) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	ch := make(chan Event, 100)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], ch)
	return ch
}

func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	subs, ok := eb.subscribers[event.Type]
	eb.mu.RUnlock()

	if ok {
		for _, ch := range subs {
			// Non-blocking send or buffer
			select {
			case ch <- event:
			default:
			}
		}
	}
}
