package netproto

type MsgType string

const (
	MsgCreateRoom   MsgType = "CREATE_ROOM"
	MsgJoinRoom     MsgType = "JOIN_ROOM"
	MsgPeerInfo     MsgType = "PEER_INFO"
	MsgPunch        MsgType = "PUNCH"
	MsgPunchAck     MsgType = "PUNCH_ACK"
	MsgGameEvent    MsgType = "GAME_EVENT"
	MsgPuzRequest   MsgType = "PUZ_REQ"
	MsgPuzTransfer  MsgType = "PUZ_DATA"
	MsgRelay        MsgType = "RELAY" // Packets to be forwarded by the bootstrap/relay server
)

type NetworkMessage struct {
	Type        MsgType `json:"type"`
	RoomID      string  `json:"room_id,omitempty"`
	PeerIP      string  `json:"peer_ip,omitempty"`
	Payload     []byte  `json:"payload,omitempty"`      // Serialized engine.Event or puzzle chunks
	ChunkIndex  int     `json:"chunk_index,omitempty"`  // For multi-packet transfers
	TotalChunks int     `json:"total_chunks,omitempty"` // For multi-packet transfers
}
