package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// NBT tag type IDs.
const (
	TagEnd       byte = 0
	TagByte      byte = 1
	TagShort     byte = 2
	TagInt       byte = 3
	TagLong      byte = 4
	TagFloat     byte = 5
	TagDouble    byte = 6
	TagByteArray byte = 7
	TagString    byte = 8
	TagList      byte = 9
	TagCompound  byte = 10
	TagIntArray  byte = 11
	TagLongArray byte = 12
)

// NBTWriter builds a binary NBT payload into an internal buffer.
// It uses a nested-builder style: root writes a named compound, children are
// added through CompoundBuilder. This is enough to encode the registry/tag
// data sent during configuration.
type NBTWriter struct {
	buf *bytes.Buffer
}

// NewNBTWriter creates a fresh writer.
func NewNBTWriter() *NBTWriter {
	return &NBTWriter{buf: &bytes.Buffer{}}
}

// Bytes returns the encoded NBT payload.
func (n *NBTWriter) Bytes() []byte { return n.buf.Bytes() }

// Reset clears the buffer for reuse.
func (n *NBTWriter) Reset() { n.buf.Reset() }

// Compound writes a complete compound tag as the NBT ROOT.
// fn, if non-nil, populates the compound's children.
//
// Network NBT (protocol 764+/1.20.2+): the ROOT tag has NO name length field,
// only the type byte followed directly by the payload. (Nested tags inside the
// payload still carry their names.)
func (n *NBTWriter) Compound(name string, fn func(*CompoundBuilder)) {
	_ = name
	writeOne(n.buf, TagCompound)
	if fn != nil {
		fn(&CompoundBuilder{buf: n.buf})
	}
	writeOne(n.buf, TagEnd)
}

// StringRoot writes a root TAG_String (used for plain text components).
// Network NBT: no root name length field — type byte directly followed by the
// string payload (length + bytes).
func (n *NBTWriter) StringRoot(name, value string) {
	_ = name
	writeOne(n.buf, TagString)
	writeNBTName(n.buf, value)
}

// PlainTextComponent returns NBT bytes for a plain-text component as a root
// TAG_String with an empty name (minimal valid Text Component encoding).
func PlainTextComponent(text string) []byte {
	nw := NewNBTWriter()
	nw.StringRoot("", text)
	return nw.Bytes()
}

// CompoundBuilder adds named children to an open compound.
type CompoundBuilder struct {
	buf *bytes.Buffer
}

func (c *CompoundBuilder) nameAndType(t byte, name string) {
	writeOne(c.buf, t)
	writeNBTName(c.buf, name)
}

// Byte writes a named byte tag.
func (c *CompoundBuilder) Byte(name string, v int8) {
	c.nameAndType(TagByte, name)
	writeOne(c.buf, byte(v))
}

// ByteUint writes a named byte tag from a uint8.
func (c *CompoundBuilder) ByteUint(name string, v uint8) {
	c.nameAndType(TagByte, name)
	writeOne(c.buf, v)
}

// Short writes a named short tag.
func (c *CompoundBuilder) Short(name string, v int16) {
	c.nameAndType(TagShort, name)
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(v))
	c.buf.Write(b[:])
}

// Int writes a named int tag.
func (c *CompoundBuilder) Int(name string, v int32) {
	c.nameAndType(TagInt, name)
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(v))
	c.buf.Write(b[:])
}

// Long writes a named long tag.
func (c *CompoundBuilder) Long(name string, v int64) {
	c.nameAndType(TagLong, name)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v))
	c.buf.Write(b[:])
}

// Float writes a named float tag.
func (c *CompoundBuilder) Float(name string, v float32) {
	c.nameAndType(TagFloat, name)
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], math.Float32bits(v))
	c.buf.Write(b[:])
}

// String writes a named string tag.
func (c *CompoundBuilder) String(name, v string) {
	c.nameAndType(TagString, name)
	writeNBTName(c.buf, v)
}

// Compound writes a nested named compound.
func (c *CompoundBuilder) Compound(name string, fn func(*CompoundBuilder)) {
	c.nameAndType(TagCompound, name)
	if fn != nil {
		fn(&CompoundBuilder{buf: c.buf})
	}
	writeOne(c.buf, TagEnd)
}

// StringList writes a named list of strings.
func (c *CompoundBuilder) StringList(name string, items []string) {
	c.nameAndType(TagList, name)
	writeOne(c.buf, TagString)
	writeNBTLength(c.buf, int32(len(items)))
	for _, it := range items {
		writeNBTName(c.buf, it)
	}
}

// CompoundList writes a named list of `count` compounds.
func (c *CompoundBuilder) CompoundList(name string, count int, fn func(index int, cb *CompoundBuilder)) {
	c.nameAndType(TagList, name)
	writeOne(c.buf, TagCompound)
	writeNBTLength(c.buf, int32(count))
	for i := 0; i < count; i++ {
		fn(i, &CompoundBuilder{buf: c.buf})
		writeOne(c.buf, TagEnd)
	}
}

// IntArray writes a named int array tag.
func (c *CompoundBuilder) IntArray(name string, items []int32) {
	c.nameAndType(TagIntArray, name)
	writeNBTLength(c.buf, int32(len(items)))
	var b [4]byte
	for _, v := range items {
		binary.BigEndian.PutUint32(b[:], uint32(v))
		c.buf.Write(b[:])
	}
}

// ByteArray writes a named byte array tag.
func (c *CompoundBuilder) ByteArray(name string, items []int8) {
	c.nameAndType(TagByteArray, name)
	writeNBTLength(c.buf, int32(len(items)))
	for _, v := range items {
		writeOne(c.buf, byte(v))
	}
}

// RawNBT returns the bytes of a complete named compound built by fn.
// Convenience for embedding registry entry data.
func RawNBT(name string, fn func(*CompoundBuilder)) ([]byte, error) {
	nw := NewNBTWriter()
	var buildErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				buildErr = fmt.Errorf("nbt build panic: %v", r)
			}
		}()
		nw.Compound(name, fn)
	}()
	return nw.Bytes(), buildErr
}

func writeNBTName(buf *bytes.Buffer, s string) {
	writeNBTLength(buf, int32(len(s)))
	buf.WriteString(s)
}

func writeNBTLength(buf *bytes.Buffer, v int32) {
	// NBT string/name lengths are unsigned short (2 bytes, big-endian), NOT int32.
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(v))
	buf.Write(b[:])
}
