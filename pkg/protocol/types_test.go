package protocol

import (
	"bytes"
	"testing"
)

func TestVarIntRoundTrip(t *testing.T) {
	cases := []int32{0, 1, 127, 128, 255, 25565, 776, 2147483647, -1, -2147483648}
	for _, v := range cases {
		var buf bytes.Buffer
		if err := WriteVarInt(&buf, v); err != nil {
			t.Fatalf("WriteVarInt(%d): %v", v, err)
		}
		got, err := ReadVarInt(&buf)
		if err != nil {
			t.Fatalf("ReadVarInt: %v", err)
		}
		if got != v {
			t.Errorf("VarInt roundtrip %d -> %d", v, got)
		}
	}
}

// TestVarIntKnownBytes checks encoding against wiki.vg/minecraft.wiki sample values.
func TestVarIntKnownBytes(t *testing.T) {
	cases := []struct {
		v int32
		b []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{2, []byte{0x02}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{255, []byte{0xff, 0x01}},
		{25565, []byte{0xdd, 0xc7, 0x01}},
		{776, []byte{0x88, 0x06}},
		{2147483647, []byte{0xff, 0xff, 0xff, 0xff, 0x07}},
		{-1, []byte{0xff, 0xff, 0xff, 0xff, 0x0f}},
		{-2147483648, []byte{0x80, 0x80, 0x80, 0x80, 0x08}},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if err := WriteVarInt(&buf, c.v); err != nil {
			t.Fatalf("WriteVarInt(%d): %v", c.v, err)
		}
		if !bytes.Equal(buf.Bytes(), c.b) {
			t.Errorf("VarInt(%d) bytes = % x, want % x", c.v, buf.Bytes(), c.b)
		}
	}
}

func TestPositionRoundTrip(t *testing.T) {
	cases := []struct{ x, y, z int }{
		{0, 0, 0},
		{10, 64, -10},
		{18357644, 831, -20882616}, // wiki example
		{-1, -1, -1},
		{1000, -50, 2000},
	}
	for _, c := range cases {
		p := EncodePosition(c.x, c.y, c.z)
		x, y, z := p.Decode()
		if x != c.x || y != c.y || z != c.z {
			t.Errorf("Position(%d,%d,%d) -> (%d,%d,%d)", c.x, c.y, c.z, x, y, z)
		}
	}
}

func TestCompressionRoundTrip(t *testing.T) {
	data := bytes.Repeat([]byte("minecraft"), 1000)
	compressed, err := Compress(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) >= len(data) {
		t.Logf("warning: compression did not shrink (%d >= %d)", len(compressed), len(data))
	}
	back, err := Decompress(compressed, len(data))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, data) {
		t.Error("decompressed data mismatch")
	}
}

func TestDecompressRejectsOversize(t *testing.T) {
	// 10KB of data, but declare a tiny max -> must error.
	data := bytes.Repeat([]byte{0xAB}, 10000)
	compressed, err := Compress(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decompress(compressed, 100); err == nil {
		t.Error("expected oversize error, got nil")
	}
}

func TestStringRoundTrip(t *testing.T) {
	cases := []string{"", "hello", "minecraft:overworld", "日本語のテキスト"}
	for _, s := range cases {
		var buf bytes.Buffer
		if err := WriteString(&buf, s); err != nil {
			t.Fatal(err)
		}
		got, err := ReadString(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if got != s {
			t.Errorf("string roundtrip %q -> %q", s, got)
		}
	}
}
