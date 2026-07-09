package protocol

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// AssemblePayload builds the uncompressed packet body = VarInt(packetID) || data.
// This is the unit that is optionally compressed before framing.
func AssemblePayload(packetID int32, data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := WriteVarInt(&buf, packetID); err != nil {
		return nil, err
	}
	if _, err := buf.Write(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Compress zlib-compresses data.
func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decompress zlib-decompresses data, rejecting results larger than maxLen.
func Decompress(data []byte, maxLen int) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, int64(maxLen)+1))
	if err != nil {
		return nil, err
	}
	if len(out) > maxLen {
		return nil, fmt.Errorf("protocol: decompressed packet too large (%d > %d)", len(out), maxLen)
	}
	return out, nil
}

// SplitPayload separates a decompressed (or uncompressed-after-0) payload into
// (packetID, data). payload = VarInt(packetID) || data.
func SplitPayload(payload []byte) (int32, []byte, error) {
	br := bytes.NewReader(payload)
	id, err := ReadVarInt(br)
	if err != nil {
		return 0, nil, err
	}
	rest := make([]byte, br.Len())
	if _, err := io.ReadFull(br, rest); err != nil {
		return 0, nil, err
	}
	return id, rest, nil
}
