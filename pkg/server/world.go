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
// persisted to disk via gob. Block state is owned by the server so it survives
// player reconnects and (with Save) server restarts.
type World struct {
	mu     sync.RWMutex
	chunks map[chunkKey]*Chunk
	path   string // persistence directory; "" = in-memory only
	dirty  bool   // set on any SetBlock, cleared on Save
}

// NewWorld returns an empty world rooted at path (used for persistence; "" is
// purely in-memory).
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

// GetOrCreateChunk returns the chunk at (cx, cz), creating an all-air chunk if
// it does not yet exist. The returned chunk is safe to mutate while holding no
// other lock (block edits go through SetBlock, which is mutex-guarded at the
// world level).
func (w *World) GetOrCreateChunk(cx, cz int32) *Chunk {
	key := chunkKey{x: cx, z: cz}
	w.mu.Lock()
	defer w.mu.Unlock()
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

// chunkOf returns the chunk owning world coordinate x,z, or nil.
func (w *World) chunkOf(x, z int) *Chunk {
	key := chunkKey{x: int32(x >> 4), z: int32(z >> 4)}
	w.mu.RLock()
	c := w.chunks[key]
	w.mu.RUnlock()
	return c
}

// SetBlock sets the block state at world coordinates (x, y, z), allocating the
// owning chunk if necessary, and marks the world dirty for the next save.
func (w *World) SetBlock(x, y, z int, stateID int32) {
	cx, cz := int32(x>>4), int32(z>>4)
	c := w.GetOrCreateChunk(cx, cz)
	c.SetBlock(x, y, z, stateID)
	w.mu.Lock()
	w.dirty = true
	w.mu.Unlock()
}

// GetBlock returns the block state at world coordinates (x, y, z), or air (0)
// if the owning chunk is not loaded.
func (w *World) GetBlock(x, y, z int) int32 {
	c := w.chunkOf(x, z)
	if c == nil {
		return 0
	}
	return c.GetBlock(x, y, z)
}
