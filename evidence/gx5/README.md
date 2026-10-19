# GX-5 实机验证证据（talgent/GX-5 分支）

《实机验证报告》见 `docs/verify-report.md`（同内容为 GX-5 Intent 的 Artifact）。本目录是其证据与工具。

## 目录

- `final/`——细验局完整对局记录（--dump JSON，含 score）：
  - `seed-6-selfdraw.json`：吃＋碰＋明杠＋自摸（12 番；R2 样本：全求人×自摸）
  - `seed-13-discard.json`：暗杠＋点炮（2 番；**R1 样本**：未达 8 番起和线仍判和结算）
  - `seed-93-discard.json`：补杠＋点炮（**R3 样本**：单钓将漏计，8 番实应 9）
  - `seed-1067-selfdraw.json`：补杠＋自摸（10 番；R2 样本）
  - `seed-20261018-draw.json`：流局（零和结算）
  - （Assignee Windows 人工试玩局：见 GX-5 Intent 附件）
- `paths/`——规则路径代表局（每条路径一局，含庄家开局暗杠种子 2075、杠上开花种子 2626）
- `probe-illegal.log`——非法动作/协议探测 12 用例全程输出（12/12 通过）
- `../gx5-tools/`——验证工具（不依赖本仓库产品代码，只消费 dump JSON 与真实服务端；含 `run_game_biased.mjs`——N4 的偏置驱动实验，证实碰/杠优先反而 0 和牌）：
  - `check_dump.py`：流程回放核对器（轮转/吃碰杠时机/杠尾补牌/终局收束/牌张守恒）
  - `check_score.py`：计分机械核对（番值/总番/起和线/结算公式/零和）
  - `probe_illegal.mjs`：非法动作探测（逐用例独立服务端）
  - `run_game.mjs`：种子局运行器（e2e 同驱动、端口/目录可参数化）

- `reverify-v11/`——R2/R3 修复（GX-3 @ 1a302c8）后合并树上的复验局：种子 6→6 番（自摸无全求人）、93→8 番（全求人不计单钓将）、1067→4 番。

## 大规模扫描（不入库、可确定性再生）

4355 局留存扫描（种子 1–4300，e2e 驱动，Linux 同构）用于两份核对器的全量通过与问题清单统计（R1/R2/R3 发生率、流局率 88%、抢杠和 0 命中）。任何一局可用 `cd client && npm run e2e <种子>` 确定性重演并取 dump（`GX_E2E_KEEP=1`）。

## 复现命令

```bash
cd server && go test ./internal/score/ -run TestGX5 -v   # 计分重算互证（dump result 直算 vs dump.score）
python3 evidence/gx5-tools/check_dump.py evidence/gx5/final/*.json evidence/gx5/paths/*.json
python3 evidence/gx5-tools/check_score.py evidence/gx5/final/*.json evidence/gx5/paths/*.json
node evidence/gx5-tools/probe_illegal.mjs
cd client && npm run e2e 13    # R1：2 番未达线和牌
cd client && npm run e2e 6     # R2：全求人＋自摸
cd client && npm run e2e 93    # R3：单钓将漏计
```

## 归属与限制

- 验证对象：`talgent/GX-4` @ 8f71ecd（GX-3 score 经 GX-4 复演接入）；引擎未改动。
- `server/internal/score/zz_gx5_*_test.go`：GX-5 添加的验证测试（复现/互证），不属产品代码。
- 抢杠和路径（N4）：随机驱动不可达，未取得实局证据；引擎层有定点单测。
