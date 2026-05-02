// Package network: MC Server Protocal
package network

import (
	"bytes"
	"errors"
	"io"
)

const segmentBits int = 0x7f
const continueBit int = 0x80

func ReadVarInt(r io.Reader) (int, error) {
	var res int
	position := 0

	buf := make([]byte, 1)
	for {
		_, err := r.Read(buf)
		if err != nil {
			return 0, err
		}
		currentByte := buf[0]

		res |= (int(currentByte) & segmentBits) << position

		if int(currentByte)&continueBit == 0 {
			break
		}

		position += 7

		if position >= 32 {
			return 0, errors.New("VarInt is too big")
		}
	}

	return res, nil
}

func WriteVarInt(w io.Writer, value int) error {
	buf := make([]byte, 1)

	for {
		if value & ^segmentBits == 0 {
			buf[0] = byte(value)
			_, err := w.Write(buf)
			return err
		}

		buf[0] = byte((value & segmentBits) | continueBit)
		_, err := w.Write(buf)
		if err != nil {
			return err
		}

		value = int(uint(value) >> 7)
	}
}

func ReadString(r io.Reader) (string, error) {
	length, err := ReadVarInt(r)
	if err != nil {
		return "", err
	}

	buf := make([]byte, length)
	{
		_, err := io.ReadFull(r, buf)
		return string(buf), err
	}
}

func WriteString(w io.Writer, s string) error {
	err := WriteVarInt(w, int(len(s)))
	if err != nil { return err }

	_, err = w.Write([]byte(s))
	return err
}

func SendPacket(w io.Writer, id int, fn func(*bytes.Buffer)) error {
	var body bytes.Buffer
	WriteVarInt(&body, id)
	fn(&body)

	var packet bytes.Buffer
	err := WriteVarInt(&packet, body.Len())
	if err != nil { return err }
	_, err = packet.Write(body.Bytes())	
	if err != nil { return err }

	_, err = w.Write(packet.Bytes())

	return nil
}
