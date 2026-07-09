// Package net provides connection-level networking for the Minecraft server:
// state tracking, zlib-compressed packet framing, and read/write helpers.
package net

import (
	"bytes"
	"fmt"
	stdnet "net"
	"sync"
	"time"

	"github.com/zakla/mc-server/pkg/protocol"
)

// MaxUncompressedLen bounds decompressed serverbound packets (2^23 bytes, per spec).
const MaxUncompressedLen = 8388608

// ConnectionState is the high-level protocol state of a connection.
type ConnectionState int

const (
	StateHandshake ConnectionState = iota
	StateStatus
	StateLogin
	StateConfiguration
	StatePlay
)

// String returns a human-readable state name.
func (s ConnectionState) String() string {
	switch s {
	case StateHandshake:
		return "Handshake"
	case StateStatus:
		return "Status"
	case StateLogin:
		return "Login"
	case StateConfiguration:
		return "Configuration"
	case StatePlay:
		return "Play"
	default:
		return "Unknown"
	}
}

// Connection wraps a TCP connection with Minecraft packet framing.
type Connection struct {
	conn  stdnet.Conn
	state ConnectionState

	writeMu              sync.Mutex
	compressionThreshold int // < 0 means compression disabled

	closed   bool
	closedMu sync.Mutex
}

// NewConnection wraps a raw TCP connection in the Handshake state.
func NewConnection(conn stdnet.Conn) *Connection {
	return &Connection{
		conn:                 conn,
		state:                StateHandshake,
		compressionThreshold: -1,
	}
}

// Conn returns the underlying network connection.
func (c *Connection) Conn() stdnet.Conn { return c.conn }

// SetState transitions the connection to a new protocol state.
func (c *Connection) SetState(s ConnectionState) { c.state = s }

// State returns the current protocol state.
func (c *Connection) State() ConnectionState { return c.state }

// SetCompression enables/disables zlib compression for subsequent packets.
// threshold < 0 disables compression; otherwise packets whose uncompressed
// body length >= threshold are compressed.
func (c *Connection) SetCompression(threshold int) {
	c.writeMu.Lock()
	c.compressionThreshold = threshold
	c.writeMu.Unlock()
}

// CompressionThreshold returns the current threshold (<0 = disabled).
func (c *Connection) CompressionThreshold() int {
	return c.compressionThreshold
}

// SetReadDeadline sets a deadline for the next read operation.
func (c *Connection) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// ReadPacket reads one framed packet, transparently handling compression,
// and returns its (packetID, payload).
func (c *Connection) ReadPacket() (int32, []byte, error) {
	// Packet Length = VarInt (length of remaining bytes).
	length, err := protocol.ReadVarInt(c.conn)
	if err != nil {
		return 0, nil, err
	}
	if length <= 0 {
		return 0, nil, nil
	}
	if length > protocol.MaxPacketLength {
		return 0, nil, errPacketTooLarge(length)
	}

	data := make([]byte, length)
	if _, err := readFull(c.conn, data); err != nil {
		return 0, nil, err
	}

	var payload []byte
	if c.compressionThreshold >= 0 {
		br := bytes.NewReader(data)
		dataLen, err := protocol.ReadVarInt(br)
		if err != nil {
			return 0, nil, err
		}
		if dataLen > 0 {
			// Compressed: the rest is zlib data that decompresses to dataLen bytes.
			if dataLen > MaxUncompressedLen {
				return 0, nil, errDecompressTooLarge(dataLen)
			}
			rest := make([]byte, br.Len())
			if _, err := readFull(br, rest); err != nil {
				return 0, nil, err
			}
			payload, err = protocol.Decompress(rest, int(dataLen))
			if err != nil {
				return 0, nil, err
			}
		} else {
			// Uncompressed: data length prefix was 0; remaining bytes are the packet.
			payload = make([]byte, br.Len())
			if _, err := readFull(br, payload); err != nil {
				return 0, nil, err
			}
		}
	} else {
		payload = data
	}

	return protocol.SplitPayload(payload)
}

// WritePacket frames and writes one packet, compressing if enabled and large enough.
func (c *Connection) WritePacket(packetID int32, data []byte) error {
	payload, err := protocol.AssemblePayload(packetID, data)
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var frame bytes.Buffer
	if c.compressionThreshold >= 0 {
		var body bytes.Buffer
		if len(payload) >= c.compressionThreshold {
			compressed, err := protocol.Compress(payload)
			if err != nil {
				return err
			}
			_ = protocol.WriteVarInt(&body, int32(len(payload)))
			_, _ = body.Write(compressed)
		} else {
			_ = protocol.WriteVarInt(&body, 0) // data length = 0 (uncompressed)
			_, _ = body.Write(payload)
		}
		_ = protocol.WriteVarInt(&frame, int32(body.Len()))
		_, _ = frame.Write(body.Bytes())
	} else {
		_ = protocol.WriteVarInt(&frame, int32(len(payload)))
		_, _ = frame.Write(payload)
	}

	_, err = c.conn.Write(frame.Bytes())
	return err
}

// RemoteAddr returns the remote address.
func (c *Connection) RemoteAddr() stdnet.Addr { return c.conn.RemoteAddr() }

// Close closes the underlying connection.
func (c *Connection) Close() error {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

// IsClosed reports whether the connection has been closed.
func (c *Connection) IsClosed() bool {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	return c.closed
}

// readFull is io.ReadFull without importing io here explicitly.
func readFull(r interface{ Read([]byte) (int, error) }, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func errPacketTooLarge(length int32) error {
	return fmt.Errorf("net: packet length %d too large", length)
}

func errDecompressTooLarge(length int32) error {
	return fmt.Errorf("net: decompressed length %d exceeds limit", length)
}
