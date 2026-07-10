package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	mcnet "github.com/zakla/mc-server/pkg/net"
	"github.com/zakla/mc-server/pkg/protocol"
)

// handle is the per-connection state machine entry point.
func (s *Server) handle(conn *mcnet.Connection) {
	defer conn.Close()

	// --- Handshake ---
	id, data, err := conn.ReadPacket()
	if err != nil {
		log.Printf("[%s] handshake read error: %v", conn.RemoteAddr(), err)
		return
	}
	if id != protocol.PacketIDHandshake {
		log.Printf("[%s] expected handshake, got packet 0x%x", conn.RemoteAddr(), id)
		return
	}
	hs, err := protocol.DecodeHandshake(data)
	if err != nil {
		log.Printf("[%s] invalid handshake: %v", conn.RemoteAddr(), err)
		return
	}

	switch hs.Intent {
	case protocol.IntentStatus:
		conn.SetState(mcnet.StateStatus)
		s.handleStatus(conn)
	case protocol.IntentLogin, protocol.IntentTransfer:
		if hs.ProtocolVersion != protocol.ProtocolVersion {
			s.disconnectLogin(conn, fmt.Sprintf("Unsupported protocol version %d (server runs 26.2 / protocol %d)", hs.ProtocolVersion, protocol.ProtocolVersion))
			return
		}
		conn.SetState(mcnet.StateLogin)
		s.handleLogin(conn)
	default:
		log.Printf("[%s] unknown handshake intent %d", conn.RemoteAddr(), hs.Intent)
	}
}

// --- Status phase (server list ping) ---

func (s *Server) handleStatus(conn *mcnet.Connection) {
	// Status Request (0x00, empty body).
	if _, _, err := conn.ReadPacket(); err != nil {
		return
	}
	status := s.buildStatus()
	payload, err := protocol.EncodeStatusResponse(status)
	if err != nil {
		return
	}
	if err := conn.WritePacket(protocol.PacketIDStatusResponse, payload); err != nil {
		return
	}
	// Ping Request (0x01).
	id, data, err := conn.ReadPacket()
	if err != nil {
		return
	}
	if id != protocol.PacketIDPingRequest {
		return
	}
	ts, err := protocol.DecodePingRequest(data)
	if err != nil {
		return
	}
	_ = conn.WritePacket(protocol.PacketIDPongResponse, protocol.EncodePongResponse(ts))
}

func (s *Server) buildStatus() *protocol.ServerStatus {
	desc, _ := json.Marshal(map[string]string{"text": s.cfg.Motd})
	return &protocol.ServerStatus{
		Version:     protocol.ServerPingVersion{Name: "26.2", Protocol: protocol.ProtocolVersion},
		Players:     protocol.ServerPingPlayers{Max: s.cfg.MaxPlayers, Online: s.onlineCount()},
		Description: desc,
	}
}

func (s *Server) disconnectLogin(conn *mcnet.Connection, reason string) {
	reasonJSON, _ := json.Marshal(map[string]string{"text": reason})
	_ = conn.WritePacket(protocol.PacketIDLoginDisconnect, protocol.EncodeLoginDisconnect(string(reasonJSON)))
}

// --- Login phase ---

func (s *Server) handleLogin(conn *mcnet.Connection) {
	id, data, err := conn.ReadPacket()
	if err != nil {
		return
	}
	if id != protocol.PacketIDLoginStart {
		log.Printf("[%s] expected login start, got 0x%x", conn.RemoteAddr(), id)
		return
	}
	name, _, err := protocol.DecodeLoginStart(data)
	if err != nil {
		return
	}
	name = sanitizeName(name)
	uuid := OfflineUUID(name)

	// Set Compression.
	threshold := int32(256)
	if s.cfg.Network.CompressionThreshold > 0 {
		threshold = int32(s.cfg.Network.CompressionThreshold)
	}
	_ = conn.WritePacket(protocol.PacketIDSetCompression, protocol.EncodeSetCompression(threshold))
	conn.SetCompression(int(threshold))

	// Login Success (offline mode, no encryption).
	sessionID := RandomUUID()
	if err := conn.WritePacket(protocol.PacketIDLoginSuccess, protocol.EncodeLoginSuccess(uuid, name, sessionID)); err != nil {
		return
	}

	// Login Acknowledged.
	id, _, err = conn.ReadPacket()
	if err != nil {
		return
	}
	if id != protocol.PacketIDLoginAcknowledged {
		log.Printf("[%s] expected login acknowledged, got 0x%x", conn.RemoteAddr(), id)
		return
	}

	conn.SetState(mcnet.StateConfiguration)
	s.handleConfiguration(conn, name, uuid)
}

// --- Configuration phase ---

func (s *Server) handleConfiguration(conn *mcnet.Connection, name string, uuid protocol.UUID) {
	// Server brand.
	_ = conn.WritePacket(protocol.CfgIDPluginMessage, protocol.EncodePluginMessage("minecraft:brand", protocol.EncodeBrandData("vanilla")))
	// Feature flags.
	_ = conn.WritePacket(protocol.CfgIDFeatureFlags, protocol.EncodeFeatureFlags([]string{"minecraft:vanilla"}))
	// Known packs (clientbound) — client replies with the subset it knows.
	_ = conn.WritePacket(protocol.CfgIDKnownPacksCB, protocol.EncodeKnownPacks([]protocol.KnownPack{{Namespace: "minecraft", ID: "core", Version: "26.2"}}))

	registrySent := false
	finishAcked := false
	for !finishAcked {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		id, data, err := conn.ReadPacket()
		if err != nil {
			log.Printf("[%s] configuration read error: %v", name, err)
			return
		}
		_ = conn.SetReadDeadline(time.Time{})

		switch id {
		case protocol.CfgIDClientInfo:
			_, _ = protocol.DecodeClientInformation(data)
		case protocol.CfgIDPluginMessageSB:
			// client brand; ignored
		case protocol.CfgIDKnownPacksSB:
			_, _ = protocol.DecodeKnownPacks(data)
			if !registrySent {
				s.sendRegistryData(conn)
				registrySent = true
				_ = conn.WritePacket(protocol.CfgIDFinishConfig, protocol.EncodeFinishConfiguration())
			}
		case protocol.CfgIDAckFinishConfig:
			finishAcked = true
		case protocol.CfgIDKeepAliveSB, protocol.CfgIDPong:
			// ignored
		default:
			log.Printf("[%s] config: unhandled packet 0x%x", name, id)
		}
	}

	conn.SetState(mcnet.StatePlay)
	log.Printf("[%s] entering play state", name)
	s.handlePlay(conn, name, uuid)
}

func (s *Server) sendRegistryData(conn *mcnet.Connection) {
	for _, reg := range DefaultRegistries() {
		payload, err := protocol.EncodeRegistryData(reg.ID, reg.Entries)
		if err != nil {
			log.Printf("registry %s encode error: %v", reg.ID, err)
			continue
		}
		_ = conn.WritePacket(protocol.CfgIDRegistryData, payload)
	}
	// Update Tags: tags must exist (registry NBT references them by name).
	if tagPayload, err := protocol.EncodeUpdateTags(vanillaTags); err == nil {
		_ = conn.WritePacket(protocol.CfgIDUpdateTags, tagPayload)
	} else {
		log.Printf("update tags encode error: %v", err)
	}
}

// --- Play phase ---

func (s *Server) handlePlay(conn *mcnet.Connection, name string, uuid protocol.UUID) {
	entityID := NextEntityID()

	lp := &protocol.LoginPlay{
		EntityID:            entityID,
		DimensionNames:      []string{"minecraft:overworld"},
		MaxPlayers:          int32(s.cfg.MaxPlayers),
		ViewDistance:        8,
		SimulationDistance:  8,
		EnableRespawnScreen: true,
		DimensionType:       0,
		DimensionName:       "minecraft:overworld",
		GameMode:            1, // creative
		PreviousGameMode:    -1,
		IsFlat:              true,
		PortalCooldown:      300,
		SeaLevel:            63,
	}
	_ = conn.WritePacket(protocol.PlayIDLogin, protocol.EncodeLoginPlay(lp))
	_ = conn.WritePacket(protocol.PlayIDChangeDifficulty, protocol.EncodeChangeDifficulty(1, false))
	_ = conn.WritePacket(protocol.PlayIDPlayerAbilities, protocol.EncodePlayerAbilities(0, 0.05, 0.1))
	_ = conn.WritePacket(protocol.PlayIDSetHeldItem, protocol.EncodeSetHeldItem(0))
	_ = conn.WritePacket(protocol.PlayIDPlayerInfoUpdate, protocol.EncodePlayerInfoUpdateAdd(uuid, name, 1, true, 0))
	_ = conn.WritePacket(protocol.PlayIDSetDefaultSpawn, protocol.EncodeSetDefaultSpawn("minecraft:overworld", protocol.EncodePosition(0, 64, 0), 0, 0))

	teleportID := NextTeleportID()
	_ = conn.WritePacket(protocol.PlayIDSynchPlayerPos, protocol.EncodeSynchronizePlayerPos(teleportID, 0.5, 64, 0.5, 0, 0, 0, 0, 0, 0))
	_ = conn.WritePacket(protocol.PlayIDGameEvent, protocol.EncodeGameEvent(protocol.GameEventStartWaitChunks, 0))
	_ = conn.WritePacket(protocol.PlayIDSetCenterChunk, protocol.EncodeSetCenterChunk(0, 0))

	// The vanilla client does NOT render a chunk that has no neighbors (wiki:
	// Chunk format — "client generally does not render chunks that lack
	// neighbors, although you can still interact with them"). So we send a
	// square of chunks around spawn; only (0,0) holds the platform, the rest
	// are empty air columns that exist purely so (0,0) gets rendered.
	const spawnRadius = 4
	for cz := -spawnRadius; cz <= spawnRadius; cz++ {
		for cx := -spawnRadius; cx <= spawnRadius; cx++ {
			if payload, err := s.world.GetOrCreateChunk(int32(cx), int32(cz)).Encode(); err == nil {
				_ = conn.WritePacket(protocol.PlayIDChunkDataLight, payload)
			} else {
				log.Printf("[%s] chunk (%d,%d) unavailable: %v", name, cx, cz, err)
			}
		}
	}
	_ = conn.WritePacket(protocol.PlayIDSetHealth, protocol.EncodeSetHealth(20, 20, 5))

	player := &Player{conn: conn, Name: name, UUID: uuid, EntityID: entityID}
	s.addPlayer(player)
	s.broadcastChat(fmt.Sprintf("§e%s joined the game", name))

	ctx, cancel := context.WithCancel(context.Background())
	go s.keepAliveLoop(ctx, conn)

	defer func() {
		cancel()
		s.removePlayer(uuid)
		s.broadcastChat(fmt.Sprintf("§e%s left the game", name))
	}()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		id, data, err := conn.ReadPacket()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				log.Printf("[%s] connection timed out", name)
			} else {
				log.Printf("[%s] play read error: %v", name, err)
			}
			return
		}
		switch id {
		case protocol.PlayIDConfirmTeleport, protocol.PlayIDKeepAliveSB, protocol.PlayIDPlayerLoaded,
			protocol.PlayIDClientTickEnd, protocol.PlayIDClientInfoSB, protocol.PlayIDPlayerPosRot:
			// accepted/ignored in MVP
		case protocol.PlayIDChatMessage:
			if msg, mErr := protocol.DecodeChatMessage(data); mErr == nil && len(msg) > 0 {
				log.Printf("[chat] <%s> %s", name, msg)
				s.broadcastChat(fmt.Sprintf("<%s> %s", name, msg))
			}
		case protocol.PlayIDPlayerAction:
			act, aErr := protocol.DecodePlayerAction(data)
			if aErr != nil {
				log.Printf("[%s] invalid player action: %v", name, aErr)
				break
			}
			if act.Action == protocol.PlayerActionFinishedDigging {
				x, y, z := act.Position.Decode()
				s.world.SetBlock(x, y, z, 0) // replace with air
				s.broadcastBlockUpdate(act.Position, 0)
				log.Printf("[%s] mined block (%d,%d,%d) -> air, ack seq=%d", name, x, y, z, act.Sequence)
			}
			// Every Player Action carrying a sequence must be acked, or the
			// client freezes further block edits.
			_ = conn.WritePacket(protocol.PlayIDBlockChangedAck, protocol.EncodeBlockChangedAck(act.Sequence))
		default:
			// unhandled; ignore
		}
	}
}

func (s *Server) keepAliveLoop(ctx context.Context, conn *mcnet.Connection) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload := protocol.EncodeKeepAlive(time.Now().UnixMilli())
			if err := conn.WritePacket(protocol.PlayIDKeepAlive, payload); err != nil {
				return
			}
		}
	}
}

// sanitizeName truncates a username to the 16-character vanilla limit.
func sanitizeName(name string) string {
	if len(name) > 16 {
		name = name[:16]
	}
	return name
}
