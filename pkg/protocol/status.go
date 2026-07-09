package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Status-phase packet IDs (PVN 776).
const (
	PacketIDStatusRequest  int32 = 0x00
	PacketIDPingRequest    int32 = 0x01
	PacketIDStatusResponse int32 = 0x00
	PacketIDPongResponse   int32 = 0x01
)

// PlayerSample is one entry in the ping "players.sample" array.
type PlayerSample struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// ServerStatus is the JSON payload of a Status Response.
type ServerStatus struct {
	Version            ServerPingVersion `json:"version"`
	Players            ServerPingPlayers `json:"players"`
	Description        json.RawMessage   `json:"description"`
	Favicon            string            `json:"favicon,omitempty"`
	EnforcesSecureChat bool              `json:"enforcesSecureChat"`
}

// ServerPingVersion is the "version" object of a ping response.
type ServerPingVersion struct {
	Name     string `json:"name"`
	Protocol int32  `json:"protocol"`
}

// ServerPingPlayers is the "players" object of a ping response.
type ServerPingPlayers struct {
	Max    int            `json:"max"`
	Online int            `json:"online"`
	Sample []PlayerSample `json:"sample,omitempty"`
}

// EncodeStatusResponse builds the payload for a Status Response packet (Status 0x00).
func EncodeStatusResponse(s *ServerStatus) ([]byte, error) {
	js, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := WriteString(&buf, string(js)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodePingRequest decodes a Ping Request payload into its timestamp.
func DecodePingRequest(data []byte) (int64, error) {
	r := bytes.NewReader(data)
	ts, err := ReadInt64(r)
	if err != nil {
		return 0, fmt.Errorf("read timestamp: %w", err)
	}
	return ts, nil
}

// EncodePongResponse builds the payload for a Pong Response packet (Status 0x01).
func EncodePongResponse(timestamp int64) []byte {
	var buf bytes.Buffer
	_ = WriteInt64(&buf, timestamp)
	return buf.Bytes()
}
