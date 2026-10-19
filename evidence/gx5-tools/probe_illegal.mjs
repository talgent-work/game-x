// GX-5 非法动作探测：每个用例独立起一个真实服务端，发送非法输入，
// 断言 rejected 回到发起者（带 code/reason），并记录连接是否被关闭。
// 用法: node probe_illegal.mjs
import { spawn } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const serverBin = "/workspace/repos/game-x/client/resources/server/gameserver-host";
const basePort = 8810;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const results = [];
function report(name, pass, detail) {
  results.push({ name, pass, detail });
  console.log(`${pass ? "PASS" : "FAIL"} ${name}${detail ? "｜" + detail : ""}`);
}

class Conn {
  constructor(port) {
    this.port = port;
    this.queue = [];
    this.waiters = [];
    this.closed = false;
    this.ws = new WebSocket(`ws://127.0.0.1:${port}/ws`);
    this.ready = new Promise((resolve) => {
      this.ws.addEventListener("open", () => resolve());
      this.ws.addEventListener("error", () => resolve());
    });
    this.ws.addEventListener("message", (ev) => {
      this.queue.push(JSON.parse(String(ev.data)));
      this.pump();
    });
    this.ws.addEventListener("close", () => { this.closed = true; this.pump(); });
  }
  async send(msg) { await this.ready; if (!this.closed) this.ws.send(JSON.stringify(msg)); }
  async sendRaw(text) { await this.ready; if (!this.closed) this.ws.send(text); }
  pump() {
    while (this.waiters.length && (this.queue.length || this.closed)) {
      const w = this.waiters.shift();
      w(this.queue.length ? this.queue.shift() : { type: "__closed__" });
    }
  }
  async next(timeoutMs = 3000) {
    if (this.queue.length) return this.queue.shift();
    if (this.closed) return { type: "__closed__" };
    return Promise.race([
      new Promise((r) => this.waiters.push(r)),
      sleep(timeoutMs).then(() => ({ type: "__timeout__" })),
    ]);
  }
  async expectRejected(name, code) {
    const m = await this.next(3000);
    if (m.type === "rejected") {
      const ok = code ? m.code === code : true;
      report(name, ok, `code=${m.code} reason=${m.reason}${this.closed ? "｜连接随后被关闭" : "｜连接保持"}`);
      return m;
    }
    report(name, false, `未收到 rejected，收到 ${m.type}`);
    return null;
  }
}

async function startServer(idx) {
  const port = basePort + idx;
  const dump = path.join(mkdtempSync(path.join(tmpdir(), "gx5-probe-")), "g.json");
  const server = spawn(serverBin, ["--addr", `127.0.0.1:${port}`, "--seed", "6", "--dump", dump]);
  for (let i = 0; i < 100; i++) {
    try {
      await new Promise((resolve, reject) => {
        const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`);
        const to = setTimeout(() => { ws.close(); reject(new Error("timeout")); }, 500);
        ws.addEventListener("open", () => { clearTimeout(to); ws.close(); resolve(); });
        ws.addEventListener("error", () => { clearTimeout(to); reject(new Error("refused")); });
      });
      return { server, port };
    } catch { await sleep(80); }
  }
  server.kill();
  throw new Error(`server ${idx} not ready`);
}

async function bind4(port) {
  const seats = [0, 1, 2, 3].map(() => new Conn(port));
  await Promise.all(seats.map(async (c, i) => c.send({ type: "hello", version: 1, seat: i })));
  const states = [];
  for (const c of seats) {
    const m = await c.next(4000);
    states.push(m);
  }
  return { seats, states };
}

// 每个用例：独立服务端
const cases = [
  {
    name: "未入座即发动作被拒（not_bound）",
    code: "not_bound",
    async run(port) {
      const c = new Conn(port);
      await c.send({ type: "action", action: { kind: "pass" } });
      await c.expectRejected(this.name, this.code);
    },
  },
  {
    name: "hello 版本非 1 被拒（bad_version）",
    code: "bad_version",
    async run(port) {
      const c = new Conn(port);
      await c.send({ type: "hello", version: 0, seat: 2 });
      await c.expectRejected(this.name, this.code);
    },
  },
  {
    name: "座位 9 被拒（bad_seat）",
    code: "bad_seat",
    async run(port) {
      const c = new Conn(port);
      await c.send({ type: "hello", version: 1, seat: 9 });
      await c.expectRejected(this.name, this.code);
    },
  },
  {
    name: "抢占已占座位被拒（seat_taken）",
    code: "seat_taken",
    async run(port) {
      await bind4(port);
      const c = new Conn(port);
      await c.send({ type: "hello", version: 1, seat: 0 });
      await c.expectRejected(this.name, this.code);
    },
  },
  {
    name: "已绑定连接重复 hello 被拒",
    code: null,
    async run(port) {
      const { seats } = await bind4(port);
      await seats[1].send({ type: "hello", version: 1, seat: 2 });
      await seats[1].expectRejected(this.name, this.code);
    },
  },
  {
    name: "非本座行动被拒（illegal_action）",
    code: "illegal_action",
    async run(port) {
      const { seats } = await bind4(port);
      // 庄家(座位0)先行动：座位1抢先弃牌
      await seats[1].send({ type: "action", action: { kind: "discard", tile: { suit: 0, rank: 1 } } });
      await seats[1].expectRejected(this.name, this.code);
    },
  },
  {
    name: "弃手中没有的牌被拒",
    code: "illegal_action",
    async run(port) {
      const { seats, states } = await bind4(port);
      const st = states[0].state;
      const hand = st.snapshot.hand;
      const candidates = [{ suit: 3, rank: 4 }, { suit: 4, rank: 1 }, { suit: 2, rank: 9 }];
      const missing = candidates.find((t) => !hand.some((h) => h.suit === t.suit && h.rank === t.rank));
      if (!missing) { report(this.name, true, "（起手全含候选牌，跳过）"); return; }
      await seats[0].send({ type: "action", action: { kind: "discard", tile: missing } });
      await seats[0].expectRejected(this.name, this.code);
    },
  },
  {
    name: "无声明窗口时吃被拒",
    code: "illegal_action",
    async run(port) {
      const { seats } = await bind4(port);
      await seats[2].send({ type: "action", action: { kind: "chi", with: [{ suit: 0, rank: 1 }, { suit: 0, rank: 3 }] } });
      await seats[2].expectRejected(this.name, this.code);
    },
  },
  {
    name: "未知动作 kind 被拒",
    code: null,
    async run(port) {
      const { seats } = await bind4(port);
      await seats[0].send({ type: "action", action: { kind: "explode" } });
      await seats[0].expectRejected(this.name, this.code);
    },
  },
  {
    name: "未知消息类型被拒（unknown_type）",
    code: "unknown_type",
    async run(port) {
      const { seats } = await bind4(port);
      await seats[0].send({ type: "blahblah" });
      await seats[0].expectRejected(this.name, this.code);
    },
  },
  {
    name: "畸形 JSON 被拒（bad_json）",
    code: "bad_json",
    async run(port) {
      const { seats } = await bind4(port);
      await seats[0].sendRaw("not json at all {");
      await seats[0].expectRejected(this.name, this.code);
    },
  },
  {
    name: "拒绝后合法动作照常推进（状态未损）",
    code: null,
    async run(port) {
      const { seats, states } = await bind4(port);
      // 先制造一次拒绝
      await seats[1].send({ type: "action", action: { kind: "discard", tile: { suit: 0, rank: 1 } } });
      await seats[1].next(3000);
      // 座位 0（庄家）合法打牌
      const hand = states[0].state.snapshot.hand;
      const tile = hand[0];
      await seats[0].send({ type: "action", action: { kind: "discard", tile } });
      let m = await seats[0].next(3000);
      let ok = false;
      for (let t = 0; t < 8 && m && m.type !== "state" && m.type !== "rejected"; t++) m = await seats[0].next(3000);
      ok = m?.type === "state";
      report(this.name, ok, ok ? "拒绝后庄家打牌成功，收到新 state" : `收到 ${m?.type}`);
    },
  },
];

for (let i = 0; i < cases.length; i++) {
  const c = cases[i];
  try {
    const { server, port } = await startServer(i);
    await c.run(port);
    server.kill();
  } catch (e) {
    report(c.name, false, `异常：${e?.message ?? e}`);
  }
}

const failed = results.filter((r) => !r.pass);
console.log(`\n== 探测 ${results.length} 项：通过 ${results.length - failed.length}，失败 ${failed.length} ==`);
process.exit(failed.length ? 1 : 0);
