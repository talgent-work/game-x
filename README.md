# game-x

国标麻将在线对战游戏（Steam · Windows · 简体中文 · 2D）。

- 技术选型基线（先读）：[docs/tech-selection.md](docs/tech-selection.md)
- 对局引擎（国标一局：发牌/摸打/吃碰杠/胡流局）：[server/internal/engine](server/internal/engine)，说明见 [docs/engine.md](docs/engine.md)
- 本地对局原型（GX-4；Electron 客户端 + 本机 Go 权威服务端）：[docs/prototype.md](docs/prototype.md)，客户端在 [client/](client/)
- 最小实证（不进产品代码）：[spikes/](spikes/)