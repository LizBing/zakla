package protocol

import (
	"bytes"
	"fmt"
)

// Configuration-phase packet IDs (PVN 776).
const (
	CfgIDPluginMessage int32 = 0x01 // clientbound
	CfgIDDisconnect    int32 = 0x02 // clientbound
	CfgIDFinishConfig  int32 = 0x03 // clientbound
	CfgIDKeepAlive     int32 = 0x04 // both
	CfgIDPing          int32 = 0x05 // clientbound
	CfgIDResetChat     int32 = 0x06 // clientbound
	CfgIDRegistryData  int32 = 0x07 // clientbound
	CfgIDFeatureFlags  int32 = 0x0C // clientbound
	CfgIDUpdateTags    int32 = 0x0D // clientbound
	CfgIDKnownPacksCB  int32 = 0x0E // clientbound
	CfgIDCustomReport  int32 = 0x0F // clientbound
	CfgIDServerLinks   int32 = 0x10 // clientbound

	CfgIDClientInfo      int32 = 0x00 // serverbound
	CfgIDPluginMessageSB int32 = 0x02 // serverbound
	CfgIDAckFinishConfig int32 = 0x03 // serverbound
	CfgIDKeepAliveSB     int32 = 0x04 // serverbound
	CfgIDPong            int32 = 0x05 // serverbound
	CfgIDKnownPacksSB    int32 = 0x07 // serverbound
)

// KnownPack names a data pack whose contents the sender already has.
type KnownPack struct {
	Namespace string
	ID        string
	Version   string
}

// EncodeKnownPacks builds a Known Packs payload (used by both directions).
func EncodeKnownPacks(packs []KnownPack) []byte {
	var buf bytes.Buffer
	_ = WriteVarInt(&buf, int32(len(packs)))
	for _, p := range packs {
		_ = WriteString(&buf, p.Namespace)
		_ = WriteString(&buf, p.ID)
		_ = WriteString(&buf, p.Version)
	}
	return buf.Bytes()
}

// DecodeKnownPacks parses a Known Packs payload.
func DecodeKnownPacks(data []byte) ([]KnownPack, error) {
	r := bytes.NewReader(data)
	n, err := ReadVarInt(r)
	if err != nil {
		return nil, fmt.Errorf("read count: %w", err)
	}
	if n < 0 {
		return nil, fmt.Errorf("negative known pack count %d", n)
	}
	packs := make([]KnownPack, n)
	for i := int32(0); i < n; i++ {
		packs[i].Namespace, err = ReadString(r)
		if err != nil {
			return nil, err
		}
		packs[i].ID, err = ReadString(r)
		if err != nil {
			return nil, err
		}
		packs[i].Version, err = ReadString(r)
		if err != nil {
			return nil, err
		}
	}
	return packs, nil
}

// RegistryEntry is one entry of a synchronized registry sent in Registry Data.
// If NBT is empty, the client resolves the data from its known packs.
type RegistryEntry struct {
	Name string // Identifier
	NBT  []byte // optional NBT (a full root compound); empty = omit
}

// EncodeRegistryData builds a Registry Data payload (Configuration 0x07).
func EncodeRegistryData(registryID string, entries []RegistryEntry) ([]byte, error) {
	var buf bytes.Buffer
	if err := WriteIdentifier(&buf, registryID); err != nil {
		return nil, err
	}
	if err := WriteVarInt(&buf, int32(len(entries))); err != nil {
		return nil, err
	}
	for _, e := range entries {
		if err := WriteIdentifier(&buf, e.Name); err != nil {
			return nil, err
		}
		hasData := len(e.NBT) > 0
		if err := WriteBool(&buf, hasData); err != nil {
			return nil, err
		}
		if hasData {
			if _, err := buf.Write(e.NBT); err != nil {
				return nil, err
			}
		}
	}
	return buf.Bytes(), nil
}

// EncodeFeatureFlags builds a Feature Flags payload (Configuration 0x0C).
func EncodeFeatureFlags(flags []string) []byte {
	var buf bytes.Buffer
	_ = WriteVarInt(&buf, int32(len(flags)))
	for _, f := range flags {
		_ = WriteIdentifier(&buf, f)
	}
	return buf.Bytes()
}

// EncodePluginMessage builds a Plugin Message payload (Configuration 0x01 / Play 0x18).
func EncodePluginMessage(channel string, data []byte) []byte {
	var buf bytes.Buffer
	_ = WriteIdentifier(&buf, channel)
	_, _ = buf.Write(data)
	return buf.Bytes()
}

// EncodeBrandData builds the body of a minecraft:brand plugin channel message
// (a single String: the brand name).
func EncodeBrandData(brand string) []byte {
	var buf bytes.Buffer
	_ = WriteString(&buf, brand)
	return buf.Bytes()
}

// EncodeFinishConfiguration returns the empty Finish Configuration payload.
func EncodeFinishConfiguration() []byte { return nil }

// EncodePing builds a Ping payload (Int id).
func EncodePing(id int32) []byte {
	var buf bytes.Buffer
	_ = WriteInt32(&buf, id)
	return buf.Bytes()
}

// EncodePong builds a Pong payload (Int id).
func EncodePong(id int32) []byte {
	var buf bytes.Buffer
	_ = WriteInt32(&buf, id)
	return buf.Bytes()
}

// TagDefinition is one tag and its member IDs within an Update Tags payload.
type TagDefinition struct {
	Name    string  // tag identifier
	Entries []int32 // registry element IDs (empty = tag exists but has no members)
}

// TagRegistry groups tags for one registry in an Update Tags payload.
type TagRegistry struct {
	Registry string
	Tags     []TagDefinition
}

// EncodeUpdateTags builds the Update Tags payload (Configuration 0x0D / Play 0x86).
func EncodeUpdateTags(registries []TagRegistry) ([]byte, error) {
	var buf bytes.Buffer
	if err := WriteVarInt(&buf, int32(len(registries))); err != nil {
		return nil, err
	}
	for _, r := range registries {
		if err := WriteIdentifier(&buf, r.Registry); err != nil {
			return nil, err
		}
		if err := WriteVarInt(&buf, int32(len(r.Tags))); err != nil {
			return nil, err
		}
		for _, t := range r.Tags {
			if err := WriteIdentifier(&buf, t.Name); err != nil {
				return nil, err
			}
			if err := WriteVarInt(&buf, int32(len(t.Entries))); err != nil {
				return nil, err
			}
			for _, e := range t.Entries {
				if err := WriteVarInt(&buf, e); err != nil {
					return nil, err
				}
			}
		}
	}
	return buf.Bytes(), nil
}

// ClientInformation holds the serverbound Client Information payload
// (sent at the start of configuration and on settings changes).
type ClientInformation struct {
	Locale             string
	ViewDistance       int8
	ChatMode           int32 // 0=enabled, 1=commands only, 2=hidden
	ChatColors         bool
	DisplayedSkinParts uint8
	MainHand           int32 // 0=left, 1=right
	EnableTextFilter   bool
	AllowServerListing bool
	ParticleStatus     int32 // 0=all, 1=decreased, 2=minimal
}

// DecodeClientInformation parses the Client Information payload.
func DecodeClientInformation(data []byte) (*ClientInformation, error) {
	r := bytes.NewReader(data)
	ci := &ClientInformation{}
	var err error
	if ci.Locale, err = ReadString(r); err != nil {
		return nil, err
	}
	if ci.ViewDistance, err = ReadInt8(r); err != nil {
		return nil, err
	}
	if ci.ChatMode, err = ReadVarInt(r); err != nil {
		return nil, err
	}
	if ci.ChatColors, err = ReadBool(r); err != nil {
		return nil, err
	}
	if ci.DisplayedSkinParts, err = ReadUint8(r); err != nil {
		return nil, err
	}
	if ci.MainHand, err = ReadVarInt(r); err != nil {
		return nil, err
	}
	if ci.EnableTextFilter, err = ReadBool(r); err != nil {
		return nil, err
	}
	if ci.AllowServerListing, err = ReadBool(r); err != nil {
		return nil, err
	}
	if ci.ParticleStatus, err = ReadVarInt(r); err != nil {
		return nil, err
	}
	return ci, nil
}
