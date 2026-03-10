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

type Room struct {
	HostAddr  *net.UDPAddr
	JoinAddr  *net.UDPAddr
	Created   time.Time
	LastPing  time.Time
}

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
	return &RelayServer{
		addr:  addr,
		conn:  conn,
		rooms: make(map[string]*Room),
	}, nil
}

func (s *RelayServer) Run() {
	log.Printf("Relay server running on UDP port %d", s.addr.Port)

	// Background cleanup of old rooms
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			s.mu.Lock()
			now := time.Now()
			for id, room := range s.rooms {
				if now.Sub(room.LastPing) > 10*time.Minute {
					log.Printf("Cleaning up stale room %s", id)
					delete(s.rooms, id)
				}
			}
			s.mu.Unlock()
		}
	}()

	buffer := make([]byte, 4096)
	for {
		n, remoteAddr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read error: %v", err)
			continue
		}

		var msg netproto.NetworkMessage
		if err := json.Unmarshal(buffer[:n], &msg); err != nil {
			continue // Invalid packet
		}

		s.handleMessage(&msg, remoteAddr)
	}
}

func (s *RelayServer) handleMessage(msg *netproto.NetworkMessage, sender *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch msg.Type {
	case netproto.MsgCreateRoom:
		if msg.RoomID == "" {
			return
		}
		s.rooms[msg.RoomID] = &Room{
			HostAddr: sender,
			Created:  time.Now(),
			LastPing: time.Now(),
		}
		log.Printf("[CREATE] Room %s by %v", msg.RoomID, sender)

		// Send STUN equivalent back
		resp := netproto.NetworkMessage{
			Type:   netproto.MsgPeerInfo,
			PeerIP: sender.String(),
		}
		bResp, _ := json.Marshal(resp)
		s.conn.WriteToUDP(bResp, sender)

	case netproto.MsgJoinRoom:
		if msg.RoomID == "" {
			return
		}
		room, exists := s.rooms[msg.RoomID]
		if !exists {
			log.Printf("[JOIN FAILED] Room %s not found (from %v)", msg.RoomID, sender)
			return
		}

		room.JoinAddr = sender
		room.LastPing = time.Now()

		// Send host's info to the joiner
		joinResp := netproto.NetworkMessage{
			Type:   netproto.MsgPeerInfo,
			PeerIP: room.HostAddr.String(),
		}
		bJoin, _ := json.Marshal(joinResp)
		s.conn.WriteToUDP(bJoin, sender)

		// Send joiner's info to the host
		hostResp := netproto.NetworkMessage{
			Type:   netproto.MsgPeerInfo,
			PeerIP: sender.String(),
		}
		bHost, _ := json.Marshal(hostResp)
		s.conn.WriteToUDP(bHost, room.HostAddr)

		log.Printf("[MATCHED] Room %s: %v <-> %v", msg.RoomID, room.HostAddr, sender)

	case netproto.MsgRelay:
		// Forward payload blindly to the OTHER peer in the room
		if msg.RoomID == "" {
			return
		}
		room, exists := s.rooms[msg.RoomID]
		if !exists {
			return
		}

		room.LastPing = time.Now()

		var targetAddr *net.UDPAddr
		if sender.String() == room.HostAddr.String() {
			if room.JoinAddr != nil {
				targetAddr = room.JoinAddr
			}
		} else if room.JoinAddr != nil && sender.String() == room.JoinAddr.String() {
			targetAddr = room.HostAddr
		}

		if targetAddr != nil {
			// Forwarding the exact same RelayMessage is easiest so client knows it's a relay.
			bRelay, _ := json.Marshal(msg)
			s.conn.WriteToUDP(bRelay, targetAddr)
		}
	}
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
