package networksystem

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"time"
)

// GetPublicAddress queries a STUN server to discover the public NAT mapped address for the given socket.
func GetPublicAddress(conn *net.UDPConn, stunServer string) (*net.UDPAddr, error) {
	serverAddr, err := net.ResolveUDPAddr("udp", stunServer)
	if err != nil {
		return nil, err
	}

	// RFC 5389 STUN Binding Request
	req := make([]byte, 20)
	req[0] = 0x00
	req[1] = 0x01 // Binding Request
	req[2] = 0x00
	req[3] = 0x00 // Message Length

	// Magic Cookie
	req[4] = 0x21
	req[5] = 0x12
	req[6] = 0xA4
	req[7] = 0x42

	// Transaction ID
	rand.Read(req[8:20])

	if _, err := conn.WriteToUDP(req, serverAddr); err != nil {
		return nil, err
	}

	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := conn.ReadFromUDP(buf)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		return nil, err
	}

	if n < 20 {
		return nil, errors.New("STUN response too short")
	}

	// Parse Attributes
	offset := 20
	for offset+4 <= n {
		attrType := binary.BigEndian.Uint16(buf[offset : offset+2])
		attrLen := int(binary.BigEndian.Uint16(buf[offset+2 : offset+4]))
		offset += 4

		if offset+attrLen > n {
			break
		}

		if attrType == 0x0020 { // XOR-MAPPED-ADDRESS
			// Skip reserved byte
			family := buf[offset+1]
			if family == 0x01 { // IPv4
				xport := binary.BigEndian.Uint16(buf[offset+2 : offset+4])
				port := xport ^ 0x2112

				ip := make(net.IP, 4)
				ip[0] = buf[offset+4] ^ 0x21
				ip[1] = buf[offset+5] ^ 0x12
				ip[2] = buf[offset+6] ^ 0xA4
				ip[3] = buf[offset+7] ^ 0x42

				return &net.UDPAddr{IP: ip, Port: int(port)}, nil
			}
		}

		// Align to 4 bytes boundary
		pad := attrLen % 4
		if pad != 0 {
			attrLen += 4 - pad
		}
		offset += attrLen
	}

	return nil, errors.New("failed to find XOR-MAPPED-ADDRESS in STUN response")
}
