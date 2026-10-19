// GX-5 证据局运行器：与 client/e2e/full-game.mjs 同驱动方式，但端口/输出目录/种子可参数化。
// 用法: node run_game.mjs <seed> <port> <outdir>
import { spawn } from "node:child_process";
import { mkdtempSync, readFileSync, existsSync, copyFileSync, mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const serverBin = "/workspace/repos/game-x/client/resources/server/gameserver-host";
const seed = Number(process.argv[2] ?? "6");
const port = Number(process.argv[3] ?? "8796");
const outDir = process.argv[4] ?? "/workspace/gx5-final";
const dumpPath = path.join(mkdtempSync(path.join(tmpdir(), "gx5-run-")), "game.json");

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function mulberry32(a0) {
  let a = a0 >>> 0;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

class SeatConn {
  constructor(seat) {
    this.seat = seat;
    this.queue = [];
    this.waiters = [];
    this.closed = false;
    this.ws = new WebSocket(`ws://127.0.0.1:${port}/ws`);
    this.ws.addEventListener("open", () => this.send({ type: "hello", version: 1, seat }));
    this.ws.addEventListener("message", (ev) => {
      this.queue.push(JSON.parse(String(ev.data)));
      this.pump();
    });
    this.ws.addEventListener("close", () => { this.closed = true; this.pump(); });
  }
  send(msg) { this.ws.send(JSON.stringify(msg)); }
  pump() {
    while (this.waiters.length && (this.queue.length || this.closed)) {
      const w = this.waiters.shift();
      w(this.queue.length ? this.queue.shift() : null);
    }
  }
  next() {
    if (this.queue.length) return Promise.resolve(this.queue.shift());
    if (this.closed) return Promise.resolve(null);
    return new Promise((resolve) => this.waiters.push(resolve));
  }
  async until(kinds) {
    for (;;) {
      const m = await this.next();
      if (m === null) return null;
      if (kinds.includes(m.type)) return m;
    }
  }
}

const server = spawn(serverBin, ["--addr", `127.0.0.1:${port}`, "--seed", String(seed), "--dump", dumpPath], {
  stdio: ["ignore", "pipe", "pipe"],
});
let serverLog = "";
server.stdout.on("data", (d) => (serverLog += d));
server.stderr.on("data", (d) => (serverLog += d));

async function waitServer(url) {
  for (let i = 0; i < 100; i++) {
    try {
      await new Promise((resolve, reject) => {
        const ws = new WebSocket(url);
        const to = setTimeout(() => { ws.close(); reject(new Error("connect timeout")); }, 1000);
        ws.addEventListener("open", () => { clearTimeout(to); ws.close(); resolve(); });
        ws.addEventListener("error", () => { clearTimeout(to); reject(new Error("refused")); });
      });
      return;
    } catch { await sleep(100); }
  }
  throw new Error("服务端未在 10s 内就绪");
}

const PRIORITY = ["win", "kongPromote", "kongDiscard", "pong", "chi", "kongConcealed", "discard", "pass"];
function pickAction(acts, rng) {
  for (const k of PRIORITY) {
    const cands = acts.filter((a) => a.kind === k);
    if (cands.length) return cands[Math.floor(rng() * cands.length)];
  }
  return acts[Math.floor(rng() * acts.length)];
}

let code = 1;
try {
  await waitServer(`ws://127.0.0.1:${port}/ws`);
  const seats = [0, 1, 2, 3].map((s) => new SeatConn(s));
  const lastState = new Map();
  for (const c of seats) {
    const m = await c.until(["state"]);
    if (!m || !m.state) throw new Error(`座位 ${c.seat} 未收到首推 state`);
    lastState.set(c.seat, m.state);
  }
  const rng = mulberry32(seed);
  let endEvent = null;
  for (let round = 0; round < 100000 && !endEvent; round++) {
    let acted = false;
    for (const c of seats) {
      const st = lastState.get(c.seat);
      if (!st || st.actions.length === 0) continue;
      c.send({ type: "action", action: pickAction(st.actions, rng) });
      const mine = await c.until(["state", "rejected"]);
      if (mine === null) throw new Error(`座位 ${c.seat} 连接意外关闭`);
      if (mine.type === "state") lastState.set(c.seat, mine.state);
      for (const other of seats) {
        if (other === c) continue;
        const m = await other.until(["state"]);
        if (m === null) throw new Error(`座位 ${other.seat} 连接意外关闭`);
        lastState.set(other.seat, m.state);
      }
      for (const [, st2] of lastState) {
        if (st2?.snapshot?.result) { endEvent = st2.snapshot.result; break; }
      }
      acted = true;
      if (endEvent) break;
    }
    if (!acted && !endEvent) await sleep(20);
  }
  if (!endEvent) throw new Error("一局未在步数上限内收束");
  for (let i = 0; i < 100 && !existsSync(dumpPath); i++) await sleep(100);
  if (!existsSync(dumpPath)) throw new Error(`未找到 dump：${dumpPath}`);
  // 等写入完成（非原子写出问题）：读成功为止
  let dump = null;
  for (let i = 0; i < 20; i++) {
    try { dump = JSON.parse(readFileSync(dumpPath, "utf8")); break; } catch { await sleep(200); }
  }
  if (!dump) throw new Error("dump 读取失败（截断）");
  mkdirSync(outDir, { recursive: true });
  const tag = dump.result.exhaustive ? "draw" : ["selfdraw", "discard", "robkong"][dump.result.source ?? 0];
  const dst = path.join(outDir, `seed-${seed}-${tag}.json`);
  copyFileSync(dumpPath, dst);
  const outcome = dump.result.exhaustive ? "流局" : `座位 ${dump.result.winner} ${["自摸", "点和", "抢杠和"][dump.result.source]} 和牌`;
  console.log(`OK 种子 ${seed}：${outcome}｜${dump.actions.length} 步｜dump → ${dst}`);
  code = 0;
} catch (e) {
  console.error(`FAIL 种子 ${seed}：`, e?.message ?? e);
  console.error(serverLog);
} finally {
  server.kill();
}
process.exit(code);
