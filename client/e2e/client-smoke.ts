// 客户端模块 headless 冒烟（无显示环境下的同构证据）：
// 使用真实客户端模块——net.SeatConnection / game.SeatView / autoplay.chooseAuto（非复制的逻辑）——
// 四条座位连接同走协议路径，确定性打完一局，并与服务端 --dump 完整记录互证。
//
// 运行：npm run smoke [种子]（需先 npm run build:server）
import { spawn } from "node:child_process";
import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { SeatConnection } from "../src/net";
import { SeatView } from "../src/game";
import { chooseAuto, mulberry32 } from "../src/autoplay";
import type { Msg } from "../src/protocol/protocol";
import type { Result } from "../src/protocol/engine";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const clientDir = path.resolve(__dirname, "..");
const serverBin = path.join(
  clientDir,
  "resources",
  "server",
  process.platform === "win32" ? "gameserver-win.exe" : "gameserver-host",
);

const seed = Number(process.argv[2] ?? "6");
const port = Number(process.env.GX_SMOKE_PORT ?? "8793");
const dumpPath = path.join(mkdtempSync(path.join(tmpdir(), "gx4-smoke-")), "game.json");
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// 一条座位：真实 SeatConnection + SeatView，消息按序排队供驱动等待。
class Seat {
  readonly conn: SeatConnection;
  readonly view = new SeatView();
  private queue: Msg[] = [];
  private waiters: Array<(m: Msg | null) => void> = [];

  constructor(seat: number, url: string) {
    this.conn = new SeatConnection(url, seat);
    this.conn.onMessage((m) => {
      this.view.apply(m);
      this.queue.push(m);
      this.pump();
    });
  }

  private pump(): void {
    while (this.waiters.length && this.queue.length) this.waiters.shift()!(this.queue.shift()!);
  }

  next(): Promise<Msg | null> {
    if (this.queue.length) return Promise.resolve(this.queue.shift()!);
    return new Promise((r) => this.waiters.push(r));
  }

  // 等本连接上「一条 state 或 rejected」——本座位的动作已被服务端处理完
  async untilState(): Promise<void> {
    for (;;) {
      const m = await this.next();
      if (m === null || m.type === "state" || m.type === "rejected") return;
    }
  }

  drain(): void {
    this.queue = [];
  }
}

async function waitServer(url: string): Promise<void> {
  for (let i = 0; i < 100; i++) {
    try {
      await new Promise<void>((resolve, reject) => {
        const ws = new WebSocket(url);
        const to = setTimeout(() => {
          ws.close();
          reject(new Error("timeout"));
        }, 1000);
        ws.addEventListener("open", () => {
          clearTimeout(to);
          ws.close();
          resolve();
        });
        ws.addEventListener("error", () => {
          clearTimeout(to);
          reject(new Error("refused"));
        });
      });
      return;
    } catch {
      await sleep(100);
    }
  }
  throw new Error("服务端未在 10s 内就绪");
}

async function main(): Promise<void> {
  const server = spawn(serverBin, ["--addr", `127.0.0.1:${port}`, "--seed", String(seed), "--dump", dumpPath], {
    stdio: ["ignore", "pipe", "pipe"],
  });
  let serverLog = "";
  server.stdout.on("data", (d) => (serverLog += d));
  server.stderr.on("data", (d) => (serverLog += d));

  const bail = setTimeout(() => {
    console.error("冒烟超时（30s）");
    server.kill();
    process.exit(1);
  }, 30000);

  try {
    const url = `ws://127.0.0.1:${port}/ws`;
    await waitServer(url);

    const seats = [0, 1, 2, 3].map((s) => new Seat(s, url));
    for (let i = 0; i < 200; i++) {
      if (seats.every((s) => s.view.snapshot() !== null)) break;
      await sleep(25);
    }
    if (!seats.every((s) => s.view.snapshot() !== null)) throw new Error("有座位未收到首推 state");
    seats.forEach((s) => s.drain());

    const rng = mulberry32(seed);
    let result: Result | null = null;
    for (let round = 0; round < 100000 && !result; round++) {
      let acted = false;
      for (const s of seats) {
        const acts = s.view.actions();
        if (acts.length === 0) continue;
        const pick = chooseAuto(acts, rng);
        if (!pick) continue;
        s.conn.send({ type: "action", action: pick });
        await s.untilState();
        for (const o of seats) if (o !== s) await o.untilState();
        acted = true;
        result = seats.map((x) => x.view.result).find((r) => r !== null) ?? null;
        if (result) break;
      }
      if (!acted && !result) await sleep(20);
    }
    if (!result) throw new Error("一局未在步数上限内收束");

    // 与服务端 --dump 完整记录互证：等待「可解析」而非「存在」——GX-5 E1 修复配套
    let dump: {
      result?: Result;
      score?: { best: { total: number }; settlement: { deltas: number[] } };
    } | null = null;
    for (let i = 0; i < 100 && !dump; i++) {
      try {
        dump = JSON.parse(readFileSync(dumpPath, "utf8")) as NonNullable<typeof dump>;
      } catch {
        await sleep(100);
      }
    }
    if (!dump) throw new Error(`未读到可解析的 dump：${dumpPath}`);
    const outcome = result.exhaustive ? "流局" : `座位 ${result.winner} 和牌`;
    const serverOutcome = dump.result?.exhaustive ? "流局" : `座位 ${dump.result?.winner} 和牌`;
    if (outcome !== serverOutcome) {
      throw new Error(`客户端观察（${outcome}）与服务端 dump（${serverOutcome}）不一致`);
    }
    if (!dump.score) throw new Error("dump.score 缺失（GX-3 计分）");
    const deltaSum = dump.score.settlement.deltas.reduce((a, b) => a + b, 0);
    if (deltaSum !== 0) throw new Error(`四家得失分应零和，得 ${deltaSum}`);

    console.log(
      `客户端模块 headless 冒烟通过：种子 ${seed}｜${outcome}｜计分 ${dump.score.best.total} 番｜客户端观察与服务端 dump 一致`,
    );
    console.log("（真实模块：net.SeatConnection / game.SeatView / autoplay.chooseAuto；座位 0 事件末 6 条：）");
    console.log(seats[0].view.log.slice(-6).join("\n"));
    clearTimeout(bail);
    server.kill();
    process.exit(0);
  } catch (e) {
    clearTimeout(bail);
    console.error("冒烟失败：", e instanceof Error ? e.message : e);
    console.error(serverLog);
    server.kill();
    process.exit(1);
  }
}

void main();