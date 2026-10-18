# GX-4 本地对局原型（Windows 可运行）

在 Windows 上可运行的本地对局原型：**Electron/TS 客户端 ＋ 本机运行的 Go 权威服务端**（localhost、JSON over WebSocket）。四人一桌完整一局（发牌 → 摸打 → 吃碰杠 → 胡牌/流局 → 终局展示）；**对局逻辑与界面分离、按服务器权威结构**，可直接用于下一 Cycle 的联机服务端。

## 结构

```
client/（Electron + TypeScript）
├─ electron/main.ts      主进程：窗口 + 拉起本机服务端（不碰对局逻辑）
├─ electron/preload.cjs  最小 preload（只暴露版本；对局数据不经 IPC）
├─ src/protocol/         ← tygo 生成（勿手改）：engine.ts / protocol.ts
├─ src/net.ts            SeatConnection：一条座位连接（浏览器/Node 的 WebSocket 同一 API）
├─ src/game.ts           SeatView：协议消息 → 可渲染模型（界面只读模型、只发意图，零规则）
├─ src/autoplay.ts       ActionSource：无策略随机合法动作（种子化）；真人/自动座/E2E 同一路径
├─ src/ui.ts             TableScreen（PixiJS 牌桌）+ Hud（DOM 按钮/日志/结算；按钮完全由服务器 actions 驱动）
├─ e2e/full-game.mjs     确定性 E2E：真服务端 + 4 条真 WebSocket 打完一局
└─ e2e/client-smoke.ts   真实客户端模块（net/game/autoplay）的 headless 整局冒烟

server/（Go 1.26）
├─ internal/engine/      纯逻辑引擎（GX-2，零改动消费）
├─ internal/score/       番种计分与一局结算（GX-3，零改动消费）
├─ internal/protocol/    消息集与编解码（Go 唯一源头 → tygo → TS）
├─ internal/table/       一桌会话深模块：座位绑定、动作入口、声明窗口、按座位可见性、终局收束
├─ internal/transport/   WebSocket 层：读/写泵（不懂协议内容、不懂座位、不懂规则）
└─ cmd/gameserver/       进程入口：--addr --path --seed --dealer --dump
```

设计说明（深模块与 seam）：四份并行接口设计的对比与定稿记录见 `docs/` 交付说明引用；一句话概括——`table` 是唯一的权威会话层（单串行环、engine 唯一接触者），每连接出站经有界通道（生产＝WS 写泵／测试＝直读），慢消费者只丢它自己。**本 Cycle 不建 `room`**（单桌下只是转发层；多桌随下一 Cycle 引入，`table` API 不变）。

说明：`tygo.yaml` 放在 `server/`（Go module 根）而非仓库根——tygo 以 Go 包解析为输入，需在 module 上下文运行（GX-1 §5.1 的落位在本工程实践中如此调整，命令见下）。

## 协议（消息表）

C→S：

| 消息 | 形状 |
|---|---|
| hello | `{"type":"hello","version":1,"seat":0}`（版本须为 1；重复绑定/座位占用/越界均被拒） |
| action | `{"type":"action","action":{"kind":"discard","tile":{"suit":0,"rank":1}}}` |

S→C：

| 消息 | 形状 |
|---|---|
| state | `{"type":"state","state":{"snapshot":{…},"actions":[…]}}`——该座位可见快照＋当前合法动作（按钮由它驱动）；终局（和牌/流局）时另携带 `"score"`（GX-3 计分与结算） |
| event | `{"type":"event","event":{"kind":"discarded","seat":0,"tile":{…}}}`——按可见性分发 |
| rejected | `{"type":"rejected","code":"illegal_action","reason":"还没轮到座位 0（当前 1）"}`——只回发起者，reason 为中文人话 |

kind 词表（动作）：`discard` `chi` `pong` `kongDiscard` `kongConcealed` `kongPromote` `win` `pass`；
（事件）：`discarded` `drawn` `meld` `kongPromoted` `win` `exhaustive`。
engine 载荷（快照/副子/结果/牌）原样复用其 JSON 形状；快照即 `engine.ViewFor(seat)`。

**可见性规则**（服务器权威）：完整状态与洗牌随机只在服务端；`drawn` 事件只发摸牌者本人；他人暗杠的牌面抹去；终局 `result` 全揭示。

**计分与结算**：终局由服务端调用 GX-3 的 `score.Score(Result, Context)`（圈风暂用东风圈常量），结果随终局 `state.score` 与完整记录下发；已知限制照 GX-3（见 `docs/score-rules.md` §6：含暗杠的手数口径、和绝张按保守子集判定）。

## 运行

前置：Go ≥ 1.26、Node ≥ 24（npm）。

```bash
cd client
npm install
npm run build          # 产出：resources/server/gameserver-host[.exe]、gameserver-win.exe、dist/、dist-electron/
npm start              # 或 npx electron .（自动拉起本机服务端并开窗）
```

- 座位 0 由人操作（点击手牌打牌；吃/碰/杠/胡/过用按钮）；座位 1–3 以无策略随机合法动作自动驱动（种子化）；终局结算面板展示胜者/和法/全揭示＋GX-3 计分（番种清单与分项、总番、起和线、四家得失分，零和）。
- 环境变量：`GX_PORT`（默认 8787）、`GX_SEED`（固定对局种子）。
- 弹窗环境（Linux 无显示）不可用——GUI 实机冒烟与 Windows 实机运行验证归 GX-5。

**Windows**：

- 出包：`npm run dist:win` —— electron-builder 打 Windows zip（内含 `resources/server/gameserver.exe`，客户端启动时自动拉起；找不到会提示先构建）。
- 或将 `resources/server/gameserver-win.exe` 与客户端分别为两步运行：先起服务端，再 `electron .`（渲染层连 `ws://127.0.0.1:8787/ws`）。
- 完整对局记录：客户端启动时服务端以 `--dump <userData>/last-game.json` 运行，主进程日志会在退出时打印该路径。

## 验证与复现（证据）

```bash
# 服务端测试（含 300 固定种子「双终局可达性」27 和/273 流、整局驱动、隐藏信息泄露断言、慢消费者、可见性单测）
cd server && go test ./... -count=1

# 确定性 E2E：真服务端 + 4 条真 WebSocket（同种子同结果）
cd client && npm run e2e 20261018   # → 流局（零和）
npm run e2e 6                       # → 座位 0 和牌（12 番，达起和线）
npm run e2e 13                      # → 座位 3 和牌（2 番，未达起和线）

# 客户端模块 headless 冒烟：真实 net/game/autoplay 模块跑完一局并与服务端 dump 互证
npm run smoke 6
```

**完整对局记录（--dump）**：服务端 `--dump <路径>` 在一局结束写出人工可读 JSON：

| 字段 | 内容 |
|---|---|
| `seed` / `dealer` | 对局种子与庄家（同种子＋动作序＝完全重演） |
| `actions[]` | 按序的每一步入站动作（座位、动作、成功／被拒原因）——含被拒 |
| `events[]` | 引擎事件日志（服务端视角，含摸牌、暗杠等隐藏信息） |
| `result` | 终局全揭示：四家手牌/副子、和牌分解 `decompositions`、上下文 `context` |
| `score` | GX-3 计分与结算：番种清单与分项 `best.fans`、总番 `best.total`、起和线 `eligible`、四家得失分 `settlement.deltas`（零和；东风圈常量） |

用途：GX-5 实机验证的规则对照基准（界面只有本人视角；`score` 字段即 GX-3 计分与结算的完整输出，可供逐项核对）。

## 与下一 Cycle 的接口

- 服务端换网络部署：`cmd` 的监听参数与将来鉴权适配器；`table`/`protocol` 不变。
- 自动座换真人连接：同一协议路径；座位改由服务端按 Steam 票据定座（`hello.seat` 变请求）。
- 计分已接入 GX-3（`score.Score`，东风圈常量；含暗杠等已知限制见 score-rules.md §6）；圈风轮转与跨局结算随下一 Cycle（面板与协议已按此预留）。

## 本原型的边界（经 Assignee 确认）

计分与结算消费 GX-3（东风圈常量；圈风轮转/跨局结算不在本次）；不做美术与音效正式制作（最简占位）；不含联机部署与 Steam 接入；不含 AI 决策（自动座仅为无策略随机合法动作驱动）。