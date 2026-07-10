#!/usr/bin/env python3
# Generates pkg/server/block_states.go from Mojang data reports (blocks.json).
# Source: `java -DbundlerMainClass=net.minecraft.data.Main -jar server.jar --reports`
# Run after a version bump; the generated file is committed for standalone builds.
import json, os, sys

REPORTS = os.environ.get("BLOCKS_JSON", "/tmp/generated/reports/blocks.json")
b = json.load(open(REPORTS))


def props_key(name, props):
    # Canonical key: "name|k1=v1,k2=v2,..." with property keys sorted. The Go
    # side (stateKey in blockprops.go) MUST build the identical string.
    return name + "|" + ",".join(f"{k}={props[k]}" for k in sorted(props))


default_id = {}        # name -> default state id
default_props = {}     # name -> default state's properties (only if non-empty)
state_name = {}        # state id -> name
state_by_props = {}    # "name|k=v,..." -> state id (only states with properties)
max_id = 0
for name, blk in b.items():
    states = blk.get("states", [])
    dflt_state = next((s for s in states if s.get("default")), None)
    if dflt_state is None and states:
        dflt_state = states[0]
    if dflt_state is not None:
        default_id[name] = dflt_state["id"]
        dp = dflt_state.get("properties", {})
        if dp:
            default_props[name] = dp
    for s in states:
        sid = s["id"]
        state_name[sid] = name
        props = s.get("properties", {})
        if props:
            state_by_props[props_key(name, props)] = sid
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
out.append('// blockStateByProps maps "name|k=v,..." (property keys sorted) to a state id,')
out.append("// for every state that has properties. Used by ResolveStateID to pick a specific")
out.append("// state given a block name + property overrides.")
out.append("var blockStateByProps = map[string]int32{")
for k in sorted(state_by_props):
    out.append(f'\t"{k}": {state_by_props[k]},')
out.append("}")
out.append("")
out.append("// blockDefaultProps maps a block name to its default state's full property set")
out.append("// (only blocks whose default state has properties). ResolveStateID merges caller")
out.append("// overrides on top of this so unspecified properties inherit the default.")
out.append("var blockDefaultProps = map[string]map[string]string{")
for name in sorted(default_props):
    dp = default_props[name]
    parts = ", ".join(f'"{k}": "{dp[k]}"' for k in sorted(dp))
    out.append(f'\t"{name}": {{{parts}}},')
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
