package server

import "testing"

// TestWorldSaveLoadRoundTrip verifies the gob persistence round-trip: blocks
// written before Save are read back identically after Load, and untouched
// cells stay air. Uses a temp dir so it doesn't depend on a real world file.
func TestWorldSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	w := NewWorld(dir)
	w.SetBlock(1, 63, 1, BlockStateID("minecraft:grass_block")) // 9
	w.SetBlock(2, 63, 2, BlockStateID("minecraft:stone"))       // 1
	if err := w.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	w2, err := LoadWorld(dir)
	if err != nil {
		t.Fatalf("LoadWorld: %v", err)
	}
	if got := w2.GetBlock(1, 63, 1); got != 9 {
		t.Errorf("(1,63,1) = %d, want 9 (grass)", got)
	}
	if got := w2.GetBlock(2, 63, 2); got != 1 {
		t.Errorf("(2,63,2) = %d, want 1 (stone)", got)
	}
	if got := w2.GetBlock(5, 63, 5); got != 0 {
		t.Errorf("untouched (5,63,5) = %d, want 0 (air)", got)
	}
	if w2.ChunkCount() != w.ChunkCount() {
		t.Errorf("chunk count = %d, want %d", w2.ChunkCount(), w.ChunkCount())
	}
}

// TestWorldInMemoryNoSave ensures an in-memory world (path="") is a no-op for
// Save and doesn't create files.
func TestWorldInMemoryNoSave(t *testing.T) {
	w := NewWorld("")
	w.SetBlock(0, 63, 0, 1)
	if err := w.Save(); err != nil {
		t.Errorf("Save on in-memory world: %v", err)
	}
}
