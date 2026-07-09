// Command mock-client connects to a Minecraft 26.2 server and walks the full
// handshake -> login -> configuration -> play flow, then sends a chat message.
// It reuses the server's own protocol/net packages so it exercises the same
// wire code end-to-end. Used for automated self-testing.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	mcnet "github.com/zakla/mc-server/pkg/net"
	"github.com/zakla/mc-server/pkg/protocol"
)

var logger = log.New(os.Stderr, "[MOCK-CLIENT] ", log.LstdFlags|log.Lmicroseconds)

func main() {
	host := flag.String("host", envOr("MC_HOST", "localhost"), "server host")
	port := flag.Int("port", 25565, "server port")
	name := flag.String("name", envOr("MC_NAME", "TestBot"), "username")
	flag.Parse()

	addr := net.JoinHostPort(*host, fmt.Sprintf("%d", *port))
	logger.Printf("connecting to %s (protocol %d)", addr, protocol.ProtocolVersion)

	raw, err := net.Dial("tcp", addr)
	if err != nil {
		fail("dial failed: %v", err)
	}
	defer raw.Close()

	c := mcnet.NewConnection(raw)

	// --- Handshake (intent=login) ---
	var hs bytes.Buffer
	_ = protocol.WriteVarInt(&hs, protocol.ProtocolVersion)
	_ = protocol.WriteString(&hs, *host)
	_ = protocol.WriteUint16(&hs, uint16(*port))
	_ = protocol.WriteVarInt(&hs, protocol.IntentLogin)
	if err := c.WritePacket(protocol.PacketIDHandshake, hs.Bytes()); err != nil {
		fail("handshake write: %v", err)
	}
	c.SetState(mcnet.StateLogin)

	// --- Login Start ---
	var ls bytes.Buffer
	_ = protocol.WriteString(&ls, *name)
	_ = protocol.WriteUUID(&ls, playerUUID())
	if err := c.WritePacket(protocol.PacketIDLoginStart, ls.Bytes()); err != nil {
		fail("login start write: %v", err)
	}

	// --- Wait for Set Compression + Login Success ---
	for {
		id, data, err := c.ReadPacket()
		if err != nil {
			fail("login read: %v", err)
		}
		switch id {
		case protocol.PacketIDSetCompression:
			th, _ := protocol.ReadVarInt(bytes.NewReader(data))
			c.SetCompression(int(th))
			logger.Printf("compression enabled (threshold=%d)", th)
		case protocol.PacketIDLoginSuccess:
			logger.Printf("LOGIN SUCCESS")
		case protocol.PacketIDLoginDisconnect:
			reason, _ := protocol.ReadString(bytes.NewReader(data))
			fail("disconnected during login: %s", reason)
		default:
			logger.Printf("login: unexpected packet 0x%x", id)
		}
		if id == protocol.PacketIDLoginSuccess {
			break
		}
	}

	// --- Login Acknowledged -> Configuration ---
	if err := c.WritePacket(protocol.PacketIDLoginAcknowledged, nil); err != nil {
		fail("login ack write: %v", err)
	}
	c.SetState(mcnet.StateConfiguration)

	finished := false
	registryCount := 0
	for !finished {
		id, data, err := c.ReadPacket()
		if err != nil {
			fail("configuration read: %v", err)
		}
		switch id {
		case protocol.CfgIDPluginMessage, protocol.CfgIDFeatureFlags:
			// brand / feature flags; ignore
		case protocol.CfgIDKnownPacksCB:
			reply := protocol.EncodeKnownPacks([]protocol.KnownPack{{Namespace: "minecraft", ID: "core", Version: "26.2"}})
			_ = c.WritePacket(protocol.CfgIDKnownPacksSB, reply)
		case protocol.CfgIDRegistryData:
			registryCount++
		case protocol.CfgIDKeepAlive:
			_ = c.WritePacket(protocol.CfgIDKeepAliveSB, data)
		case protocol.CfgIDPing:
			_ = c.WritePacket(protocol.CfgIDPong, data)
		case protocol.CfgIDFinishConfig:
			_ = c.WritePacket(protocol.CfgIDAckFinishConfig, nil)
			finished = true
		default:
			// ignore
		}
	}
	logger.Printf("CONFIGURATION COMPLETE (%d registry packets)", registryCount)
	c.SetState(mcnet.StatePlay)

	// --- Play: read login sequence in background, confirm teleport/keepalive ---
	done := make(chan struct{})
	go func() {
		playReader(c)
		close(done)
	}()

	time.Sleep(1500 * time.Millisecond)
	msg := "Hello from mock client"
	if err := sendChat(c, msg); err != nil {
		logger.Printf("chat send error: %v", err)
	} else {
		logger.Printf("SENT CHAT: %q", msg)
	}

	// Test block mining: send a Finished-digging action at a known solid block
	// and expect the server to ack and broadcast a Block Update (air).
	time.Sleep(500 * time.Millisecond)
	digPos := protocol.EncodePosition(1, 63, 1)
	if err := sendPlayerAction(c, protocol.PlayerActionFinishedDigging, digPos, 1, 1); err != nil {
		logger.Printf("player action send error: %v", err)
	} else {
		logger.Printf("SENT PLAYER ACTION (Finished dig at 1,63,1 seq=1)")
	}
	time.Sleep(2 * time.Second)
	logger.Printf("DONE")
}

func playReader(c *mcnet.Connection) {
	for {
		id, data, err := c.ReadPacket()
		if err != nil {
			return
		}
		switch id {
		case protocol.PlayIDLogin:
			logger.Printf("received Login (play)")
		case protocol.PlayIDSynchPlayerPos:
			tid, _ := protocol.ReadVarInt(bytes.NewReader(data))
			var conf bytes.Buffer
			_ = protocol.WriteVarInt(&conf, tid)
			_ = c.WritePacket(protocol.PlayIDConfirmTeleport, conf.Bytes())
		case protocol.PlayIDKeepAlive:
			_ = c.WritePacket(protocol.PlayIDKeepAliveSB, data)
		case protocol.PlayIDSystemChat:
			logger.Printf("SYSTEM CHAT received (%d bytes)", len(data))
		case protocol.PlayIDChunkDataLight:
			logger.Printf("chunk data received (%d bytes)", len(data))
		case protocol.PlayIDBlockChangedAck:
			seq, _ := protocol.ReadVarInt(bytes.NewReader(data))
			logger.Printf("BLOCK CHANGED ACK (seq=%d)", seq)
		case protocol.PlayIDBlockUpdate:
			pos, _ := protocol.ReadPosition(bytes.NewReader(data))
			state, _ := protocol.ReadVarInt(bytes.NewReader(data))
			x, y, z := pos.Decode()
			logger.Printf("BLOCK UPDATE at %d,%d,%d state=%d", x, y, z, state)
		case protocol.PlayIDDisconnect:
			logger.Printf("play disconnect")
			return
		default:
			// ignore
		}
	}
}

func sendChat(c *mcnet.Connection, msg string) error {
	var b bytes.Buffer
	_ = protocol.WriteString(&b, msg)
	_ = protocol.WriteInt64(&b, time.Now().UnixMilli()) // timestamp
	_ = protocol.WriteInt64(&b, 0)                      // salt
	_ = protocol.WriteBool(&b, false)                   // no signature
	_ = protocol.WriteVarInt(&b, 0)                     // message count
	_, _ = b.Write([]byte{0, 0, 0})                     // acknowledged fixed bitset(20) = 3 bytes
	_ = protocol.WriteInt8(&b, 0)                       // checksum
	return c.WritePacket(protocol.PlayIDChatMessage, b.Bytes())
}

func sendPlayerAction(c *mcnet.Connection, action int32, pos protocol.Position, face int8, seq int32) error {
	var b bytes.Buffer
	_ = protocol.WriteVarInt(&b, action)
	_ = protocol.WritePosition(&b, pos)
	_ = protocol.WriteInt8(&b, face)
	_ = protocol.WriteVarInt(&b, seq)
	return c.WritePacket(protocol.PlayIDPlayerAction, b.Bytes())
}

func playerUUID() protocol.UUID {
	return protocol.UUID{0x06, 0x9b, 0x6e, 0x47, 0x36, 0x1f, 0x4a, 0x2b, 0x9c, 0x11, 0x8a, 0x76, 0x3d, 0x62, 0x4a, 0x1d}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func fail(format string, args ...any) {
	logger.Printf(format, args...)
	os.Exit(2)
}
