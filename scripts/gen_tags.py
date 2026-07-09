#!/usr/bin/env python3
# Extracts vanilla tag names from the 26.2 client.jar and emits a Go data file.
# Tag registries = hardcoded (client always has) + the synchronized registries
# we actually send (read from vanilla_data.go). Tags for any other registry
# (villager_trade, configured_feature, structure, ...) are excluded or the
# client reports "Missing registry".
import zipfile, os, re

JAR = os.path.expanduser("~/Games/MC/.minecraft/versions/26.2/26.2.jar")
_VANILLA = os.path.join(os.path.dirname(__file__), "..", "pkg", "server", "vanilla_data.go")

_sync = set()
if os.path.exists(_VANILLA):
    _sync = set(re.findall(r'ID: "([^"]+)"', open(_VANILLA).read()))

SYNC_TAG_REGISTRIES = _sync | {
    "minecraft:block", "minecraft:item", "minecraft:fluid", "minecraft:entity_type",
    "minecraft:game_event", "minecraft:point_of_interest_type", "minecraft:potion",
}

z = zipfile.ZipFile(JAR)
regs = {}
for n in z.namelist():
    if not n.startswith("data/minecraft/tags/") or not n.endswith(".json"):
        continue
    rest = n[len("data/minecraft/tags/"):]
    parts = rest.split("/")
    if len(parts) < 2:
        continue
    if parts[0] == "worldgen":
        if len(parts) < 3:
            continue
        reg = "worldgen/" + parts[1]
        tag_parts = parts[2:]
    else:
        reg = parts[0]
        tag_parts = parts[1:]
    regid = "minecraft:" + reg
    if regid not in SYNC_TAG_REGISTRIES:
        continue
    name = "/".join(tag_parts)[:-5]
    regs.setdefault(regid, set()).add("minecraft:" + name)

out = [
    "// Code generated from Minecraft 26.2 client.jar data/tags/ by scripts/gen_tags.py. DO NOT EDIT.",
    "package server",
    "",
    'import "github.com/zakla/mc-server/pkg/protocol"',
    "",
    "// vanillaTags lists vanilla tags with EMPTY entries, limited to registries",
    "// the client has (sync registries we send + hardcoded). Member IDs omitted.",
    "var vanillaTags = []protocol.TagRegistry{",
]
for regid in sorted(regs):
    out.append('\t{Registry: "%s", Tags: []protocol.TagDefinition{' % regid)
    for t in sorted(regs[regid]):
        out.append('\t\t{Name: "%s"},' % t)
    out.append('\t}},')
out.append("}")
print("\n".join(out))
