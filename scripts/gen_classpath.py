import json, os
base = os.path.expanduser("~/Games/MC/.minecraft")
with open(base + "/versions/26.2/26.2.json") as f:
    v = json.load(f)
print("MAIN:", v.get("mainClass"))
print("ASSET:", v.get("assetIndex", {}).get("id"))
cps = [base + "/versions/26.2/26.2.jar"]
missing = []
for lib in v.get("libraries", []):
    # skip natives-only / rules-excluded if present
    art = lib.get("downloads", {}).get("artifact", {})
    p = art.get("path")
    if p:
        full = base + "/libraries/" + p
        if os.path.exists(full):
            cps.append(full)
        else:
            missing.append(full)
print("CP_COUNT:", len(cps))
print("MISSING:", len(missing))
for m in missing[:5]:
    print("  miss:", m)
with open("/tmp/mc-cp.txt", "w") as f:
    f.write(":".join(cps))
