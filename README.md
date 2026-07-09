# zakla — Go Minecraft 26.2 服务端

用 Go 从零实现的 Minecraft Java Edition **26.2**（协议版本 **776**）服务端。

## 当前状态：✅ 真实客户端可进入空世界并聊天

| 能力 | 状态 | 验证方式 |
|------|------|---------|
| 完整协议栈（PVN 776） | ✅ | 单元测试（VarInt 逐字节对照 minecraft.wiki） |
| 状态机：握手→状态→登录→配置→游戏 | ✅ | mock + 真实客户端 |
| zlib 压缩协商 | ✅ | mock + 真实客户端 |
| offline 模式登录 | ✅ | mock + 真实客户端 |
| 配置阶段（known packs + 完整同步 registry + Update Tags） | ✅ | 真实客户端 |
| chunk 编码（含 1.21.5+ Fluid count） | ✅ | 真实客户端进入世界 |
| **vanilla 数据层（registry + tags，从 client.jar 生成）** | ✅ | 真实客户端 |
| **进入空世界 + 聊天** | ✅ | **真实 MC 26.2 客户端验证** |

真实 MC 26.2 客户端（HMCL 实例 + Temurin Java 25）已成功：完成握手/登录/配置、进入空世界、收发聊天。

## 快速开始

```bash
go build -o bin/server ./cmd/server
go build -o bin/mock-client ./cmd/mock-client
./bin/server -config config/config.toml          # 另开终端
./bin/mock-client -host localhost -port 25565      # 端到端自测
```

## 测试

```bash
go test ./... && go vet ./...
```

## 架构

详见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

```
cmd/server        服务端入口
cmd/mock-client   测试客户端（端到端自测）
pkg/protocol      数据类型/包读写/压缩/NBT(network)/各阶段包
pkg/net           连接状态机 + 压缩
pkg/server        状态机 + chunk + registry + tags + handler
scripts/gen_registries.py   从 client.jar 生成同步 registry 数据
scripts/gen_tags.py         从 client.jar 生成 tags 数据
```

## vanilla 数据层

26.2 客户端在配置阶段末尾校验完整的同步 registry + Update Tags。本项目从 26.2 client.jar 的 `data/` 提取真实 vanilla 数据生成 Go 数据文件（`pkg/server/vanilla_data.go`、`vanilla_tags.go`）：

```bash
python3 scripts/gen_registries.py > pkg/server/vanilla_data.go
python3 scripts/gen_tags.py       > pkg/server/vanilla_tags.go
```

## Docker 部署

```bash
docker compose up -d --build          # 构建并后台运行
# 或手动：
docker build -f docker/Dockerfile.server -t zakla/mc-server:26.2 .
docker run -d --name mc-server -p 25565:25565 zakla/mc-server:26.2
docker logs mc-server                 # 看日志
docker rm -f mc-server                # 停止清理
```

镜像约 21MB。容器监听 25565，用任意 26.2 客户端连 `localhost:25565` 即可。

## 用真实客户端验证

```bash
./bin/server -config config/config.toml
# 用任意启动器的 26.2 实例 Quick Play 连 localhost:25565
bash scripts/run_real_client.sh   # 参考脚本
```

## 路线图

- [x] 协议栈 + 状态机 + 压缩 + offline 登录
- [x] 配置阶段 + registry data + Update Tags
- [x] chunk 编码 + 空世界 + 聊天
- [x] mock 客户端端到端验证
- [x] vanilla 数据层（registry + tags，从 client.jar 生成）
- [x] **真实 MC 26.2 客户端进入空世界 + 聊天**
- [x] **Docker 镜像交付**（`docker compose up`，镜像 21MB）

## 协议文档来源

- https://minecraft.wiki/w/Java_Edition_protocol/Packets （26.2 / PVN 776）

## License

MIT
