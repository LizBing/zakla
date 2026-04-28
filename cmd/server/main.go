// Package main: The CMD entry of zakla.
package main

import (
	"bufio"
	"fmt"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", ":25565")
	if err != nil {
		return
	}
	fmt.Println("Listening on :25565")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to listen: ", err)
			return
		}

		go hexdump(conn)
	}
}

func hexdump(conn net.Conn) {
	defer conn.Close()

	for {
		reader := bufio.NewReader(conn)

		var buf [128]byte

		_, err := reader.Read(buf[:])
		if err != nil {
			fmt.Println("Failed to read: ", err)
			return
		}

		for _, v := range buf {
			fmt.Printf("%x ", v)
		}

		fmt.Println()
	}
}

