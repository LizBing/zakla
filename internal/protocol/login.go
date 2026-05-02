package protocol

import (
	"io"
	"zakla/internal/network"
)

type LoginStartPacket struct {
	Name string
	UUID [16]byte
}

func (p *LoginStartPacket) Decode(r io.Reader) error {
	name, err := network.ReadString(r)
	if err != nil {
		return err
	}
	p.Name = name

	_, err = io.ReadFull(r, p.UUID[:])
	return err
}
