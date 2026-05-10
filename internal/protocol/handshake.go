// Package protocol: MC Server Protocal
package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"zakla/internal/network"
)

type HandshakePacketData struct {
	ProtocolVersion int
	ServerAddr string
	ServerPort uint16
	NextState int
}

func(hs *HandshakePacketData) Decode(packet *network.Packet) error {
	r := bytes.NewReader(packet.Data)

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

func(hs *HandshakePacketData) String() string {
	return fmt.Sprintf("Handshake:\n\tVersion: %v\n\tServer: %s:%v\n\tNext State: %x", hs.ProtocolVersion, hs.ServerAddr, hs.ServerPort, hs.NextState)
}

