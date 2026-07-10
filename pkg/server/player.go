package server

import (
	"encoding/gob"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/zakla/mc-server/pkg/protocol"
)

// playerData is the on-disk per-player snapshot (gob). Player holds unexported
// fields that gob skips, so we round-trip through this exported mirror.
type playerData struct {
	Inventory  [46]invSlot
	HeldSlot   int32
	X, Y, Z    float64
	Yaw, Pitch float32
}

// invSlot is one inventory slot in the persisted form.
type invSlot struct {
	ItemID int32
	Count  int32
}

func playerDataPath(worldPath string, uuid protocol.UUID) string {
	return filepath.Join(worldPath, "players", hex.EncodeToString(uuid[:])+".gob")
}

// LoadPlayerData loads a player's snapshot, or returns nil if the player has no
// saved data yet (new player → caller issues the starter inventory).
func LoadPlayerData(worldPath string, uuid protocol.UUID) (*playerData, error) {
	if worldPath == "" {
		return nil, nil
	}
	f, err := os.Open(playerDataPath(worldPath, uuid))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var d playerData
	if err := gob.NewDecoder(f).Decode(&d); err != nil {
		return nil, err
	}
	return &d, nil
}

// SavePlayerData atomically writes a player's snapshot to
// <world>/players/<uuid>.gob (tmp + rename, like the world save).
func SavePlayerData(worldPath string, uuid protocol.UUID, d *playerData) error {
	if worldPath == "" {
		return nil
	}
	dir := filepath.Join(worldPath, "players")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := hex.EncodeToString(uuid[:])
	tmp := filepath.Join(dir, name+".gob.tmp")
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(d); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name+".gob"))
}
