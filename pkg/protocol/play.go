package protocol

import (
	"bytes"
	"fmt"
)

// Play-phase packet IDs (PVN 776).
const (
	// Clientbound
	PlayIDChangeDifficulty  int32 = 0x0A
	PlayIDGameEvent         int32 = 0x26
	PlayIDDisconnect        int32 = 0x20
	PlayIDKeepAlive         int32 = 0x2C
	PlayIDChunkDataLight    int32 = 0x2D
	PlayIDLogin             int32 = 0x31
	PlayIDPlayerAbilities   int32 = 0x40
	PlayIDPlayerChat        int32 = 0x41
	PlayIDPlayerInfoUpdate  int32 = 0x46
	PlayIDSynchPlayerPos    int32 = 0x48
	PlayIDSetCenterChunk    int32 = 0x5E
	PlayIDSetDefaultSpawn   int32 = 0x61
	PlayIDSetHealth         int32 = 0x68
	PlayIDSetHeldItem       int32 = 0x69
	PlayIDUpdateTime        int32 = 0x71
	PlayIDSystemChat        int32 = 0x79
	PlayIDPluginMessage     int32 = 0x18

	// Serverbound
	PlayIDConfirmTeleport   int32 = 0x00
	PlayIDChatMessage       int32 = 0x09
	PlayIDKeepAliveSB       int32 = 0x1C
	PlayIDPlayerPosRot      int32 = 0x1F
	PlayIDPlayerLoaded      int32 = 0x2C
	PlayIDClientTickEnd     int32 = 0x0D
	PlayIDClientInfoSB      int32 = 0x0E
)

// Game events (Game Event packet).
const (
	GameEventStartWaitChunks uint8 = 13
)

// LoginPlay carries the fields of the clientbound Login (play) packet (0x31).
type LoginPlay struct {
	EntityID            int32
	IsHardcore          bool
	DimensionNames      []string // identifiers
	MaxPlayers          int32
	ViewDistance        int32
	SimulationDistance  int32
	ReducedDebugInfo    bool
	EnableRespawnScreen bool
	DoLimitedCrafting   bool
	DimensionType       int32  // ID in dimension_type registry
	DimensionName       string // identifier
	HashedSeed          int64
	GameMode            uint8
	PreviousGameMode   int8
	IsDebug             bool
	IsFlat              bool
	PortalCooldown      int32
	SeaLevel            int32
	OnlineMode          bool
	EnforcesSecureChat  bool
}

// EncodeLoginPlay builds the Login (play) payload (Play 0x31).
func EncodeLoginPlay(l *LoginPlay) []byte {
	var b bytes.Buffer
	_ = WriteInt32(&b, l.EntityID)
	_ = WriteBool(&b, l.IsHardcore)
	_ = WriteVarInt(&b, int32(len(l.DimensionNames)))
	for _, d := range l.DimensionNames {
		_ = WriteIdentifier(&b, d)
	}
	_ = WriteVarInt(&b, l.MaxPlayers)
	_ = WriteVarInt(&b, l.ViewDistance)
	_ = WriteVarInt(&b, l.SimulationDistance)
	_ = WriteBool(&b, l.ReducedDebugInfo)
	_ = WriteBool(&b, l.EnableRespawnScreen)
	_ = WriteBool(&b, l.DoLimitedCrafting)
	_ = WriteVarInt(&b, l.DimensionType)
	_ = WriteIdentifier(&b, l.DimensionName)
	_ = WriteInt64(&b, l.HashedSeed)
	_ = WriteUint8(&b, l.GameMode)
	_ = WriteInt8(&b, l.PreviousGameMode)
	_ = WriteBool(&b, l.IsDebug)
	_ = WriteBool(&b, l.IsFlat)
	_ = WriteBool(&b, false) // has death location = false
	_ = WriteVarInt(&b, l.PortalCooldown)
	_ = WriteVarInt(&b, l.SeaLevel)
	_ = WriteBool(&b, l.OnlineMode)
	_ = WriteBool(&b, l.EnforcesSecureChat)
	return b.Bytes()
}

// EncodeGameEvent builds the Game Event payload (Play 0x26).
func EncodeGameEvent(event uint8, value float32) []byte {
	var b bytes.Buffer
	_ = WriteUint8(&b, event)
	_ = WriteFloat32(&b, value)
	return b.Bytes()
}

// EncodeSetCenterChunk builds the Set Center Chunk payload (Play 0x5E).
func EncodeSetCenterChunk(x, z int32) []byte {
	var b bytes.Buffer
	_ = WriteVarInt(&b, x)
	_ = WriteVarInt(&b, z)
	return b.Bytes()
}

// EncodeSynchronizePlayerPos builds the Synchronize Player Position payload (Play 0x48).
// flags=0 means all axes absolute.
func EncodeSynchronizePlayerPos(teleportID int32, x, y, z, vx, vy, vz float64, yaw, pitch float32, flags int32) []byte {
	var b bytes.Buffer
	_ = WriteVarInt(&b, teleportID)
	_ = WriteFloat64(&b, x)
	_ = WriteFloat64(&b, y)
	_ = WriteFloat64(&b, z)
	_ = WriteFloat64(&b, vx)
	_ = WriteFloat64(&b, vy)
	_ = WriteFloat64(&b, vz)
	_ = WriteFloat32(&b, yaw)
	_ = WriteFloat32(&b, pitch)
	_ = WriteInt32(&b, flags)
	return b.Bytes()
}

// EncodePlayerAbilities builds the Player Abilities payload (Play 0x40).
func EncodePlayerAbilities(flags int8, flySpeed, fov float32) []byte {
	var b bytes.Buffer
	_ = WriteInt8(&b, flags)
	_ = WriteFloat32(&b, flySpeed)
	_ = WriteFloat32(&b, fov)
	return b.Bytes()
}

// EncodeSetHeldItem builds the Set Held Item payload (Play 0x69).
func EncodeSetHeldItem(slot int32) []byte {
	var b bytes.Buffer
	_ = WriteVarInt(&b, slot)
	return b.Bytes()
}

// EncodeChangeDifficulty builds the Change Difficulty payload (Play 0x0A).
func EncodeChangeDifficulty(difficulty uint8, locked bool) []byte {
	var b bytes.Buffer
	_ = WriteUint8(&b, difficulty)
	_ = WriteBool(&b, locked)
	return b.Bytes()
}

// EncodeSetDefaultSpawn builds the Set Default Spawn Position payload (Play 0x61).
func EncodeSetDefaultSpawn(dimension string, pos Position, yaw, pitch float32) []byte {
	var b bytes.Buffer
	_ = WriteIdentifier(&b, dimension)
	_ = WritePosition(&b, pos)
	_ = WriteFloat32(&b, yaw)
	_ = WriteFloat32(&b, pitch)
	return b.Bytes()
}

// EncodeSetHealth builds the Set Health payload (Play 0x68).
func EncodeSetHealth(health float32, food int32, saturation float32) []byte {
	var b bytes.Buffer
	_ = WriteFloat32(&b, health)
	_ = WriteVarInt(&b, food)
	_ = WriteFloat32(&b, saturation)
	return b.Bytes()
}

// EncodeKeepAlive builds the Keep Alive payload (Long id).
func EncodeKeepAlive(id int64) []byte {
	var b bytes.Buffer
	_ = WriteInt64(&b, id)
	return b.Bytes()
}

// EncodePlayerInfoUpdateAdd builds a Player Info Update payload that adds a
// single player with game mode, listed flag, and latency (Play 0x46).
func EncodePlayerInfoUpdateAdd(playerUUID UUID, name string, gamemode int32, listed bool, ping int32) []byte {
	var b bytes.Buffer
	// actions: 0x01 Add Player | 0x04 Game Mode | 0x08 Listed | 0x10 Latency
	_ = WriteUint8(&b, 0x01|0x04|0x08|0x10)
	_ = WriteVarInt(&b, 1) // one player
	_ = WriteUUID(&b, playerUUID)
	// Add Player
	_ = WriteString(&b, name)
	_ = WriteVarInt(&b, 0) // properties count
	// Game Mode
	_ = WriteVarInt(&b, gamemode)
	// Listed
	_ = WriteBool(&b, listed)
	// Latency
	_ = WriteVarInt(&b, ping)
	return b.Bytes()
}

// EncodeSystemChat builds the System Chat Message payload (Play 0x79).
// contentNBT is a serialized NBT Text Component (e.g. from PlainTextComponent).
func EncodeSystemChat(contentNBT []byte, overlay bool) []byte {
	var b bytes.Buffer
	_, _ = b.Write(contentNBT)
	_ = WriteBool(&b, overlay)
	return b.Bytes()
}

// EncodeDisconnect builds the Disconnect payload (Play 0x20).
// reasonNBT is a serialized NBT Text Component.
func EncodeDisconnect(reasonNBT []byte) []byte {
	return reasonNBT
}

// DecodeConfirmTeleport reads the Confirm Teleportation payload (Play 0x00).
func DecodeConfirmTeleport(data []byte) (int32, error) {
	r := bytes.NewReader(data)
	return ReadVarInt(r)
}

// DecodeKeepAlive reads the Keep Alive payload (Long id).
func DecodeKeepAlive(data []byte) (int64, error) {
	r := bytes.NewReader(data)
	return ReadInt64(r)
}

// DecodeChatMessage reads only the chat message text from a Chat Message payload
// (Play 0x09). Remaining signature/ack fields are ignored — only the message
// content is needed for echo/broadcast.
func DecodeChatMessage(data []byte) (string, error) {
	r := bytes.NewReader(data)
	msg, err := ReadString(r)
	if err != nil {
		return "", fmt.Errorf("read chat message: %w", err)
	}
	return msg, nil
}
