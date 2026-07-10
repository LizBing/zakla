package server

import (
	"encoding/gob"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// chunkKey identifies one chunk column.
type chunkKey struct{ x, z int32 }

// World is the in-memory world: a sparse map of chunk columns, optionally
// persisted to disk via gob, plus a minimal tick engine (gravity MVP).
type World struct {
	mu     sync.RWMutex
	chunks map[chunkKey]*Chunk
	path   string // persistence directory; "" = in-memory only
	dirty  bool   // set on any SetBlock, cleared on Save

	// tick engine
	currentTick int
	blockTicks  []scheduledTick // scheduled block ticks (gravity)
}

// NewWorld returns an empty world rooted at path ("" = purely in-memory).
func NewWorld(path string) *World {
	return &World{chunks: make(map[chunkKey]*Chunk), path: path}
}

// ChunkCount returns the number of loaded chunk columns.
func (w *World) ChunkCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.chunks)
}

// --- on-disk format (gob) ---
// Chunk/ChunkSection hold unexported fields that gob skips, so we round-trip
// through these exported mirror structs.

type chunkData struct {
	X, Z     int32
	Sections []sectionEntry // only non-empty sections (gob can't encode nil array elems)
}

type sectionEntry struct {
	Index  int
	States [4096]int32
	Biomes [64]int32
}

func (c *Chunk) toData() chunkData {
	d := chunkData{X: c.X, Z: c.Z}
	for i, s := range c.sections {
		if s != nil {
			d.Sections = append(d.Sections, sectionEntry{Index: i, States: s.states, Biomes: s.biomes})
		}
	}
	return d
}

func chunkFromData(d chunkData) *Chunk {
	c := NewChunk(d.X, d.Z)
	for _, e := range d.Sections {
		c.sections[e.Index] = &ChunkSection{states: e.States, biomes: e.Biomes}
	}
	return c
}

// LoadWorld loads chunks from <path>/world.gob, or returns an empty world if the
// file does not yet exist (new world). The directory is created if needed.
func LoadWorld(path string) (*World, error) {
	w := NewWorld(path)
	if path == "" {
		return w, nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(path, "world.gob"))
	if err != nil {
		if os.IsNotExist(err) {
			return w, nil // new world
		}
		return nil, err
	}
	defer f.Close()
	var data []chunkData
	if err := gob.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}
	w.mu.Lock()
	for _, d := range data {
		c := chunkFromData(d)
		w.chunks[chunkKey{c.X, c.Z}] = c
	}
	w.mu.Unlock()
	log.Printf("world: loaded %d chunks from %s", len(data), path)
	return w, nil
}

// Save writes all chunks to <path>/world.gob atomically (tmp file + rename) so
// a crash mid-save cannot corrupt the world. No-op for in-memory worlds.
func (w *World) Save() error {
	if w.path == "" {
		return nil
	}
	w.mu.RLock()
	data := make([]chunkData, 0, len(w.chunks))
	for _, c := range w.chunks {
		data = append(data, c.toData())
	}
	w.mu.RUnlock()

	tmp := filepath.Join(w.path, "world.gob.tmp")
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	w.mu.Lock()
	w.dirty = false
	w.mu.Unlock()
	return os.Rename(tmp, filepath.Join(w.path, "world.gob"))
}

// GetOrCreateChunk returns the chunk at (cx, cz), generating terrain if it does
// not yet exist.
func (w *World) GetOrCreateChunk(cx, cz int32) *Chunk {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.getOrCreateChunkLocked(cx, cz)
}

func (w *World) getOrCreateChunkLocked(cx, cz int32) *Chunk {
	key := chunkKey{x: cx, z: cz}
	c := w.chunks[key]
	if c == nil {
		c = NewChunk(cx, cz)
		fillFloor(c) // worldgen MVP: flat grass-on-stone so there's always ground
		w.chunks[key] = c
		w.dirty = true
	}
	return c
}

// fillFloor generates a flat floor in a chunk: grass at Y=63, stone Y=59-62.
func fillFloor(c *Chunk) {
	grass := BlockStateID("minecraft:grass_block")
	stone := BlockStateID("minecraft:stone")
	for x := 0; x < chunkBlockExtent; x++ {
		for z := 0; z < chunkBlockExtent; z++ {
			for y := 59; y < 63; y++ {
				c.SetBlock(x, y, z, stone)
			}
			c.SetBlock(x, 63, z, grass)
		}
	}
}

// SetBlock sets the block state at world coordinates (x, y, z), allocating the
// owning chunk if necessary, and triggers gravity checks for the edited cell
// and the one above it.
func (w *World) SetBlock(x, y, z int, stateID int32) {
	w.mu.Lock()
	w.setBlockLocked(x, y, z, stateID)
	w.mu.Unlock()
}

// setBlockLocked sets a block and triggers gravity checks. Caller holds w.mu.
func (w *World) setBlockLocked(x, y, z int, stateID int32) {
	c := w.getOrCreateChunkLocked(int32(x>>4), int32(z>>4))
	c.SetBlock(x, y, z, stateID)
	w.dirty = true
	w.scheduleGravityLocked(x, y, z)
	w.scheduleFluidLocked(x, y, z)
}

// GetBlock returns the block state at world coordinates (x, y, z), or air (0)
// if the owning chunk is not loaded.
func (w *World) GetBlock(x, y, z int) int32 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.getBlockLocked(x, y, z)
}

func (w *World) getBlockLocked(x, y, z int) int32 {
	c := w.chunks[chunkKey{int32(x >> 4), int32(z >> 4)}]
	if c == nil {
		return 0
	}
	return c.GetBlock(x, y, z)
}

// --- tick engine (gravity MVP) ---

// BlockChange is one block update produced by the tick engine (broadcast to
// clients).
type BlockChange struct {
	X, Y, Z int
	StateID int32
}

type scheduledTick struct {
	x, y, z int
	atTick  int
	kind    tickKind
}

type tickKind uint8

const (
	kindGravity tickKind = iota
	kindFluid
)

const (
	gravityTickDelay = 2   // ticks before a gravity block falls one cell
	fluidTickDelay   = 5   // water flows ~1 block / 5 ticks
	lavaFluidDelay   = 30  // lava in the Overworld flows ~1 block / 30 ticks
	waterBaseState   = 86  // minecraft:water[level=0]
	lavaBaseState    = 102 // minecraft:lava[level=0]
)

// gravityBlockNames lists blocks affected by gravity (MVP: the common ones).
var gravityBlockNames = map[string]bool{
	"minecraft:sand":          true,
	"minecraft:red_sand":      true,
	"minecraft:gravel":        true,
	"minecraft:anvil":         true,
	"minecraft:chipped_anvil": true,
	"minecraft:damaged_anvil": true,
	"minecraft:dragon_egg":    true,
}

func isGravityBlock(stateID int32) bool {
	name, ok := blockStateName[stateID]
	return ok && gravityBlockNames[name]
}

// isReplaceable reports whether a falling block can move into a cell. Gravity
// blocks sink through air AND fluids (water/lava are non-solid); they stop on
// any solid block.
func isReplaceable(stateID int32) bool {
	return stateID == 0 || isFluid(stateID)
}

// scheduleGravityLocked schedules a fall tick for a gravity block at (x,y,z)
// if it is now unsupported, and for the block above if it may have lost support.
// Caller holds w.mu.
func (w *World) scheduleGravityLocked(x, y, z int) {
	if isGravityBlock(w.getBlockLocked(x, y, z)) && y > minWorldY && isReplaceable(w.getBlockLocked(x, y-1, z)) {
		w.scheduleBlockTick(x, y, z, gravityTickDelay, kindGravity)
	}
	if isGravityBlock(w.getBlockLocked(x, y+1, z)) && isReplaceable(w.getBlockLocked(x, y, z)) {
		w.scheduleBlockTick(x, y+1, z, gravityTickDelay, kindGravity)
	}
}

func (w *World) scheduleBlockTick(x, y, z, delay int, kind tickKind) {
	w.blockTicks = append(w.blockTicks, scheduledTick{x, y, z, w.currentTick + delay, kind})
}

// Tick advances the world by one game tick, running scheduled block ticks
// (gravity), and returns the block changes to broadcast to clients.
func (w *World) Tick() []BlockChange {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.currentTick++
	if len(w.blockTicks) == 0 {
		return nil
	}
	// Snapshot the due list and rebuild w.blockTicks from scratch: applyGravity
	// appends reschedules to w.blockTicks, which would alias a [:0] slice.
	due := w.blockTicks
	w.blockTicks = nil
	var changes []BlockChange
	for _, t := range due {
		if t.atTick > w.currentTick {
			w.blockTicks = append(w.blockTicks, t) // not due yet, keep
			continue
		}
		switch t.kind {
		case kindGravity:
			changes = append(changes, w.applyGravityLocked(t)...)
		case kindFluid:
			changes = append(changes, w.applyFluidLocked(t)...)
		}
	}
	return changes
}

// applyGravityLocked moves a gravity block down one cell if it is unsupported.
// Caller holds w.mu. Re-scheduling via setBlockLocked keeps it falling.
func (w *World) applyGravityLocked(t scheduledTick) []BlockChange {
	state := w.getBlockLocked(t.x, t.y, t.z)
	if !isGravityBlock(state) {
		return nil
	}
	if t.y <= minWorldY {
		return nil
	}
	below := w.getBlockLocked(t.x, t.y-1, t.z)
	if !isReplaceable(below) {
		return nil
	}
	// Snap down one cell (no falling_block entity — the client accepts this).
	// SWAP with the cell below rather than clearing it to air: when sinking
	// through water/lava the fluid bubbles back up on top instead of being
	// deleted, so the fluid column survives and the block keeps falling.
	w.setBlockLocked(t.x, t.y, t.z, below)
	w.setBlockLocked(t.x, t.y-1, t.z, state)
	return []BlockChange{
		{t.x, t.y, t.z, below},
		{t.x, t.y - 1, t.z, state},
	}
}

// --- fluid (water flow) ---

// fluidLevel returns (base state id, level, ok) for water/lava, else not ok.
func fluidLevel(stateID int32) (base int32, level int, ok bool) {
	if stateID >= waterBaseState && stateID <= waterBaseState+15 {
		return waterBaseState, int(stateID - waterBaseState), true
	}
	if stateID >= lavaBaseState && stateID <= lavaBaseState+15 {
		return lavaBaseState, int(stateID - lavaBaseState), true
	}
	return 0, -1, false
}

func isFluid(stateID int32) bool { _, _, ok := fluidLevel(stateID); return ok }

// scheduleFluidLocked schedules a fluid tick for a water/lava block at (x,y,z),
// and — if that cell was just cleared (e.g. dug out) — wakes the fluid above it
// so it can now flow down. Caller holds w.mu.
func (w *World) scheduleFluidLocked(x, y, z int) {
	schedule := func(fx, fy, fz int) {
		base, _, ok := fluidLevel(w.getBlockLocked(fx, fy, fz))
		if !ok {
			return
		}
		delay := fluidTickDelay
		if base == lavaBaseState {
			delay = lavaFluidDelay // lava spreads much slower in the Overworld
		}
		w.scheduleBlockTick(fx, fy, fz, delay, kindFluid)
	}
	schedule(x, y, z)
	if w.getBlockLocked(x, y, z) == 0 { // cell cleared: the fluid above may now fall
		schedule(x, y+1, z)
	}
}

// applyFluidLocked advances a fluid (water/lava) one step. Block levels 0-7 are
// spreading flows (0 = source); levels 8-15 are "falling" streams whose sideways
// spread level is (level - 8). The rules:
//
//   - A falling stream keeps falling through air (carrying its spread level).
//     When it lands on a solid block it converts into a spreading flow, so a
//     waterfall forms a puddle at the bottom. Mid-stream (same fluid below) it
//     stays put.
//   - A source (level 0) always spreads one ring, and falls if air is below.
//   - A supported flow (level 1-7, solid/fluid below) spreads a flat puddle.
//   - A flow over air falls, but does not spread sideways — that keeps a
//     waterfall a thin column instead of flooding outward level by level.
//
// Caller holds w.mu.
func (w *World) applyFluidLocked(t scheduledTick) []BlockChange {
	base, level, ok := fluidLevel(w.getBlockLocked(t.x, t.y, t.z))
	if !ok {
		return nil
	}
	flowState := func(lvl int) int32 { return base + int32(lvl) }
	var changes []BlockChange

	// Falling stream: keep falling, or land into a spreading puddle.
	if level >= 8 {
		if t.y <= minWorldY {
			return nil
		}
		spread := level - 8
		below := w.getBlockLocked(t.x, t.y-1, t.z)
		switch {
		case below == 0: // air — keep falling, carry the spread level down
			w.setBlockLocked(t.x, t.y-1, t.z, flowState(8+spread))
			changes = append(changes, BlockChange{t.x, t.y - 1, t.z, flowState(8 + spread)})
		case !isFluid(below): // solid — land and become a spreading flow (puddle)
			if spread+1 <= 7 {
				w.setBlockLocked(t.x, t.y, t.z, flowState(spread+1))
				changes = append(changes, BlockChange{t.x, t.y, t.z, flowState(spread + 1)})
			}
		}
		// below is the same fluid → mid-stream; leave it.
		return changes
	}

	// Source (level 0) or spreading flow (level 1-7).
	belowAir := t.y > minWorldY && w.getBlockLocked(t.x, t.y-1, t.z) == 0
	if belowAir {
		// Fall as a stream, carrying the current spread level.
		w.setBlockLocked(t.x, t.y-1, t.z, flowState(8+level))
		changes = append(changes, BlockChange{t.x, t.y - 1, t.z, flowState(8 + level)})
	}
	// Spread sideways: a source always spreads (pours off a pillar); a supported
	// flow spreads (flat puddle). A flow that is itself falling does not.
	if (level == 0 || !belowAir) && level < 7 {
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, nz := t.x+d[0], t.z+d[1]
			if w.getBlockLocked(nx, t.y, nz) == 0 {
				w.setBlockLocked(nx, t.y, nz, flowState(level+1))
				changes = append(changes, BlockChange{nx, t.y, nz, flowState(level + 1)})
			}
		}
	}
	return changes
}
