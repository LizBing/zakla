package server

import (
	"crypto/md5"
	"crypto/rand"
	"sync/atomic"

	"github.com/zakla/mc-server/pkg/protocol"
)

// OfflineUUID computes the offline-mode player UUID, matching vanilla's
// UUIDv3 of "OfflinePlayer:"+name (bytes hashed with MD5, version/variant bits set).
func OfflineUUID(name string) protocol.UUID {
	sum := md5.Sum([]byte("OfflinePlayer:" + name))
	sum[6] = (sum[6] & 0x0f) | 0x30 // version 3
	sum[8] = (sum[8] & 0x3f) | 0x80 // IETF variant
	var u protocol.UUID
	copy(u[:], sum[:16])
	return u
}

// RandomUUID generates a random UUIDv4.
func RandomUUID() protocol.UUID {
	var u protocol.UUID
	_, _ = rand.Read(u[:])
	u[6] = (u[6] & 0x0f) | 0x40 // version 4
	u[8] = (u[8] & 0x3f) | 0x80 // IETF variant
	return u
}

var (
	entityCounter   int32
	teleportCounter int32
)

// NextEntityID returns a fresh server-wide entity id.
func NextEntityID() int32 { return atomic.AddInt32(&entityCounter, 1) }

// NextTeleportID returns a fresh teleport confirmation id.
func NextTeleportID() int32 { return atomic.AddInt32(&teleportCounter, 1) }
