// Package protocal: MC Server Protocal
package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"zakla/internal/network"
)

type HandshakePacket struct {
	ProtocolVersion int
	ServerAddr string
	ServerPort uint16
	NextState int
}

func(hs *HandshakePacket) Decode(r io.Reader) error {
	version, err := network.ReadVarInt(r)
	if err != nil { return err }
	hs.ProtocolVersion = version

	ipAddr, err := network.ReadString(r)
	if err != nil { return err }
	hs.ServerAddr = ipAddr

	err = binary.Read(r, binary.BigEndian, &hs.ServerPort)
	if err != nil { return err }

	next, err := network.ReadVarInt(r)
	if err != nil { return err }
	hs.NextState = next

	return nil
}

func(hs *HandshakePacket) String() string {
	return fmt.Sprintf("Handshake:\n\tVersion: %v\n\tServer: %s:%v\n\tNext State: %x", hs.ProtocolVersion, hs.ServerAddr, hs.ServerPort, hs.NextState)
}

