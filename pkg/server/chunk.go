package server

import (
	"bytes"

	"github.com/zakla/mc-server/pkg/protocol"
)

// Chunk layout constants for an overworld-style world (Y -64..319).
const (
	overworldSectionCount = 24            // 384 blocks / 16
	lightSectionCount     = overworldSectionCount + 2 // +1 below, +1 above
	lightArrayLen         = 2048          // 4096 nibbles per light section
)

// BuildEmptyChunk constructs the Chunk Data and Update Light payload (Play 0x2D)
// for an all-air, all-plains chunk, per PVN 776 (26.2):
//
//   - 24 sections, each = block count (Short) + fluid count (Short, since 1.21.5)
//     + single-valued block-states container (air=0) + single-valued biomes
//     container (plains=0). A single-valued container is BPE byte 0x00 followed
//     by a bare VarInt value (no count prefix, no data array).
//   - heightmaps: empty array (legal & minimal).
//   - light: sky light fully lit (0xFF), block light empty.
//
// NOTES (may need client-driven iteration):
//   - sky light arrays use an inner VarInt(2048) length prefix, per the wiki's
//     "Prefixed Array (2048) of Byte" definition.
//   - heightmaps are sent as an empty "Prefixed Array of Heightmap" (1.21.5+
//     format); older builds used an NBT compound here.
func BuildEmptyChunk(chunkX, chunkZ int32) ([]byte, error) {
	var buf bytes.Buffer

	// Chunk X, Z (Int).
	_ = protocol.WriteInt32(&buf, chunkX)
	_ = protocol.WriteInt32(&buf, chunkZ)

	// Heightmaps: empty array.
	_ = protocol.WriteVarInt(&buf, 0)

	// Data: 24 sections concatenated, prefixed by total byte length.
	var sections bytes.Buffer
	for i := 0; i < overworldSectionCount; i++ {
		writeEmptySection(&sections)
	}
	_ = protocol.WriteVarInt(&buf, int32(sections.Len()))
	_, _ = buf.Write(sections.Bytes())

	// Block entities: empty array.
	_ = protocol.WriteVarInt(&buf, 0)

	// Light data.
	writeLightData(&buf)

	return buf.Bytes(), nil
}

// writeEmptySection writes one all-air/all-plains section.
func writeEmptySection(w *bytes.Buffer) {
	_ = protocol.WriteInt16(w, 0)  // block count
	_ = protocol.WriteInt16(w, 0)  // fluid count (since 1.21.5)
	_ = protocol.WriteUint8(w, 0)  // block-states BPE = 0 (single value)
	_ = protocol.WriteVarInt(w, 0) // block-states value = air (state id 0)
	_ = protocol.WriteUint8(w, 0)  // biomes BPE = 0 (single value)
	_ = protocol.WriteVarInt(w, 0) // biomes value = plains (registry id 0)
}

// writeLightData writes the Light Data structure for an empty world.
// Sky light is fully lit (0xFF) for all 26 light sections; block light is empty.
func writeLightData(w *bytes.Buffer) {
	// 26 bits set => 0x03FFFFFF, fits in one long.
	const allBits int64 = (1 << lightSectionCount) - 1

	// Sky Light Mask: all bits set (1 long).
	_ = protocol.WriteVarInt(w, 1)
	_ = protocol.WriteInt64(w, allBits)
	// Block Light Mask: empty (0 longs).
	_ = protocol.WriteVarInt(w, 0)
	// Empty Sky Light Mask: empty.
	_ = protocol.WriteVarInt(w, 0)
	// Empty Block Light Mask: all bits set (no block light).
	_ = protocol.WriteVarInt(w, 1)
	_ = protocol.WriteInt64(w, allBits)

	// Sky Light arrays: 26 fully-lit sections.
	full := bytes.Repeat([]byte{0xFF}, lightArrayLen)
	_ = protocol.WriteVarInt(w, lightSectionCount) // outer count
	for i := 0; i < lightSectionCount; i++ {
		_ = protocol.WriteVarInt(w, lightArrayLen) // inner prefixed length
		_, _ = w.Write(full)
	}
	// Block Light arrays: empty.
	_ = protocol.WriteVarInt(w, 0)
}
