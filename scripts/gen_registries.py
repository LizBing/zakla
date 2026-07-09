#!/usr/bin/env python3
# Extracts every synchronized registry's real vanilla entry names from the
# 26.2 client.jar data pack and emits a Go data file. NBT is omitted per entry;
# the client resolves it from its known pack (minecraft:core 26.2).
import zipfile, os, sys

JAR = os.path.expanduser("~/Games/MC/.minecraft/versions/26.2/26.2.jar")
EXCLUDE = {"tags", "worldgen", "loot_table", "recipe", "structure", "advancement", "datapacks"}

z = zipfile.ZipFile(JAR)
regs = {}
for n in z.namelist():
    if not n.startswith("data/minecraft/") or not n.endswith(".json"):
        continue
    rest = n[len("data/minecraft/"):]
    parts = rest.split("/")
    if len(parts) < 2:
        continue
    if parts[0] == "worldgen":
        if len(parts) >= 3 and parts[1] == "biome":
            reg, entry = "biome", parts[2]
        else:
            continue
    elif parts[0] in EXCLUDE:
        continue
    else:
        reg, entry = parts[0], parts[1]
    if not entry.endswith(".json"):
        continue
    regs.setdefault(reg, set()).add("minecraft:" + entry[:-5])

out = [
    "// Code generated from Minecraft 26.2 client.jar data/ by scripts/gen_registries.py. DO NOT EDIT.",
    "package server",
    "",
    'import "github.com/zakla/mc-server/pkg/protocol"',
    "",
    "// vanillaSyncRegistries lists every synchronized registry with its real",
    "// vanilla entry names. Per-entry NBT is omitted; the client fills it from",
    "// its known pack (minecraft:core 26.2) agreed during configuration.",
    "var vanillaSyncRegistries = []Registry{",
]
for reg in sorted(regs):
    regid = "minecraft:worldgen/biome" if reg == "biome" else "minecraft:" + reg
    out.append('\t{ID: "%s", Entries: []protocol.RegistryEntry{' % regid)
    for e in sorted(regs[reg]):
        out.append('\t\t{Name: "%s"},' % e)
    out.append('\t}},')
out.append("}")
print("\n".join(out))
