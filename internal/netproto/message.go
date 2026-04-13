package netproto

type MsgType string

const (
	MsgCreateRoom   MsgType = "CREATE_ROOM"
	MsgJoinRoom     MsgType = "JOIN_ROOM"
	MsgPeerInfo     MsgType = "PEER_INFO"
	MsgPunch        MsgType = "PUNCH"     // Reserved; kept for protocol compatibility
	MsgPunchAck     MsgType = "PUNCH_ACK" // Reserved; kept for protocol compatibility
	MsgGameEvent    MsgType = "GAME_EVENT"
	MsgPuzRequest   MsgType = "PUZ_REQ"
	MsgPuzTransfer  MsgType = "PUZ_DATA"
	MsgRelay        MsgType = "RELAY" // Packets to be forwarded by the relay server
	MsgCellUpdate   MsgType = "CELL_UPDATE"
	MsgPeerCursor   MsgType = "PEER_CURSOR"
	MsgPeerUsername MsgType = "PEER_USERNAME"
	MsgClueSolved   MsgType = "CLUE_SOLVED"
	MsgReady        MsgType = "READY"       // Joiner signals it's ready to receive puzzle
	MsgReRegister   MsgType = "RE_REGISTER" // Re-register with relay to refresh NAT mapping
	MsgKeepalive    MsgType = "KEEPALIVE"   // Periodic heartbeat to keep NAT pinhole open
	MsgRelayReady   MsgType = "RELAY_READY" // Relay confirms both peers are registered and relay is active
	MsgSameLAN      MsgType = "SAME_LAN"    // Relay signals that both peers share the same public IP
)

type NetworkMessage struct {
	Type        MsgType `json:"type"`
	RoomID      string  `json:"room_id,omitempty"`
	PeerIP      string  `json:"peer_ip,omitempty"`
	Payload     []byte  `json:"payload,omitempty"`      // Serialized engine.Event or puzzle chunks
	ChunkIndex  *int    `json:"chunk_index,omitempty"`  // For multi-packet transfers
	TotalChunks *int    `json:"total_chunks,omitempty"` // For multi-packet transfers

	// Collab Mode Fields
	X           *int    `json:"x,omitempty"`
	Y           *int    `json:"y,omitempty"`
	CellValue   *byte   `json:"cell_value,omitempty"`
	Username    *string `json:"username,omitempty"`
	PlayerID    *int    `json:"player_id,omitempty"`
	SolvedCount *int    `json:"solved_count,omitempty"`
	CursorDir   *int    `json:"cursor_dir,omitempty"` // Cast to puzzle.Direction (0/1)
	SubMode     string  `json:"sub_mode,omitempty"`   // Synced game rules

	// Cryptographic Authenticity Fields
	PublicKey []byte `json:"pub_key,omitempty"`  // Standard Ed25519 PubKey injected during handshakes
	Signature []byte `json:"sig,omitempty"`      // Ed25519 hash of the message slice
	Sequence  int64  `json:"sequence,omitempty"` // Incrementing sequence to drop replay attacks
}
