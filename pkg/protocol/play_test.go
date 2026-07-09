package protocol

import (
	"bytes"
	"testing"
)

func TestBlockUpdateRoundTrip(t *testing.T) {
	pos := EncodePosition(10, 64, -5)
	payload := EncodeBlockUpdate(pos, 1) // stone

	r := bytes.NewReader(payload)
	gotPos, err := ReadPosition(r)
	if err != nil {
		t.Fatal(err)
	}
	gotState, err := ReadVarInt(r)
	if err != nil {
		t.Fatal(err)
	}
	if gotPos != pos {
		t.Errorf("pos = %v, want %v", gotPos, pos)
	}
	if gotState != 1 {
		t.Errorf("state = %d, want 1", gotState)
	}
}

func TestBlockChangedAck(t *testing.T) {
	payload := EncodeBlockChangedAck(42)
	seq, err := ReadVarInt(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if seq != 42 {
		t.Errorf("seq = %d, want 42", seq)
	}
}

func TestDecodePlayerActionRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteVarInt(&buf, PlayerActionFinishedDigging)
	_ = WritePosition(&buf, EncodePosition(3, 63, 7))
	_ = WriteInt8(&buf, 1) // face +Y
	_ = WriteVarInt(&buf, 5)

	got, err := DecodePlayerAction(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != PlayerActionFinishedDigging {
		t.Errorf("action = %d, want %d", got.Action, PlayerActionFinishedDigging)
	}
	x, y, z := got.Position.Decode()
	if x != 3 || y != 63 || z != 7 {
		t.Errorf("pos = %d,%d,%d, want 3,63,7", x, y, z)
	}
	if got.Face != 1 {
		t.Errorf("face = %d, want 1", got.Face)
	}
	if got.Sequence != 5 {
		t.Errorf("seq = %d, want 5", got.Sequence)
	}
}

func TestDecodeUseItemOnRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteVarInt(&buf, 0) // main hand
	_ = WritePosition(&buf, EncodePosition(0, 64, 0))
	_ = WriteVarInt(&buf, 2) // face -Z (VarInt, not byte!)
	_ = WriteFloat32(&buf, 0.5)
	_ = WriteFloat32(&buf, 0.5)
	_ = WriteFloat32(&buf, 0.5)
	_ = WriteBool(&buf, false)
	_ = WriteBool(&buf, false)
	_ = WriteVarInt(&buf, 9)

	got, err := DecodeUseItemOn(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got.Hand != 0 || got.Face != 2 || got.CursorX != 0.5 || got.WorldBorder || got.Sequence != 9 {
		t.Errorf("useItemOn = %+v", got)
	}
}

func TestSectionBlocksUpdateEncoding(t *testing.T) {
	changes := []SectionBlockChange{
		{LocalX: 1, LocalY: 2, LocalZ: 3, StateID: 9},
		{LocalX: 0, LocalY: 0, LocalZ: 0, StateID: 0},
	}
	payload := EncodeSectionBlocksUpdate(0, 4, 0, changes)

	r := bytes.NewReader(payload)
	secPos, err := ReadInt64(r)
	if err != nil {
		t.Fatal(err)
	}
	if secPos != packSectionPos(0, 4, 0) {
		t.Errorf("secPos = %#x, want %#x", secPos, packSectionPos(0, 4, 0))
	}
	count, err := ReadVarInt(r)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	// entry 0: (stateID<<12) | (localX<<8 | localZ<<4 | localY) = (9<<12)|(1<<8)|(3<<4)|2
	e0, err := ReadVarLong(r)
	if err != nil {
		t.Fatal(err)
	}
	want0 := int64(9)<<12 | int64(1<<8|3<<4|2)
	if e0 != want0 {
		t.Errorf("entry0 = %#x, want %#x", e0, want0)
	}
}
