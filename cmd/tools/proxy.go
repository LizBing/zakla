package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"zakla/internal/network"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Panicked due to: %v\n", err)
		}
	}()

	l, err := net.Listen("tcp", ":8081")
	if err != nil {
		panic("Failed to listen :8081")
	}
	fmt.Println("Listen on :8081")

	for {
		conn, err := l.Accept()
		if err != nil {
			panic(err)
		}

		go handleConn(conn)
	}
}

func pipe(rd io.Reader, wr io.Writer, link string) {
	for {
		length, err := network.ReadVarInt(rd)
		if err != nil {
			fmt.Println(err)
			break
		}

		payload := make([]byte, length)
		io.ReadFull(rd, payload)

		var rawPacket bytes.Buffer
		network.WriteVarInt(&rawPacket, length)
		rawPacket.Write(payload)

		wr.Write(rawPacket.Bytes())

		fmt.Printf("[%s] Len: %d\n", link, length)
	}
}

func handleConn(client net.Conn) {
	defer client.Close()

	server, err := net.Dial("tcp", "localhost:25565")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer server.Close()

	done := make(chan struct{}, 2)

	go func() {
		pipe(client, server, "C->S")
		done <- struct{}{}
	}()
	go func() {
		pipe(server, client, "S->C")
		done <- struct{}{}
	}()

	<-done
}
