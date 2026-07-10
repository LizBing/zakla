# zakla — Go Minecraft 26.2 服务端

用 Go 从零实现的 Minecraft Java Edition **26.2**（协议版本 **776**）服务端。支持玩家进入空世界并聊天，已用真实 MC 26.2 客户端验证。

## 状态

| 能力 | 状态 |
|------|------|
| 完整协议栈（PVN 776）：握手/状态/登录/配置/游戏 | ✅ |
| zlib 压缩协商 / offline 模式登录 | ✅ |
| 配置阶段（known packs + 30 个同步 registry + Update Tags） | ✅ |
| chunk 编码（含 1.21.5+ Fluid count）/ paletted container / network NBT | ✅ |
| vanilla 数据层（从 client.jar + Mojang reports 生成） | ✅ |
| **进入空世界 + 聊天** | ✅ 真实 MC 26.2 客户端实测 |
| **方块挖掘（破坏方块 + 多人同步）** | ✅ 真实 MC 26.2 客户端实测 |
| **方块放置（物品栏 + 9 种方块 + 多人同步）** | ✅ 真实 MC 26.2 客户端实测 |
| Docker 交付（21MB 镜像） | ✅ |

---

## 构造（构建）

### 前置

- **Go 1.21+**（`go version` 确认）
- **（可选）Docker** —— 用容器方式运行
- **（可选）Python 3** —— 仅在重新生成 vanilla 数据时需要

### 构建二进制

```bash
# 在项目根目录
go build -o bin/server       ./cmd/server
go build -o bin/mock-client  ./cmd/mock-client
```

构建产物在 `bin/`。

### vanilla 数据（已内置，正常无需操作）

`pkg/server/vanilla_data.go`、`vanilla_tags.go` 是从 26.2 `client.jar` 的 `data/` 提取生成的；`block_states.go` 是从 26.2 `server.jar --reports` 的 `reports/blocks.json` 生成的（block state id 运行时计算，不在 client.jar）。都**已提交到仓库**，开箱即可编译运行。

**仅当升级到别的 MC 版本时**才需要重新生成：

```bash
# 1) 改 scripts/gen_*.py 顶部的 JAR 路径指向新版本（client.jar / server.jar）
# 2) 重新生成 registry / tags / block states
python3 scripts/gen_registries.py > pkg/server/vanilla_data.go
python3 scripts/gen_tags.py       > pkg/server/vanilla_tags.go
BLOCKS_JSON=/path/to/reports/blocks.json python3 scripts/gen_blocks.py > pkg/server/block_states.go
# 3) 重新构建
go build ./...
```

### 验证构建

```bash
go vet ./...      # 静态检查
go test ./...     # 单元测试（VarInt/Position/压缩/chunk section）
```

---

## 使用方法

### 1. 启动服务端

**方式 A：二进制直接运行**

```bash
./bin/server -config config/config.toml
```

**方式 B：Docker（推荐，免装 Go）**

```bash
docker compose up -d --build       # 构建并后台运行
docker logs -f mc-server           # 看日志
docker rm -f mc-server             # 停止并清理
```

或手动：

```bash
docker build -f docker/Dockerfile.server -t zakla/mc-server:26.2 .
docker run -d --name mc-server -p 25565:25565 zakla/mc-server:26.2
```

服务端默认监听 `0.0.0.0:25565`。看到 `Minecraft server listening on 0.0.0.0:25565 (protocol 776, version 26.2)` 即就绪。

### 2. 配置（`config/config.toml`）

```toml
host = "0.0.0.0"          # 监听地址
port = 25565              # 监听端口
max_players = 20          # 最大玩家数
motd = "A Go Minecraft Server"   # 服务器列表显示文本

[network]
compression_threshold = 256      # 压缩阈值（字节），<=0 关闭压缩

[world]
name = "world"
seed = 123456789
difficulty = "normal"
gamemode = "survival"
hardcore = false
```

Docker 方式想改配置：编辑 `config/config.toml` 后 `docker compose up -d`（compose 挂载了配置文件）。

### 3. 连接

#### 方法 ①：mock 客户端自测（最快，无需 MC 客户端）

```bash
./bin/mock-client -host localhost -port 25565
```

会自动走完 `握手 → 登录 → 配置 → 进入游戏 → 收发聊天` 全流程，日志打印每一步。这是验证服务端协议正确性的最快方式。

#### 方法 ②：真实 MC 客户端（真正的玩法验证）

用任意启动器（HMCL / Prism 等）准备一个 **26.2** 的 Java Edition 实例，然后：

- **GUI 方式**：启动实例 → 多人游戏 → 直接连接 → 输入 `localhost` → 加入
- **Quick Play 方式**：启动器里设服务器地址 `localhost`，启动即自动连
- **命令行方式**（参考 `scripts/run_real_client.sh`）：
  ```bash
  # 需先 python3 scripts/gen_classpath.py 生成 /tmp/mc-cp.txt
  bash scripts/run_real_client.sh
  ```

> 服务端是 **offline 模式**（不校验正版），所以正版账号、离线账号都能连。

---

## 项目结构

```
cmd/
  server/           服务端入口
  mock-client/      测试客户端（端到端自测）
pkg/
  protocol/         协议原语：数据类型 / 包读写 / 压缩 / network NBT / 各阶段包
  net/              连接层：状态机（5 阶段）+ 压缩
  server/           应用层：状态机 / chunk / world / registry / tags / handler
    chunk.go          Chunk/ChunkSection + paletted container（单值/间接/直接）+ 位打包
    world.go          World 稀疏 chunk 存储 + SetBlock/GetBlock + 出生点平台
    block_states.go   从 Mojang reports 生成的 block state id 映射
    vanilla_data.go   生成的同步 registry 条目（30 个 registry）
    vanilla_tags.go   生成的 tags（15 个 tag registry）
  config/           TOML 配置
scripts/
  gen_registries.py   从 client.jar 生成 vanilla_data.go
  gen_tags.py         从 client.jar 生成 vanilla_tags.go
  gen_blocks.py       从 server.jar --reports 生成 block_states.go
  gen_classpath.py    真实客户端启动辅助：生成 classpath
  run_real_client.sh  真实客户端一键启动 + Quick Play 连接
  auto-debug.sh       自动化调试循环（占位）
docker/Dockerfile.server   多阶段构建
docker-compose.yml         一键部署
docs/ARCHITECTURE.md       架构 + 26.2 协议流程 + 踩坑点
```

## 架构

详见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)（含 26.2 完整协议流程图、Configuration 阶段、network NBT、chunk section 等踩坑点）。

分层：
- `pkg/protocol` —— 无状态协议原语
- `pkg/net` —— 连接状态机 + 压缩（`ReadPacket`/`WritePacket` 透明处理压缩）
- `pkg/server` —— 每连接状态机：`handle → handleStatus / handleLogin / handleConfiguration / handlePlay`

## 测试

```bash
go test ./...     # 单元测试
go vet ./...      # 静态检查
```

覆盖：VarInt 编码（逐字节对照 minecraft.wiki 样例）、Position、压缩往返、chunk section 字节布局。

## 路线图

- [x] 协议栈 + 状态机 + 压缩 + offline 登录
- [x] 配置阶段 + registry data + Update Tags
- [x] chunk 编码 + 空世界 + 聊天
- [x] mock 客户端端到端验证
- [x] vanilla 数据层（从 client.jar 生成）
- [x] 真实 MC 26.2 客户端进入空世界 + 聊天
- [x] 方块挖掘 MVP（Player Action → 破坏 → 广播 + Ack）
- [x] 方块放置（物品栏 + Use Item On → 放置 + 广播 + Ack）
- [x] Docker 镜像交付（21MB）
- [ ] 方块物理（重力/水）、多 chunk 视距、世界持久化（未来）

## 协议文档来源

- https://minecraft.wiki/w/Java_Edition_protocol/Packets （26.2 / PVN 776）

## License

MIT
