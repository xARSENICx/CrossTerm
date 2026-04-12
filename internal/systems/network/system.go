package networksystem

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync/atomic"
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

// keepaliveInterval is how often the client sends a heartbeat to the relay
// to keep the NAT pinhole open. Most consumer NATs expire mappings in 30-60s.
const keepaliveInterval = 15 * time.Second

type NetworkSystem struct {
	EventBus *engine.EventBus
	State    *engine.GameState

	conn      *net.UDPConn
	peerAddr  *net.UDPAddr // last-known address of the peer (for direct / same-LAN mode)
	relayAddr *net.UDPAddr
	roomID    string
	isHost    bool
	playerID  int // 1 = host, 2 = joiner — sent with every relay/keepalive message

	// connected = true means we have a verified direct path to peerAddr.
	// connected = false means ALL traffic goes through the relay.
	connected atomic.Bool

	hostReady bool // true when host is ready to respond to MsgReady with puzzle

	puzTransferChan chan bool
	peerReadyChan   chan bool
	receivedChunks  map[int][]byte
}

// NewNetworkSystem configures a relay-first P2P networked session.
// peerAddr is obtained from the relay's PEER_INFO and is used only for
// same-LAN direct connections; all other traffic goes through relayAddr.
func NewNetworkSystem(eb *engine.EventBus, state *engine.GameState, conn *net.UDPConn, peerAddr *net.UDPAddr, isHost bool) *NetworkSystem {
	playerID := 2
	if isHost {
		playerID = 1
	}
	return &NetworkSystem{
		EventBus:        eb,
		State:           state,
		conn:            conn,
		peerAddr:        peerAddr,
		isHost:          isHost,
		playerID:        playerID,
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
	// Step 1: Tell Relay we are formally here.
	if s.relayAddr != nil && s.roomID != "" {
		s.reRegisterWithRelay()
	}

	// Step 2: Start background P2P racer and relay keepalives!
	// We no longer block! Default everything to relay logic initially.
	s.connected.Store(false)

	if s.relayAddr != nil {
		go s.keepaliveLoop()
	}

	if s.peerAddr != nil {
		go s.punchLoop()
	}

	// Step 3: Start receiving messages.
	go s.readLoop()

	// Step 4: Host/joiner puzzle handshake.
	if s.isHost {
		s.hostReady = true
		log.Printf("Host is ready. Waiting for joiner's READY signal in readLoop...")

		select {
		case <-s.peerReadyChan:
			log.Printf("Puzzle delivery confirmed via READY handshake!")
		case <-time.After(120 * time.Second):
			log.Printf("Timed out waiting for joiner. Proceeding anyway...")
		}
	} else {
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

	// In multiplayer, host fires GAME_START after setup
	if s.isHost {
		s.EventBus.Publish(engine.Event{Type: engine.EventGameStart})
	}

	if s.State.IsCollab {
		username := s.State.LocalUsername
		s.sendMessage(netproto.NetworkMessage{Type: netproto.MsgPeerUsername, Username: &username})
	}

	s.writeLoop() // blocks
}

// reRegisterWithRelay sends RE_REGISTER to the relay 3× for UDP reliability.
func (s *NetworkSystem) reRegisterWithRelay() {
	role := s.playerID
	reRegMsg := netproto.NetworkMessage{
		Type:     netproto.MsgReRegister,
		RoomID:   s.roomID,
		PlayerID: &role,
	}
	bMsg, _ := json.Marshal(reRegMsg)

	for i := 0; i < 3; i++ {
		s.conn.WriteToUDP(bMsg, s.relayAddr)
		time.Sleep(100 * time.Millisecond)
	}
	log.Printf("Sent RE_REGISTER ×3 to relay for room %s (playerID=%d)", s.roomID, role)
}

// punchLoop aggressively fires UDP hole-punches at the peerAddr continuously
// until a direct route is confirmed.
func (s *NetworkSystem) punchLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	punchMsg := netproto.NetworkMessage{Type: netproto.MsgPunch}
	bPunch, _ := json.Marshal(punchMsg)
	shutdownCh := s.EventBus.Subscribe(engine.EventShutdown)

	for {
		if s.connected.Load() {
			return // Racing is over, we won the direct connection!
		}
		select {
		case <-shutdownCh:
			return
		case <-ticker.C:
			s.conn.WriteToUDP(bPunch, s.peerAddr) // FIRE!
		}
	}
}

// keepaliveLoop sends a periodic heartbeat to the relay to keep NAT
// pinholes open. Runs only in relay mode (connected=false).
func (s *NetworkSystem) keepaliveLoop() {
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()

	shutdownCh := s.EventBus.Subscribe(engine.EventShutdown)

	pid := s.playerID
	kaMsg := netproto.NetworkMessage{
		Type:     netproto.MsgKeepalive,
		RoomID:   s.roomID,
		PlayerID: &pid,
	}
	b, _ := json.Marshal(kaMsg)

	for {
		select {
		case <-shutdownCh:
			return
		case <-ticker.C:
			if s.relayAddr != nil {
				s.conn.WriteToUDP(b, s.relayAddr)
			}
		}
	}
}

// ── Puzzle Transfer ────────────────────────────────────────────────────────────

func (s *NetworkSystem) sendPuzzle() {
	if s.State.Puzzle == nil {
		return
	}
	data, err := json.Marshal(s.State.Puzzle)
	if err != nil {
		log.Printf("Failed to marshal puzzle for sync: %v", err)
		return
	}

	chunkSize := 512
	total := (len(data) + chunkSize - 1) / chunkSize

	for i := 0; i < total; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}

		ci := i
		tc := total
		msg := netproto.NetworkMessage{
			Type:        netproto.MsgPuzTransfer,
			Payload:     data[start:end],
			ChunkIndex:  &ci,
			TotalChunks: &tc,
		}
		s.sendMessage(msg)
		time.Sleep(50 * time.Millisecond) // throttle for UDP stability
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
		fullData := make([]byte, 0)
		for i := 0; i < *msg.TotalChunks; i++ {
			fullData = append(fullData, s.receivedChunks[i]...)
		}

		var p puzzle.Puzzle
		if err := json.Unmarshal(fullData, &p); err == nil {
			s.State.Puzzle = &p
			// Position cursor at first non-black cell
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

// ── Message Transport ──────────────────────────────────────────────────────────

// sendMessage sends a game message. In direct mode it goes straight
// to peerAddr; in relay mode it is wrapped in a MsgRelay envelope.
func (s *NetworkSystem) sendMessage(msg netproto.NetworkMessage) {
	bMsg, _ := json.Marshal(msg)
	if s.connected.Load() && s.peerAddr != nil {
		s.conn.WriteToUDP(bMsg, s.peerAddr)
	} else if s.relayAddr != nil {
		pid := s.playerID
		wrap := netproto.NetworkMessage{
			Type:     netproto.MsgRelay,
			RoomID:   s.roomID,
			PlayerID: &pid,
			Payload:  bMsg,
		}
		bWrap, _ := json.Marshal(wrap)
		s.conn.WriteToUDP(bWrap, s.relayAddr)
	}
}

// ── Read Loop ──────────────────────────────────────────────────────────────────

func (s *NetworkSystem) readLoop() {
	buffer := make([]byte, 8192)
	for {
		n, rAddr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		var msg netproto.NetworkMessage
		if err := json.Unmarshal(buffer[:n], &msg); err != nil {
			continue
		}

		// Unwrap relay envelopes
		if msg.Type == netproto.MsgRelay {
			var inner netproto.NetworkMessage
			if err := json.Unmarshal(msg.Payload, &inner); err == nil {
				msg = inner
			}
		}

		// Ignore infra-level messages that escape into the game loop
		switch msg.Type {
		case netproto.MsgKeepalive, netproto.MsgRelayReady, netproto.MsgSameLAN,
			netproto.MsgPeerInfo, netproto.MsgReRegister:
			continue
		}

		if msg.Type == netproto.MsgPunch {
			if s.peerAddr != nil {
				ack := netproto.NetworkMessage{Type: netproto.MsgPunchAck}
				b, _ := json.Marshal(ack)
				s.conn.WriteToUDP(b, s.peerAddr)
			}
			continue
		}

		if msg.Type == netproto.MsgPunchAck {
			if rAddr != nil && s.peerAddr != nil && rAddr.String() == s.peerAddr.String() {
				if !s.connected.Load() {
					log.Printf("✅ DIRECT P2P HOLE PUNCH SUCCESS! Upgrading connection to native UDP...")
					s.connected.Store(true)
				}
			}
			continue
		}

		if msg.Type == netproto.MsgReady {
			if s.isHost && s.hostReady {
				log.Printf("Received READY from joiner — sending puzzle...")
				go func() {
					s.sendPuzzle()
					// Send a second time for UDP reliability
					time.Sleep(1 * time.Second)
					s.sendPuzzle()
					select {
					case s.peerReadyChan <- true:
					default:
					}
				}()
				s.hostReady = false
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
						cell.TypedBy = 2
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

// ── Write Loop ────────────────────────────────────────────────────────────────

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
				continue // collab syncs state, not raw keypresses
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
						X:    &cx, Y: &cy, CellValue: &val,
					})
				}
			}
		case <-stateSub:
			if !s.State.IsCollab {
				continue
			}
			if s.State.Cursor.X != lastCursor.X || s.State.Cursor.Y != lastCursor.Y || s.State.Cursor.Direction != lastCursor.Direction {
				cx, cy := s.State.Cursor.X, s.State.Cursor.Y
				dir := 0
				if s.State.Cursor.Direction == puzzle.DirDown {
					dir = 1
				}
				s.sendMessage(netproto.NetworkMessage{
					Type: netproto.MsgPeerCursor,
					X:    &cx, Y: &cy, CursorDir: &dir,
				})
				lastCursor = s.State.Cursor
			}
			if s.State.LocalSolvedClues != lastSolvedCount {
				c := s.State.LocalSolvedClues
				s.sendMessage(netproto.NetworkMessage{
					Type:        netproto.MsgClueSolved,
					SolvedCount: &c,
				})
				lastSolvedCount = s.State.LocalSolvedClues
			}
		}
	}
}

// ── Unexported helpers ─────────────────────────────────────────────────────────

func addrStr(a *net.UDPAddr) string {
	if a == nil {
		return "<nil>"
	}
	return a.String()
}

// suppress the unused import warning during compilation when fmt is not directly used
var _ = fmt.Sprintf
