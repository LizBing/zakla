// Package server implements the Minecraft server state machine for Java
// Edition 26.2: handshake → status/login → configuration → play.
package server

import (
	"context"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/zakla/mc-server/pkg/config"
	mcnet "github.com/zakla/mc-server/pkg/net"
	"github.com/zakla/mc-server/pkg/protocol"
)

// Player is a connected, authenticated player.
type Player struct {
	conn       *mcnet.Connection
	Name       string
	UUID       protocol.UUID
	EntityID   int32
	heldSlot   int32                 // selected hotbar slot 0-8 (maps to inventory slot 36+heldSlot)
	inventory  [46]protocol.SlotData // full player inventory; 36-44 = hotbar
	x, y, z    float64               // last known position
	yaw, pitch float32

	lastChunkX, lastChunkZ int32             // last chunk the player was known to be in
	sentChunks             map[chunkKey]bool // chunks the client currently has loaded
}

// Server is the Minecraft server.
type Server struct {
	cfg      *config.Config
	listener *mcnet.Listener

	mu      sync.RWMutex
	players map[protocol.UUID]*Player

	world *World

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a server with the given configuration and a fresh world with the
// spawn platform pre-filled.
func New(cfg *config.Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	world, err := LoadWorld(cfg.World.Name)
	if err != nil {
		log.Fatalf("load world %q: %v", cfg.World.Name, err)
	}
	if world.ChunkCount() == 0 {
		// Terrain is generated on demand per chunk (GetOrCreateChunk → fillFloor).
		if err := world.Save(); err != nil {
			log.Printf("initial world save: %v", err)
		}
	}
	return &Server{
		cfg:     cfg,
		players: make(map[protocol.UUID]*Player),
		world:   world,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start begins listening and blocks until the server is stopped.
func (s *Server) Start() error {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	s.listener = mcnet.NewListener(addr, s)
	if err := s.listener.Start(); err != nil {
		return err
	}
	log.Printf("Minecraft server listening on %s (protocol %d, version 26.2)", addr, protocol.ProtocolVersion)
	go s.saveLoop(s.ctx)
	<-s.ctx.Done()
	return s.Stop()
}

// Stop gracefully shuts the server down (idempotent).
func (s *Server) Stop() error {
	s.cancel()
	if s.listener != nil {
		_ = s.listener.Stop()
	}
	s.mu.Lock()
	conns := make([]*mcnet.Connection, 0, len(s.players))
	for _, p := range s.players {
		conns = append(conns, p.conn)
	}
	s.players = make(map[protocol.UUID]*Player)
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
	s.wg.Wait()
	if err := s.world.Save(); err != nil {
		log.Printf("final world save: %v", err)
	}
	log.Println("Server stopped")
	return nil
}

// saveLoop periodically persists the world so a crash never loses more than
// the last interval of edits.
func (s *Server) saveLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.world.Save(); err != nil {
				log.Printf("world save: %v", err)
			}
		}
	}
}

// HandleConnection satisfies mcnet.ConnectionHandler.
func (s *Server) HandleConnection(conn *mcnet.Connection) {
	s.wg.Add(1)
	defer s.wg.Done()
	s.handle(conn)
}

func (s *Server) addPlayer(p *Player) {
	s.mu.Lock()
	s.players[p.UUID] = p
	s.mu.Unlock()
}

func (s *Server) removePlayer(uuid protocol.UUID) {
	s.mu.Lock()
	delete(s.players, uuid)
	s.mu.Unlock()
}

func (s *Server) onlineCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.players)
}

// broadcastChat sends a system chat message to all online players.
func (s *Server) broadcastChat(text string) {
	payload := protocol.EncodeSystemChat(protocol.PlainTextComponent(text), false)
	s.mu.RLock()
	conns := make([]*mcnet.Connection, 0, len(s.players))
	for _, p := range s.players {
		conns = append(conns, p.conn)
	}
	s.mu.RUnlock()
	for _, c := range conns {
		_ = c.WritePacket(protocol.PlayIDSystemChat, payload)
	}
}

// broadcastBlockUpdate sends a single block change to every online player
// (including the initiator), so all clients stay in sync with server state.
func (s *Server) broadcastBlockUpdate(pos protocol.Position, stateID int32) {
	payload := protocol.EncodeBlockUpdate(pos, stateID)
	s.mu.RLock()
	conns := make([]*mcnet.Connection, 0, len(s.players))
	for _, p := range s.players {
		conns = append(conns, p.conn)
	}
	s.mu.RUnlock()
	for _, c := range conns {
		_ = c.WritePacket(protocol.PlayIDBlockUpdate, payload)
	}
}
