package server

import (
	"bytes"
	"testing"
)

// TestEmptySectionBytes verifies the per-section encoding against the layout
// derived from minecraft.wiki (26.2): block count + fluid count + single-valued
// block states (air) + single-valued biomes (plains=0).
func TestEmptySectionBytes(t *testing.T) {
	var w bytes.Buffer
	writeEmptySection(&w)
	// block count(Short=0) + fluid count(Short=0) + states[0x00,0x00] + biomes[0x00,0x00]
	want := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("empty section = % x, want % x", w.Bytes(), want)
	}
}

// TestBuildEmptyChunkStructure sanity-checks the assembled chunk payload:
// 24 sections (24*8=192 bytes) plus light data (~53KB).
func TestBuildEmptyChunkStructure(t *testing.T) {
	data, err := BuildEmptyChunk(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 200 {
		t.Errorf("chunk payload suspiciously small: %d bytes", len(data))
	}
	// The section data block is 192 bytes; the full payload is dominated by light.
	if len(data) < 24*8 {
		t.Errorf("chunk payload smaller than 24 sections: %d", len(data))
	}
}
