// ActionSource：无策略随机合法动作（种子化、可复现）。
// 同一协议路径的三种来源：真人（座位 0，用 actions 画按钮）、随机自动座（1–3）、E2E（四座全自动）。
import type { Action } from "./protocol/protocol";

// mulberry32：小而稳的可复现伪随机数发生器。
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// chooseAuto 从服务端给出的合法动作里挑一条：能和则和，否则等概率随机（无策略、非 AI）。
export function chooseAuto(actions: Action[], rng: () => number): Action | null {
  if (!actions || actions.length === 0) return null;
  const win = actions.find((a) => a.kind === "win");
  if (win) return win;
  return actions[Math.floor(rng() * actions.length)] ?? null;
}