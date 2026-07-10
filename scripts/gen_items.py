#!/usr/bin/env python3
# Generates pkg/server/items.go from Mojang data reports (registries.json,
# minecraft:item registry). Source: `server.jar --reports`.
# Run after a version bump; the generated file is committed for standalone builds.
import json, os

REG = os.environ.get("REGISTRIES_JSON", "/tmp/generated/reports/registries.json")
r = json.load(open(REG))
e = r["minecraft:item"]["entries"]

# name -> protocol_id
name_id = {name: v["protocol_id"] for name, v in e.items()}
id_name = {v: name for name, v in name_id.items()}
max_id = max(name_id.values())

out = [
    "// Code generated from Minecraft 26.2 server.jar --reports (registries.json) by scripts/gen_items.py. DO NOT EDIT.",
    "package server",
    "",
    "// Item id mappings generated from the Mojang data reports minecraft:item registry.",
    f"const TotalItems int32 = {max_id + 1}",
    "",
    "// itemNameToID maps a vanilla item name to its protocol id.",
    "var itemNameToID = map[string]int32{",
]
for name in sorted(name_id):
    out.append(f'\t"{name}": {name_id[name]},')
out.append("}")
out.append("")
out.append("// itemIDToName maps a protocol item id back to its name.")
out.append("var itemIDToName = map[int32]string{")
for iid in sorted(id_name):
    out.append(f'\t{iid}: "{id_name[iid]}",')
out.append("}")
out.append("")
out.append("// ItemID returns the protocol item id for a name, or -1 if unknown.")
out.append("func ItemID(name string) int32 {")
out.append("\tif id, ok := itemNameToID[name]; ok {")
out.append("\t\treturn id")
out.append("\t}")
out.append("\treturn -1")
out.append("}")
print("\n".join(out))
