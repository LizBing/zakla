// Package main: The CMD entry of zakla.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
		length, err := network.ReadVarInt(conn)
		if err != nil {
			break
		}
		packetID, err := network.ReadVarInt(conn)
		if err != nil {
			break
		}

		switch packetID {
		case 0x00:
			hs := &protocol.HandshakePacket{}
			err := hs.Decode(conn)
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
			fmt.Printf("Unknown packet(len: %v, ID: %v).\n", length, packetID)
		}
	}
}

func handleStatus(conn net.Conn) error {
	// Flush buffer.
	network.ReadVarInt(conn)
	network.ReadVarInt(conn)

	cl := func(body *bytes.Buffer) {
		network.WriteString(body, protocol.StatusResponseStr())
	}
	network.SendPacket(conn, 0x00, cl)

	pingLen, _ := network.ReadVarInt(conn)
	pingID, _ := network.ReadVarInt(conn)
	if pingID != 0x01 {
		e := fmt.Sprintf("Unknown Ping ID: %x\n", pingID)
		return errors.New(e)
	}

	cl = func(body *bytes.Buffer) {
		payload := make([]byte, pingLen-1)
		io.ReadFull(conn, payload)
		body.Write(payload)
	}
	network.SendPacket(conn, 0x01, cl)

	return nil
}

func handleLogin(conn net.Conn) {
	network.ReadVarInt(conn)
	network.ReadVarInt(conn)

	ls := protocol.LoginStartPacket{}
	ls.Decode(conn)

	fmt.Printf("Player %s(UUID: %X) is logining in...\n", ls.Name, ls.UUID)

	cl := func(body *bytes.Buffer) {
		body.Write(ls.UUID[:])
		network.WriteString(body, ls.Name)
		network.WriteVarInt(body, 0)
	}
	network.SendPacket(conn, 0x02, cl)
}
