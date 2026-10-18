// TableScreen（PixiJS 牌桌）＋ Hud（DOM 信息/按钮/日志/结算）。
// 只读 SeatView 的模型、只发意图；按钮与可点手牌完全由服务器下发的 actions 驱动——界面零规则。
import { Application, Container, Graphics, Text } from "pixi.js";
import type { Action } from "./protocol/protocol";
import type { DiscardEntry, Meld, Result, Snapshot, Tile } from "./protocol/engine";
import type { ScoreResult } from "./protocol/score";
import { SeatView, payoutText, shapeText, statePhaseText, tileLabel, windText, winSourceText } from "./game";

const TW = 44;
const TH = 62;
const GAP = 6;

const COLOR_LABEL = 0x9fc0a8;
const COLOR_ACTIVE = 0xffd76a;

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
    const canAct = view.actions().length > 0;

    // 自己：标签 + 牌河 + 手牌（可点击打牌）
    const hand = snap.hand ?? [];
    const handY = h - TH - 30;
    const handW = hand.length * (TW + GAP) - GAP;
    const handX = Math.max(16, (w - handW) / 2);
    const myActive = snap.phase !== 2 && (snap.turn === seat || canAct);
    this.label(
      handX,
      handY + TH + 8,
      `座位 ${seat}·${windText(snap.dealer, seat)}${seat === snap.dealer ? "（庄）" : ""}${myActive ? "　轮到你行动" : ""}`,
      myActive,
    );
    hand.forEach((t, i) => {
      const can = view.canDiscard(t);
      const c = this.tile(t, can ? () => this.intent({ kind: "discard", tile: t }) : null);
      c.position.set(handX + i * (TW + GAP), handY + (can ? -10 : 0));
      this.root.addChild(c);
    });
    this.riverOf(snap.discards?.[seat] ?? [], handX, handY - TH - 22, 16);

    // 其余三家：标签（座位风/庄/轮次/窗口状态）+ 副子 + 牌河（手牌不可见——载荷里本就没有）
    for (const s of [0, 1, 2, 3]) {
      if (s === seat) continue;
      const rel = (s - seat + 4) % 4; // 1 下家（右）2 对家（上）3 上家（左）
      let x: number;
      let y: number;
      if (rel === 1) {
        x = w - 150;
        y = 240;
      } else if (rel === 2) {
        x = w / 2 - 150;
        y = 24;
      } else {
        x = 20;
        y = 240;
      }
      this.seatBlock(s, snap, x, y);
    }

    // 中央信息
    const lines: string[] = [`圈风：东　牌墙：${snap.wallCount}`];
    if (snap.phase === 1 && snap.pending) {
      lines.push(
        `声明窗口：${tileLabel(snap.pending.tile)}（来自座位 ${snap.pending.seat}·${windText(snap.dealer, snap.pending.seat)}${snap.pending.rob ? "，抢杠" : ""}）`,
      );
      const waiting = [0, 1, 2, 3].filter((s) => s !== snap.pending!.seat && !(snap.declared?.[s] ?? false));
      lines.push(`等待声明：${waiting.map((s) => `座位 ${s}`).join("、") || "—"}`);
    } else if (snap.phase === 2) {
      lines.push("对局结束");
    } else {
      lines.push(`轮到：座位 ${snap.turn}·${windText(snap.dealer, snap.turn)}`);
    }
    const info = new Text({
      text: lines.join("\n"),
      style: { fontSize: 15, fill: 0xcfe4d4, align: "center", fontFamily: "Microsoft YaHei, sans-serif" },
    });
    info.anchor.set(0.5);
    info.position.set(w / 2, h / 2 - 30);
    this.root.addChild(info);
  }

  // 某座位块：标签 + 副子 + 牌河
  private seatBlock(s: number, snap: Snapshot, x: number, y: number): void {
    const isTurn = snap.phase === 0 && snap.turn === s;
    const isSupplier = snap.phase === 1 && snap.pending?.seat === s;
    let suffix = "";
    if (isSupplier) suffix = "　▶ 供牌中";
    else if (snap.phase === 1 && snap.declared?.[s]) suffix = "　已声明";
    else if (isTurn) suffix = "　▶ 行动中";
    this.label(
      x,
      y - 20,
      `座位 ${s}·${windText(snap.dealer, s)}${s === snap.dealer ? "（庄）" : ""}${suffix}`,
      isTurn || isSupplier,
    );

    let cx = x;
    for (const m of snap.melds?.[s] ?? []) {
      cx += this.meld(m, cx, y) + GAP;
    }
    this.riverOf(snap.discards?.[s] ?? [], x, y + 56, 8);
  }

  private label(x: number, y: number, text: string, active: boolean): void {
    const t = new Text({
      text,
      style: {
        fontSize: 13,
        fill: active ? COLOR_ACTIVE : COLOR_LABEL,
        fontFamily: "Microsoft YaHei, sans-serif",
        fontWeight: active ? "700" : "400",
      },
    });
    t.position.set(x, y);
    this.root.addChild(t);
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
  private settleTimer: ReturnType<typeof setTimeout> | null = null;
  private settleShown = false;

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
    const myWind = windText(snap.dealer, snap.seat);
    const declared = (snap.declared ?? []).map((d, s) => (d ? `座位 ${s}` : null)).filter(Boolean);
    const hint =
      snap.phase === 2
        ? "对局结束"
        : view.actions().length > 0
          ? "轮到你行动"
          : snap.phase === 1 && snap.pending
            ? "等待其他家声明…"
            : "等待其他家…";
    this.els.info.textContent = [
      `你的座位：${snap.seat}·${myWind}${snap.dealer === snap.seat ? "（庄家）" : ""}　圈风：东`,
      `阶段：${statePhaseText(snap.phase)}　剩牌：${snap.wallCount}`,
      snap.pending
        ? `声明窗口：${tileLabel(snap.pending.tile)}（来自座位 ${snap.pending.seat}${snap.pending.rob ? "，抢杠" : ""}）`
        : `轮到：座位 ${snap.turn}·${windText(snap.dealer, snap.turn)}`,
      `已声明：${declared.length ? declared.join("、") : "无"}　——　${hint}`,
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

    // 结算：稍等一拍再浮出（让最后一张牌/事件先走完），只弹一次
    if (view.result) {
      if (!this.settleShown) {
        this.settleShown = true;
        this.settleTimer = setTimeout(() => this.showSettle(view.result!, view.score()), 900);
      }
    } else {
      this.settleShown = false;
      if (this.settleTimer) {
        clearTimeout(this.settleTimer);
        this.settleTimer = null;
      }
      this.els.settle.classList.add("hidden");
    }
  }

  showSettle(result: Result, score: ScoreResult | null): void {
    const el = this.els.settle;
    el.classList.remove("hidden");
    const parts: HTMLElement[] = [];
    const h2 = document.createElement("h2");
    h2.textContent = result.exhaustive
      ? "流局"
      : `和牌：座位 ${result.winner}（${winSourceText(result.source)}${result.feeder >= 0 ? `，放铳座位 ${result.feeder}` : ""}）`;
    parts.push(h2);

    // GX-3 计分与结算
    if (score) {
      const box = document.createElement("div");
      box.className = "fan-slot";
      const rows: string[] = [];
      if (score.exhaustive) {
        rows.push("流局：四家零和，无得失分。");
      } else {
        const fans = score.best.fans
          .map((f) => (f.count > 1 ? `${f.kind}×${f.count}（${f.value} 分）` : `${f.kind}（${f.value} 分）`))
          .join("、");
        rows.push(`番种：${fans || "无"}`);
        rows.push(`总番：${score.best.total}　起和线：${score.eligible ? "已达（≥8 番）" : "未达（<8 番）"}`);
        rows.push(
          `结算：${payoutText(score.settlement.kind)}　底分 ${score.settlement.base}　番种分 ${score.settlement.fanScore}`,
        );
        rows.push(
          `四家得失分（零和）：${score.settlement.deltas
            .map((d, s) => `座位 ${s} ${d >= 0 ? "+" : ""}${d}`)
            .join("　")}`,
        );
      }
      box.textContent = `GX-3 计分与结算（东风圈常量）\n${rows.join("\n")}`;
      box.style.whiteSpace = "pre-wrap";
      parts.push(box);
    }

    // 四家终局揭示
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