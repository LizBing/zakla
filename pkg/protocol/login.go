package protocol

import (
	"bytes"
	"fmt"
)

// Login-phase packet IDs (PVN 776).
const (
	PacketIDLoginStart        int32 = 0x00 // serverbound
	PacketIDLoginDisconnect   int32 = 0x00 // clientbound
	PacketIDEncryptionRequest int32 = 0x01 // clientbound
	PacketIDLoginSuccess      int32 = 0x02 // clientbound
	PacketIDSetCompression    int32 = 0x03 // clientbound
	PacketIDLoginAcknowledged int32 = 0x03 // serverbound
)

// DecodeLoginStart reads the serverbound Login Start payload.
// Fields: Name (String 16), Player UUID (UUID).
func DecodeLoginStart(data []byte) (name string, playerUUID UUID, err error) {
	r := bytes.NewReader(data)
	name, err = ReadString(r)
	if err != nil {
		return "", UUID{}, fmt.Errorf("read name: %w", err)
	}
	playerUUID, err = ReadUUID(r)
	if err != nil {
		return "", UUID{}, fmt.Errorf("read player uuid: %w", err)
	}
	return name, playerUUID, nil
}

// EncodeLoginSuccess builds the clientbound Login Success payload (Login 0x02).
// Fields: Game Profile (UUID, Username, Properties) + Session ID (UUID).
// In offline mode, Properties is an empty array.
func EncodeLoginSuccess(playerUUID UUID, name string, sessionID UUID) []byte {
	var buf bytes.Buffer
	_ = WriteUUID(&buf, playerUUID)
	_ = WriteString(&buf, name)
	_ = WriteVarInt(&buf, 0) // property count = 0
	_ = WriteUUID(&buf, sessionID)
	return buf.Bytes()
}

// EncodeSetCompression builds the Set Compression payload (Login 0x03).
// A non-negative threshold enables compression for all following packets.
func EncodeSetCompression(threshold int32) []byte {
	var buf bytes.Buffer
	_ = WriteVarInt(&buf, threshold)
	return buf.Bytes()
}

// EncodeLoginDisconnect builds the Login Disconnect payload (Login 0x00).
// The reason is a JSON Text Component (sent as a String).
func EncodeLoginDisconnect(reasonJSON string) []byte {
	var buf bytes.Buffer
	_ = WriteString(&buf, reasonJSON)
	return buf.Bytes()
}
