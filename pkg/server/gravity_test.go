package server

import "testing"

func TestIsGravityBlock(t *testing.T) {
	gravity := []string{
		"minecraft:sand", "minecraft:red_sand", "minecraft:gravel",
		"minecraft:anvil", "minecraft:chipped_anvil", "minecraft:damaged_anvil",
		"minecraft:dragon_egg",
		// all 16 concrete powder colors are matched by suffix
		"minecraft:white_concrete_powder", "minecraft:black_concrete_powder",
		"minecraft:light_blue_concrete_powder", "minecraft:purple_concrete_powder",
	}
	for _, name := range gravity {
		id := BlockStateID(name)
		if id == 0 {
			t.Errorf("%s: no state id", name)
			continue
		}
		if !isGravityBlock(id) {
			t.Errorf("isGravityBlock(%s) = false, want true", name)
		}
	}
	notGravity := []string{"minecraft:stone", "minecraft:dirt", "minecraft:oak_planks"}
	for _, name := range notGravity {
		if id := BlockStateID(name); id != 0 && isGravityBlock(id) {
			t.Errorf("isGravityBlock(%s) = true, want false", name)
		}
	}
	// fluids are not gravity blocks themselves (they flow, not fall).
	if isGravityBlock(BlockStateID("minecraft:water")) {
		t.Error("water should not be a gravity block")
	}
}
