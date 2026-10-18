// 渲染层入口：4 条座位连接同走协议路径——座位 0 由人操作（点击手牌/按钮），
// 座位 1–3 由无策略随机合法动作自动驱动（种子可复现）。
import { SeatConnection } from "./net";
import { SeatView } from "./game";
import { chooseAuto, mulberry32 } from "./autoplay";
import { Hud, TableScreen } from "./ui";
import type { Action } from "./protocol/protocol";

const params = new URLSearchParams(location.search);
const PORT = params.get("port") ?? "8787";
const URL_WS = `ws://127.0.0.1:${PORT}/ws`;
const HUMAN_SEAT = 0;
const AUTO_SEED = Number(params.get("seed") ?? "20261018");

async function boot(): Promise<void> {
  const board = document.getElementById("board");
  const info = document.getElementById("info");
  const buttons = document.getElementById("buttons");
  const log = document.getElementById("log");
  const settle = document.getElementById("settle");
  if (!board || !info || !buttons || !log || !settle) throw new Error("页面结构缺失");

  const screen = new TableScreen();
  await screen.init(board);
  const hud = new Hud({ info, buttons, log, settle });

  const conns = new Map<number, SeatConnection>();
  for (let seat = 0; seat < 4; seat++) {
    const view = new SeatView();
    const conn = new SeatConnection(URL_WS, seat);
    conns.set(seat, conn);
    const rng = mulberry32(AUTO_SEED + seat * 7919);

    conn.onMessage((m) => {
      view.apply(m);
      if (seat === HUMAN_SEAT) {
        screen.render(view);
        hud.render(view);
      } else if (m.type === "state" && m.state && m.state.actions.length > 0) {
        // 自动座：从服务器给的合法动作里挑一条（与 E2E、与引擎随机模拟同一逻辑）
        const pick = chooseAuto(m.state.actions, rng);
        if (pick) setTimeout(() => conn.send({ type: "action", action: pick }), 90);
      }
    });
  }

  const sendHuman = (a: Action): void => {
    conns.get(HUMAN_SEAT)?.send({ type: "action", action: a });
  };
  screen.setIntentHandler(sendHuman);
  hud.setIntentHandler(sendHuman);
}

void boot();