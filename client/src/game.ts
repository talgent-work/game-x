// SeatView：把协议消息折叠成「可渲染模型」——界面只读模型、只发意图，不含任何规则判定。
import type { Action, Event, Msg, State } from "./protocol/protocol";
import type { Result, Snapshot } from "./protocol/engine";
import type { ScoreResult } from "./protocol/score";

export function tileLabel(t: { suit: number; rank: number }): string {
  if (t.suit === 3) return ["东", "南", "西", "北"][t.rank - 1] ?? "?";
  if (t.suit === 4) return ["中", "发", "白"][t.rank - 1] ?? "?";
  return `${t.rank}${["万", "筒", "条"][t.suit] ?? "?"}`;
}

export function statePhaseText(phase: number): string {
  return ["等待打牌", "等待声明", "终局"][phase] ?? "?";
}

// 座位风：庄家东、下家南、对家西、上家北（引擎口径）。
export function windText(dealer: number, seat: number): string {
  return ["东", "南", "西", "北"][(seat - dealer + 4) % 4] ?? "?";
}

// 结算方式（score.PayoutKind）。
export function payoutText(kind: string): string {
  return ({ discard: "点炮（含抢杠）", selfDraw: "自摸", exhaustive: "流局" } as Record<string, string>)[kind] ?? kind;
}

export function shapeText(shape: number): string {
  return ["标准型", "组合龙型", "七对", "全不靠", "十三幺"][shape] ?? `牌型${shape}`;
}

export function winSourceText(source: number): string {
  return ["自摸", "点和", "抢杠和"][source] ?? "?";
}

export class SeatView {
  state: State | null = null;
  log: string[] = [];
  result: Result | null = null;

  apply(m: Msg): void {
    if (m.type === "state" && m.state) {
      this.state = m.state;
      if (m.state.snapshot.result) this.result = m.state.snapshot.result;
    } else if (m.type === "event" && m.event) {
      this.push(this.describe(m.event));
      if (m.event.result) this.result = m.event.result;
    } else if (m.type === "rejected") {
      this.push(`被拒（${m.code ?? ""}）：${m.reason ?? ""}`);
    }
  }

  snapshot(): Snapshot | null {
    return this.state?.snapshot ?? null;
  }

  actions(): Action[] {
    return this.state?.actions ?? [];
  }

  score(): ScoreResult | null {
    return this.state?.score ?? null;
  }

  canDiscard(t: { suit: number; rank: number }): boolean {
    return this.actions().some(
      (a) => a.kind === "discard" && a.tile != null && a.tile.suit === t.suit && a.tile.rank === t.rank,
    );
  }

  private push(line: string): void {
    this.log.push(line);
    if (this.log.length > 300) this.log.shift();
  }

  private describe(ev: Event): string {
    const who = `座位 ${ev.seat}`;
    switch (ev.kind) {
      case "drawn":
        return `${who} 摸牌`;
      case "discarded":
        return `${who} 打出 ${ev.tile ? tileLabel(ev.tile) : "?"}`;
      case "meld": {
        const m = ev.meld;
        const kind = m?.kind === 0 ? "吃" : m?.kind === 1 ? "碰" : "杠";
        const face = m?.tiles && m.tiles.length > 0 ? tileLabel(m.tiles[0]!) : "暗杠";
        return `${who} ${kind} ${face}`;
      }
      case "kongPromoted":
        return `${who} 补杠 ${ev.tile ? tileLabel(ev.tile) : ""}（抢杠窗口）`;
      case "win":
        return `和牌！${who}`;
      case "exhaustive":
        return "流局";
      default:
        return String(ev.kind);
    }
  }
}