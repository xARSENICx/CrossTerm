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

// Default public bootstrap server port
const defaultPort = 9000

type Room struct {
	HostAddr *net.UDPAddr
	Created  time.Time
}

type BootstrapServer struct {
	addr  *net.UDPAddr
	conn  *net.UDPConn
	rooms map[string]*Room
	mu    sync.RWMutex
}

func NewBootstrapServer(port int) (*BootstrapServer, error) {
	addr := &net.UDPAddr{Port: port, IP: net.ParseIP("0.0.0.0")}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return &BootstrapServer{
		addr:  addr,
		conn:  conn,
		rooms: make(map[string]*Room),
	}, nil
}

func (s *BootstrapServer) Run() {
	log.Printf("Bootstrap server running on UDP port %d", s.addr.Port)

	// Background cleanup of old rooms
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			s.mu.Lock()
			now := time.Now()
			for id, room := range s.rooms {
				if now.Sub(room.Created) > 30*time.Minute {
					log.Printf("Cleaning up stale room %s", id)
					delete(s.rooms, id)
				}
			}
			s.mu.Unlock()
		}
	}()

	buffer := make([]byte, 2048)
	for {
		n, remoteAddr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read error: %v", err)
			continue
		}

		var msg netproto.NetworkMessage
		if err := json.Unmarshal(buffer[:n], &msg); err != nil {
			log.Printf("Invalid message from %v: %v", remoteAddr, err)
			continue
		}

		s.handleMessage(&msg, remoteAddr)
	}
}

func (s *BootstrapServer) handleMessage(msg *netproto.NetworkMessage, sender *net.UDPAddr) {
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
		}
		log.Printf("Room %s created by %v", msg.RoomID, sender)

	case netproto.MsgJoinRoom:
		if msg.RoomID == "" {
			return
		}
		room, exists := s.rooms[msg.RoomID]
		if !exists {
			log.Printf("Join failed: room %s not found (from %v)", msg.RoomID, sender)
			return
		}

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

		log.Printf("Paired room %s: Host %v <-> Joiner %v", msg.RoomID, room.HostAddr, sender)
		// We can remove the room once joined to prevent others joining, 
		// or keep it for multi-spectator (for now 1v1).
		delete(s.rooms, msg.RoomID)
	}
}

func main() {
	port := flag.Int("port", defaultPort, "Port to run bootstrap server on")
	flag.Parse()

	srv, err := NewBootstrapServer(*port)
	if err != nil {
		fmt.Printf("Fatal: %v\n", err)
		return
	}
	srv.Run()
}
