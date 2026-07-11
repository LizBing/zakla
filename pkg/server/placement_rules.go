package server

import (
	"math"
	"strconv"
	"strings"
)

// Direction constants for the six block faces (matching the clicked-face enum
// 0=-Y,1=+Y,2=-Z,3=+Z,4=-X,5=+X used by UseItemOn / FaceOffset).
const (
	faceDown  = int32(0)
	faceUp    = int32(1)
	faceNorth = int32(2)
	faceSouth = int32(3)
	faceWest  = int32(4)
	faceEast  = int32(5)
)

// ruleKind classifies a block by how its placement orientation is derived.
type ruleKind uint8

const (
	ruleDefault ruleKind = iota // no orientation; place default state
	ruleStair                   // facing=look, half=clicked face
	ruleDoor                    // doors: 2-tall, lower+upper halves
	ruleBed                     // beds: 2-wide, foot+head along facing
	ruleButtonLever             // buttons + lever: face + facing
	ruleSlab                    // type=clicked face
	ruleTorch                   // torch/redstone_torch/soul_torch: floor vs wall
	ruleSign                    // signs: standing (ground) vs wall variant
	ruleHangingSign             // hanging signs: ceiling/ground vs wall variant
	ruleFencePost               // fence/wall: standalone post (no neighbor connect)
	ruleLook                    // stairs/fence_gate: facing = player look dir
	ruleAnvil                   // anvil: facing ⟂ look (its long axis is perpendicular to the player)
	ruleFront                   // chest/furnace/pumpkin/repeater/comparator: facing = opposite of look
	ruleFaceBlock               // piston/dispenser/dropper: facing = clicked face direction
	ruleOppositeFace            // observer/hopper: facing = opposite of clicked face
)

// torchWallVariant maps a floor torch item to its wall-mounted block id.
var torchWallVariant = map[string]string{
	"minecraft:torch":          "minecraft:wall_torch",
	"minecraft:redstone_torch": "minecraft:redstone_wall_torch",
	"minecraft:soul_torch":     "minecraft:soul_wall_torch",
}

// isRegularSign reports whether a name is a (non-hanging) sign item/block that
// dispatches between standing and wall variants.
func isRegularSign(name string) bool {
	return strings.HasSuffix(name, "_sign") && !strings.HasSuffix(name, "_hanging_sign")
}

// wallSignVariant maps a standing sign name to its wall variant
// (oak_sign → oak_wall_sign).
func wallSignVariant(name string) string {
	return strings.Replace(name, "_sign", "_wall_sign", 1)
}

// wallHangingSignVariant maps a hanging sign name to its wall variant
// (oak_hanging_sign → oak_wall_hanging_sign).
func wallHangingSignVariant(name string) string {
	return strings.Replace(name, "_hanging_sign", "_wall_hanging_sign", 1)
}

// signRotation maps a player yaw to a standing/hanging sign rotation (0-15),
// one step per 22.5°. Per minecraft.wiki the sign faces TOWARD the placer, so
// this is 180° (8 steps) off the raw look direction.
func signRotation(yaw float32) string {
	r := int(math.Floor(float64(yaw)/22.5+0.5)) + 8
	for r < 0 {
		r += 16
	}
	for r >= 16 {
		r -= 16
	}
	return strconv.Itoa(r)
}

// signRotation4 maps a player yaw to one of the 4 cardinal hanging-sign
// rotations (0/4/8/12), snapped to the nearest, with the same +8 faces-player
// offset. Per minecraft.wiki a hanging sign under/over a wide block only has 4
// directions (parallel chains); the 16-direction up-arrow variant (narrow
// block / sneak) is not handled here.
func signRotation4(yaw float32) string {
	r := int(math.Floor(float64(yaw)/90+0.5))*4 + 8
	for r < 0 {
		r += 16
	}
	for r >= 16 {
		r -= 16
	}
	return strconv.Itoa(r)
}

// classifyBlock picks the placement rule for a block name. Checked in an order
// that keeps suffix rules from shadowing each other (e.g. fence_gate before
// fence, torch before wall).
func classifyBlock(name string) ruleKind {
	switch {
	case strings.HasSuffix(name, "_stairs"):
		return ruleStair
	case strings.HasSuffix(name, "_door"):
		return ruleDoor
	case name == "minecraft:lever" || strings.HasSuffix(name, "_button"):
		return ruleButtonLever
	case strings.HasSuffix(name, "_slab"):
		return ruleSlab
	case torchWallVariant[name] != "" || strings.HasSuffix(name, "_wall_torch"):
		return ruleTorch
	case isRegularSign(name):
		return ruleSign
	case strings.HasSuffix(name, "_hanging_sign"):
		return ruleHangingSign
	case strings.HasSuffix(name, "_fence_gate"):
		return ruleLook
	case strings.HasSuffix(name, "_fence"):
		return ruleFencePost
	case strings.HasSuffix(name, "_wall"):
		return ruleFencePost
	case name == "minecraft:observer" || name == "minecraft:hopper":
		return ruleOppositeFace
	case name == "minecraft:piston" || name == "minecraft:sticky_piston" ||
		name == "minecraft:dispenser" || name == "minecraft:dropper" || name == "minecraft:ladder":
		return ruleFaceBlock
	case name == "minecraft:chest" || name == "minecraft:trapped_chest" ||
		name == "minecraft:furnace" || name == "minecraft:blast_furnace" || name == "minecraft:smoker" ||
		name == "minecraft:pumpkin" || name == "minecraft:carved_pumpkin" || name == "minecraft:jack_o_lantern" ||
		name == "minecraft:repeater" || name == "minecraft:comparator":
		return ruleFront
	case name == "minecraft:anvil" || strings.HasSuffix(name, "_anvil"):
		return ruleAnvil
	case strings.HasSuffix(name, "_bed"):
		return ruleBed
	default:
		return ruleDefault
	}
}

// resolvePlacement returns the effective block name to place and the property
// overrides, given the item-resolved block name, the clicked face (0-5), the
// player yaw (degrees), and the cursor Y within the clicked block (0..1).
//
// Returns an empty name when placement is not allowed (e.g. a torch on a
// ceiling); the caller must skip placement but still send the ack.
//
// This is the simplified ruleset: fences/walls are standalone posts, stairs
// don't compute corner shapes, waterlogged is never set (defaults to false).
func resolvePlacement(name string, face int32, yaw, cursorY float32) (string, map[string]string) {
	look := horizontalFacing(yaw)
	switch classifyBlock(name) {
	case ruleStair:
		return name, map[string]string{
			"facing": look,
			"half":   verticalHalf(face, cursorY),
		}
	case ruleButtonLever:
		af := attachFace(face)
		// A wall-mounted button/lever faces the wall it is on (the clicked face
		// direction); one on the floor/ceiling points the way the player faces.
		f := look
		if af == "wall" {
			f = faceToDirection(face)
		}
		return name, map[string]string{
			"face":    af,
			"facing":  f,
			"powered": "false",
		}
	case ruleSlab:
		return name, map[string]string{
			"type": verticalHalf(face, cursorY),
		}
	case ruleTorch:
		switch face {
		case faceUp: // placed on the ground → standing torch (no facing)
			return name, nil
		case faceNorth, faceSouth, faceWest, faceEast: // against a wall → wall variant facing out
			return torchWallVariant[name], map[string]string{"facing": faceToDirection(face)}
		default: // ceiling: torches can't hang in vanilla → reject
			return "", nil
		}
	case ruleSign:
		switch face {
		case faceUp: // on the ground → standing sign, rotation from player yaw
			return name, map[string]string{"rotation": signRotation(yaw)}
		case faceNorth, faceSouth, faceWest, faceEast: // on a wall → wall sign facing out
			return wallSignVariant(name), map[string]string{"facing": faceToDirection(face)}
		default: // ceiling: regular signs can't hang → reject
			return "", nil
		}
	case ruleHangingSign:
		switch face {
		case faceDown: // under a block → hangs from above (ceiling), 4 cardinal dirs
			return name, map[string]string{"rotation": signRotation4(yaw), "attached": "false"}
		case faceUp: // on the ground → hanging signs have no standing form (their
			// chains always reach up to a block above); reject so they can't be
			// placed on / float above the ground.
			return "", nil
		default: // side of a block/pillar → wall hanging sign. Its board renders
			// PERPENDICULAR to `facing`, so to make the board face the player
			// (along the clicked-face normal) facing must run ALONG the wall — i.e.
			// 90° off the clicked face. The board is double-sided, so CW or CCW
			// both read correctly; we use the 90°-clockwise helper for determinism.
			return wallHangingSignVariant(name), map[string]string{"facing": horizontalFacingPerp(faceToDirection(face))}
		}
	case ruleFencePost:
		return name, nil // north/south/east/west all default false → standalone post
	case ruleLook: // stairs/fence_gate: facing = where the player looks
		return name, map[string]string{"facing": look}
	case ruleAnvil: // anvil long axis is perpendicular to the player's look
		return name, map[string]string{"facing": horizontalFacingPerp(look)}
	case ruleFront: // chest/furnace/anvil/pumpkin: front faces the player (opposite of look)
		return name, map[string]string{"facing": oppositeDir(look)}
	case ruleFaceBlock: // piston/dispenser/dropper: facing = clicked face direction
		return name, map[string]string{"facing": faceToDirection(face)}
	case ruleOppositeFace: // observer/hopper: facing = opposite of clicked face
		return name, map[string]string{"facing": oppositeDir(faceToDirection(face))}
	default:
		return name, nil
	}
}

// placement is one block to write as part of a placement action, at an offset
// relative to the target cell (the cell adjacent to the clicked face). Most
// blocks are a single placement at (0,0,0); doors add a second at (0,+1,0).
type placement struct {
	dx, dy, dz int
	name       string
	props      map[string]string
}

// resolvePlacements returns the block(s) to place for one Use Item On action.
// An empty slice rejects placement (e.g. a torch on a ceiling) — the caller
// must still send the ack.
func resolvePlacements(name string, face int32, yaw, cursorY float32) []placement {
	if classifyBlock(name) == ruleDoor {
		look := horizontalFacing(yaw)
		// A door is two blocks: lower half at the target, upper half on top.
		// Hinge/open/powered default (left / closed / off); facing = player look.
		// Both halves share the block id, distinguished by `half`.
		base := map[string]string{"facing": look, "hinge": "left", "open": "false", "powered": "false"}
		lower := map[string]string{"half": "lower"}
		upper := map[string]string{"half": "upper"}
		for k, v := range base {
			lower[k] = v
			upper[k] = v
		}
		return []placement{
			{0, 0, 0, name, lower},
			{0, 1, 0, name, upper},
		}
	}
	if classifyBlock(name) == ruleBed {
		look := horizontalFacing(yaw)
		dx, dz := cardinalOffset(look)
		// A bed is two blocks: foot at the target, head one block further in
		// the player's look direction. facing = look (head points that way);
		// occupied defaults to false. Both halves share the block id (`part`).
		base := map[string]string{"facing": look, "occupied": "false"}
		foot := map[string]string{"part": "foot"}
		head := map[string]string{"part": "head"}
		for k, v := range base {
			foot[k] = v
			head[k] = v
		}
		return []placement{
			{0, 0, 0, name, foot},
			{dx, 0, dz, name, head},
		}
	}
	n, props := resolvePlacement(name, face, yaw, cursorY)
	if n == "" {
		return nil
	}
	return []placement{{0, 0, 0, n, props}}
}

// cardinalOffset returns the (dx, dz) of a horizontal cardinal direction.
func cardinalOffset(facing string) (int, int) {
	switch facing {
	case "north":
		return 0, -1
	case "south":
		return 0, 1
	case "west":
		return -1, 0
	case "east":
		return 1, 0
	}
	return 0, 0
}

// multiBlockBreakOffsets returns neighbor offsets to also clear when a block of
// the given name is mined — the other half of a door (vertical) or bed
// (horizontal). Empty for ordinary blocks.
func multiBlockBreakOffsets(name string) [][3]int {
	switch {
	case strings.HasSuffix(name, "_door"):
		return [][3]int{{0, 1, 0}, {0, -1, 0}}
	case strings.HasSuffix(name, "_bed"):
		return [][3]int{{1, 0, 0}, {-1, 0, 0}, {0, 0, 1}, {0, 0, -1}}
	}
	return nil
}

// isFenceFamily reports blocks that make a wall render "tall" when next to it:
// fences, walls, and fence gates.
func isFenceFamily(name string) bool {
	return strings.HasSuffix(name, "_fence") || strings.HasSuffix(name, "_wall") ||
		strings.HasSuffix(name, "_fence_gate")
}

// isConnectable reports whether a block has north/south/east/west connection
// properties we recompute from neighbors: fences, walls, glass panes, iron bars,
// and redstone dust.
func isConnectable(name string) bool {
	return isFenceFamily(name) || strings.HasSuffix(name, "_pane") ||
		name == "minecraft:iron_bars" || name == "minecraft:redstone_wire"
}

// connectableSide reports whether a block of sourceName should link to the given
// neighbor. Fences/walls/panes/iron bars connect to solids + any connectable;
// redstone dust links only to other dust (full redstone power is deferred).
func connectableSide(sourceName string, state int32) bool {
	if state == 0 || isFluid(state) {
		return false
	}
	name, ok := blockStateName[state]
	if !ok {
		return true // unknown non-air → treat as solid
	}
	if sourceName == "minecraft:redstone_wire" {
		return name == "minecraft:redstone_wire"
	}
	return isConnectable(name) || !needsSolidSupport(name)
}

// connectionProps returns the north/south/east/west connection overrides for a
// connectable block at (x,y,z), by inspecting its 4 horizontal neighbors via get.
// Value type depends on the block: fences/panes/iron bars use "true"; walls use
// "low"/"tall" (tall next to a fence/wall/gate); redstone dust uses "side".
func connectionProps(get func(int, int, int) int32, x, y, z int) map[string]string {
	state := get(x, y, z)
	name, ok := blockStateName[state]
	if !ok || !isConnectable(name) {
		return nil
	}
	wall := strings.HasSuffix(name, "_wall")
	dust := name == "minecraft:redstone_wire"
	props := map[string]string{}
	if dust {
		props["power"] = "0" // no redstone power engine yet
	}
	sides := []struct {
		key    string
		dx, dz int
	}{
		{"north", 0, -1}, {"south", 0, 1}, {"west", -1, 0}, {"east", 1, 0},
	}
	for _, s := range sides {
		ns := get(x+s.dx, y, z+s.dz)
		if !connectableSide(name, ns) {
			continue
		}
		nn, _ := blockStateName[ns]
		switch {
		case dust:
			props[s.key] = "side"
		case wall:
			if isFenceFamily(nn) {
				props[s.key] = "tall"
			} else {
				props[s.key] = "low"
			}
		default:
			props[s.key] = "true"
		}
	}
	return props
}

// horizontalFacing maps a player yaw (degrees, MC convention: 0=south, 90=west,
// 180=north, 270=east) to the cardinal the player is looking toward.
func horizontalFacing(yaw float32) string {
	y := float64(yaw) + 45
	for y < 0 {
		y += 360
	}
	for y >= 360 {
		y -= 360
	}
	switch int(y / 90) {
	case 0:
		return "south"
	case 1:
		return "west"
	case 2:
		return "north"
	default:
		return "east"
	}
}

// horizontalFacingPerp returns the cardinal 90° clockwise from the given facing
// (south→west→north→east). Used for anvils, whose long axis is perpendicular to
// the player's look.
func horizontalFacingPerp(facing string) string {
	switch facing {
	case "south":
		return "west"
	case "west":
		return "north"
	case "north":
		return "east"
	default:
		return "south"
	}
}

// faceToDirection maps a clicked-face enum value to its world-space direction
// (the outward normal of the face): 0=down,1=up,2=north,3=south,4=west,5=east.
func faceToDirection(face int32) string {
	switch face {
	case faceDown:
		return "down"
	case faceUp:
		return "up"
	case faceNorth:
		return "north"
	case faceSouth:
		return "south"
	case faceWest:
		return "west"
	case faceEast:
		return "east"
	}
	return "up"
}

// oppositeDir returns the opposite cardinal/direction.
func oppositeDir(d string) string {
	switch d {
	case "down":
		return "up"
	case "up":
		return "down"
	case "north":
		return "south"
	case "south":
		return "north"
	case "west":
		return "east"
	case "east":
		return "west"
	}
	return d
}

// needsSolidSupport reports whether a block must be placed against a solid
// surface (a wall/floor). These non-full blocks would visibly float without a
// solid block to attach to, so the handler rejects placing them on another
// non-solid block (e.g. dust on dust, a button on a button).
func needsSolidSupport(name string) bool {
	switch {
	case strings.HasSuffix(name, "_button"),
		strings.HasSuffix(name, "_pressure_plate"),
		strings.HasSuffix(name, "rail"),  // rail, powered_rail, detector_rail, activator_rail
		strings.HasSuffix(name, "torch"), // torch, wall_torch, redstone_torch, soul_torch, …
		strings.HasSuffix(name, "_sign"), // signs are non-solid (so buttons/torches can't attach)
		strings.HasSuffix(name, "_carpet"):
		return true
	case name == "minecraft:lever",
		name == "minecraft:redstone_wire",
		name == "minecraft:repeater",
		name == "minecraft:comparator",
		name == "minecraft:ladder",
		name == "minecraft:vine":
		return true
	}
	return false
}

// canStandOnGround reports whether a block has a standing/floor variant we can
// fall back to when its wall placement is rejected — torches (→ standing torch)
// and regular signs (→ standing sign). Used so clicking a non-supporting block
// (e.g. a glass pane) still places the torch/sign on the ground beside it
// rather than failing.
func canStandOnGround(name string) bool {
	return torchWallVariant[name] != "" || isRegularSign(name)
}

// isSignName reports whether a name is any sign (standing/wall/hanging).
func isSignName(name string) bool {
	return strings.HasSuffix(name, "_sign")
}

// supportOK reports whether a block of placingName can be placed with the given
// support (the clicked block). Attachables (torches, buttons, signs, ladders,
// redstone, …) need a solid, FULL-cube face to attach to, so a thin connectable
// block — fence, wall, glass pane, iron bars, fence gate — is NOT a valid
// support. The lone exception is regular signs, which vanilla lets stack on
// another sign (sign-on-sign) even though a sign is itself non-solid.
func supportOK(placingName string, supportState int32) bool {
	if isSolidBlock(supportState) {
		// Thin connectables don't present a full face, so no attachable may be
		// placed against (or on top of) them.
		if name, ok := blockStateName[supportState]; ok && isConnectable(name) {
			return false
		}
		return true
	}
	if isRegularSign(placingName) {
		if name, ok := blockStateName[supportState]; ok && isSignName(name) {
			return true
		}
	}
	return false
}

// isSolidBlock reports whether a block state is a solid support: not air, not a
// fluid, and not one of the non-full attachable blocks. Unknown non-air states
// are treated as solid.
func isSolidBlock(stateID int32) bool {
	if stateID == 0 || isFluid(stateID) {
		return false
	}
	if name, ok := blockStateName[stateID]; ok {
		return !needsSolidSupport(name)
	}
	return true
}

// attachFace maps the clicked face to the button/lever "face" property:
// top→floor, bottom→ceiling, side→wall.
func attachFace(face int32) string {
	switch face {
	case faceUp:
		return "floor"
	case faceDown:
		return "ceiling"
	default:
		return "wall"
	}
}

// verticalHalf returns "top" or "bottom" for a stair half / slab type based on
// which face was clicked: clicking the top face places a bottom half (sitting
// on the surface), the bottom face a top half (under a block), and a side face
// uses the cursor Y within the clicked block.
func verticalHalf(face int32, cursorY float32) string {
	switch face {
	case faceUp:
		return "bottom"
	case faceDown:
		return "top"
	default:
		if cursorY >= 0.5 {
			return "top"
		}
		return "bottom"
	}
}
