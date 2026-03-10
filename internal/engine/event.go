package engine

import "github.com/gdamore/tcell/v2"

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
	EventStateUpdate  EventType = "STATE_UPDATE"
)

type KeyEventPayload struct {
	Key  tcell.Key `json:"key"`
	Rune rune      `json:"rune"`
}

type Event struct {
	Type    EventType   `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// EventBus handles pub/sub for all engine events.
type EventBus struct {
	subscribers map[EventType][]chan Event
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]chan Event),
	}
}

func (eb *EventBus) Subscribe(eventType EventType) <-chan Event {
	ch := make(chan Event, 100)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], ch)
	return ch
}

func (eb *EventBus) Publish(event Event) {
	if subs, ok := eb.subscribers[event.Type]; ok {
		for _, ch := range subs {
			// Non-blocking send or buffer
			select {
			case ch <- event:
			default:
			}
		}
	}
}
