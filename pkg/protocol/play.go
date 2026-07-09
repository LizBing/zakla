package protocol

import (
	"bytes"
	"fmt"
)

// Play-phase packet IDs (PVN 776).
const (
	// Clientbound
	PlayIDChangeDifficulty    int32 = 0x0A
	PlayIDGameEvent           int32 = 0x26
	PlayIDDisconnect          int32 = 0x20
	PlayIDKeepAlive           int32 = 0x2C
	PlayIDChunkDataLight      int32 = 0x2D
	PlayIDLogin               int32 = 0x31
	PlayIDPlayerAbilities     int32 = 0x40
	PlayIDPlayerChat          int32 = 0x41
	PlayIDPlayerInfoUpdate    int32 = 0x46
	PlayIDSynchPlayerPos      int32 = 0x48
	PlayIDSetCenterChunk      int32 = 0x5E
	PlayIDSetDefaultSpawn     int32 = 0x61
	PlayIDSetHealth           int32 = 0x68
	PlayIDSetHeldItem         int32 = 0x69
	PlayIDUpdateTime          int32 = 0x71
	PlayIDSystemChat          int32 = 0x79
	PlayIDPluginMessage       int32 = 0x18
	PlayIDBlockUpdate         int32 = 0x08
	PlayIDBlockChangedAck     int32 = 0x04
	PlayIDSectionBlocksUpdate int32 = 0x54

	// Serverbound
	PlayIDConfirmTeleport int32 = 0x00
	PlayIDChatMessage     int32 = 0x09
	PlayIDKeepAliveSB     int32 = 0x1C
	PlayIDPlayerPosRot    int32 = 0x1F
	PlayIDPlayerLoaded    int32 = 0x2C
	PlayIDClientTickEnd   int32 = 0x0D
	PlayIDClientInfoSB    int32 = 0x0E
	PlayIDPlayerAction    int32 = 0x29
	PlayIDUseItemOn       int32 = 0x42
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
	PreviousGameMode    int8
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

// --- Block interaction packets (PVN 776) ---

// EncodeBlockUpdate builds the Block Update payload (Play 0x08, clientbound):
// a single absolute block Position and its new state id.
func EncodeBlockUpdate(pos Position, stateID int32) []byte {
	var b bytes.Buffer
	_ = WritePosition(&b, pos)
	_ = WriteVarInt(&b, stateID)
	return b.Bytes()
}

// EncodeBlockChangedAck builds the Block Changed Ack payload (Play 0x04,
// clientbound). Every serverbound Player Action / Use Item On carrying a
// sequence must be acked, or the client freezes further block edits.
func EncodeBlockChangedAck(sequence int32) []byte {
	var b bytes.Buffer
	_ = WriteVarInt(&b, sequence)
	return b.Bytes()
}

// SectionBlockChange is one entry in an Update Section Blocks payload.
type SectionBlockChange struct {
	LocalX, LocalY, LocalZ int // 0..15 within the section
	StateID                int32
}

// EncodeSectionBlocksUpdate builds the Update Section Blocks payload (Play 0x54,
// clientbound): a packed chunk-section position followed by a prefixed array of
// VarLong entries, each = (stateID << 12) | (localX<<8 | localZ<<4 | localY).
func EncodeSectionBlocksUpdate(chunkX, sectionY, chunkZ int32, changes []SectionBlockChange) []byte {
	var b bytes.Buffer
	_ = WriteInt64(&b, packSectionPos(chunkX, sectionY, chunkZ))
	_ = WriteVarInt(&b, int32(len(changes)))
	for _, c := range changes {
		entry := (int64(c.StateID) << 12) | int64((c.LocalX&0xF)<<8|(c.LocalZ&0xF)<<4|(c.LocalY&0xF))
		_ = WriteVarLong(&b, entry)
	}
	return b.Bytes()
}

// packSectionPos packs chunk X, section Y, chunk Z into the Long the wiki
// defines: ((X & 0x3FFFFF) << 42) | (Y & 0xFFFFF) | ((Z & 0x3FFFFF) << 20).
func packSectionPos(x, y, z int32) int64 {
	return (int64(x&0x3FFFFF) << 42) | int64(y&0xFFFFF) | (int64(z&0x3FFFFF) << 20)
}

// Player Action (serverbound) Status enum (PVN 776).
const (
	PlayerActionStartedDigging  int32 = 0
	PlayerActionCancelledDig    int32 = 1
	PlayerActionFinishedDigging int32 = 2
	PlayerActionDropItemStack   int32 = 3
	PlayerActionDropItem        int32 = 4
	PlayerActionReleaseItem     int32 = 5 // shoot arrow / stop using item
	PlayerActionSwapHands       int32 = 6
	PlayerActionStab            int32 = 7 // new in PVN 776
)

// PlayerAction carries the Player Action payload (Play 0x29, serverbound).
type PlayerAction struct {
	Action   int32 // Status enum
	Position Position
	Face     int8 // Byte enum: 0=-Y .. 5=+X
	Sequence int32
}

// DecodePlayerAction reads the Player Action payload.
func DecodePlayerAction(data []byte) (PlayerAction, error) {
	r := bytes.NewReader(data)
	var a PlayerAction
	var err error
	if a.Action, err = ReadVarInt(r); err != nil {
		return a, err
	}
	if a.Position, err = ReadPosition(r); err != nil {
		return a, err
	}
	if a.Face, err = ReadInt8(r); err != nil {
		return a, err
	}
	if a.Sequence, err = ReadVarInt(r); err != nil {
		return a, err
	}
	return a, nil
}

// UseItemOn carries the Use Item On payload (Play 0x42, serverbound).
type UseItemOn struct {
	Hand        int32 // 0=main, 1=off
	Position    Position
	Face        int32 // VarInt enum: 0=-Y .. 5=+X
	CursorX     float32
	CursorY     float32
	CursorZ     float32
	Inside      bool
	WorldBorder bool
	Sequence    int32
}

// DecodeUseItemOn reads the Use Item On payload.
func DecodeUseItemOn(data []byte) (UseItemOn, error) {
	r := bytes.NewReader(data)
	var u UseItemOn
	var err error
	if u.Hand, err = ReadVarInt(r); err != nil {
		return u, err
	}
	if u.Position, err = ReadPosition(r); err != nil {
		return u, err
	}
	if u.Face, err = ReadVarInt(r); err != nil {
		return u, err
	}
	if u.CursorX, err = ReadFloat32(r); err != nil {
		return u, err
	}
	if u.CursorY, err = ReadFloat32(r); err != nil {
		return u, err
	}
	if u.CursorZ, err = ReadFloat32(r); err != nil {
		return u, err
	}
	if u.Inside, err = ReadBool(r); err != nil {
		return u, err
	}
	if u.WorldBorder, err = ReadBool(r); err != nil {
		return u, err
	}
	if u.Sequence, err = ReadVarInt(r); err != nil {
		return u, err
	}
	return u, nil
}
