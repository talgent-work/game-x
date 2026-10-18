// TableScreen（PixiJS 牌桌）＋ Hud（DOM 信息/按钮/日志/结算）。
// 只读 SeatView 的模型、只发意图；按钮与可点手牌完全由服务器下发的 actions 驱动——界面零规则。
import { Application, Container, Graphics, Text } from "pixi.js";
import type { Action } from "./protocol/protocol";
import type { DiscardEntry, Meld, Result, Snapshot, Tile } from "./protocol/engine";
import { SeatView, shapeText, statePhaseText, tileLabel, winSourceText } from "./game";

const TW = 44;
const TH = 62;
const GAP = 6;

export class TableScreen {
  private app = new Application();
  private root = new Container();
  private intent: (a: Action) => void = () => {};
  private ready = false;

  async init(mount: HTMLElement): Promise<void> {
    await this.app.init({ background: 0x0e2417, resizeTo: mount, antialias: true });
    mount.appendChild(this.app.canvas);
    this.app.stage.addChild(this.root);
    this.ready = true;
  }

  setIntentHandler(fn: (a: Action) => void): void {
    this.intent = fn;
  }

  render(view: SeatView): void {
    if (!this.ready) return;
    this.root.removeChildren();
    const snap = view.snapshot();
    if (!snap) return;
    const w = this.app.renderer.width;
    const h = this.app.renderer.height;
    const seat = snap.seat;

    // 自己：牌河在手上方，手牌在底部（可点击打牌）
    const hand = snap.hand ?? [];
    const handY = h - TH - 26;
    const handW = hand.length * (TW + GAP) - GAP;
    const handX = Math.max(16, (w - handW) / 2);
    hand.forEach((t, i) => {
      const can = view.canDiscard(t);
      const c = this.tile(t, can ? () => this.intent({ kind: "discard", tile: t }) : null);
      c.position.set(handX + i * (TW + GAP), handY + (can ? -10 : 0));
      this.root.addChild(c);
    });
    this.riverOf(snap.discards?.[seat] ?? [], handX, handY - TH - 22, 16);

    // 其余三家：副子 + 牌河（手牌不可见——载荷里本就没有）
    for (const s of [0, 1, 2, 3]) {
      if (s === seat) continue;
      const rel = (s - seat + 4) % 4; // 1 下家（右）2 对家（上）3 上家（左）
      let x: number;
      let y: number;
      if (rel === 1) {
        x = w - 150;
        y = 220;
      } else if (rel === 2) {
        x = w / 2 - 150;
        y = 20;
      } else {
        x = 20;
        y = 220;
      }
      this.seatBlock(s, snap, x, y, rel === 2);
    }

    // 中央信息
    const lines = [
      `牌墙 ${snap.wallCount}`,
      snap.phase === 1 && snap.pending
        ? `等待声明：座位 ${snap.pending.seat} 的 ${tileLabel(snap.pending.tile)}${snap.pending.rob ? "（抢杠）" : ""}`
        : `轮到座位 ${snap.turn}`,
      statePhaseText(snap.phase),
    ];
    const info = new Text({
      text: lines.join("\n"),
      style: { fontSize: 16, fill: 0xcfe4d4, align: "center", fontFamily: "Microsoft YaHei, sans-serif" },
    });
    info.anchor.set(0.5);
    info.position.set(w / 2, h / 2 - 40);
    this.root.addChild(info);
  }

  // 某座位块：标签 + 副子 + 牌河
  private seatBlock(s: number, snap: Snapshot, x: number, y: number, wide: boolean): void {
    const label = new Text({
      text: `座位 ${s}${s === snap.dealer ? "（庄）" : ""}`,
      style: { fontSize: 13, fill: 0x9fc0a8, fontFamily: "Microsoft YaHei, sans-serif" },
    });
    label.position.set(x, y - 18);
    this.root.addChild(label);

    let cx = x;
    for (const m of snap.melds?.[s] ?? []) {
      cx += this.meld(m, cx, y) + GAP;
    }
    this.riverOf(snap.discards?.[s] ?? [], x, y + 56, wide ? 16 : 8);
  }

  // 一副副子（他人暗杠牌面为 null → 画背面）
  private meld(m: Meld, x: number, y: number): number {
    const tiles: (Tile | null)[] = m.tiles && m.tiles.length > 0 ? m.tiles : [null, null, null, null];
    tiles.forEach((t, i) => {
      const c = this.tile(t, null, 0.62);
      c.position.set(x + i * (TW * 0.62 + 2), y);
      this.root.addChild(c);
    });
    return tiles.length * (TW * 0.62 + 2);
  }

  private riverOf(entries: DiscardEntry[], x: number, y: number, perRow: number): void {
    entries.forEach((e, i) => {
      const col = i % perRow;
      const row = Math.floor(i / perRow);
      const c = this.tile(e.tile, null, 0.62);
      c.alpha = e.claimed ? 0.35 : 0.9;
      c.position.set(x + col * (TW * 0.62 + 2), y + row * (TH * 0.62 + 2));
      this.root.addChild(c);
    });
  }

  private tile(t: Tile | null, onClick: (() => void) | null, scale = 1): Container {
    const c = new Container();
    const g = new Graphics();
    const hidden = !t;
    g.roundRect(0, 0, TW, TH, 6).fill(hidden ? 0x33604a : 0xf2efe4);
    g.roundRect(0, 0, TW, TH, 6).stroke({ color: 0x14301f, width: 2 });
    c.addChild(g);
    if (t) {
      const label = new Text({
        text: tileLabel(t),
        style: { fontSize: 17, fill: 0x1c2a1f, fontFamily: "Microsoft YaHei, sans-serif", fontWeight: "600" },
      });
      label.anchor.set(0.5);
      label.position.set(TW / 2, TH / 2);
      c.addChild(label);
    }
    c.scale.set(scale);
    if (onClick) {
      c.eventMode = "static";
      c.cursor = "pointer";
      c.on("pointertap", onClick);
    }
    return c;
  }
}

// --- DOM HUD ---

export class Hud {
  private intent: (a: Action) => void = () => {};

  constructor(
    private els: { info: HTMLElement; buttons: HTMLElement; log: HTMLElement; settle: HTMLElement },
  ) {}

  setIntentHandler(fn: (a: Action) => void): void {
    this.intent = fn;
  }

  render(view: SeatView): void {
    const snap = view.snapshot();
    if (!snap) {
      this.els.info.textContent = "连接中…";
      return;
    }
    const declared = (snap.declared ?? []).map((d, s) => (d ? `座位 ${s}` : null)).filter(Boolean);
    this.els.info.textContent = [
      `你的座位：${snap.seat}${snap.dealer === snap.seat ? "（庄家）" : ""}`,
      `阶段：${statePhaseText(snap.phase)}　剩牌：${snap.wallCount}`,
      snap.pending
        ? `声明窗口：座位 ${snap.pending.seat} 的 ${tileLabel(snap.pending.tile)}${snap.pending.rob ? "（抢杠）" : ""}`
        : `轮到座位 ${snap.turn}`,
      `已声明：${declared.length ? declared.join("、") : "无"}`,
    ].join("\n");

    // 按钮：非「打牌」的动作（打牌＝点击手牌）；完全由服务器 actions 驱动
    const acts = view.actions().filter((a) => a.kind !== "discard");
    this.els.buttons.replaceChildren(
      ...acts.map((a) => {
        const b = document.createElement("button");
        b.textContent = actionLabel(a);
        b.onclick = () => this.intent(a);
        return b;
      }),
    );

    this.els.log.textContent = view.log.slice(-80).join("\n");

    if (view.result) {
      this.showSettle(view.result);
    } else {
      this.els.settle.classList.add("hidden");
    }
  }

  showSettle(result: Result): void {
    const el = this.els.settle;
    el.classList.remove("hidden");
    const parts: HTMLElement[] = [];
    const h2 = document.createElement("h2");
    h2.textContent = result.exhaustive
      ? "流局"
      : `和牌：座位 ${result.winner}（${winSourceText(result.source)}${result.feeder >= 0 ? `，放铳座位 ${result.feeder}` : ""}）`;
    parts.push(h2);

    for (let s = 0; s < 4; s++) {
      const div = document.createElement("div");
      const hand = (result.hands?.[s] ?? []).map(tileLabel).join(" ");
      const melds = (result.melds?.[s] ?? [])
        .map((m) => `[${(m.tiles ?? []).map(tileLabel).join("")}]`)
        .join("");
      div.textContent = `座位 ${s}：${hand}${melds ? `　副子：${melds}` : ""}`;
      parts.push(div);
    }

    if (!result.exhaustive && (result.decompositions?.length ?? 0) > 0) {
      const d = document.createElement("div");
      const shapes = [...new Set((result.decompositions ?? []).map((h) => shapeText(h.shape)))];
      d.textContent = `和牌分解候选 ${result.decompositions!.length} 个：${shapes.join("、")}`;
      parts.push(d);
    }

    const slot = document.createElement("div");
    slot.className = "fan-slot";
    slot.textContent =
      "番种计分接入位：本原型按已确认边界不含番种计分与 8 分起和线（GX-3）。" +
      "接入后此处展示：番种清单与分项、总番、是否成和、四家得分（零和）。";
    parts.push(slot);

    el.replaceChildren(...parts);
  }
}

function actionLabel(a: Action): string {
  switch (a.kind) {
    case "win":
      return "胡（和牌）";
    case "chi":
      return `吃 ${(a.with ?? []).map(tileLabel).join("+")}`;
    case "pong":
      return "碰";
    case "kongDiscard":
      return "明杠";
    case "kongConcealed":
      return `暗杠 ${a.tile ? tileLabel(a.tile) : ""}`;
    case "kongPromote":
      return `补杠 ${a.tile ? tileLabel(a.tile) : ""}`;
    case "pass":
      return "过";
    default:
      return a.kind;
  }
}