package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"crossterm/internal/netproto"
)

// Default public relay server port
const defaultPort = 9000

// peer holds the last-known address for one side of a room.
type peer struct {
	addr     *net.UDPAddr
	publicIP string // just the IP part, used for same-LAN detection
	lastSeen time.Time
}

// Room tracks both players connected through the relay.
type Room struct {
	host    *peer
	joiner  *peer
	subMode string
	created time.Time

	// Track whether RELAY_READY has been sent this session.
	relayReadySent bool
}

// bothPresent returns true when both peers have registered/re-registered.
func (r *Room) bothPresent() bool {
	return r.host != nil && r.host.addr != nil &&
		r.joiner != nil && r.joiner.addr != nil
}

// sameLAN returns true when both peers share the same public-facing IP.
func (r *Room) sameLAN() bool {
	if !r.bothPresent() {
		return false
	}
	return r.host.publicIP == r.joiner.publicIP
}

// RelayServer is the central TURN-style relay.
type RelayServer struct {
	addr  *net.UDPAddr
	conn  *net.UDPConn
	rooms map[string]*Room
	mu    sync.RWMutex
}

func NewRelayServer(port int) (*RelayServer, error) {
	addr := &net.UDPAddr{Port: port, IP: net.ParseIP("0.0.0.0")}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	conn.SetReadBuffer(256 * 1024)
	conn.SetWriteBuffer(256 * 1024)
	return &RelayServer{
		addr:  addr,
		conn:  conn,
		rooms: make(map[string]*Room),
	}, nil
}

func (s *RelayServer) Run() {
	log.Printf("Relay server running on UDP :%d", s.addr.Port)

	// Background: clean up stale rooms
	go s.cleanup()

	buffer := make([]byte, 8192)
	for {
		n, remoteAddr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read error: %v", err)
			continue
		}

		var msg netproto.NetworkMessage
		if err := json.Unmarshal(buffer[:n], &msg); err != nil {
			continue // ignore malformed packets
		}

		s.handleMessage(&msg, remoteAddr)
	}
}

func (s *RelayServer) cleanup() {
	for {
		time.Sleep(1 * time.Minute)
		s.mu.Lock()
		now := time.Now()
		for id, room := range s.rooms {
			// Stale if no keepalive from either peer in 10 minutes
			hostStale := room.host == nil || now.Sub(room.host.lastSeen) > 10*time.Minute
			joinerStale := room.joiner == nil || now.Sub(room.joiner.lastSeen) > 10*time.Minute
			if hostStale && joinerStale {
				log.Printf("[CLEANUP] Removing stale room %s", id)
				delete(s.rooms, id)
			}
		}
		s.mu.Unlock()
	}
}

// send is a convenience helper.
func (s *RelayServer) send(msg netproto.NetworkMessage, to *net.UDPAddr) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	s.conn.WriteToUDP(b, to)
}

func (s *RelayServer) handleMessage(msg *netproto.NetworkMessage, sender *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch msg.Type {

	// ── Room creation ──────────────────────────────────────────────────────────
	case netproto.MsgCreateRoom:
		if msg.RoomID == "" {
			return
		}
		s.rooms[msg.RoomID] = &Room{
			host: &peer{
				addr:     sender,
				publicIP: sender.IP.String(),
				lastSeen: time.Now(),
			},
			subMode:  msg.SubMode,
			created:  time.Now(),
		}
		log.Printf("[CREATE] Room %s by %v (mode: %s)", msg.RoomID, sender, msg.SubMode)

	// ── Joiner handshake ──────────────────────────────────────────────────────
	case netproto.MsgJoinRoom:
		if msg.RoomID == "" {
			return
		}
		room, exists := s.rooms[msg.RoomID]
		if !exists {
			log.Printf("[JOIN FAIL] Room %s not found (from %v)", msg.RoomID, sender)
			return
		}

		room.joiner = &peer{
			addr:     sender,
			publicIP: sender.IP.String(),
			lastSeen: time.Now(),
		}
		room.relayReadySent = false // reset in case of reconnect

		// Send each peer the other's address (informational — clients may
		// optionally attempt direct connect if same LAN is signalled).
		s.send(netproto.NetworkMessage{
			Type:    netproto.MsgPeerInfo,
			PeerIP:  room.host.addr.String(),
			SubMode: room.subMode,
		}, sender)

		s.send(netproto.NetworkMessage{
			Type:   netproto.MsgPeerInfo,
			PeerIP: sender.String(),
		}, room.host.addr)

		log.Printf("[JOIN] Room %s: host=%v joiner=%v", msg.RoomID, room.host.addr, sender)

		// Immediately check if both peers are present and send RELAY_READY
		s.maybeSendRelayReady(msg.RoomID, room)

	// ── Re-register (NAT mapping refresh) ─────────────────────────────────────
	// Sent by both clients after setup. The relay updates its stored address
	// on every packet, so explicit re-register just updates the role assignment.
	case netproto.MsgReRegister:
		if msg.RoomID == "" || msg.PlayerID == nil {
			return
		}
		room, exists := s.rooms[msg.RoomID]
		if !exists {
			log.Printf("[RE-REG FAIL] Room %s not found (from %v)", msg.RoomID, sender)
			return
		}

		role := *msg.PlayerID
		if role == 1 { // host
			if room.host == nil {
				room.host = &peer{}
			}
			old := addrStr(room.host.addr)
			room.host.addr = sender
			room.host.publicIP = sender.IP.String()
			room.host.lastSeen = time.Now()
			log.Printf("[RE-REG] Host in room %s: %s → %v", msg.RoomID, old, sender)
		} else if role == 2 { // joiner
			if room.joiner == nil {
				room.joiner = &peer{}
			}
			old := addrStr(room.joiner.addr)
			room.joiner.addr = sender
			room.joiner.publicIP = sender.IP.String()
			room.joiner.lastSeen = time.Now()
			log.Printf("[RE-REG] Joiner in room %s: %s → %v", msg.RoomID, old, sender)
		}

		// After both re-register, send RELAY_READY to both
		s.maybeSendRelayReady(msg.RoomID, room)

	// ── Keepalive ─────────────────────────────────────────────────────────────
	// Clients send this every ~15s to keep the NAT pinhole open.
	// The relay uses it to update the sender's stored address transparently.
	case netproto.MsgKeepalive:
		if msg.RoomID == "" || msg.PlayerID == nil {
			return
		}
		room, exists := s.rooms[msg.RoomID]
		if !exists {
			return
		}
		if *msg.PlayerID == 1 && room.host != nil {
			room.host.addr = sender
			room.host.publicIP = sender.IP.String()
			room.host.lastSeen = time.Now()
		} else if *msg.PlayerID == 2 && room.joiner != nil {
			room.joiner.addr = sender
			room.joiner.publicIP = sender.IP.String()
			room.joiner.lastSeen = time.Now()
		}
		// ACK the keepalive so the client knows the relay is alive
		s.send(netproto.NetworkMessage{Type: netproto.MsgKeepalive, RoomID: msg.RoomID}, sender)

	// ── Relay forwarding ──────────────────────────────────────────────────────
	case netproto.MsgRelay:
		if msg.RoomID == "" {
			return
		}
		room, exists := s.rooms[msg.RoomID]
		if !exists {
			log.Printf("[RELAY DROP] Room %s not found (from %v)", msg.RoomID, sender)
			return
		}

		// Determine which peer is sending. Try exact address match first,
		// then fall back to PlayerID (critical when both peers share a
		// public IP and NAT rebinds change ports).
		var targetAddr *net.UDPAddr
		if matchesPeer(sender, room.host) {
			room.host.addr = sender
			room.host.lastSeen = time.Now()
			if room.joiner != nil {
				targetAddr = room.joiner.addr
			}
		} else if matchesPeer(sender, room.joiner) {
			room.joiner.addr = sender
			room.joiner.lastSeen = time.Now()
			targetAddr = room.host.addr
		} else if msg.PlayerID != nil {
			// Address match failed — use PlayerID to route.
			if *msg.PlayerID == 1 && room.host != nil {
				log.Printf("[RELAY REMAP] Host in room %s: %s → %v (by PlayerID)",
					msg.RoomID, addrStr(room.host.addr), sender)
				room.host.addr = sender
				room.host.lastSeen = time.Now()
				if room.joiner != nil {
					targetAddr = room.joiner.addr
				}
			} else if *msg.PlayerID == 2 && room.joiner != nil {
				log.Printf("[RELAY REMAP] Joiner in room %s: %s → %v (by PlayerID)",
					msg.RoomID, addrStr(room.joiner.addr), sender)
				room.joiner.addr = sender
				room.joiner.lastSeen = time.Now()
				targetAddr = room.host.addr
			}
		} else {
			log.Printf("[RELAY DROP] Unknown sender %v for room %s (host=%v, joiner=%v)",
				sender, msg.RoomID, addrStr(room.host.addr), addrStr(room.joiner.addr))
			return
		}

		if targetAddr != nil {
			b, _ := json.Marshal(msg)
			s.conn.WriteToUDP(b, targetAddr)
		}
	}
}

// maybeSendRelayReady sends RELAY_READY to both peers once they are both
// registered. It also signals SAME_LAN if both share the same public IP.
func (s *RelayServer) maybeSendRelayReady(roomID string, room *Room) {
	if room.relayReadySent || !room.bothPresent() {
		return
	}
	room.relayReadySent = true
	log.Printf("[RELAY_READY] Room %s — both peers registered (same_lan=%v)", roomID, room.sameLAN())

	readyMsg := netproto.NetworkMessage{Type: netproto.MsgRelayReady, RoomID: roomID}
	if room.sameLAN() {
		// Piggyback each peer's local address so the other can try a direct connection.
		readyMsg.Type = netproto.MsgSameLAN
	}

	// Send each peer the other's address when on same LAN
	hostMsg := readyMsg
	joinerMsg := readyMsg
	if room.sameLAN() {
		hostMsg.PeerIP = room.joiner.addr.String()
		joinerMsg.PeerIP = room.host.addr.String()
	}

	s.send(hostMsg, room.host.addr)
	s.send(joinerMsg, room.joiner.addr)
}

// matchesPeer returns true if the sender address matches a peer's recorded address.
// It compares by IP+Port. If the peer's address is nil, always false.
func matchesPeer(sender *net.UDPAddr, p *peer) bool {
	if p == nil || p.addr == nil {
		return false
	}
	return sender.String() == p.addr.String()
}

// addrStr safely stringifies a possibly-nil *net.UDPAddr.
func addrStr(a *net.UDPAddr) string {
	if a == nil {
		return "<nil>"
	}
	return a.String()
}

func main() {
	port := flag.Int("port", defaultPort, "Port to run relay server on")
	flag.Parse()

	srv, err := NewRelayServer(*port)
	if err != nil {
		fmt.Printf("Fatal: %v\n", err)
		return
	}
	srv.Run()
}
