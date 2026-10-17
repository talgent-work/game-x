# 技术选型报告（GX-1）

**版本** v1.0 · **日期** 2026-10-17 · **状态** 交付稿
**范围**：Game X（国标麻将在线对战 · Steam · Windows · 简体中文 · 2D）的技术栈基线。本报告是 Cycle「本地可玩的核心原型」的起点，GX-2～GX-5 以本报告为准开工。

**已由 Assignee 指定的基线**（不在本报告论证范围）：客户端 TypeScript/Node；服务端 Go。

**一句话架构**：**瘦客户端（只渲染、只发意图）+ Go 权威服务端（全部规则与隐藏信息）**。本 Cycle 服务端以本机进程形式运行，下一 Cycle 原样搬上服务器。

---

## 0. 结论速览

| 维度 | 结论 |
|---|---|
| ① 客户端 | **Electron 44 + TypeScript 7 + PixiJS 8.22（牌桌渲染）+ DOM（常规 UI）**；Vite 构建，electron-builder 打包；禁用自更新、更新交给 Steam depot |
| ② 服务端与联机 | **Go 1.26 + coder/websocket v1.8.15**；**自建房间管理器**；**JSON over WebSocket**（消息 `{"type":…, …payload}`）；单静态二进制部署 |
| ③ Steam 接入 | **steamworks.js 0.4.0**（Electron）。首发接：成就、Steam 云、Overlay、**会话票据认证**、（可选）大厅/邀请；上线流程与费用见 §4.3 |
| ④ 代码组织 | **单仓** `server/`(Go) + `client/`(TS/Electron) + `protocol/`：**Go 结构体为唯一源头，tygo 生成 TS 类型** |

---

## 1. 前提与硬约束

- **产品**：国标麻将、四人一桌、在线对战；首发 Windows/Steam、简体中文、2D 简洁路线（来源：项目方向「程序与工程」「美术与音效」边界）。
- **工程约束**：服务器权威；对局逻辑与界面分离；为下一 Cycle 接联机预留结构（来源：Cycle「本地可玩的核心原型」目标与 GX-4）。
- **团队**：极小（1–2 人）——因此"少依赖、少运维、不造轮子"是贯穿全篇的取舍原则。
- **本报告不做**：产品级实现（GX-2～GX-5 之责）；美术/音效选型；商店页与发行操作；付费与账号办理（仅列清单，由 Assignee 决定）。

---

## 2. 客户端技术（结论：Electron + PixiJS + DOM）

### 2.1 外壳：Electron（对比 Electron / NW.js / Tauri）

| 候选 | 现状（核实于 2026-10-17） | 判定 |
|---|---|---|
| **Electron 44** | 最新稳定 44.9.0；内嵌 Chromium 152 / Node 24；每 8 周一个大版、官方支持最近 3 个大版本；MIT | ✅ **选定** |
| NW.js | v0.118.0（2026-10-10 发版；Node 26.7 / Chromium 155）——仍在活跃发版 | 否决：生态与工具链规模比 Electron 小一号；steamworks.js 两者都支持，但社区答案、打包工具（electron-builder）、Steam 成功案例以 Electron 为主 |
| Tauri | Rust 内核 + 系统 WebView，**不跑 Node** | 否决：与"客户端 TS/Node"基线直接冲突 |

**理由**：客户端基线是"TS/Node 桌面应用"，Electron 是该形态的事实标准：用 electron-builder 一条命令出 Windows 包；调试、崩溃报告链路成熟；Steam 上先例最多。NW.js 可作为备选保留（切换成本低，外壳很薄）。
来源：[Electron 发布节奏与支持政策](https://www.electronjs.org/docs/latest/tutorial/electron-timelines) · [endoflife.date/electron](https://endoflife.date/electron) · [NW.js](https://github.com/nwjs/nw.js) · [npm electron](https://www.npmjs.com/package/electron)

### 2.2 渲染：PixiJS + DOM（对比 PixiJS / Phaser / 原生 Canvas）

| 候选 | 现状 | 判定 |
|---|---|---|
| **PixiJS 8.22.0** | WebGL + WebGPU 2D 渲染引擎、TS 原生、MIT、48.4k stars、活跃 | ✅ **选定**（牌桌渲染） |
| Phaser 4.2.1 | 全功能游戏框架（场景/物理/音频/资源管理） | 否决：麻将桌不需要物理与游戏循环那一套，框架越大升级成本越高（若日后需要"全家桶"，仍可回头） |
| 原生 Canvas / DOM | 零依赖 | 否决：绘制、命中测试、动画全部自写，人力成本不划算 |

**分工**：牌桌、牌面、动画用 **PixiJS**；大厅/按钮/设置/结算等常规界面用 **DOM（HTML/CSS）**——这类 UI 用 DOM 开发最快、最好维护。
**理由**：本作是"重表现、轻物理"的桌面牌类游戏，PixiJS 正为此设计。
来源：[PixiJS 仓库](https://github.com/pixijs/pixijs) · [npm pixi.js](https://www.npmjs.com/package/pixi.js) · [npm phaser](https://www.npmjs.com/package/phaser)

### 2.3 构建、打包与更新（结论）

- 构建：**Vite 8.3.4** + **TypeScript 7.0.2**（对比 webpack：Vite 是当前新项目的默认选择，快且配置少；版本在工程启动时锁定复核）。
- 打包：**electron-builder 26.15.3**（Windows 目标，出绿色包/安装包）。
- 更新：**禁用 electron-updater**，游戏更新交给 **Steam depot**（避免两套更新机制互相打架）。
- Steam 相关：发布构建按 steamworks.js 要求把 `sdk/redistributable_bin` 文件拷入产物；overlay 在主进程调用 `electronEnableSteamOverlay`；native 模块只在主进程/preload 使用、经 IPC 向渲染层暴露最小接口。
来源：[npm electron-builder](https://www.npmjs.com/package/electron-builder) · [npm vite](https://www.npmjs.com/package/vite) · [steamworks.js](https://github.com/ceifa/steamworks.js)

---

## 3. 服务端与联机方案（结论：Go + coder/websocket + 自建房间 + JSON）

### 3.1 WebSocket 库：coder/websocket v1.8.15（对比 gorilla / gobwas）

| 候选 | 现状（Go module proxy） | 判定 |
|---|---|---|
| **coder/websocket v1.8.15** | 发版 2026-06-15；nhooyr 2019–2024 开发、后由 Coder 接棒；极简 API、context 优先、零依赖、过 Autobahn 一致性测试 | ✅ **选定** |
| gorilla/websocket v1.5.3 | 发版 2024-06-14；老牌、资料最全（24.9k stars） | 否决：相对停更（最近发版停留在 2024 年）；功能上完全可用 |
| gobwas/ws | 更底层（缓冲区自管） | 否决：不值得为这点性能自加复杂度 |

来源：[coder/websocket](https://github.com/coder/websocket) · [gorilla/websocket](https://github.com/gorilla/websocket) · 版本时间戳见附录 B（proxy.golang.org）。

### 3.2 房间与对局管理：自建（对比 自建 / Nakama / Pitaya）

- **现实需求**：四人一桌、回合制、消息低频；单进程单机可支撑相当规模。
- **Nakama**（全功能游戏后端：账号/社交/存储/匹配/排行榜）与 **Pitaya**（分布式多节点房间路由）：能力大多不在需求面上，却要背运行模型、学习与运维成本 → **否决**。
- **结论**：自建"房间管理器"（数百行量级）：`map[roomID] → 桌`；每桌一个 goroutine 串行处理动作；连接读/写泵分离。这是回合制服务端最稳、最好排障的模型。

### 3.3 协议：JSON over WebSocket（对比 JSON+tygo / protobuf）

- **结论**：JSON 文本消息，形如 `{"type":"…", …payload}`。消息低频（每秒几条），可读性对调试与联机排障的价值远大于体积/编解码收益。
- **跨语言类型不漂移**：Go 结构体是唯一源头，用 **tygo** 生成 TS 类型（§5.3）。**protobuf 否决**：强 schema/紧凑编码在此量级无收益，却先付出 .proto 工具链成本（将来热路径需要时可在编解码层替换，不动其它）。
- **关键约定**：
  1. **隐藏信息只在服务端**：任何 S→C 消息只含该座位可见数据（快照形状即协议载荷，见实证 4a/4b）；洗牌随机数只在服务端。
  2. 客户端只发**意图**（action）；服务端校验后回执（`discarded`…）或拒绝（`rejected{reason}`）。客户端不做规则推算、不预测状态。
  3. **版本**：连接建立时 `hello` 携带协议版本（v1）；演进以"加字段"为主，不破坏旧客户端。
  4. **鉴权**（下一 Cycle 联机上线时启用）：`hello` 附带 Steam 会话票据；服务端经 Steamworks Web API 校验后绑定 SteamID 入座（§4.2）。

### 3.4 部署（结论）

- **单个静态二进制**（`CGO_ENABLED=0`）部署到小 VPS，systemd 托管；标准库结构化日志起步。
- 否决：Docker-first（本阶段无必要）、托管平台/编排（过度）；TLS 用反向代理（Caddy/nginx）终结，服务端只监听内网。
- 下一 Cycle 需要时加：心跳、断线重连、多桌容量压测——均不改本选型。

---

## 4. Steam 接入

### 4.1 绑定库：steamworks.js 0.4.0（对比 greenworks / 自制绑定）

| 候选 | 现状 | 判定 |
|---|---|---|
| **steamworks.js 0.4.0** | 活跃；MIT；Rust + N-API；npm 分发；明确支持 Electron/NW.js | ✅ **选定** |
| greenworks | 项目自述"best-effort 维护，active development is not a priority" | 否决：维护停滞 |
| 自制绑定（FFI / 自写 N-API） | — | 否决：纯成本 |

**能力面**（核自其 `client.d.ts`）：`init/runCallbacks`；`achievement`、`cloud`、`auth`（**会话票据**）、`overlay`、`matchmaking`（大厅/邀请）、`stats`、`input`、`localplayer`、`networking`(P2P)、`workshop` 等——首发所需全部覆盖。
来源：[steamworks.js 仓库](https://github.com/ceifa/steamworks.js) · [client.d.ts](https://raw.githubusercontent.com/ceifa/steamworks.js/main/client.d.ts) · [greenworks](https://github.com/greenheartgames/greenworks)

### 4.2 首发接入功能（结论）

- **接**：成就（achievement）、Steam 云存档（cloud）、Steam Overlay（含好友/邀请入口）、**会话票据认证**（auth：客户端取票据 → 我们的服务端经 Steamworks Web API 校验 → 绑 SteamID 入座）。
- **可选/后置**：大厅与邀请（matchmaking）在联机 Cycle 按匹配需求评估；Steam Input 留待 Deck 决策。
- **不接**：内购/微交易、创意工坊（UGC）、P2P 联机（我们自建权威服务端）。
来源：[Steam 鉴权文档（Session Tickets）](https://partner.steamgames.com/doc/features/auth)

### 4.3 上线流程与前置清单（费用/时序；由 Assignee 决定与办理）

1. **Steamworks 合作伙伴注册**（数字文书）。**Steam Direct 费：$100 USD/款**——不可退；产品调整后总收入满 **$1,000** 后随月度付款返还；需带 Admin 权限的账号支付。
2. 创建 AppID → 完成商店页清单 → **提交商店页审查（通常 3–5 个工作日；建议至少提前 7 天提交）** → 上线 Coming Soon → 发售前完成构建审查 → 发售。发布 Coming Soon / 发售需要两个权限：`Publish app changes`、`Manage pricing and discounts`。
3. 开发期自测：`steam_appid.txt` 填 **480**（官方示例应用 SpaceWar）；该文件**不得**进入发布构建。**本 Cycle 不需要 AppID**。
4. 建议尽早启动账号/费用办理（外部提前期长）。商店页最短 Coming Soon 提前期以 Steamworks 现行文档为准（本报告未取到原文）。

来源：[Steam Direct Fee](https://partner.steamgames.com/doc/gettingstarted/appfee) · [Release Process](https://partner.steamgames.com/doc/store/releasing) · [Steamworks API（steam_appid.txt/SpaceWar）](https://partner.steamgames.com/doc/sdk/api)

---

## 5. 代码组织（可执行约定）

### 5.1 单仓布局（结论；对比多仓：小团队多仓只增加协调成本）

```
game-x/
├─ server/                    # Go 权威服务端（本 Cycle 以本机进程运行）
│  ├─ go.mod
│  ├─ cmd/gameserver/         # 进程入口：配置、装配、监听
│  └─ internal/
│     ├─ engine/              # 纯逻辑：牌/牌墙/发牌/摸打/吃碰杠/胡牌流局（GX-2 填充）
│     ├─ score/               # 番种识别与计分，纯逻辑（GX-3 填充）
│     ├─ table/               # 一桌对局：状态机、动作校验、出快照（服务器权威落点）
│     ├─ room/                # 房间/大厅：建桌、入座、生命周期
│     ├─ protocol/            # 消息结构体（Go 为唯一源头）
│     └─ transport/           # WebSocket 连接层：读/写泵、连接生命周期
├─ client/                    # Electron + TypeScript
│  ├─ package.json
│  ├─ electron/               # 主进程 + preload（steamworks.js 只在此层）
│  └─ src/
│     ├─ protocol/            # ← tygo 生成，勿手改
│     ├─ net/                 # WS 客户端：连接、重连、消息路由
│     ├─ game/                # PixiJS 牌桌渲染
│     └─ ui/                  # DOM UI：大厅/设置/结算
├─ tygo.yaml                  # 类型生成配置
├─ docs/tech-selection.md     # 本报告
└─ spikes/                    # 最小实证（不进产品代码）
```

### 5.2 逻辑模块接口形态（以实证 4a 为样）

- `engine` 纯逻辑、无网络/界面依赖；状态对内，**对外只有 `ViewFor(seat) Snapshot`**——`Snapshot` 的 JSON 形状即协议载荷；"隐藏他人手牌"由类型收口。
- 动作流：客户端 `action` → `table` 层校验（回合、合法性）→ `engine` 变更状态 → 回执/事件（`discarded` / `drawn` / `rejected`…）。
- GX-2 在 `engine` 内补全规则；GX-3 在 `score` 内实现计分（保持纯逻辑、可单测）；GX-4 的组装 = Electron 客户端 + **本机 Go 服务端进程**（localhost，协议与实证 4b 同形）。

### 5.3 跨语言类型：Go 结构体 → tygo → TS（结论）

- `server/internal/protocol` 为唯一源头；`tygo.yaml` 输出到 `client/src/protocol`；`tygo generate` 生成；CI 检查"生成后无 diff"即通过（漂移即报错）。
- 工具：[tygo v0.2.21](https://github.com/gzuidhof/tygo)（2026-02 发版；MIT）。对比：手写双份类型（必漂移，否决）、protobuf codegen（§3.3 已否决其成本）。

### 5.4 版本清单（工程启动时复核并锁定）

Electron 44.9.x（内嵌 Node 24）· TypeScript 7.0.2 · Vite 8.3.4 · PixiJS 8.22.0 · electron-builder 26.15.3 · ws 8.22.0 · steamworks.js 0.4.0 ｜ Go 1.26 · coder/websocket v1.8.15 · tygo v0.2.21

---

## 6. 最小实证（验收标准 4 的证据）

| 实证 | 证明了什么 | 证据位置（含复现步骤与运行输出） |
|---|---|---|
| **4a** Go 纯逻辑层 + 分离调用方 | 逻辑与界面/网络零耦合；按座位出快照、**不泄露他人手牌**（测试断言）；5 项测试全过 | `spikes/go-engine/README.md` |
| **4b** Node↔Go 协议回环 | coder/websocket ↔ ws 的 JSON 往返；合法动作执行、非法动作被权威拒绝；`go-server` 跨模块复用 `go-engine` | `spikes/ts-go-loopback/README.md` |

实测环境：go1.26.6 / Node v24.21.0（linux/amd64）。提交：`f8e0fbf`（4a）、`f676ed6`（4b），分支 `talgent/GX-1`。

---

## 7. 被否决方案汇总（一句话理由）

| 候选 | 场景 | 否决理由 |
|---|---|---|
| Unity / Godot | 客户端 | 与"客户端 TS/Node"基线冲突 |
| Tauri | 外壳 | 不跑 Node，基线冲突 |
| NW.js | 外壳 | 活跃但生态小一号（备选保留，切换成本低） |
| Phaser | 渲染 | 全功能框架超出需求（备选保留） |
| 原生 Canvas/DOM | 渲染 | 绘制/命中/动画全自写，人力不划算 |
| C#/.NET / Node | 服务端 | 与"服务端 Go"基线冲突 |
| protobuf | 协议 | 本量级无收益、先付工具链成本（可在编解码层后换） |
| Nakama / Pitaya | 房间 | 全功能后端/多节点路由，超出需求 |
| gorilla/websocket v1.5.3 | WS 库 | 功能可用但相对停更（发版停在 2024-06） |
| gobwas/ws | WS 库 | 过低层，复杂度不划算 |
| greenworks | Steam 绑定 | 自述 best-effort、停更 |
| 自制 Steam 绑定 | Steam 绑定 | 纯成本 |
| 多仓 | 组织 | 小团队只增协调成本 |
| 手写双份类型 | 类型共享 | 必漂移 |
| electron-updater | 更新 | 与 Steam depot 双更新打架，禁用 |

---

## 8. 对 GX-2～GX-5 的直接指引

- **GX-2（对局引擎）**：`server/internal/engine`（Go、纯逻辑），参照 `spikes/go-engine` 的形状（动作/状态/视图分离）。
- **GX-3（番种计分）**：`server/internal/score`（Go、纯逻辑、可单测）。
- **GX-4（本地原型）**：Electron + TS 客户端 + **本机 Go 服务端进程**；协议与实证 4b 同形；UI = PixiJS + DOM。
- **GX-5（实机验证）**：按 §5/§6 的落点验证；问题清单按模块归属。
- **下一 Cycle（联机）**：服务端二进制原样部署 + Steam 票据鉴权 + 断线重连 + 多桌——结构已预留。

---

## 9. 开放问题与风险

1. **Windows 实机验证**（打包、overlay 行为、性能）留在 GX-4/GX-5；本工作环境为 Linux 容器，未做 Windows 实机。
2. **Steam 账号/费用/AppID**：由 Assignee 办理（§4.3 清单）；商店页最短 Coming Soon 提前期以现行文档为准。
3. **TypeScript 7 与 PixiJS/Vite/electron-builder 兼容**：工程启动时以实测锁定；若遇摩擦，回退同线旧版即可（结构不受影响）。
4. **协议演进细则**（版本、加字段规则、错误码）在联机 Cycle 细化；JSON→protobuf 可后置替换。
5. **服务器托管地区/费用**：由 Assignee 决定（本工作不代办）。

---

## 附录 A：一手来源清单

- Electron：https://www.electronjs.org/docs/latest/tutorial/electron-timelines ；https://endoflife.date/electron ；https://www.npmjs.com/package/electron
- NW.js：https://github.com/nwjs/nw.js
- PixiJS：https://github.com/pixijs/pixijs ；https://www.npmjs.com/package/pixi.js ；Phaser：https://www.npmjs.com/package/phaser
- Vite / electron-builder：https://www.npmjs.com/package/vite ；https://www.npmjs.com/package/electron-builder
- coder/websocket：https://github.com/coder/websocket ；gorilla/websocket：https://github.com/gorilla/websocket
- tygo：https://github.com/gzuidhof/tygo
- steamworks.js：https://github.com/ceifa/steamworks.js （`client.d.ts`：https://raw.githubusercontent.com/ceifa/steamworks.js/main/client.d.ts ）；greenworks：https://github.com/greenheartgames/greenworks
- Steam 文档：Steam Direct Fee https://partner.steamgames.com/doc/gettingstarted/appfee ；Release Process https://partner.steamgames.com/doc/store/releasing ；鉴权（Session Tickets）https://partner.steamgames.com/doc/features/auth ；Steamworks API https://partner.steamgames.com/doc/sdk/api
- 版本时间戳：Go module proxy（https://proxy.golang.org/…/@latest ）；npm registry（https://registry.npmjs.org/…/latest ）

## 附录 B：版本与日期快照（2026-10-17 核实）

| 组件 | 版本 | 备注（发版/支持） |
|---|---|---|
| Electron | 44.9.0 | 44 线：2026-08-25 起；Chromium 152 / Node 24；支持最近 3 个大版本 |
| NW.js | 0.118.0 | 2026-10-10；Chromium 155 / Node 26.7 |
| TypeScript | 7.0.2 | — |
| Vite | 8.3.4 | — |
| PixiJS | 8.22.0 | MIT；WebGL+WebGPU |
| Phaser | 4.2.1 | — |
| electron-builder / electron-updater | 26.15.3 / 6.8.9 | — |
| ws | 8.22.0 | 实证 4b 客户端所用 |
| steamworks.js | 0.4.0 | MIT；Electron/NW.js |
| Go（环境） | 1.26.6 | 本工作环境实测 |
| coder/websocket | v1.8.15 | 2026-06-15 |
| gorilla/websocket | v1.5.3 | 2024-06-14 |
| tygo | v0.2.21 | 2026-02-03 |
| Node（环境） | v24.21.0 | 直接运行 .ts（类型剥离） |