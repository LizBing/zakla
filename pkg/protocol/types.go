// Package protocol implements the Minecraft Java Edition 26.2 wire protocol (PVN 776).
//
// This package provides low-level primitives: data type (de)serialization,
// packet framing helpers, optional zlib compression, and a minimal NBT writer.
// Higher-level packet definitions live in handshake.go, status.go, login.go,
// configuration.go, and play.go.
package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// ProtocolVersion is the wire protocol version for Minecraft Java Edition 26.2.
const ProtocolVersion = 776

// MaxVarIntLen is the maximum number of bytes a VarInt may occupy.
const MaxVarIntLen = 5

// MaxVarLongLen is the maximum number of bytes a VarLong may occupy.
const MaxVarLongLen = 10

// MaxPacketLength is the largest packet body (2^21 - 1) expressible in a 3-byte VarInt.
const MaxPacketLength = 2097151

// ErrVarIntTooBig is returned when a VarInt exceeds 5 bytes.
var ErrVarIntTooBig = errors.New("protocol: VarInt too big")

// ErrVarLongTooBig is returned when a VarLong exceeds 10 bytes.
var ErrVarLongTooBig = errors.New("protocol: VarLong too big")

// ReadVarInt reads a Minecraft VarInt as a signed 32-bit integer.
func ReadVarInt(r io.Reader) (int32, error) {
	var result int32
	for shift := uint(0); shift < 35; shift += 7 {
		b, err := readOne(r)
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
	}
	return 0, ErrVarIntTooBig
}

// WriteVarInt writes a signed 32-bit integer as a Minecraft VarInt.
func WriteVarInt(w io.Writer, v int32) error {
	uv := uint32(v)
	var buf [MaxVarIntLen]byte
	n := 0
	for {
		b := byte(uv & 0x7F)
		uv >>= 7
		if uv != 0 {
			b |= 0x80
		}
		buf[n] = b
		n++
		if uv == 0 {
			break
		}
	}
	_, err := w.Write(buf[:n])
	return err
}

// VarIntSize returns the number of bytes needed to encode v as a VarInt.
func VarIntSize(v int32) int {
	uv := uint32(v)
	n := 1
	for uv >= 0x80 {
		n++
		uv >>= 7
	}
	return n
}

// ReadVarLong reads a Minecraft VarLong as a signed 64-bit integer.
func ReadVarLong(r io.Reader) (int64, error) {
	var result int64
	for shift := uint(0); shift < 70; shift += 7 {
		b, err := readOne(r)
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
	}
	return 0, ErrVarLongTooBig
}

// WriteVarLong writes a signed 64-bit integer as a Minecraft VarLong.
func WriteVarLong(w io.Writer, v int64) error {
	uv := uint64(v)
	var buf [MaxVarLongLen]byte
	n := 0
	for {
		b := byte(uv & 0x7F)
		uv >>= 7
		if uv != 0 {
			b |= 0x80
		}
		buf[n] = b
		n++
		if uv == 0 {
			break
		}
	}
	_, err := w.Write(buf[:n])
	return err
}

// --- Booleans ---

// ReadBool reads a single boolean byte (0 or 1).
func ReadBool(r io.Reader) (bool, error) {
	b, err := readOne(r)
	return b != 0, err
}

// WriteBool writes a boolean byte.
func WriteBool(w io.Writer, v bool) error {
	if v {
		return writeOne(w, 1)
	}
	return writeOne(w, 0)
}

// --- Bytes / shorts / ints / longs (big-endian) ---

// ReadUint8 reads an unsigned byte.
func ReadUint8(r io.Reader) (uint8, error) {
	b, err := readOne(r)
	return b, err
}

// WriteUint8 writes an unsigned byte.
func WriteUint8(w io.Writer, v uint8) error {
	return writeOne(w, v)
}

// ReadInt8 reads a signed byte.
func ReadInt8(r io.Reader) (int8, error) {
	b, err := readOne(r)
	return int8(b), err
}

// WriteInt8 writes a signed byte.
func WriteInt8(w io.Writer, v int8) error {
	return writeOne(w, byte(v))
}

// ReadUint16 reads a big-endian unsigned short.
func ReadUint16(r io.Reader) (uint16, error) {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf[:]), nil
}

// WriteUint16 writes a big-endian unsigned short.
func WriteUint16(w io.Writer, v uint16) error {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

// ReadInt16 reads a big-endian signed short.
func ReadInt16(r io.Reader) (int16, error) {
	v, err := ReadUint16(r)
	return int16(v), err
}

// WriteInt16 writes a big-endian signed short.
func WriteInt16(w io.Writer, v int16) error {
	return WriteUint16(w, uint16(v))
}

// ReadInt32 reads a big-endian signed int.
func ReadInt32(r io.Reader) (int32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(buf[:])), nil
}

// WriteInt32 writes a big-endian signed int.
func WriteInt32(w io.Writer, v int32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(v))
	_, err := w.Write(buf[:])
	return err
}

// ReadInt64 reads a big-endian signed long.
func ReadInt64(r io.Reader) (int64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(buf[:])), nil
}

// WriteInt64 writes a big-endian signed long.
func WriteInt64(w io.Writer, v int64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(v))
	_, err := w.Write(buf[:])
	return err
}

// --- Floats ---

// ReadFloat32 reads a single-precision float.
func ReadFloat32(r io.Reader) (float32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.BigEndian.Uint32(buf[:])), nil
}

// WriteFloat32 writes a single-precision float.
func WriteFloat32(w io.Writer, v float32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], math.Float32bits(v))
	_, err := w.Write(buf[:])
	return err
}

// ReadFloat64 reads a double-precision float.
func ReadFloat64(r io.Reader) (float64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return math.Float64frombits(binary.BigEndian.Uint64(buf[:])), nil
}

// WriteFloat64 writes a double-precision float.
func WriteFloat64(w io.Writer, v float64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], math.Float64bits(v))
	_, err := w.Write(buf[:])
	return err
}

// --- Strings / Identifiers ---

// ReadString reads a VarInt-prefixed UTF-8 string.
func ReadString(r io.Reader) (string, error) {
	length, err := ReadVarInt(r)
	if err != nil {
		return "", err
	}
	if length < 0 {
		return "", fmt.Errorf("protocol: negative string length %d", length)
	}
	if length > 1<<20 {
		return "", fmt.Errorf("protocol: string length %d too large", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// WriteString writes a VarInt-prefixed UTF-8 string.
func WriteString(w io.Writer, v string) error {
	buf := []byte(v)
	if err := WriteVarInt(w, int32(len(buf))); err != nil {
		return err
	}
	_, err := w.Write(buf)
	return err
}

// ReadIdentifier reads a namespaced identifier string.
func ReadIdentifier(r io.Reader) (string, error) { return ReadString(r) }

// WriteIdentifier writes a namespaced identifier string.
func WriteIdentifier(w io.Writer, v string) error { return WriteString(w, v) }

// --- UUID (16 raw bytes, big-endian unsigned 128-bit) ---

// UUID is a 128-bit identifier, sent as 16 raw bytes.
type UUID [16]byte

// WriteUUID writes 16 raw bytes.
func WriteUUID(w io.Writer, id UUID) error {
	_, err := w.Write(id[:])
	return err
}

// ReadUUID reads 16 raw bytes.
func ReadUUID(r io.Reader) (UUID, error) {
	var id UUID
	_, err := io.ReadFull(r, id[:])
	return id, err
}

// --- Angle (1 byte, steps of 1/256 turn) ---

// Angle is a byte-sized rotation where 256 units = 360°.
type Angle uint8

// AngleFromDegrees converts degrees to an Angle.
func AngleFromDegrees(deg float32) Angle {
	normalized := deg / 360.0 * 256.0
	return Angle(uint8(int(math.Round(float64(normalized))) & 0xFF))
}

// WriteAngle writes a single angle byte.
func WriteAngle(w io.Writer, a Angle) error { return WriteUint8(w, uint8(a)) }

// ReadAngle reads a single angle byte.
func ReadAngle(r io.Reader) (Angle, error) {
	b, err := ReadUint8(r)
	return Angle(b), err
}

// --- Position (64-bit packed block coordinates) ---
//
// Encoded as ((x & 0x3FFFFFF) << 38) | ((z & 0x3FFFFFF) << 12) | (y & 0xFFF).

// Position is a packed 26/26/12-bit block position.
type Position uint64

// EncodePosition packs x, y, z into a Position.
func EncodePosition(x, y, z int) Position {
	return Position((uint64(x&0x3FFFFFF) << 38) | (uint64(z&0x3FFFFFF) << 12) | uint64(y&0xFFF))
}

// Decode returns the (x, y, z) block coordinates.
func (p Position) Decode() (x, y, z int) {
	v := int64(p)
	x = int(v >> 38)
	z = int(v << 26 >> 38)
	y = int(v << 52 >> 52)
	if x >= 1<<25 {
		x -= 1 << 26
	}
	if z >= 1<<25 {
		z -= 1 << 26
	}
	if y >= 1<<11 {
		y -= 1 << 12
	}
	return
}

// WritePosition writes a packed position as a long.
func WritePosition(w io.Writer, p Position) error {
	return WriteInt64(w, int64(p))
}

// ReadPosition reads a packed position from a long.
func ReadPosition(r io.Reader) (Position, error) {
	v, err := ReadInt64(r)
	return Position(v), err
}

// --- Prefixed byte arrays ---

// ReadPrefixedBytes reads a VarInt-prefixed byte slice.
func ReadPrefixedBytes(r io.Reader) ([]byte, error) {
	length, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("protocol: negative byte array length %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// WritePrefixedBytes writes a VarInt-prefixed byte slice.
func WritePrefixedBytes(w io.Writer, b []byte) error {
	if err := WriteVarInt(w, int32(len(b))); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

// ReadBytes reads exactly n bytes.
func ReadBytes(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

// readOne reads exactly one byte.
func readOne(r io.Reader) (byte, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

// writeOne writes exactly one byte.
func writeOne(w io.Writer, b byte) error {
	_, err := w.Write([]byte{b})
	return err
}
