package server

import "sync"

// chunkKey identifies one chunk column.
type chunkKey struct{ x, z int32 }

// World is the in-memory world: a sparse map of chunk columns. Block state is
// owned by the server so it persists across player reconnects and is broadcast
// to every connected player.
type World struct {
	mu     sync.RWMutex
	chunks map[chunkKey]*Chunk
}

// NewWorld returns an empty world.
func NewWorld() *World {
	return &World{chunks: make(map[chunkKey]*Chunk)}
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
		w.chunks[key] = c
	}
	return c
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
// owning chunk if necessary.
func (w *World) SetBlock(x, y, z int, stateID int32) {
	cx, cz := int32(x>>4), int32(z>>4)
	c := w.GetOrCreateChunk(cx, cz)
	c.SetBlock(x, y, z, stateID)
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

// fillSpawnPlatform fills chunk (0,0) with a small stone/grass platform so
// players spawn on solid ground they can mine.
func (w *World) fillSpawnPlatform() {
	grass := BlockStateID("minecraft:grass_block")
	stone := BlockStateID("minecraft:stone")
	for x := 0; x < chunkBlockExtent; x++ {
		for z := 0; z < chunkBlockExtent; z++ {
			for y := 59; y < 63; y++ {
				w.SetBlock(x, y, z, stone)
			}
			w.SetBlock(x, 63, z, grass)
		}
	}
}
