// Package main: The CMD entry of zakla.
package main

import (
	"bytes"
	"fmt"
	"net"
	"zakla/internal/network"
	"zakla/internal/protocol"
)

func main() {
	l, err := net.Listen("tcp", ":25565")
	if err != nil {
		return
	}
	fmt.Println("Listening on :25565")

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Failed to listen: ", err)
			return
		}

		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	for {
		packet, err := network.ReceivePacket(conn)
		if err != nil {
			break
		}

		switch packet.ID {
		case 0x00:
			hs := &protocol.HandshakePacketData{}
			err := hs.Decode(packet)
			if err != nil {
				fmt.Println("Failed to handshake: ", err)
				break
			}

			fmt.Printf("Received Handshake(Protocol %v, Next State: %x)\n", hs.ProtocolVersion, hs.NextState)
			switch hs.NextState {
			case 0x01:
				handleStatus(conn)

			case 0x02:
				handleLogin(conn)

			default:
				fmt.Println("Unimplemented or unknown state: ", hs.NextState)
			}

		default:
			fmt.Printf("Unknown packet(len: %v, ID: %v).\n", packet.Length, packet.ID)
		}
	}
}

func handleStatus(conn net.Conn) error {
	// Flush buffer.
	network.ReceivePacket(conn)

	protocol.SendStatusResponsePacket(conn)

	pongPacket, err := network.ReceivePacket(conn)
	if err != nil {
		return err
	}

	return network.SendPacket(conn, pongPacket)
}

func handleLogin(conn net.Conn) {
	lsPacket, err := network.ReceivePacket(conn)
	if err != nil {
		return
	}

	ls := protocol.LoginStartPacketData{}
	ls.Decode(lsPacket)

	fmt.Printf("Player %s(UUID: %X) is logining in...\n", ls.Name, ls.UUID)

	cl := func(body *bytes.Buffer) {
		body.Write(ls.UUID[:])
		network.WriteString(body, ls.Name)
		network.WriteVarInt(body, 0)
	}
	network.CreateAndSendPacket(conn, 0x02, cl)
}
