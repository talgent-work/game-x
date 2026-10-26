// GX-4 E2E：起真实服务端，四条真实 WebSocket 连接（与客户端同一协议路径），
// 以无策略随机合法动作把一局打到终局，并校验 --dump 完整对局记录。
//
// 驱动方式为「同一时刻至多一个动作在途」的确定性顺序驱动：
// 种子 → 动作序列 → 终局结果 完全可复现（同一命令多次运行结果一致）。
// 复现：npm run e2e [种子]
import { spawn } from "node:child_process";
import { mkdtempSync, readFileSync, copyFileSync, mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const clientDir = path.resolve(import.meta.dirname, "..");
const serverBin = path.join(
  clientDir,
  "resources",
  "server",
  process.platform === "win32" ? "gameserver-win.exe" : "gameserver-host",
);
const seed = Number(process.argv[2] ?? "20261018");
const port = 8791; // E2E 专用端口
const dumpPath = path.join(mkdtempSync(path.join(tmpdir(), "gx4-e2e-")), "game.json");

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

// 一条座位连接：入座 + 消息队列（waitFor 按序消费）
class SeatConn {
  constructor(seat) {
    this.seat = seat;
    this.queue = [];
    this.waiters = [];
    this.closed = false;
    this.ws = new WebSocket(`ws://127.0.0.1:${port}/ws`);
    this.ws.addEventListener("open", () => this.send({ type: "hello", version: 1, seat }));
    this.ws.addEventListener("message", (ev) => {
      const m = JSON.parse(String(ev.data));
      this.queue.push(m);
      this.pump();
    });
    this.ws.addEventListener("close", () => {
      this.closed = true;
      this.pump();
    });
  }
  send(msg) {
    this.ws.send(JSON.stringify(msg));
  }
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
  // 取下一批消息直到遇到 kind 之一（含）；返回该消息；连接关闭返回 null
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
        const to = setTimeout(() => {
          ws.close();
          reject(new Error("connect timeout"));
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

function pickAction(acts, rng) {
  const win = acts.find((a) => a.kind === "win");
  if (win) return win;
  return acts[Math.floor(rng() * acts.length)];
}

let code = 1;
try {
  await waitServer(`ws://127.0.0.1:${port}/ws`);
  const seats = [0, 1, 2, 3].map((s) => new SeatConn(s));
  // 每个座位先拿到入座首推 state
  const lastState = new Map();
  for (const c of seats) {
    const m = await c.until(["state"]);
    if (!m || !m.state) throw new Error(`座位 ${c.seat} 未收到首推 state`);
    lastState.set(c.seat, m.state);
  }

  const rng = mulberry32(seed);
  let actions = 0;
  let endEvent = null;

  for (let round = 0; round < 100000 && !endEvent; round++) {
    let acted = false;
    for (const c of seats) {
      const st = lastState.get(c.seat);
      if (!st || st.actions.length === 0) continue;
      c.send({ type: "action", action: pickAction(st.actions, rng) });
      actions++;
      // 发者等 state/rejected；其余座位各等一条新 state —— 保证上一步扇出全部落地后再继续
      const mine = await c.until(["state", "rejected"]);
      if (mine === null) throw new Error(`座位 ${c.seat} 连接意外关闭`);
      if (mine.type === "state") lastState.set(c.seat, mine.state);
      for (const other of seats) {
        if (other === c) continue;
        const m = await other.until(["state"]);
        if (m === null) throw new Error(`座位 ${other.seat} 连接意外关闭`);
        lastState.set(other.seat, m.state);
      }
      for (const [s, st2] of lastState) {
        if (st2?.snapshot?.result) {
          endEvent = st2.snapshot.result;
          break;
        }
      }
      acted = true;
      if (endEvent) break;
    }
    if (!acted && !endEvent) await sleep(20);
  }

  if (!endEvent) throw new Error("一局未在步数上限内收束");

  // 等 dump 落盘并可解析：服务端以「临时文件 + rename」原子写出；
  // 消费侧判据＝「可解析」而非「存在」——GX-5 E1 修复配套
  let dump = null;
  for (let i = 0; i < 100 && !dump; i++) {
    try {
      dump = JSON.parse(readFileSync(dumpPath, "utf8"));
    } catch {
      await sleep(100);
    }
  }
  if (!dump) throw new Error(`未读到可解析的 dump：${dumpPath}`);
  if (!Array.isArray(dump.actions) || dump.actions.length === 0) throw new Error("dump.actions 为空");
  if (!Array.isArray(dump.events) || dump.events.length === 0) throw new Error("dump.events 为空");
  if (!dump.result) throw new Error("dump.result 缺失");
  if (dump.seed !== seed) throw new Error(`dump.seed 不符：${dump.seed}`);
  if (!dump.score) throw new Error("dump.score 缺失（GX-3 计分）");
  const deltaSum = dump.score.settlement.deltas.reduce((a, b) => a + b, 0);
  if (deltaSum !== 0) throw new Error(`四家得失分应零和，得 ${deltaSum}`);
  const rejected = dump.actions.filter((a) => a.error).length;
  const outcome = dump.result.exhaustive ? "流局" : `座位 ${dump.result.winner} 和牌`;
  const scoreTxt = dump.result.exhaustive ? "流局零和" : `${dump.score.best.total} 番（${dump.score.eligible ? "达起和线" : "未达起和线"}）`;
  const sameResult = Boolean(endEvent?.exhaustive) === Boolean(dump.result.exhaustive);

  if (process.env.GX_E2E_KEEP === "1") {
    const dstDir = process.env.GX_E2E_DIR ?? "/tmp/gx4-evidence";
    mkdirSync(dstDir, { recursive: true });
    const dst = path.join(dstDir, `seed-${seed}-${dump.result.exhaustive ? "draw" : "win"}.json`);
    copyFileSync(dumpPath, dst);
    console.log(`dump 已保留：${dst}`);
  }

  console.log(
    `E2E 通过：种子 ${seed}（确定性）｜动作 ${dump.actions.length} 步（其中被拒 ${rejected}）｜事件 ${dump.events.length} 条｜结果：${outcome}｜计分：${scoreTxt}`,
  );
  console.log(`终局结果一致：${sameResult ? "是" : "否（客户端观察与服务端 dump 不一致，请检查）"}`);
  console.log(`dump：${dumpPath}`);
  code = 0;
} catch (e) {
  console.error("E2E 失败：", e?.message ?? e);
  console.error(serverLog);
} finally {
  server.kill();
}
process.exit(code);