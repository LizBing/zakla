package server

import (
	"bytes"
	"encoding/binary"

	"github.com/zakla/mc-server/pkg/protocol"
)

// Chunk layout constants for an overworld-style world (Y -64..319).
const (
	overworldSectionCount = 24                        // 384 blocks / 16
	lightSectionCount     = overworldSectionCount + 2 // +1 below, +1 above
	lightArrayLen         = 2048                      // 4096 nibbles per light section
	minWorldY             = -64                       // lowest block Y
	chunkBlockExtent      = 16                        // blocks per chunk side
)

// Palette thresholds for the block-states container (PVN 776):
// indirect mode is forced to at least 4 bits and may grow to 8; above that the
// container switches to the direct (global palette) encoding.
const (
	blockMinIndirectBits = 4
	blockMaxIndirectBits = 8
	// biomeMinIndirectBits / biomeMaxIndirectBits govern the biomes container.
	biomeMinIndirectBits = 0
	biomeMaxIndirectBits = 3
)

// ChunkSection holds one 16³ section: 4096 block states and 64 biomes.
// The states array is indexed as (y<<8)|(z<<4)|x (YZX order, like the Anvil
// format), matching how the client reads a paletted container. The non-air
// block count is recomputed at encode time rather than maintained, so there is
// a single source of truth.
type ChunkSection struct {
	fluidCount int16       // number of fluid blocks (since 1.21.5)
	states     [4096]int32 // block state ids (0 = air)
	biomes     [64]int32   // biome registry ids (0 = plains)
}

// Chunk is one column of overworldSectionCount sections at chunk (X, Z).
type Chunk struct {
	X, Z     int32
	sections [overworldSectionCount]*ChunkSection
}

// NewChunk returns an all-air chunk.
func NewChunk(x, z int32) *Chunk {
	return &Chunk{X: x, Z: z}
}

// sectionIndex maps a world Y to its section index (0..23) and section-local Y.
func sectionIndex(y int) (sec, localY int) {
	rel := y - minWorldY
	return rel / chunkBlockExtent, rel % chunkBlockExtent
}

// GetBlock returns the block state id at world coordinates (x, y, z), or 0
// (air) if the position is outside this chunk or the section is empty.
func (c *Chunk) GetBlock(x, y, z int) int32 {
	if y < minWorldY || y >= minWorldY+overworldSectionCount*chunkBlockExtent {
		return 0
	}
	sec, ly := sectionIndex(y)
	s := c.sections[sec]
	if s == nil {
		return 0
	}
	return s.states[(ly<<8)|((z&15)<<4)|(x&15)]
}

// SetBlock sets the block state id at world coordinates (x, y, z) and maintains
// the non-air block count. Coordinates outside the vertical range are ignored.
func (c *Chunk) SetBlock(x, y, z int, stateID int32) {
	if y < minWorldY || y >= minWorldY+overworldSectionCount*chunkBlockExtent {
		return
	}
	sec, ly := sectionIndex(y)
	s := c.sections[sec]
	if s == nil {
		s = &ChunkSection{}
		c.sections[sec] = s
	}
	idx := (ly << 8) | ((z & 15) << 4) | (x & 15)
	s.states[idx] = stateID
}

// Encode serializes the chunk into a Chunk Data and Update Light payload
// (Play 0x2D) per PVN 776 (26.2):
//
//   - Chunk X, Z (Int)
//   - Heightmaps: empty prefixed array (legal & minimal).
//   - Data: 24 sections, each = block count (Short) + fluid count (Short,
//     since 1.21.5) + block-states paletted container + biomes paletted
//     container. The section blob is length-prefixed.
//   - Block entities: empty array.
//   - Light data (sky fully lit, block light empty).
func (c *Chunk) Encode() ([]byte, error) {
	var buf bytes.Buffer

	_ = protocol.WriteInt32(&buf, c.X)
	_ = protocol.WriteInt32(&buf, c.Z)

	// Heightmaps: empty prefixed array.
	_ = protocol.WriteVarInt(&buf, 0)

	// Sections.
	var sections bytes.Buffer
	for i := 0; i < overworldSectionCount; i++ {
		encodeSection(&sections, c.sections[i])
	}
	_ = protocol.WriteVarInt(&buf, int32(sections.Len()))
	_, _ = buf.Write(sections.Bytes())

	// Block entities: empty array.
	_ = protocol.WriteVarInt(&buf, 0)

	// Light data.
	writeLightData(&buf)

	return buf.Bytes(), nil
}

// encodeSection writes one section: counts + block-states container + biomes
// container. A nil section is encoded as all-air/all-plains. The block count is
// recomputed from the states so it always reflects reality.
func encodeSection(w *bytes.Buffer, s *ChunkSection) {
	var states [4096]int32
	var biomes [64]int32
	var fluidCount int16
	if s != nil {
		states = s.states
		biomes = s.biomes
		fluidCount = s.fluidCount
	}
	_ = protocol.WriteInt16(w, nonAirCount(&states))
	_ = protocol.WriteInt16(w, fluidCount)
	encodePalettedContainer(w, states[:], blockMinIndirectBits, blockMaxIndirectBits, globalBlockBits())
	encodePalettedContainer(w, biomes[:], biomeMinIndirectBits, biomeMaxIndirectBits, globalBiomeBits())
}

// nonAirCount counts the non-air blocks in a section's states (the block count
// the client uses for rendering optimization).
func nonAirCount(states *[4096]int32) int16 {
	var n int16
	for _, v := range states {
		if v != 0 {
			n++
		}
	}
	return n
}

// encodePalettedContainer writes a paletted container (26.2 / PVN 776):
//
//   - Single value (one unique entry): BPE byte 0x00 + a bare VarInt value
//     (no count, no data array). This is the path already exercised by the
//     all-air world.
//   - Indirect (few unique entries): BPE = max(minBits, ceilLog2(paletteLen)),
//     up to maxIndirectBits; then a VarInt-prefixed palette followed by the
//     packed data long array.
//   - Direct (too many unique entries): BPE = globalBits; no palette, the data
//     long array holds raw global ids.
//
// The data long array is flat-packed (entries may span long boundaries), has
// longCount = ceil(n*bpe/64), and is written WITHOUT a VarInt length prefix
// (1.21.5+ derives the count from BPE).
func encodePalettedContainer(w *bytes.Buffer, entries []int32, minBits, maxIndirect, globalBits int) {
	// Collect unique values into a palette.
	uniq := map[int32]int{}
	palette := make([]int32, 0, 8)
	for _, e := range entries {
		if _, ok := uniq[e]; !ok {
			uniq[e] = len(palette)
			palette = append(palette, e)
		}
	}

	// Single value.
	if len(palette) == 1 {
		_ = protocol.WriteUint8(w, 0)           // BPE = 0
		_ = protocol.WriteVarInt(w, palette[0]) // bare value
		return
	}

	bpe := max(ceilLog2(len(palette)), minBits)

	if bpe > maxIndirect {
		// Direct: entries hold global ids directly.
		_ = protocol.WriteUint8(w, byte(globalBits))
		writeBitPack(w, entries, globalBits)
		return
	}

	// Indirect: palette + packed palette indices.
	_ = protocol.WriteUint8(w, byte(bpe))
	_ = protocol.WriteVarInt(w, int32(len(palette)))
	for _, p := range palette {
		_ = protocol.WriteVarInt(w, p)
	}
	indices := make([]int32, len(entries))
	for i, e := range entries {
		indices[i] = int32(uniq[e])
	}
	writeBitPack(w, indices, bpe)
}

// writeBitPack packs entries (each bpe bits) into a big-endian long array
// WITHOUT spanning long boundaries (1.16+ format): each long holds floor(64/bpe)
// entries with the high bits as padding; longCount = ceil(N / floor(64/bpe)).
// No length prefix is written (PVN 776 / 1.21.5+). For bpe that divides 64
// (e.g. 4) this matches the flat layout; for others (5, 6, 8) it differs.
func writeBitPack(w *bytes.Buffer, entries []int32, bpe int) {
	entriesPerLong := 64 / bpe
	longCount := (len(entries) + entriesPerLong - 1) / entriesPerLong
	data := make([]uint64, longCount)
	mask := (uint64(1) << uint(bpe)) - 1
	for i, e := range entries {
		v := uint64(e) & mask
		data[i/entriesPerLong] |= v << uint((i%entriesPerLong)*bpe)
	}
	var raw [8]byte
	for _, l := range data {
		binary.BigEndian.PutUint64(raw[:], l)
		_, _ = w.Write(raw[:])
	}
}

// ceilLog2 returns ceil(log2(n)) for n >= 1 (ceilLog2(1) = 0).
func ceilLog2(n int) int {
	if n <= 1 {
		return 0
	}
	b := 0
	v := n - 1
	for v > 0 {
		v >>= 1
		b++
	}
	return b
}

// globalBlockBits is the BPE used by direct block-states containers.
func globalBlockBits() int {
	return ceilLog2(int(TotalBlockStates))
}

// globalBiomeBits is the BPE used by direct biome containers, derived from the
// number of entries in the synchronized biome registry we send.
func globalBiomeBits() int {
	count := 0
	for _, reg := range DefaultRegistries() {
		if reg.ID == "minecraft:worldgen/biome" {
			count = len(reg.Entries)
			break
		}
	}
	if count < 1 {
		count = 64
	}
	return ceilLog2(count)
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
