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
	"crossterm/internal/puzzle"
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
	hostReady bool // true when host is ready to respond to MsgReady with puzzle

	puzTransferChan chan bool
	peerReadyChan   chan bool
	receivedChunks  map[int][]byte
}

// NewNetworkSystem configures a serverless P2P networked session using a pre-established UDP socket and the decoded Peer address.
func NewNetworkSystem(eb *engine.EventBus, state *engine.GameState, conn *net.UDPConn, peerAddr *net.UDPAddr, isHost bool) *NetworkSystem {
	return &NetworkSystem{
		EventBus:        eb,
		State:           state,
		conn:            conn,
		peerAddr:        peerAddr,
		isHost:          isHost,
		puzTransferChan: make(chan bool),
		peerReadyChan:   make(chan bool, 1),
		receivedChunks:  make(map[int][]byte),
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

		// Re-register with relay to refresh NAT mapping — send 3 times for UDP reliability
		if s.relayAddr != nil && s.roomID != "" {
			role := 1 // host
			if !s.isHost {
				role = 2 // joiner
			}
			for i := 0; i < 3; i++ {
				reRegMsg := netproto.NetworkMessage{Type: netproto.MsgReRegister, RoomID: s.roomID, PlayerID: &role}
				bMsg, _ := json.Marshal(reRegMsg)
				s.conn.WriteToUDP(bMsg, s.relayAddr)
				time.Sleep(100 * time.Millisecond)
			}
			log.Printf("Sent RE-REGISTER x3 to relay for room %s (role=%d)", s.roomID, role)
		}
	} else {
		s.connected = true  // Direct Mode
		log.Printf("P2P Complete! Linked exactly to %v", s.peerAddr)
	}

	// Start readLoop AFTER hole punching completes so there's no concurrent reader conflict
	go s.readLoop()

	if s.isHost {
		// The host doesn't actively wait here. Instead, the readLoop will respond
		// to MsgReady from the joiner by sending the puzzle (see readLoop).
		// We just mark that we are ready to respond.
		s.hostReady = true
		log.Printf("Host is ready. Waiting for joiner's READY signal in readLoop...")

		// Block until the joiner has received the puzzle (signaled via peerReadyChan)
		// or a generous timeout expires
		select {
		case <-s.peerReadyChan:
			log.Printf("Puzzle delivery confirmed via READY handshake!")
		case <-time.After(120 * time.Second):
			log.Printf("Timed out waiting for joiner. Proceeding anyway...")
		}
	} else {
		// Joiner: signal the host repeatedly that we are ready to receive the puzzle
		readyDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-readyDone:
					return
				case <-ticker.C:
					s.sendMessage(netproto.NetworkMessage{Type: netproto.MsgReady})
					log.Printf("Sent READY signal to host...")
				}
			}
		}()

		if s.State.Puzzle == nil {
			log.Printf("Waiting for puzzle from host...")
			select {
			case <-s.puzTransferChan:
				log.Printf("Puzzle received and loaded!")
			case <-time.After(120 * time.Second):
				log.Printf("Timed out waiting for puzzle!")
			}
		}
		close(readyDone)
	}

	// In multiplayer, send GAME_START after establishing mapping if host
	if s.isHost {
		s.EventBus.Publish(engine.Event{
			Type: engine.EventGameStart,
		})
	}

	if s.State.IsCollab {
		username := s.State.LocalUsername
		s.sendMessage(netproto.NetworkMessage{Type: netproto.MsgPeerUsername, Username: &username})
	}

	s.writeLoop() // Blocking background
}

func (s *NetworkSystem) holePunch() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

func (s *NetworkSystem) sendPuzzle() {
	if s.State.Puzzle == nil {
		return
	}
	data, err := json.Marshal(s.State.Puzzle)
	if err != nil {
		log.Printf("Failed to marshal puzzle for sync: %v", err)
		return
	}

	chunkSize := 1024
	total := (len(data) + chunkSize - 1) / chunkSize

	for i := 0; i < total; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}

		msg := netproto.NetworkMessage{
			Type:        netproto.MsgPuzTransfer,
			Payload:     data[start:end],
			ChunkIndex:  new(i),
			TotalChunks: new(total),
		}
		s.sendMessage(msg)
		time.Sleep(50 * time.Millisecond) // Throttling for stability over UDP
	}
}

func (s *NetworkSystem) handlePuzTransfer(msg netproto.NetworkMessage) {
	if s.receivedChunks == nil {
		s.receivedChunks = make(map[int][]byte)
	}
	if msg.ChunkIndex != nil {
		s.receivedChunks[*msg.ChunkIndex] = msg.Payload
	}

	if msg.TotalChunks != nil && len(s.receivedChunks) == *msg.TotalChunks {
		// Reassemble
		fullData := make([]byte, 0)
		for i := 0; i < *msg.TotalChunks; i++ {
			fullData = append(fullData, s.receivedChunks[i]...)
		}

		var p puzzle.Puzzle
		if err := json.Unmarshal(fullData, &p); err == nil {
			s.State.Puzzle = &p
			// Find start pos for cursor now that puzzle is here
			if p.Grid != nil {
				for y := 0; y < p.Grid.Height; y++ {
					for x := 0; x < p.Grid.Width; x++ {
						if !p.Grid.Cells[y][x].IsBlack {
							s.State.Cursor.X = x
							s.State.Cursor.Y = y
							goto Found
						}
					}
				}
			Found:
			}
			select {
			case s.puzTransferChan <- true:
			default:
			}
		}
	}
}

func (s *NetworkSystem) sendMessage(msg netproto.NetworkMessage) {
	bMsg, _ := json.Marshal(msg)
	if s.connected {
		s.conn.WriteToUDP(bMsg, s.peerAddr)
	} else if s.relayAddr != nil {
		relayWrap := netproto.NetworkMessage{
			Type:    netproto.MsgRelay,
			RoomID:  s.roomID,
			Payload: bMsg,
		}
		bWrap, _ := json.Marshal(relayWrap)
		s.conn.WriteToUDP(bWrap, s.relayAddr)
	}
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

		if msg.Type == netproto.MsgReady {
			if s.isHost && s.hostReady {
				log.Printf("Received READY from joiner! Sending puzzle now...")
				go func() {
					s.sendPuzzle()
					// Send puzzle a second time for UDP reliability
					time.Sleep(1 * time.Second)
					s.sendPuzzle()
					select {
					case s.peerReadyChan <- true:
					default:
					}
				}()
				s.hostReady = false // Only send once
			}
			continue
		}

		if msg.Type == netproto.MsgPuzTransfer {
			s.handlePuzTransfer(msg)
			continue
		}

		if s.State.IsCollab {
			switch msg.Type {
			case netproto.MsgPeerUsername:
				if msg.Username != nil {
					s.State.PeerUsername = *msg.Username
					s.EventBus.Publish(engine.Event{Type: engine.EventStateUpdate})
				}
				continue
			case netproto.MsgPeerCursor:
				if msg.X != nil && msg.Y != nil && msg.CursorDir != nil {
					s.State.PeerCursor.X = *msg.X
					s.State.PeerCursor.Y = *msg.Y
					s.State.PeerCursor.Direction = puzzle.DirAcross
					if *msg.CursorDir == 1 {
						s.State.PeerCursor.Direction = puzzle.DirDown
					}
					s.EventBus.Publish(engine.Event{Type: engine.EventStateUpdate})
				}
				continue
			case netproto.MsgCellUpdate:
				if msg.X != nil && msg.Y != nil && msg.CellValue != nil {
					cell := s.State.Puzzle.Grid.GetCell(*msg.X, *msg.Y)
					if cell != nil && !cell.IsBlack {
						cell.Value = *msg.CellValue
						cell.TypedBy = 2 // 2 means Peer
						s.EventBus.Publish(engine.Event{Type: engine.EventRemoteCellTyped})
					}
				}
				continue
			case netproto.MsgClueSolved:
				if msg.SolvedCount != nil {
					s.State.PeerSolvedClues = *msg.SolvedCount
					s.EventBus.Publish(engine.Event{Type: engine.EventStateUpdate})
				}
				continue
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
	stopSub := s.EventBus.Subscribe(engine.EventShutdown)
	
	cellSub := s.EventBus.Subscribe(engine.EventCellTyped)
	stateSub := s.EventBus.Subscribe(engine.EventStateUpdate)
	
	var lastCursor engine.CursorPos
	var lastSolvedCount int
	
	if s.State != nil {
		lastCursor = s.State.Cursor
		lastSolvedCount = s.State.LocalSolvedClues
	}

	for {
		select {
		case <-stopSub:
			return
		case evt := <-intentSub:
			if s.State.IsCollab {
				continue // Collab does not sync raw keypresses, it syncs explicit game state changes
			}
			bEvt, err := json.Marshal(evt)
			if err == nil {
				s.sendMessage(netproto.NetworkMessage{Type: netproto.MsgGameEvent, Payload: bEvt})
			}
		case evt := <-cellSub:
			if !s.State.IsCollab {
				continue
			}
			if coords, ok := evt.Payload.([]int); ok && len(coords) == 2 {
				x, y := coords[0], coords[1]
				cell := s.State.Puzzle.Grid.GetCell(x, y)
				if cell != nil {
					cx, cy := x, y
					val := cell.Value
					s.sendMessage(netproto.NetworkMessage{
						Type: netproto.MsgCellUpdate,
						X: &cx, Y: &cy, CellValue: &val,
					})
				}
			}
		case <-stateSub:
			if !s.State.IsCollab {
				continue
			}
			// Send cursor if changed
			if s.State.Cursor.X != lastCursor.X || s.State.Cursor.Y != lastCursor.Y || s.State.Cursor.Direction != lastCursor.Direction {
				cx, cy := s.State.Cursor.X, s.State.Cursor.Y
				dir := 0
				if s.State.Cursor.Direction == puzzle.DirDown {
					dir = 1
				}
				s.sendMessage(netproto.NetworkMessage{
					Type: netproto.MsgPeerCursor,
					X: &cx, Y: &cy, CursorDir: &dir,
				})
				lastCursor = s.State.Cursor
			}
			// Send solved count if changed
			if s.State.LocalSolvedClues != lastSolvedCount {
				c := s.State.LocalSolvedClues
				s.sendMessage(netproto.NetworkMessage{
					Type: netproto.MsgClueSolved,
					SolvedCount: &c,
				})
				lastSolvedCount = s.State.LocalSolvedClues
			}
		}
	}
}
