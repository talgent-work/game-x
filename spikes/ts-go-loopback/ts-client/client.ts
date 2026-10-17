// GX-1 最小实证 4b：Node↔Go 协议回环的 TS 客户端（Node 运行时）。
// Node ≥ 23.6 可直接运行 TypeScript（类型剥离），无需编译步骤：node client.ts
import WebSocket from "ws";

type Tile = { suit: number; rank: number };
type Snapshot = { seat: number; hand: Tile[]; wallCount: number; turn: number };
type InMsg = {
  type: string;
  seat?: number;
  snapshot?: Snapshot;
  tile?: Tile;
  turn?: number;
  wallCount?: number;
  reason?: string;
};

const url = "ws://127.0.0.1:8787/ws";
const TILE_1W: Tile = { suit: 0, rank: 1 }; // 1万（座位 0 起手牌里确定有的，固定种子）
const TILE_2W: Tile = { suit: 0, rank: 2 }; // 2万

const ws = new WebSocket(url);
let step = 0;

const guard = setTimeout(() => {
  console.error("超时：未在 15 秒内完成回环");
  process.exit(1);
}, 15000);

function send(obj: unknown): void {
  const text = JSON.stringify(obj);
  console.log("→ " + text);
  ws.send(text);
}

ws.on("open", () => {
  console.log("已连接 " + url);
  send({ type: "hello", seat: 0 });
});

ws.on("message", (raw: WebSocket.RawData) => {
  const text = raw.toString();
  console.log("← " + text);
  const msg = JSON.parse(text) as InMsg;

  if (step === 0 && msg.type === "snapshot") {
    const s = msg.snapshot as Snapshot;
    console.log(`   （快照：座位 ${s.seat}，手牌 ${s.hand.length} 张，牌墙余 ${s.wallCount}，轮到 ${s.turn}）`);
    step = 1;
    // 回合内打出 1万：这就是"客户端只发意图、服务端执行并回执"
    send({ type: "action", action: { kind: "discard", tile: TILE_1W } });
  } else if (step === 1 && msg.type === "discarded") {
    step = 2;
    // 已经不是自己的回合，再打一张：预期被权威方拒绝
    send({ type: "action", action: { kind: "discard", tile: TILE_2W } });
  } else if (step === 2 && msg.type === "rejected") {
    console.log(`   （服务端按权威规则拒绝：${msg.reason}）`);
    clearTimeout(guard);
    ws.close();
  }
});

ws.on("close", () => {
  console.log("连接关闭，回环完成");
  process.exit(0);
});

ws.on("error", (e: Error) => {
  console.error("错误：" + e.message);
  process.exit(1);
});