package networksystem

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"crossterm/internal/engine"
	"crossterm/internal/netproto"
)

type NetworkRole string

const (
	RoleHost NetworkRole = "HOST"
	RoleJoin NetworkRole = "JOIN"
)

type NetworkSystem struct {
	EventBus *engine.EventBus
	State    *engine.GameState

	conn      *net.UDPConn
	peerAddr  *net.UDPAddr
	relayAddr *net.UDPAddr
	roomID    string
	isHost    bool

	connected bool
}

// NewNetworkSystem configures a serverless P2P networked session using a pre-established UDP socket and the decoded Peer address.
func NewNetworkSystem(eb *engine.EventBus, state *engine.GameState, conn *net.UDPConn, peerAddr *net.UDPAddr, isHost bool) *NetworkSystem {
	return &NetworkSystem{
		EventBus: eb,
		State:    state,
		conn:     conn,
		peerAddr: peerAddr,
		isHost:   isHost,
	}
}

func (s *NetworkSystem) SetRelayFallback(relayIPPort, roomID string) {
	addr, _ := net.ResolveUDPAddr("udp", relayIPPort)
	s.relayAddr = addr
	s.roomID = roomID
}

func (s *NetworkSystem) Run() {
	err := s.holePunch()
	if err != nil {
		log.Printf("Direct hole punch failed, falling back to robust TURN-relay!")
		s.connected = false // Relay Mode
	} else {
		s.connected = true  // Direct Mode
		log.Printf("P2P Complete! Linked exactly to %v", s.peerAddr)
	}

	// In multiplayer, send GAME_START after establishing mapping if host
	if s.isHost {
		s.EventBus.Publish(engine.Event{
			Type: engine.EventGameStart,
		})
	}

	go s.readLoop()
	s.writeLoop() // Blocking background
}

func (s *NetworkSystem) holePunch() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	punchMsg := netproto.NetworkMessage{Type: netproto.MsgPunch}
	bPunch, _ := json.Marshal(punchMsg)

	// Start pumping "PUNCH" to peer
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.conn.WriteToUDP(bPunch, s.peerAddr)
			}
		}
	}()

	buffer := make([]byte, 2048)
	var msg netproto.NetworkMessage

	// Wait for a "PUNCH" from peer
	for {
		s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, rAddr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			// keep trying until context cancels or success
			select {
			case <-ctx.Done():
				return fmt.Errorf("hole punch failed/timeout")
			default:
				continue
			}
		}

		if rAddr.String() == s.peerAddr.String() {
			if err := json.Unmarshal(buffer[:n], &msg); err == nil && msg.Type == netproto.MsgPunch {
				// Success! Send a single PUNCH_ACK and stop punching loop
				ackMsg := netproto.NetworkMessage{Type: netproto.MsgPunchAck}
				bAck, _ := json.Marshal(ackMsg)
				s.conn.WriteToUDP(bAck, s.peerAddr)
				break
			}
		}
	}

	s.conn.SetReadDeadline(time.Time{})
	return nil
}

func (s *NetworkSystem) readLoop() {
	buffer := make([]byte, 4096)
	for {
		n, _, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			continue // Ignore errs
		}

		var msg netproto.NetworkMessage
		if err := json.Unmarshal(buffer[:n], &msg); err != nil {
			continue
		}

		if msg.Type == netproto.MsgRelay {
			var inner netproto.NetworkMessage
			if err := json.Unmarshal(msg.Payload, &inner); err == nil {
				msg = inner
			}
		}

		if msg.Type == netproto.MsgGameEvent {
			var rawEvent struct {
				Type    engine.EventType `json:"type"`
				Payload json.RawMessage  `json:"payload"`
			}
			if err := json.Unmarshal(msg.Payload, &rawEvent); err == nil {
				engineEvent := engine.Event{Type: rawEvent.Type}
				
				if rawEvent.Type == engine.EventKeyPress {
					var keyPayload engine.KeyEventPayload
					if err := json.Unmarshal(rawEvent.Payload, &keyPayload); err == nil {
						engineEvent.Payload = keyPayload
					}
				}
				s.EventBus.Publish(engineEvent)
			}
		}
	}
}

func (s *NetworkSystem) writeLoop() {
	intentSub := s.EventBus.Subscribe(engine.EventKeyPress)

	for evt := range intentSub {
		// Serialize and send
		bEvt, err := json.Marshal(evt)
		if err != nil {
			continue
		}
		netMsg := netproto.NetworkMessage{
			Type:    netproto.MsgGameEvent,
			Payload: bEvt,
		}

		if s.connected { // Direct P2P Hole Punch mapping open!
			bNetMsg, _ := json.Marshal(netMsg)
			s.conn.WriteToUDP(bNetMsg, s.peerAddr)
		} else if s.relayAddr != nil { // Fallback standard Relay
			relayWrap := netproto.NetworkMessage{
				Type:   netproto.MsgRelay,
				RoomID: s.roomID,
			}
			bInner, _ := json.Marshal(netMsg)
			relayWrap.Payload = bInner
			bWrapBytes, _ := json.Marshal(relayWrap)
			s.conn.WriteToUDP(bWrapBytes, s.relayAddr)
		}
	}
}
