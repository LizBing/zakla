package protocol

import (
	"bytes"
	"fmt"
)

// Handshake intents (Next State field of the intention packet).
const (
	IntentStatus   int32 = 1
	IntentLogin    int32 = 2
	IntentTransfer int32 = 3
)

// PacketIDHandshake is the serverbound intention packet ID (Handshaking 0x00).
const PacketIDHandshake int32 = 0x00

// Handshake is the serverbound intention packet sent right after TCP connect.
type Handshake struct {
	ProtocolVersion int32
	ServerAddress   string
	ServerPort      uint16
	Intent          int32 // 1=status, 2=login, 3=transfer
}

// DecodeHandshake decodes a handshake packet payload.
func DecodeHandshake(data []byte) (*Handshake, error) {
	r := bytes.NewReader(data)
	h := &Handshake{}
	var err error
	if h.ProtocolVersion, err = ReadVarInt(r); err != nil {
		return nil, fmt.Errorf("read protocol version: %w", err)
	}
	if h.ServerAddress, err = ReadString(r); err != nil {
		return nil, fmt.Errorf("read server address: %w", err)
	}
	if h.ServerPort, err = ReadUint16(r); err != nil {
		return nil, fmt.Errorf("read server port: %w", err)
	}
	if h.Intent, err = ReadVarInt(r); err != nil {
		return nil, fmt.Errorf("read intent: %w", err)
	}
	return h, nil
}
