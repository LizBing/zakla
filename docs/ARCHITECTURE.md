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
  chunk.go        Chunk/ChunkSection 数据结构 + paletted container（单值/间接/直接）+ 位打包 + light
  world.go        World 稀疏 chunk 存储 + SetBlock/GetBlock + 出生点平台
  block_states.go 从 Mojang reports 生成的 block state id 映射（name→id / id→name）
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
── 方块交互（挖掘） ──
C→S Player Action (Finished digging, position, sequence)
S→C Block Update (position, air)         ← 广播给所有在线玩家（含挖掘者）
S→C Block Changed Ack (sequence)         ← 必须回，否则客户端冻结
```

## 26.2 特定点（实现中踩过的坑）

- **Configuration 阶段**：1.20.2+ 登录成功后必须经过配置阶段（registry data + finish configuration）。很多老教程/老实现没有这步。
- **Login Success 含 Session ID**：26.2 在 Game Profile 之后多了 Session ID (UUID) 字段。
- **Chunk Section 含 Fluid count**：1.21.5+ 每个 section = `block count (Short)` + `fluid count (Short)` + block-states paletted container + biomes paletted container。漏掉 fluid count 会让后续全部错位。
- **Paletted container 三模式**（block-states：minBits=4，indirect 上限 8，超过升 direct=ceil(log2 TotalBlockStates)=15；biomes：minBits=0，上限 3，direct=ceil(log2 biomeCount)）：
  - **单值**（唯一值=1）：`BPE=0x00` + 裸 VarInt 值（无 palette，无 data）。
  - **间接**（≤阈值）：BPE + VarInt palette + data long 数组。
  - **直接**（>阈值）：BPE=globalBits + data long 数组（无 palette，entries 直接是全局 id）。
- **Paletted container data long 数组**：`long[ceil(4096×BPE/64)]`，条目**跨 long 边界扁平打包**（entry i 起始 bit = i×BPE，从 long 数值 LSB 起），**无 VarInt 长度前缀**（1.21.5+ 客户端按 BPE 推导 longCount）。这是真实客户端最容易崩的点——本项目用离线往返测试（encode→decode）+ 真实客户端渲染双重验证。
- **BlockChangedAck 不可省**：每个带 sequence 的 serverbound 方块包（Player Action / Use Item On）都**必须**回 `Block Changed Ack (0x04, sequence)`，否则客户端冻结后续方块编辑。
- **Player Action 的 Face 是 Byte，Use Item On 的 Face 是 VarInt**（类型不同，易踩坑）。
- **Heightmaps**：1.21.5+ 是 "Prefixed Array of Heightmap"，空数组（VarInt 0）合法。
- **Quick Play**：26.2 客户端已**不再识别 `--server`**，自动连接需用 `--quickPlayMultiplayer "host:port"`。
- **Network NBT**：1.20.2+（协议 764+）的**网络 NBT**，根标签**无 name length 字段**（type 字节后直接跟 payload）。system_chat 的 Text Component、registry entry NBT 都用此格式。写成文件格式 NBT（带 root name length）会让客户端把 name length 当成 payload 长度，解析错位（曾导致 "found 27 bytes extra"）。
- **world_clock registry**：26.2 新增的同步 registry，维度会引用它，必须发送。
- **客户端不渲染孤立 chunk**：wiki 明文——Notchian 客户端**一般不渲染没有邻居的 chunk**（为了正确处理光照和连接方块），但 block state 照常读入（所以碰撞/挖掘仍命中）。只发一个 (0,0) 会导致方块"透明有碰撞、只有黑色线框"。必须发 spawn 周围一圈 chunk（哪怕全是空气）当邻居。这是真实客户端才暴露的问题——mock 客户端不校验渲染，发现不了。

## 真实客户端验证发现的真实 bug

这些是只有真实客户端能抓到的（mock 客户端不校验内容）：

1. `damage_type` 列表里 `player_explosion` 写了两次 → 客户端 `Adding duplicate key` 崩溃
2. 缺 `world_clock` registry → 客户端 `Unbound values in world_clock` 崩溃
3. registry / tags 不完整 → 客户端 `FinishConfiguration` 时校验失败（dimension_type 缺 `infiniburn_overworld` tag、timeline 缺 `in_overworld` tag 等）

## 方块交互（挖掘 + 放置）

### 挖掘
- **block state id** 不在 client.jar（运行时计算），用 **Mojang reports**（`server.jar --reports` → `reports/blocks.json`）拿到计算好的数字 id，生成 `block_states.go`。prismarine-data 没有 26.2，1.21.9 的 id 对不上 26.2 新方块（硫/朱砂），不可用。
- **挖掘流程**：客户端发 Player Action（Status=2 Finished）→ 服务端 `World.SetBlock(air)` → `broadcastBlockUpdate`（广播给所有在线玩家，含挖掘者）→ 回 `Block Changed Ack`。任何 Player Action（含 Started/Cancelled 等非破坏动作）都要回 ack。

### 放置
- **item id** ≠ block state id（两套注册表）。从 reports/registries.json 的 `minecraft:item` 生成 `items.go`。多数方块 item 与 block 同名，故 `item name → BlockStateID(name)` 即得 block state（少数例外如 redstone/redstone_wire 暂未处理）。
- **物品栏**：`Set Container Content (0x12)` 发整个 player inventory（window id=0，46 槽；**hotbar 是 slot 36-44，不是 0-8**）。**Slot 用 1.20.5+ data component 格式**：`count(VarInt)` 在前，count=0 即空槽（单字节 0x00，后面无数据）；count>0 才跟 `item id(VarInt) + 组件add数(0) + 组件remove数(0)`。写成旧 `present bool + NBT` 会让客户端崩。
- **放置流程**：玩家右键 → `Use Item On (0x42, position + face)` → 服务端查手持 item（`Player.hotbar[heldSlot]`）→ item name → BlockStateID → 新位置（`position + face 偏移`，face 0=-Y..5=+X）→ 若该格为空则 `SetBlock` → `broadcastBlockUpdate` → `BlockChangedAck`。ack 必须回，否则客户端回滚（ghost block）。
- **Set Carried Item (0x35, C→S)**：玩家按数字键切 hotbar，服务端跟踪 `heldSlot`。
- **不含方块物理**：沙子重力 / 水流动 / 农作物生长等需要 tick 引擎（RandomTick/ScheduledTick + 方块行为），与"放方块"是两套机制，留作后续。

### 验证
mock 客户端自动验证挖掘协议链路；真实 26.2 客户端验证渲染（间接 paletted container）、挖掘、放置（hotbar 9 种方块 + 右键放置 + 位置正确）。

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

## 世界持久化

世界用 gob 存到 `<world.name>/world.gob`（自定义简化格式，非 vanilla Anvil）：
- `World.Save()` 原子写（tmp + rename），防崩溃损坏；只存非空 section（gob 不能编码含 nil 元素的定长数组，故用 `[]sectionEntry` 带 index）。
- 启动 `LoadWorld`（文件不存在则新世界 + 预填 spawn）；30s ticker 定期保存；`Stop` 关闭保存——崩溃最多丢最后 30s 编辑。
- 持久化往返测试（`t.TempDir`，portable）。真实环境验证：挖/放后重启服务端，加载 81 chunks，挖掉的方块保持 air、未动的保持原样。

## 已完成 & 后续

vanilla 数据层已落地：从 `26.2.jar` 的 `data/`（client.jar）生成同步 registry + tags（`vanilla_data.go` / `vanilla_tags.go`），block state id / item id 从 Mojang reports 生成（`block_states.go` / `items.go`），都编译进服务端。

方块挖掘 + 放置 + 世界持久化均已落地并经真实客户端验证。

后续：
- **方块物理**：沙子重力 / 水流动 / 农作物生长需 tick 引擎（RandomTick/ScheduledTick + 方块行为）。
- **多 chunk 视距**：当前固定发 spawn 周围 9×9，未来按玩家位置动态发送。
- **同步优化**：Section Blocks Update 批量、Anvil 存档兼容（目前是自定义 gob 格式，不与 vanilla 世界互通）。
