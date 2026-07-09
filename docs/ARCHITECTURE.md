# 架构

## 分层

```
pkg/protocol   纯协议原语（无状态）
  types.go        VarInt/VarLong/String/UUID/Position/Angle/数值类型读写
  packet.go       包构造 + zlib 压缩/解压 + payload 拆分
  nbt.go          最小 NBT 写入器（compound/string/list + 文本组件）
  handshake.go status.go login.go configuration.go play.go
                  各阶段包的编解码

pkg/net        连接层
  connection.go   TCP conn + 状态机(5态) + 压缩阈值 + ReadPacket/WritePacket
                  （透明处理压缩：读时解压，写时按阈值压缩）

pkg/server     应用层
  server.go       Server 生命周期 + 玩家管理 + 聊天广播
  handler.go      每连接状态机：handle→Status/Login/Configuration/Play
  chunk.go        空 chunk 构造（26.2 paletted container + light）
  registries.go   同步 registry 数据（dimension_type/biome/chat_type/damage_type/world_clock）
  util.go         offline UUID / entity id / teleport id
```

## 26.2 协议流程（offline 模式）

```
C→S Handshake (intent=2)
C→S Login Start (name, uuid)
S→C Set Compression (threshold)
S→C Login Success (Game Profile: UUID+name+properties, Session ID)
C→S Login Acknowledged
── 进入 Configuration ──
S→C Plugin Message (minecraft:brand)
S→C Feature Flags (minecraft:vanilla)
S→C Known Packs (clientbound: [{minecraft, core, 26.2}])
C→S Known Packs (serverbound: 客户端确认知道的)   ← vanilla 客户端会等这个才继续
S→C Registry Data × N
S→C Finish Configuration
C→S Acknowledge Finish Configuration
── 进入 Play ──
S→C Login (play)
S→C Change Difficulty / Player Abilities / Set Held Item
S→C Player Info Update (Add Player)
S→C Synchronize Player Position   →   C→S Confirm Teleportation
S→C Set Default Spawn Position
S→C Game Event (13 = Start waiting for level chunks)
S→C Set Center Chunk
S→C Chunk Data and Update Light
S→C Set Health
C↔S Keep Alive (每 ~10s) / Chat
```

## 26.2 特定点（实现中踩过的坑）

- **Configuration 阶段**：1.20.2+ 登录成功后必须经过配置阶段（registry data + finish configuration）。很多老教程/老实现没有这步。
- **Login Success 含 Session ID**：26.2 在 Game Profile 之后多了 Session ID (UUID) 字段。
- **Chunk Section 含 Fluid count**：1.21.5+ 每个 section = `block count (Short)` + `fluid count (Short)` + block-states paletted container + biomes paletted container。漏掉 fluid count 会让后续全部错位。
- **Paletted container 单值模式**：`BPE=0x00` 后跟一个**裸 VarInt 值**（无 palette count 前缀，无 data array）。1.21.5+ data array 不再发送长度前缀。
- **Heightmaps**：1.21.5+ 是 "Prefixed Array of Heightmap"，空数组（VarInt 0）合法。
- **Quick Play**：26.2 客户端已**不再识别 `--server`**，自动连接需用 `--quickPlayMultiplayer "host:port"`。
- **Network NBT**：1.20.2+（协议 764+）的**网络 NBT**，根标签**无 name length 字段**（type 字节后直接跟 payload）。system_chat 的 Text Component、registry entry NBT 都用此格式。写成文件格式 NBT（带 root name length）会让客户端把 name length 当成 payload 长度，解析错位（曾导致 "found 27 bytes extra"）。
- **world_clock registry**：26.2 新增的同步 registry，维度会引用它，必须发送。

## 真实客户端验证发现的真实 bug

这些是只有真实客户端能抓到的（mock 客户端不校验内容）：

1. `damage_type` 列表里 `player_explosion` 写了两次 → 客户端 `Adding duplicate key` 崩溃
2. 缺 `world_clock` registry → 客户端 `Unbound values in world_clock` 崩溃
3. registry / tags 不完整 → 客户端 `FinishConfiguration` 时校验失败（dimension_type 缺 `infiniburn_overworld` tag、timeline 缺 `in_overworld` tag 等）

## 自动化调试方法

本项目用「真实客户端驱动迭代」验证协议：

- **mock 客户端**（`cmd/mock-client`）：复用 server 的 protocol/net 包，验证包交换流程（握手/登录/配置/游戏/聊天闭环）。
- **真实 26.2 客户端**：HMCL 实例（Temurin Java 25）+ `--quickPlayMultiplayer localhost:25565` 直连服务端，暴露 registry/tags 的语义错误。
- 服务端日志 + 客户端崩溃堆栈定位 → 修复 → 重新编译重启 → 重试。

启动真实客户端的命令（参考）：

```bash
# 从 26.2.json 生成 classpath
python3 scripts/gen_classpath.py   # 或见仓库内实现
# 用 Temurin Java 启动并 Quick Play 连服
/Library/Java/JavaVirtualMachines/temurin-25.jdk/Contents/Home/bin/java \
  -XstartOnFirstThread -Xmx2048m \
  -Djava.library.path=<natives> \
  -cp <classpath> \
  net.minecraft.client.main.Main \
  --username Tester --version 26.2 \
  --gameDir <.minecraft> --assetsDir <assets> --assetIndex 32 \
  --uuid <uuid> --accessToken 0 --versionType release \
  --quickPlayMultiplayer "localhost:25565"
```

## 剩余工作：vanilla 数据层

让真实客户端完全进入世界，需要内置一份 vanilla 数据库：

1. **完整同步 registry 条目**：`dimension_type, biome, chat_type, damage_type, trim_pattern, trim_material, banner_pattern, wolf_variant, painting_variant, cat_variant, ..., world_clock, sulfur_cube_archetype` 等，含正确 NBT（或靠 known packs 让客户端用内置 core 填充）。
2. **完整 Update Tags**：`block, item, entity_type, game_event, fluid, biome, ...` 的所有 vanilla tags。
3. **name→ID 映射**：hardcoded registry（block/item/entity_type 等的数字 ID）——Update Tags 的 tag 条目要用这些 ID。需要从 Mojang reports 或 Burger 生成。

实现思路：写一个生成工具，解压 `26.2.jar` 的 `data/`（vanilla data pack），把 registries/tags JSON 转成 Go 数据文件 + ID 映射表，编译进服务端。
