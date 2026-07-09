#!/usr/bin/env bash
# 启动真实 MC 26.2 客户端连接本地服务端（协议验证用）。
#
# 前置：
#   1. 用 HMCL/Prism 下载好 26.2 实例（拿到 versions/26.2/{26.2.jar,natives-*} 和 assets）
#   2. python3 scripts/gen_classpath.py   # 生成 /tmp/mc-cp.txt（按需改脚本里的 GAMEDIR）
#
# 用法：SERVER=localhost:25565 NAME=Tester ./scripts/run_real_client.sh
set -e
JAVA=${JAVA:-/Library/Java/JavaVirtualMachines/temurin-25.jdk/Contents/Home/bin/java}
GAMEDIR=${GAMEDIR:-$HOME/Games/MC/.minecraft}
NATIVES="$GAMEDIR/versions/26.2/natives-macos-arm64"
SERVER=${SERVER:-localhost:25565}
CP=$(cat /tmp/mc-cp.txt)
UUID=${UUID:-0609b6e4-361f-4a2b-9c11-8a763d624a1d}
NAME=${NAME:-Tester}

exec "$JAVA" -XstartOnFirstThread -Xmx2048m -Xms1024m \
  -Djava.library.path="$NATIVES" \
  -cp "$CP" \
  net.minecraft.client.main.Main \
  --username "$NAME" --version 26.2 \
  --gameDir "$GAMEDIR" \
  --assetsDir "$GAMEDIR/assets" \
  --assetIndex 32 \
  --uuid "$UUID" --accessToken 0 --versionType release \
  --quickPlayMultiplayer "$SERVER"
