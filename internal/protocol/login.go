package protocol

import (
	"bytes"
	"io"
	"zakla/internal/network"
)

type LoginStartPacketData struct {
	Name string
	UUID [16]byte
}

func (p *LoginStartPacketData) Decode(packet *network.Packet) error {
	r := bytes.NewReader(packet.Data)

	name, err := network.ReadString(r)
	if err != nil {
		return err
	}
	p.Name = name

	_, err = io.ReadFull(r, p.UUID[:])
	return err
}
