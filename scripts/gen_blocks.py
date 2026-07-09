#!/usr/bin/env python3
# Generates pkg/server/block_states.go from Mojang data reports (blocks.json).
# Source: `java -DbundlerMainClass=net.minecraft.data.Main -jar server.jar --reports`
# Run after a version bump; the generated file is committed for standalone builds.
import json, os, sys

REPORTS = os.environ.get("BLOCKS_JSON", "/tmp/generated/reports/blocks.json")
b = json.load(open(REPORTS))

default_id = {}   # name -> default state id
state_name = {}   # state id -> name
max_id = 0
for name, blk in b.items():
    states = blk.get("states", [])
    dflt = next((s["id"] for s in states if s.get("default")), None)
    if dflt is None and states:
        dflt = states[0]["id"]
    if dflt is not None:
        default_id[name] = dflt
    for s in states:
        sid = s["id"]
        state_name[sid] = name
        if sid > max_id:
            max_id = sid

out = [
    "// Code generated from Minecraft 26.2 server.jar --reports by scripts/gen_blocks.py. DO NOT EDIT.",
    "package server",
    "",
    "// Block state id mappings generated from Mojang data reports (blocks.json).",
    f"const TotalBlockStates int32 = {max_id + 1}",
    "",
    "// blockDefaultStateID maps a block name to its default state id.",
    "var blockDefaultStateID = map[string]int32{",
]
for name in sorted(default_id):
    out.append(f'\t"{name}": {default_id[name]},')
out.append("}")
out.append("")
out.append("// blockStateName maps a state id back to its block name (debug / placement).")
out.append("var blockStateName = map[int32]string{")
for sid in sorted(state_name):
    out.append(f'\t{sid}: "{state_name[sid]}",')
out.append("}")
out.append("")
out.append("// BlockStateID returns the default state id for a block name, or air (0) if unknown.")
out.append("func BlockStateID(name string) int32 {")
out.append("\tif id, ok := blockDefaultStateID[name]; ok {")
out.append("\t\treturn id")
out.append("\t}")
out.append("\treturn 0")
out.append("}")
print("\n".join(out))
