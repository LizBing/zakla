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

		// signs: standing on ground (rotation faces the placer), wall on a side, reject ceiling
		{"minecraft:oak_sign", 1, 0, 0, "minecraft:oak_sign", "", "rotation=8"},
		{"minecraft:oak_sign", 1, 90, 0, "minecraft:oak_sign", "", "rotation=12"},
		{"minecraft:oak_sign", 2, 0, 0, "minecraft:oak_wall_sign", "north", ""}, // wall, facing the wall dir
		{"minecraft:oak_sign", 0, 0, 0, "", "", ""},                             // ceiling rejected

		// hanging signs: ceiling (under a block) → hanging variant (4-cardinal
		// rotation); side → wall hanging (facing); ground (on top) → rejected
		// (hanging signs have no standing form).
		{"minecraft:oak_hanging_sign", 0, 0, 0, "minecraft:oak_hanging_sign", "", "rotation=8"},   // ceiling, yaw 0 → 8
		{"minecraft:oak_hanging_sign", 0, 90, 0, "minecraft:oak_hanging_sign", "", "rotation=12"}, // ceiling, yaw 90 → snapped to 12
		{"minecraft:oak_hanging_sign", 1, 0, 0, "", "", ""},                                        // ground → rejected
		// hanging sign on a side → wall variant. The board faces the player, which
		// for this block means facing is 90° off the clicked face (along the wall).
		{"minecraft:oak_hanging_sign", 2, 0, 0, "minecraft:oak_wall_hanging_sign", "east", ""},  // north face → facing east
		{"minecraft:oak_hanging_sign", 3, 0, 0, "minecraft:oak_wall_hanging_sign", "west", ""},  // south face → facing west
		{"minecraft:oak_hanging_sign", 4, 0, 0, "minecraft:oak_wall_hanging_sign", "north", ""}, // west face → facing north
		{"minecraft:oak_hanging_sign", 5, 0, 0, "minecraft:oak_wall_hanging_sign", "south", ""}, // east face → facing south
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

func TestFenceConnectProps(t *testing.T) {
	oakFence := BlockStateID("minecraft:oak_fence")
	cobbleWall := BlockStateID("minecraft:cobblestone_wall")
	stone := BlockStateID("minecraft:stone")

	// Build a get(center,N,S,W,E) — neighbors keyed by direction (N=-Z,S=+Z,W=-X,E=+X).
	mkget := func(center, n, so, w, e int32) func(int, int, int) int32 {
		m := map[[3]int]int32{
			{0, 0, 0}: center, {0, 0, -1}: n, {0, 0, 1}: so, {-1, 0, 0}: w, {1, 0, 0}: e,
		}
		return func(x, y, z int) int32 {
			if v, ok := m[[3]int{x, y, z}]; ok {
				return v
			}
			return 0
		}
	}

	// A fence alone in air has no connections.
	if p := connectionProps(mkget(oakFence, 0, 0, 0, 0), 0, 0, 0); len(p) != 0 {
		t.Errorf("isolated fence props = %+v, want empty", p)
	}
	// Stone to the north → north=true (and only north).
	p := connectionProps(mkget(oakFence, stone, 0, 0, 0), 0, 0, 0)
	if p["north"] != "true" || len(p) != 1 {
		t.Errorf("fence+stone-north = %+v, want {north:true}", p)
	}
	// Wall: solid neighbor → low; fence neighbor → tall.
	if p := connectionProps(mkget(cobbleWall, stone, 0, 0, 0), 0, 0, 0); p["north"] != "low" {
		t.Errorf("wall+stone = %+v, want north=low", p)
	}
	if p := connectionProps(mkget(cobbleWall, oakFence, 0, 0, 0), 0, 0, 0); p["north"] != "tall" {
		t.Errorf("wall+fence = %+v, want north=tall", p)
	}
	// A connected fence resolves to a different state than an isolated one.
	iso := ResolveStateID("minecraft:oak_fence", nil)
	conn := ResolveStateID("minecraft:oak_fence", map[string]string{"north": "true"})
	if iso == 0 || conn == 0 || iso == conn {
		t.Errorf("connected fence state should differ (iso=%d conn=%d)", iso, conn)
	}
}

func TestConnectionPropsPanesAndDust(t *testing.T) {
	glassPane := BlockStateID("minecraft:glass_pane")
	ironBars := BlockStateID("minecraft:iron_bars")
	dust := BlockStateID("minecraft:redstone_wire")
	stone := BlockStateID("minecraft:stone")

	// get(center, N, S, W, E): the center block plus its 4 horizontal neighbors.
	mkget := func(center, n, so, w, e int32) func(int, int, int) int32 {
		m := map[[3]int]int32{
			{0, 0, 0}: center, {0, 0, -1}: n, {0, 0, 1}: so, {-1, 0, 0}: w, {1, 0, 0}: e,
		}
		return func(x, y, z int) int32 {
			if v, ok := m[[3]int{x, y, z}]; ok {
				return v
			}
			return 0
		}
	}

	// glass pane connects "true" to a solid or another pane.
	if p := connectionProps(mkget(glassPane, stone, 0, 0, 0), 0, 0, 0); p["north"] != "true" {
		t.Errorf("pane+stone = %+v, want north=true", p)
	}
	if p := connectionProps(mkget(glassPane, glassPane, 0, 0, 0), 0, 0, 0); p["north"] != "true" {
		t.Errorf("pane+pane = %+v, want north=true", p)
	}
	// iron bars likewise.
	if p := connectionProps(mkget(ironBars, stone, 0, 0, 0), 0, 0, 0); p["north"] != "true" {
		t.Errorf("iron bars+stone = %+v, want north=true", p)
	}
	// redstone dust: "side" to another dust, nothing to a plain solid; power=0.
	if p := connectionProps(mkget(dust, dust, 0, 0, 0), 0, 0, 0); p["north"] != "side" || p["power"] != "0" {
		t.Errorf("dust+dust = %+v, want north=side power=0", p)
	}
	p := connectionProps(mkget(dust, stone, 0, 0, 0), 0, 0, 0)
	if _, ok := p["north"]; ok {
		t.Errorf("dust should not connect to stone, got %+v", p)
	}
	if p["power"] != "0" {
		t.Errorf("dust power = %q, want 0", p["power"])
	}
}

func TestSupportOK(t *testing.T) {
	sign := BlockStateID("minecraft:oak_sign")
	stone := BlockStateID("minecraft:stone")
	// signs may stack on another sign, and place on solid
	if !supportOK("minecraft:oak_sign", sign) {
		t.Error("sign on sign should be allowed (stacking)")
	}
	if !supportOK("minecraft:oak_sign", stone) {
		t.Error("sign on stone should be allowed")
	}
	// hanging signs require a SOLID support — they may NOT stack on a sign
	// (regular signs can, but hanging signs cannot attach to non-solid blocks).
	hanging := BlockStateID("minecraft:oak_hanging_sign")
	if supportOK("minecraft:oak_hanging_sign", sign) {
		t.Error("hanging sign on sign should be rejected (needs solid support)")
	}
	if supportOK("minecraft:oak_hanging_sign", hanging) {
		t.Error("hanging sign on hanging sign should be rejected")
	}
	if !supportOK("minecraft:oak_hanging_sign", stone) {
		t.Error("hanging sign on stone should be allowed")
	}
	// hanging signs need a FULL-cube support: thin connectables won't do
	for _, thin := range []string{
		"minecraft:oak_fence",
		"minecraft:cobblestone_wall",
		"minecraft:glass_pane",
		"minecraft:white_stained_glass_pane",
		"minecraft:iron_bars",
		"minecraft:oak_fence_gate",
	} {
		if supportOK("minecraft:oak_hanging_sign", BlockStateID(thin)) {
			t.Errorf("hanging sign on %s should be rejected (not a full-cube support)", thin)
		}
	}
	// a full glass block (not a pane) is still fine
	if !supportOK("minecraft:oak_hanging_sign", BlockStateID("minecraft:glass")) {
		t.Error("hanging sign on glass block should be allowed")
	}
	// the same full-cube rule applies to every attachable: torches (and buttons,
	// ladders, …) can't go on thin connectables either.
	for _, thin := range []string{
		"minecraft:glass_pane", "minecraft:oak_fence", "minecraft:cobblestone_wall", "minecraft:iron_bars",
	} {
		if supportOK("minecraft:torch", BlockStateID(thin)) {
			t.Errorf("torch on %s should be rejected (not a full-cube support)", thin)
		}
	}
	if !supportOK("minecraft:torch", stone) {
		t.Error("torch on stone should be allowed")
	}
	// buttons/torches need a solid support — a sign is NOT solid for them
	if supportOK("minecraft:stone_button", sign) {
		t.Error("button on sign should be rejected")
	}
	if supportOK("minecraft:torch", sign) {
		t.Error("torch on sign should be rejected")
	}
	if !supportOK("minecraft:stone_button", stone) {
		t.Error("button on stone should be allowed")
	}
	// air is never a valid support
	if supportOK("minecraft:oak_sign", 0) || supportOK("minecraft:stone_button", 0) {
		t.Error("air should not be a valid support")
	}
}

func TestSignHelpers(t *testing.T) {
	if !isRegularSign("minecraft:oak_sign") {
		t.Error("oak_sign should be a regular sign")
	}
	if isRegularSign("minecraft:oak_hanging_sign") {
		t.Error("hanging sign should not be treated as regular sign")
	}
	if wallSignVariant("minecraft:oak_sign") != "minecraft:oak_wall_sign" {
		t.Errorf("wallSignVariant(oak_sign) = %q", wallSignVariant("minecraft:oak_sign"))
	}
	// rotation is 180° (+8 steps) off the raw look (sign faces the placer):
	// yaw 0→8, 90→12, 180→0, 270→4, -22.5→7
	for yaw, want := range map[float32]string{0: "8", 90: "12", 180: "0", 270: "4", -22.5: "7"} {
		if got := signRotation(yaw); got != want {
			t.Errorf("signRotation(%g) = %q, want %q", yaw, got, want)
		}
	}
	// signRotation4 snaps to the 4 cardinals (0/4/8/12): yaw 0→8, 45→12, 90→12, 180→0, -45→8
	for yaw, want := range map[float32]string{0: "8", 45: "12", 90: "12", 180: "0", 270: "4", -45: "8"} {
		if got := signRotation4(yaw); got != want {
			t.Errorf("signRotation4(%g) = %q, want %q", yaw, got, want)
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
