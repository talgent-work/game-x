# GX-1 最小实证 4b：Node↔Go 协议回环（JSON over WebSocket）

**证明什么**：选定的跨语言联机方案端到端可跑通——Go 权威服务端（`coder/websocket` v1.8.15）↔ TypeScript/Node 客户端（`ws` 8.22.0），JSON 消息往返；**服务端是唯一的规则执行者**：客户端只发"意图"，服务端校验后回执；非法操作被权威拒绝。协议直接复用 4a 引擎的牌型与快照 JSON 形状；`go-server` 通过 `replace` 引用 `../../go-engine`（同仓复用模块的形态）。

**协议（本次回环的消息形状）**

| 方向 | 消息 |
|---|---|
| C→S | `{"type":"hello","seat":0}` |
| S→C | `{"type":"snapshot","snapshot":{"seat":0,"hand":[…],"discards":[null,null,null,null],"wallCount":84,"turn":0}}` |
| C→S | `{"type":"action","action":{"kind":"discard","tile":{"suit":0,"rank":1}}}` |
| S→C | `{"type":"discarded","tile":{"suit":0,"rank":1},"turn":1,"wallCount":84}` 或 `{"type":"rejected","reason":"不是你的回合（当前轮到座位 1）"}` |

**复现步骤**（Linux/macOS/Windows；需 Go ≥ 1.26、Node ≥ 23.6——客户端直接运行 `.ts`，无编译步骤）

```bash
# 终端 1：服务端
cd spikes/ts-go-loopback/go-server
go run .                # 监听 127.0.0.1:8787，端点 /ws

# 终端 2：客户端
cd spikes/ts-go-loopback/ts-client
npm install
node client.ts
```

**运行输出记录**（2026-10-17 实测：go1.26.6 linux/amd64 + Node v24.21.0；`gofmt`/`go vet`/`go mod tidy` 无警告）

客户端输出：

```
已连接 ws://127.0.0.1:8787/ws
→ {"type":"hello","seat":0}
← {"type":"snapshot","snapshot":{"seat":0,"hand":[{"suit":0,"rank":1},{"suit":0,"rank":2},{"suit":0,"rank":3},{"suit":0,"rank":5},{"suit":0,"rank":9},{"suit":1,"rank":1},{"suit":1,"rank":3},{"suit":1,"rank":3},{"suit":1,"rank":5},{"suit":1,"rank":7},{"suit":3,"rank":4},{"suit":4,"rank":3},{"suit":4,"rank":3}],"discards":[null,null,null,null],"wallCount":84,"turn":0}}
   （快照：座位 0，手牌 13 张，牌墙余 84，轮到 0）
→ {"type":"action","action":{"kind":"discard","tile":{"suit":0,"rank":1}}}
← {"type":"discarded","tile":{"suit":0,"rank":1},"turn":1,"wallCount":84}
→ {"type":"action","action":{"kind":"discard","tile":{"suit":0,"rank":2}}}
← {"type":"rejected","reason":"不是你的回合（当前轮到座位 1）"}
   （服务端按权威规则拒绝：不是你的回合（当前轮到座位 1））
连接关闭，回环完成
```

服务端日志（节选）：

```
2026/10/17 20:17:23 客户端已连接: 127.0.0.1:43908
2026/10/17 20:17:23 ← {"type":"hello","seat":0}
2026/10/17 20:17:23 → {"type":"snapshot","snapshot":{…"wallCount":84,"turn":0}}
2026/10/17 20:17:23 ← {"type":"action","action":{"kind":"discard","tile":{"suit":0,"rank":1}}}
2026/10/17 20:17:23 → {"type":"discarded","tile":{"suit":0,"rank":1},"turn":1,"wallCount":84}
2026/10/17 20:17:23 ← {"type":"action","action":{"kind":"discard","tile":{"suit":0,"rank":2}}}
2026/10/17 20:17:23 → {"type":"rejected","reason":"不是你的回合（当前轮到座位 1）"}
2026/10/17 20:17:23 连接结束: …
```

**边界**：最小样例——一张桌、单连接、三个消息类型。多桌房间管理、协议版本化、断线重连、鉴权（Steam 会话票据）等均不在此，正式约定见《技术选型报告》。