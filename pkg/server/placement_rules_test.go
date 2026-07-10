package server

import "testing"

func TestHorizontalFacing(t *testing.T) {
	cases := []struct {
		yaw  float32
		want string
	}{
		{0, "south"}, {44, "south"}, {45, "west"}, {-45, "south"},
		{90, "west"}, {134, "west"}, {135, "north"},
		{180, "north"}, {224, "north"}, {225, "east"},
		{270, "east"}, {314, "east"}, {315, "south"},
		{-90, "east"}, {360, "south"},
	}
	for _, c := range cases {
		if got := horizontalFacing(c.yaw); got != c.want {
			t.Errorf("horizontalFacing(%g) = %q, want %q", c.yaw, got, c.want)
		}
	}
}

func TestFaceToDirection(t *testing.T) {
	want := map[int32]string{0: "down", 1: "up", 2: "north", 3: "south", 4: "west", 5: "east"}
	for f, w := range want {
		if got := faceToDirection(f); got != w {
			t.Errorf("faceToDirection(%d) = %q, want %q", f, got, w)
		}
		if opp := oppositeDir(w); opp == w {
			t.Errorf("oppositeDir(%q) did not flip", w)
		}
	}
}

func TestHorizontalFacingPerp(t *testing.T) {
	cases := map[string]string{
		"south": "west", "west": "north", "north": "east", "east": "south",
	}
	for in, want := range cases {
		if got := horizontalFacingPerp(in); got != want {
			t.Errorf("horizontalFacingPerp(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNeedsSolidSupport(t *testing.T) {
	need := []string{"minecraft:stone_button", "minecraft:oak_button", "minecraft:lever",
		"minecraft:redstone_wire", "minecraft:torch", "minecraft:wall_torch",
		"minecraft:repeater", "minecraft:rail", "minecraft:stone_pressure_plate"}
	for _, n := range need {
		if !needsSolidSupport(n) {
			t.Errorf("needsSolidSupport(%q) = false, want true", n)
		}
	}
	notNeed := []string{"minecraft:stone", "minecraft:oak_fence", "minecraft:chest", "minecraft:anvil"}
	for _, n := range notNeed {
		if needsSolidSupport(n) {
			t.Errorf("needsSolidSupport(%q) = true, want false", n)
		}
	}
}

func TestIsSolidBlock(t *testing.T) {
	// solid
	if !isSolidBlock(BlockStateID("minecraft:stone")) {
		t.Error("stone should be solid")
	}
	// not solid
	if isSolidBlock(0) {
		t.Error("air should not be solid")
	}
	if isSolidBlock(BlockStateID("minecraft:water")) {
		t.Error("water should not be solid")
	}
	if isSolidBlock(BlockStateID("minecraft:redstone_wire")) {
		t.Error("redstone_wire should not be solid")
	}
}

func TestVerticalHalf(t *testing.T) {
	cases := []struct {
		face    int32
		cursorY float32
		want    string
	}{
		{1, 0, "bottom"},  // top face → bottom half
		{0, 0, "top"},     // bottom face → top half
		{2, 0.4, "bottom"}, // side, lower click → bottom
		{2, 0.6, "top"},    // side, upper click → top
	}
	for _, c := range cases {
		if got := verticalHalf(c.face, c.cursorY); got != c.want {
			t.Errorf("verticalHalf(%d,%g) = %q, want %q", c.face, c.cursorY, got, c.want)
		}
	}
}

func TestResolveStateID(t *testing.T) {
	// Regression: blocks without orientation still resolve to their default id.
	if got := ResolveStateID("minecraft:grass_block", nil); got != 9 {
		t.Errorf("grass_block = %d, want 9", got)
	}
	if got := ResolveStateID("minecraft:stone", nil); got != 1 {
		t.Errorf("stone = %d, want 1", got)
	}

	// oak_stairs default = facing=north,half=bottom,shape=straight,waterlogged=false (id 3918).
	const stairsDefault int32 = 3918
	if got := ResolveStateID("minecraft:oak_stairs", map[string]string{"facing": "north"}); got != stairsDefault {
		t.Errorf("oak_stairs facing=north = %d, want default %d (other props should inherit)", got, stairsDefault)
	}
	// Overriding facing=south must yield a DIFFERENT, valid state.
	south := ResolveStateID("minecraft:oak_stairs", map[string]string{"facing": "south", "half": "bottom"})
	if south == 0 || south == stairsDefault {
		t.Errorf("oak_stairs facing=south = %d, want a non-zero id different from default %d", south, stairsDefault)
	}
	// Overriding only half=top (facing stays default north) must differ from default.
	top := ResolveStateID("minecraft:oak_stairs", map[string]string{"half": "top"})
	if top == 0 || top == stairsDefault {
		t.Errorf("oak_stairs half=top = %d, want non-zero != default", top)
	}
}

func TestResolvePlacement(t *testing.T) {
	cases := []struct {
		name     string
		face     int32
		yaw      float32
		cursorY  float32
		wantName string
		wantFace string // expected props["facing"], "" if none
		wantKey  string // one other expected prop value, format "k=v"
	}{
		// torch: floor vs wall vs ceiling
		{"minecraft:torch", 1, 0, 0, "minecraft:torch", "", ""},
		{"minecraft:torch", 2, 0, 0, "minecraft:wall_torch", "north", ""},
		{"minecraft:torch", 5, 0, 0, "minecraft:wall_torch", "east", ""},
		{"minecraft:torch", 0, 0, 0, "", "", ""}, // ceiling: rejected
		{"minecraft:redstone_torch", 3, 0, 0, "minecraft:redstone_wall_torch", "south", ""},

		// stairs: facing=yaw(0=south), half=bottom on top face
		{"minecraft:oak_stairs", 1, 0, 0, "minecraft:oak_stairs", "south", "half=bottom"},
		{"minecraft:oak_stairs", 0, 180, 0, "minecraft:oak_stairs", "north", "half=top"},
		{"minecraft:oak_stairs", 2, 0, 0.7, "minecraft:oak_stairs", "south", "half=top"},

		// button: floor uses player yaw, wall uses clicked face dir
		{"minecraft:stone_button", 1, 90, 0, "minecraft:stone_button", "west", "face=floor"},
		{"minecraft:stone_button", 5, 0, 0, "minecraft:stone_button", "east", "face=wall"},
		{"minecraft:stone_button", 0, 0, 0, "minecraft:stone_button", "south", "face=ceiling"},

		// lever
		{"minecraft:lever", 5, 0, 0, "minecraft:lever", "east", "face=wall"},

		// slab type from face
		{"minecraft:stone_slab", 1, 0, 0, "minecraft:stone_slab", "", "type=bottom"},
		{"minecraft:stone_slab", 0, 0, 0, "minecraft:stone_slab", "", "type=top"},

		// fence/wall: standalone post (no props)
		{"minecraft:oak_fence", 1, 0, 0, "minecraft:oak_fence", "", ""},
		{"minecraft:cobblestone_wall", 1, 0, 0, "minecraft:cobblestone_wall", "", ""},

		// fence gate: facing=yaw
		{"minecraft:oak_fence_gate", 1, 0, 0, "minecraft:oak_fence_gate", "south", ""},

		// face-based blocks: piston/dispenser face the clicked direction; observer opposite
		{"minecraft:piston", 1, 0, 0, "minecraft:piston", "up", ""},
		{"minecraft:dispenser", 5, 0, 0, "minecraft:dispenser", "east", ""},
		{"minecraft:observer", 1, 0, 0, "minecraft:observer", "down", ""},

		// horizontal: chest/furnace front faces the player (opposite of look dir)
		{"minecraft:chest", 1, 0, 0, "minecraft:chest", "north", ""},   // look south → front north
		{"minecraft:chest", 1, 180, 0, "minecraft:chest", "south", ""}, // look north → front south

		// anvil: facing is perpendicular to look (long axis ⟂ player)
		{"minecraft:anvil", 1, 0, 0, "minecraft:anvil", "west", ""},   // look south → perp west
		{"minecraft:anvil", 1, 90, 0, "minecraft:anvil", "north", ""}, // look west → perp north

		// uninteresting block: default, no props
		{"minecraft:stone", 1, 0, 0, "minecraft:stone", "", ""},

		// ladder: facing = the clicked wall direction
		{"minecraft:ladder", 2, 0, 0, "minecraft:ladder", "north", ""}, // north face → facing north
		{"minecraft:ladder", 5, 0, 0, "minecraft:ladder", "east", ""},  // east face → facing east
	}
	for _, c := range cases {
		name, props := resolvePlacement(c.name, c.face, c.yaw, c.cursorY)
		if name != c.wantName {
			t.Errorf("resolvePlacement(%s) name = %q, want %q", c.name, name, c.wantName)
			continue
		}
		if name == "" {
			continue
		}
		if c.wantFace != "" && props["facing"] != c.wantFace {
			t.Errorf("resolvePlacement(%s) facing = %q, want %q", c.name, props["facing"], c.wantFace)
		}
		if c.wantKey != "" {
			// wantKey is "k=v"; assert that property matches.
			for k, v := range props {
				if k+"="+v == c.wantKey {
					goto keyOK
				}
			}
			t.Errorf("resolvePlacement(%s) no prop %q (got %+v)", c.name, c.wantKey, props)
		keyOK:
		}
	}
}

// TestResolvePlacementsDoor checks doors produce a 2-tall lower+upper pair.
func TestResolvePlacementsDoor(t *testing.T) {
	ps := resolvePlacements("minecraft:oak_door", 1, 0, 0) // face=top, yaw=0(south)
	if len(ps) != 2 {
		t.Fatalf("door placements = %d, want 2", len(ps))
	}
	if ps[0].dy != 0 || ps[0].props["half"] != "lower" {
		t.Errorf("lower = %+v, want dy=0 half=lower", ps[0])
	}
	if ps[1].dy != 1 || ps[1].props["half"] != "upper" {
		t.Errorf("upper = %+v, want dy=1 half=upper", ps[1])
	}
	// both halves should resolve to real states.
	for _, p := range ps {
		if ResolveStateID(p.name, p.props) == 0 {
			t.Errorf("door half %+v resolved to state 0", p.props)
		}
	}
	// a single-block placement still returns exactly one entry at (0,0,0).
	single := resolvePlacements("minecraft:stone", 1, 0, 0)
	if len(single) != 1 || single[0].dy != 0 {
		t.Errorf("stone placements = %+v, want one entry at dy=0", single)
	}
	// torch on a ceiling is rejected.
	if got := resolvePlacements("minecraft:torch", 0, 0, 0); len(got) != 0 {
		t.Errorf("ceiling torch = %+v, want empty", got)
	}
}

// TestResolvePlacementsBed checks beds produce a foot+head pair along facing.
func TestResolvePlacementsBed(t *testing.T) {
	ps := resolvePlacements("minecraft:red_bed", 1, 0, 0) // face=top, yaw=0(south)
	if len(ps) != 2 {
		t.Fatalf("bed placements = %d, want 2", len(ps))
	}
	if ps[0].props["part"] != "foot" {
		t.Errorf("first = %+v, want part=foot", ps[0].props)
	}
	if ps[1].props["part"] != "head" {
		t.Errorf("second = %+v, want part=head", ps[1].props)
	}
	// facing south → head offset (0,0,+1)
	if ps[1].dx != 0 || ps[1].dz != 1 {
		t.Errorf("head offset = (%d,%d), want (0,1) for facing south", ps[1].dx, ps[1].dz)
	}
	for _, p := range ps {
		if ResolveStateID(p.name, p.props) == 0 {
			t.Errorf("bed part %+v resolved to state 0", p.props)
		}
	}
}

func TestMultiBlockBreakOffsets(t *testing.T) {
	if got := multiBlockBreakOffsets("minecraft:oak_door"); len(got) != 2 {
		t.Errorf("door offsets = %d, want 2", len(got))
	}
	if got := multiBlockBreakOffsets("minecraft:red_bed"); len(got) != 4 {
		t.Errorf("bed offsets = %d, want 4", len(got))
	}
	if got := multiBlockBreakOffsets("minecraft:stone"); got != nil {
		t.Errorf("stone offsets = %v, want nil", got)
	}
}

// TestResolvePlacementProducesValidStates runs each rule family through the
// full resolvePlacement + ResolveStateID pipeline and asserts a non-zero state
// id (i.e. the chosen properties actually correspond to a real block state).
func TestResolvePlacementProducesValidStates(t *testing.T) {
	cases := []struct {
		name string
		face int32
		yaw  float32
	}{
		{"minecraft:oak_stairs", 1, 0},
		{"minecraft:stone_button", 5, 0},
		{"minecraft:lever", 1, 90},
		{"minecraft:stone_slab", 0, 0},
		{"minecraft:torch", 1, 0},
		{"minecraft:torch", 3, 0},
		{"minecraft:oak_fence", 1, 0},
		{"minecraft:oak_fence_gate", 1, 0},
		{"minecraft:piston", 1, 0},
		{"minecraft:sticky_piston", 2, 0},
		{"minecraft:observer", 5, 0},
		{"minecraft:dispenser", 3, 0},
		{"minecraft:dropper", 1, 0},
		{"minecraft:hopper", 4, 0},
		{"minecraft:chest", 1, 270},
		{"minecraft:furnace", 1, 90},
		{"minecraft:repeater", 1, 180},
		{"minecraft:chipped_anvil", 1, 0},
		{"minecraft:carved_pumpkin", 1, 45},
	}
	for _, c := range cases {
		name, props := resolvePlacement(c.name, c.face, c.yaw, 0.3)
		if name == "" {
			t.Errorf("%s: resolved to empty name", c.name)
			continue
		}
		id := ResolveStateID(name, props)
		if id == 0 {
			t.Errorf("%s → %s %+v: ResolveStateID returned 0 (no matching state)", c.name, name, props)
		}
	}
}
