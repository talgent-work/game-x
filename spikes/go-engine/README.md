# GX-1 最小实证 4a：Go 纯逻辑层 + 分离的调用方

**证明什么**：对局逻辑可以作为**纯逻辑层**独立存在，与界面、网络零耦合；"服务器权威"在类型层面就能成立——完整状态只进不出，对外只有按座位生成的快照（**隐藏他人手牌**）。本机原型里的"本机服务端"与将来的权威服务端，都只是"与逻辑层分离的调用方"。

**目录**

| 路径 | 内容 |
|---|---|
| `engine/` | 纯逻辑骨架：牌型、136 张牌墙、发牌、摸打、按座位出快照 |
| `cmd/table/` | 分离的调用方演示（无界面、无网络依赖） |
| `engine/engine_test.go` | 5 项最小测试，含"视图不泄露他人手牌"断言 |

无第三方依赖（仅 Go 标准库）。

**复现步骤**（Linux/macOS/Windows 均可，需 Go ≥ 1.26）

```bash
cd spikes/go-engine
gofmt -l .        # 应无输出
go vet ./...      # 应无输出
go test -v ./...
go run ./cmd/table
```

**运行输出记录**（2026-10-17 实测于本工作环境：go1.26.6 linux/amd64，`gofmt`/`go vet` 均无输出）

测试：

```
=== RUN   TestWallSize
--- PASS: TestWallSize (0.00s)
=== RUN   TestTileString
--- PASS: TestTileString (0.00s)
=== RUN   TestDeal
--- PASS: TestDeal (0.00s)
=== RUN   TestViewHidesOtherHands
--- PASS: TestViewHidesOtherHands (0.00s)
=== RUN   TestDrawDiscardRotatesTurn
--- PASS: TestDrawDiscardRotatesTurn (0.00s)
PASS
ok      talgent-work/game-x/spikes/go-engine/engine    0.005s
```

演示（种子 `20261017`，输出确定）：

```
== GX-1 最小实证 4a：Go 纯逻辑层 + 分离的调用方（无界面、无网络依赖） ==

-- 发牌后，从各座位视角读快照 --
座位 0 视角：手牌 13 张 [1万 2万 3万 5万 9万 1筒 3筒 3筒 5筒 7筒 北 白 白] ｜ 牌墙余 84 ｜ 轮到 0
座位 1 视角：手牌 13 张 [1万 5万 7万 1筒 2筒 2筒 6筒 9筒 2条 4条 7条 北 北] ｜ 牌墙余 84 ｜ 轮到 0
座位 2 视角：手牌 13 张 [3万 6万 6万 7万 8万 8万 2筒 3筒 4筒 8筒 8筒 2条 5条] ｜ 牌墙余 84 ｜ 轮到 0
座位 3 视角：手牌 13 张 [3万 6万 8万 9万 1筒 3筒 7筒 8筒 3条 8条 东 西 中] ｜ 牌墙余 84 ｜ 轮到 0

-- 座位 0 摸入 9条 并打出 --
座位 1 视角（座位 0 打牌后）：手牌 13 张；公开牌河[座位 0] [9条]；牌墙余 83；轮到 1
（注意：座位 1 的快照里没有任何别家手牌——隐藏信息留在逻辑层内部）

-- 座位 1 快照的 JSON 载荷（将来经 WebSocket 发给该客户端的内容形状）--
```

JSON 载荷片段（完整内容由运行打印；固定种子下确定）：

```json
{
  "seat": 1,
  "hand": [ {"suit": 0, "rank": 1}, {"suit": 0, "rank": 5}, "…共 13 张（仅本人手牌）" ],
  "discards": [ [ {"suit": 2, "rank": 9} ], null, null, null ],
  "wallCount": 83,
  "turn": 1
}
```

**边界说明**：本目录是"最小实证"，不发展成产品代码——吃碰杠、胡牌/流局判定属 GX-2，番种计分属 GX-3，本地原型组装属 GX-4。正式工程代码将按《技术选型报告》的目录约定另起。