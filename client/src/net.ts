// SeatConnection：一条座位连接的客户端（seam：浏览器 WebSocket 与 Node 的全局 WebSocket 同一 API）。
// 只做连接与会话：编解码、消息路由、连接状态；不含任何对局逻辑。
import type { Msg } from "./protocol/protocol";

export type ConnStatus = "connecting" | "open" | "closed";

export class SeatConnection {
  readonly seat: number;
  status: ConnStatus = "connecting";
  onStatus?: (s: ConnStatus) => void;

  private ws: WebSocket;
  private handlers: Array<(m: Msg) => void> = [];

  constructor(url: string, seat: number) {
    this.seat = seat;
    this.ws = new WebSocket(url);
    this.ws.addEventListener("open", () => {
      this.status = "open";
      this.onStatus?.(this.status);
      // 入座：协议版本 v1 + 请求座位（下一 Cycle 联机时座位由服务端按票据覆写）
      this.send({ type: "hello", version: 1, seat });
    });
    this.ws.addEventListener("message", (ev: MessageEvent) => {
      let m: Msg;
      try {
        m = JSON.parse(String(ev.data)) as Msg;
      } catch {
        return;
      }
      for (const h of this.handlers) h(m);
    });
    this.ws.addEventListener("close", () => {
      this.status = "closed";
      this.onStatus?.(this.status);
    });
    this.ws.addEventListener("error", () => {
      /* 随后会有 close 事件 */
    });
  }

  onMessage(h: (m: Msg) => void): void {
    this.handlers.push(h);
  }

  send(m: Msg): void {
    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(m));
    }
  }

  close(): void {
    this.ws.close();
  }
}