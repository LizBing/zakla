package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"sort"
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
		ViewDistance:        playerViewDistance,
		SimulationDistance:  playerViewDistance,
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
	// Login sets GameMode=1; follow up with an explicit "Change game mode" Game
	// Event to force the client into creative (inventory UI, flight, instant
	// break). value=1.0 = creative.
	_ = conn.WritePacket(protocol.PlayIDGameEvent, protocol.EncodeGameEvent(protocol.GameEventChangeGameMode, 1.0))
	_ = conn.WritePacket(protocol.PlayIDChangeDifficulty, protocol.EncodeChangeDifficulty(1, false))
	// Creative abilities: invulnerable | allow flying | creative (instant break).
	_ = conn.WritePacket(protocol.PlayIDPlayerAbilities, protocol.EncodePlayerAbilities(0x0D, 0.05, 0.1))
	_ = conn.WritePacket(protocol.PlayIDSetHeldItem, protocol.EncodeSetHeldItem(0))
	_ = conn.WritePacket(protocol.PlayIDPlayerInfoUpdate, protocol.EncodePlayerInfoUpdateAdd(uuid, name, 1, true, 0))
	_ = conn.WritePacket(protocol.PlayIDSetDefaultSpawn, protocol.EncodeSetDefaultSpawn("minecraft:overworld", protocol.EncodePosition(0, 64, 0), 0, 0))

	// Load persisted player data (hotbar + last position); fall back to a
	// starter inventory and spawn position for brand-new players.
	player := &Player{conn: conn, Name: name, UUID: uuid, EntityID: entityID, x: 0.5, y: 64, z: 0.5}
	pdat, pErr := LoadPlayerData(s.cfg.World.Name, uuid)
	if pErr != nil {
		log.Printf("[%s] load player data: %v", name, pErr)
	}
	if pdat != nil {
		player.heldSlot = pdat.HeldSlot
		player.x, player.y, player.z = pdat.X, pdat.Y, pdat.Z
		player.yaw, player.pitch = pdat.Yaw, pdat.Pitch
		for i, slot := range pdat.Inventory {
			player.inventory[i] = protocol.SlotData{ItemID: slot.ItemID, Count: slot.Count}
		}
	} else {
		starter := []string{
			"minecraft:stone", "minecraft:grass_block", "minecraft:dirt",
			"minecraft:cobblestone", "minecraft:oak_planks", "minecraft:glass",
			"minecraft:oak_log", "minecraft:sand", "minecraft:gravel",
		}
		for i, blk := range starter {
			player.inventory[36+i] = protocol.SlotData{ItemID: ItemID(blk), Count: 64}
		}
	}

	teleportID := NextTeleportID()
	_ = conn.WritePacket(protocol.PlayIDSynchPlayerPos, protocol.EncodeSynchronizePlayerPos(teleportID, player.x, player.y, player.z, 0, 0, 0, player.yaw, player.pitch, 0))
	_ = conn.WritePacket(protocol.PlayIDGameEvent, protocol.EncodeGameEvent(protocol.GameEventStartWaitChunks, 0))
	_ = conn.WritePacket(protocol.PlayIDSetCenterChunk, protocol.EncodeSetCenterChunk(0, 0))

	// Stream initial chunks around the player (and track them for later
	// dynamic load/unload as the player moves). Sending neighbors is also
	// required because the vanilla client won't render a chunk with no
	// neighbors (wiki: Chunk format).
	player.sentChunks = map[chunkKey]bool{}
	s.updatePlayerChunks(conn, player)
	_ = conn.WritePacket(protocol.PlayIDSetHealth, protocol.EncodeSetHealth(20, 20, 5))

	// Inventory: send the player's full inventory (hotbar slots are 36-44).
	slots := make([]protocol.SlotData, 46)
	copy(slots[:], player.inventory[:])
	_ = conn.WritePacket(protocol.PlayIDSetContainerContent, protocol.EncodeSetContainerContent(0, 0, slots, protocol.EmptySlot))
	_ = conn.WritePacket(protocol.PlayIDSetHeldItem, protocol.EncodeSetHeldItem(player.heldSlot))

	s.addPlayer(player)
	s.broadcastChat(fmt.Sprintf("§e%s joined the game", name))

	ctx, cancel := context.WithCancel(context.Background())
	go s.keepAliveLoop(ctx, conn)

	defer func() {
		cancel()
		s.removePlayer(uuid)
		s.broadcastChat(fmt.Sprintf("§e%s left the game", name))
		// Persist this player's inventory + position.
		dat := &playerData{
			HeldSlot: player.heldSlot,
			X:        player.x,
			Y:        player.y,
			Z:        player.z,
			Yaw:      player.yaw,
			Pitch:    player.pitch,
		}
		for i, slot := range player.inventory {
			dat.Inventory[i] = invSlot{ItemID: slot.ItemID, Count: slot.Count}
		}
		if err := SavePlayerData(s.cfg.World.Name, uuid, dat); err != nil {
			log.Printf("[%s] save player data: %v", name, err)
		}
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
			protocol.PlayIDClientTickEnd, protocol.PlayIDClientInfoSB:
			// accepted/ignored in MVP
		case protocol.PlayIDPlayerPosRot:
			if m, mErr := protocol.DecodeMovePlayerPosRot(data); mErr == nil {
				player.x, player.y, player.z = m.X, m.Y, m.Z
				player.yaw, player.pitch = m.Yaw, m.Pitch
				// When the player crosses into a new chunk, refresh loaded
				// chunks (stream in new ones, unload far ones).
				if int32(math.Floor(m.X/16)) != player.lastChunkX || int32(math.Floor(m.Z/16)) != player.lastChunkZ {
					s.updatePlayerChunks(conn, player)
				}
			}
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
			// Creative mode breaks instantly on "Started digging"; survival breaks
			// on "Finished". We're creative, so treat both as a destroy.
			if act.Action == protocol.PlayerActionFinishedDigging || act.Action == protocol.PlayerActionStartedDigging {
				x, y, z := act.Position.Decode()
				s.world.SetBlock(x, y, z, 0) // replace with air
				s.broadcastBlockUpdate(act.Position, 0)
				log.Printf("[%s] mined block (%d,%d,%d) -> air, ack seq=%d", name, x, y, z, act.Sequence)
			}
			// Every Player Action carrying a sequence must be acked, or the
			// client freezes further block edits.
			_ = conn.WritePacket(protocol.PlayIDBlockChangedAck, protocol.EncodeBlockChangedAck(act.Sequence))
		case protocol.PlayIDUseItemOn:
			u, uErr := protocol.DecodeUseItemOn(data)
			if uErr != nil {
				log.Printf("[%s] invalid use item on: %v", name, uErr)
				break
			}
			// Place the held block (if any) at clicked-location + face offset,
			// but only into an empty cell. Hand 0 = main hand (inventory slot
			// 36+heldSlot), 1 = off-hand (slot 45). The ack must be sent
			// regardless or the client reverts the placement (ghost block).
			heldSlot := 36 + int(player.heldSlot) // main hand
			if u.Hand == 1 {
				heldSlot = 45 // off-hand
			}
			if held := player.inventory[heldSlot].ItemID; held > 0 {
				if blkName, ok := itemIDToName[held]; ok {
					if blkState := BlockStateID(blkName); blkState != 0 {
						x, y, z := u.Position.Decode()
						dx, dy, dz := protocol.FaceOffset(u.Face)
						nx, ny, nz := x+dx, y+dy, z+dz
						// Only into an empty cell, and not into the player's
						// own body (foot + head) — vanilla rejects placement
						// that would intersect the placer's collision box.
						if s.world.GetBlock(nx, ny, nz) == 0 && !playerOccupies(player, nx, ny, nz) {
							s.world.SetBlock(nx, ny, nz, blkState)
							s.broadcastBlockUpdate(protocol.EncodePosition(nx, ny, nz), blkState)
							log.Printf("[%s] placed %s at (%d,%d,%d)", name, blkName, nx, ny, nz)
						}
					}
				}
			}
			_ = conn.WritePacket(protocol.PlayIDBlockChangedAck, protocol.EncodeBlockChangedAck(u.Sequence))
		case protocol.PlayIDSetCarriedItemSB:
			if slot, sErr := protocol.DecodeSetCarriedItem(data); sErr == nil && slot >= 0 && slot < 9 {
				player.heldSlot = slot
			}
		case protocol.PlayIDSetCreativeSlot:
			// Creative player placed an item into a slot (hotbar or main
			// inventory). Track it so the player's chosen inventory persists.
			if cs, csErr := protocol.DecodeSetCreativeModeSlot(data); csErr == nil {
				if cs.Slot >= 0 && cs.Slot < 46 {
					player.inventory[cs.Slot] = cs.Item
				}
			}
		default:
			// unhandled; ignore
		}
	}
}

// playerViewDistance is the Chebyshev radius of chunks sent around the player.
const playerViewDistance = 4

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// playerOccupies reports whether block (x,y,z) intersects the player's body
// (the foot cell plus the head cell above it). Used to stop a player placing a
// block into themselves.
func playerOccupies(p *Player, x, y, z int) bool {
	fx, fy, fz := int(math.Floor(p.x)), int(math.Floor(p.y)), int(math.Floor(p.z))
	return fx == x && fz == z && (fy == y || fy+1 == y)
}

// updatePlayerChunks keeps the client's loaded chunks in sync with the player's
// position: sends Set Center Chunk, streams in newly-entered chunks (nearest
// first), and explicitly unloads chunks that left the view distance. The
// client's auto-unload is unreliable (lazy slot modulo), so we must unload
// explicitly while the chunk is still technically in range (wiki: Set Center
// Chunk — "servers should always explicitly unload any loaded chunks before
// they go outside the loading area").
func (s *Server) updatePlayerChunks(conn *mcnet.Connection, player *Player) {
	cx := int32(math.Floor(player.x / 16))
	cz := int32(math.Floor(player.z / 16))
	player.lastChunkX, player.lastChunkZ = cx, cz

	_ = conn.WritePacket(protocol.PlayIDSetCenterChunk, protocol.EncodeSetCenterChunk(cx, cz))

	want := make(map[chunkKey]bool, (2*playerViewDistance+1)*(2*playerViewDistance+1))
	for dz := int32(-playerViewDistance); dz <= playerViewDistance; dz++ {
		for dx := int32(-playerViewDistance); dx <= playerViewDistance; dx++ {
			want[chunkKey{cx + dx, cz + dz}] = true
		}
	}
	// Send newly-entered chunks, nearest first.
	type pending struct {
		k chunkKey
		d int32
	}
	var entering []pending
	for k := range want {
		if !player.sentChunks[k] {
			entering = append(entering, pending{k, max(abs32(k.x-cx), abs32(k.z-cz))})
		}
	}
	sort.Slice(entering, func(i, j int) bool { return entering[i].d < entering[j].d })
	sentN := 0
	for _, e := range entering {
		if payload, err := s.world.GetOrCreateChunk(e.k.x, e.k.z).Encode(); err == nil {
			_ = conn.WritePacket(protocol.PlayIDChunkDataLight, payload)
			player.sentChunks[e.k] = true
			sentN++
		} else {
			log.Printf("[%s] chunk (%d,%d) unavailable: %v", player.Name, e.k.x, e.k.z, err)
		}
	}
	// Unload chunks that left the view distance.
	unloadedN := 0
	for k := range player.sentChunks {
		if !want[k] {
			_ = conn.WritePacket(protocol.PlayIDUnloadChunk, protocol.EncodeUnloadChunk(k.x, k.z))
			delete(player.sentChunks, k)
			unloadedN++
		}
	}
	log.Printf("[%s] chunks: center (%d,%d) sent=%d unloaded=%d loaded=%d", player.Name, cx, cz, sentN, unloadedN, len(player.sentChunks))
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
