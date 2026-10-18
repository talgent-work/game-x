# game-x client（GX-4 本地对局原型）

Electron + TypeScript 客户端：PixiJS 牌桌 + DOM 界面；对局数据一律经 WebSocket 与本机 Go 权威服务端交换（界面零规则）。

完整说明（结构、协议消息表、运行、验证与复现、边界）见 [../docs/prototype.md](../docs/prototype.md)。

常用命令（本目录）：

```bash
npm install
npm run build        # 服务端二进制（host + windows/amd64 交叉编译）+ 渲染层 + 主进程
npm start            # 构建并启动（自动拉起本机服务端）
npm run e2e 6        # 确定性 E2E（真 WS ×4；种子 6 → 座位 0 和牌 12 番、13 → 和牌 2 番、20261018 → 流局）
npm run smoke 6      # 真实客户端模块 headless 冒烟（与本机服务端整局互证）
npm run typecheck    # src + e2e 两套 tsconfig
npm run dist:win     # electron-builder 打 Windows zip（内嵌 gameserver.exe）
npm run gen:protocol # tygo：server → src/protocol/（勿手改生成物）
```

环境变量：`GX_PORT`（默认 8787）、`GX_SEED`（固定对局种子）。