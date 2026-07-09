package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"testing"

	"github.com/zakla/mc-server/pkg/protocol"
)

// TestEmptySectionBytes verifies that a nil section encodes to the all-air /
// all-plains layout: blockCount(0) + fluidCount(0) + single-valued states
// (air) + single-valued biomes (plains) = 8 zero bytes.
func TestEmptySectionBytes(t *testing.T) {
	var w bytes.Buffer
	encodeSection(&w, nil)
	want := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("empty section = % x, want % x", w.Bytes(), want)
	}
}

// TestSetGetBlock exercises in-memory SetBlock/GetBlock round-trips across
// sections and Y boundaries.
func TestSetGetBlock(t *testing.T) {
	c := NewChunk(0, 0)
	c.SetBlock(0, 63, 0, 9) // grass at top
	c.SetBlock(0, 62, 0, 1) // stone below
	c.SetBlock(15, -64, 15, 1)
	c.SetBlock(5, 319, 5, 1)

	if got := c.GetBlock(0, 63, 0); got != 9 {
		t.Errorf("GetBlock(0,63,0) = %d, want 9", got)
	}
	if got := c.GetBlock(0, 62, 0); got != 1 {
		t.Errorf("GetBlock(0,62,0) = %d, want 1", got)
	}
	if got := c.GetBlock(15, -64, 15); got != 1 {
		t.Errorf("GetBlock(15,-64,15) = %d, want 1", got)
	}
	if got := c.GetBlock(5, 319, 5); got != 1 {
		t.Errorf("GetBlock(5,319,5) = %d, want 1", got)
	}
	// Unwritten neighbour stays air.
	if got := c.GetBlock(1, 63, 0); got != 0 {
		t.Errorf("GetBlock(1,63,0) = %d, want 0", got)
	}
	// non-air count reflects the two blocks (grass + stone) in the Y=62..63 section.
	sec, _ := sectionIndex(63)
	if got := nonAirCount(&c.sections[sec].states); got != 2 {
		t.Errorf("section %d non-air = %d, want 2", sec, got)
	}
}

// decodePalettedContainer is the inverse of encodePalettedContainer, used here
// to verify the bit-packing without a real client. maxIndirect tells the
// decoder where indirect ends and direct begins.
func decodePalettedContainer(data []byte, n, maxIndirect int) (entries []int32, rest []byte, err error) {
	r := bytes.NewReader(data)
	bpeByte, err := r.ReadByte()
	if err != nil {
		return nil, nil, err
	}
	bpe := int(bpeByte)

	if bpe == 0 {
		val, err := protocol.ReadVarInt(r)
		if err != nil {
			return nil, nil, err
		}
		entries = make([]int32, n)
		for i := range entries {
			entries[i] = val
		}
		return entries, data[len(data)-int(r.Len()):], nil
	}

	var palette []int32
	if bpe <= maxIndirect {
		palLen, err := protocol.ReadVarInt(r)
		if err != nil {
			return nil, nil, err
		}
		palette = make([]int32, palLen)
		for i := int32(0); i < palLen; i++ {
			palette[i], err = protocol.ReadVarInt(r)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	longCount := (n*bpe + 63) / 64
	longs := make([]uint64, longCount)
	for i := range longs {
		var raw [8]byte
		if _, err := io.ReadFull(r, raw[:]); err != nil {
			return nil, nil, err
		}
		longs[i] = binary.BigEndian.Uint64(raw[:])
	}

	mask := (uint64(1) << uint(bpe)) - 1
	entries = make([]int32, n)
	for i := 0; i < n; i++ {
		bitIndex := i * bpe
		longIdx := bitIndex / 64
		bitOff := bitIndex % 64
		v := (longs[longIdx] >> uint(bitOff)) & mask
		if bitOff+bpe > 64 && longIdx+1 < len(longs) {
			v |= (longs[longIdx+1] << uint(64-bitOff)) & mask
		}
		if palette != nil {
			if int(v) >= len(palette) {
				return nil, nil, fmt.Errorf("palette index %d out of range %d", v, len(palette))
			}
			entries[i] = palette[int(v)]
		} else {
			entries[i] = int32(v)
		}
	}
	return entries, data[len(data)-int(r.Len()):], nil
}

// TestSectionEncodeRoundTrip encodes a section with grass+stone, decodes the
// block-states container back, and verifies the values match.
func TestSectionEncodeRoundTrip(t *testing.T) {
	var s ChunkSection
	s.states[0] = 9 // grass
	s.states[1] = 1 // stone
	// states[2..4095] = air (0)

	var w bytes.Buffer
	encodeSection(&w, &s)
	sec := w.Bytes()

	// blockCount(Int16) at [0:2], fluidCount at [2:4].
	if got := binary.BigEndian.Uint16(sec[0:2]); got != 2 {
		t.Errorf("blockCount = %d, want 2", got)
	}
	states, rest, err := decodePalettedContainer(sec[4:], 4096, 8)
	if err != nil {
		t.Fatalf("decode states: %v", err)
	}
	if states[0] != 9 || states[1] != 1 || states[2] != 0 {
		t.Errorf("states = %v %v %v ..., want 9 1 0", states[0], states[1], states[2])
	}
	// Biomes container follows and should decode to all plains (0).
	biomes, _, err := decodePalettedContainer(rest, 64, 3)
	if err != nil {
		t.Fatalf("decode biomes: %v", err)
	}
	if biomes[0] != 0 {
		t.Errorf("biome[0] = %d, want 0", biomes[0])
	}
}

// TestPalettedContainerIndirect packs many distinct entries (forcing indirect
// mode, bpe grows past the 4-bit floor) and verifies a full round-trip.
func TestPalettedContainerIndirect(t *testing.T) {
	var entries [4096]int32
	// 16 distinct values forces bpe = ceilLog2(16) = 4 (exactly the floor).
	for i := range entries {
		entries[i] = int32(i % 16)
	}
	var w bytes.Buffer
	encodePalettedContainer(&w, entries[:], 4, 8, globalBlockBits())
	got, _, err := decodePalettedContainer(w.Bytes(), 4096, 8)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := range entries {
		if got[i] != entries[i] {
			t.Fatalf("entry %d = %d, want %d", i, got[i], entries[i])
		}
	}
}

// TestPalettedContainerDirect forces direct mode (more than 256 distinct
// values → bpe exceeds the 8-bit indirect ceiling) and round-trips it.
func TestPalettedContainerDirect(t *testing.T) {
	var entries [4096]int32
	for i := range entries {
		entries[i] = int32(i % 300) // 300 distinct values → direct (bpe = globalBits = 15)
	}
	var w bytes.Buffer
	encodePalettedContainer(&w, entries[:], 4, 8, globalBlockBits())
	if w.Bytes()[0] != byte(globalBlockBits()) {
		t.Fatalf("bpe = %d, want direct %d", w.Bytes()[0], globalBlockBits())
	}
	got, _, err := decodePalettedContainer(w.Bytes(), 4096, 8)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := range entries {
		if got[i] != entries[i] {
			t.Fatalf("entry %d = %d, want %d", i, got[i], entries[i])
		}
	}
}

// TestChunkEncodeStructure sanity-checks the assembled chunk payload size.
func TestChunkEncodeStructure(t *testing.T) {
	data, err := NewChunk(0, 0).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 24*8 {
		t.Errorf("chunk payload smaller than 24 sections: %d", len(data))
	}
}
