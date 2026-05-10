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
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(s))
	return err
}

func CreateAndSendPacket(w io.Writer, id int, fn func(*bytes.Buffer)) error {
	var data bytes.Buffer
	fn(&data)

	var body bytes.Buffer
	WriteVarInt(&body, id)
	body.Write(data.Bytes())

	var packet bytes.Buffer
	WriteVarInt(&packet, body.Len())
	packet.Write(body.Bytes())

	_, err := w.Write(packet.Bytes())
	return err
}

type Packet struct {
	Length int
	ID     int
	Data   []byte
}

func SendPacket(w io.Writer, packet *Packet) error {
	var buf bytes.Buffer
	WriteVarInt(w, packet.Length)
	WriteVarInt(w, packet.ID)
	buf.Write(packet.Data)

	_, err := w.Write(buf.Bytes())
	if err != nil { return err }

	return nil
}

func ReceivePacket(r io.Reader) (*Packet, error) {
	length, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	br := bytes.NewReader(buf)

	id, err := ReadVarInt(br)
	if err != nil {
		return nil, err
	}
	data := make([]byte, br.Len())
	_, err = io.ReadFull(br, data)
	if err != nil {
		return nil, err
	}

	return &Packet{
		Length: length,
		ID:     id,
		Data:   data,
	}, nil
}
