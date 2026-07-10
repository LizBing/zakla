package server

// itemBlockOverride maps an item name to the block name it actually places,
// for the cases where they differ. Most block items share the item's name
// (minecraft:stone item → minecraft:stone block), but seeds/wire/etc. don't.
// Without this, placing e.g. redstone resolves BlockStateID("minecraft:redstone")
// = 0 (air) and nothing gets placed.
var itemBlockOverride = map[string]string{
	"minecraft:redstone":          "minecraft:redstone_wire",
	"minecraft:reeds":             "minecraft:sugar_cane",
	"minecraft:wheat_seeds":       "minecraft:wheat",
	"minecraft:carrot":            "minecraft:carrots",
	"minecraft:potato":            "minecraft:potatoes",
	"minecraft:beetroot_seeds":    "minecraft:beetroots",
	"minecraft:melon_seeds":       "minecraft:melon_stem",
	"minecraft:pumpkin_seeds":     "minecraft:pumpkin_stem",
	"minecraft:torchflower_seeds": "minecraft:torchflower_crop",
	"minecraft:pitcher_pod":       "minecraft:pitcher_crop",
	"minecraft:sweet_berries":     "minecraft:sweet_berry_bush",
	"minecraft:glow_berries":      "minecraft:cave_vines",
	"minecraft:cocoa_beans":       "minecraft:cocoa",
}

// ItemToBlockName returns the block name a given item places, applying the
// override map and falling back to the item's own name.
func ItemToBlockName(itemName string) string {
	if b, ok := itemBlockOverride[itemName]; ok {
		return b
	}
	return itemName
}
