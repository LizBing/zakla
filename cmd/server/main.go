// Package main: The CMD entry of zakla.
package main

import (
	"bytes"
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
			if hs.NextState == 0x01 {
				handleStatus(conn)
			}

		default:
			fmt.Printf("Unknown packet(len: %v, ID: %v).\n", length, packetID)
		}
	}
}

func handleStatus(conn net.Conn) error {
	resp := protocol.StatusResponse{}
	resp.Version.Name = "Zakla 26.1"
	resp.Version.Protocol = 775
	resp.Players.Max = 10
	resp.Players.Online = 0
	resp.Description.Text = "Have a nice day!"
	jsonStr := resp.Marshal()

	fmt.Println(jsonStr)

	// Flush buffer.
	network.ReadVarInt(conn)
	network.ReadVarInt(conn)

	var packetBody bytes.Buffer
	network.WriteVarInt(&packetBody, 0x00)
	network.WriteString(&packetBody, jsonStr)

	network.WriteVarInt(conn, packetBody.Len())
	conn.Write(packetBody.Bytes())

	pingLen, _ := network.ReadVarInt(conn)
	pingID, _ := network.ReadVarInt(conn)
	if pingID == 0x01 {
		payload := make([]byte, pingLen-1)
		io.ReadFull(conn, payload)

		var pongBody bytes.Buffer
		network.WriteVarInt(&pongBody, 0x01)
		pongBody.Write(payload)

		network.WriteVarInt(conn, pingLen)
		conn.Write(pongBody.Bytes())
	}

	return nil
}
