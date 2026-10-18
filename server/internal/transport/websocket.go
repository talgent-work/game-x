// Package transport 是 GX-4 的 WebSocket 连接层：端口（table.Conn）的生产适配器。
//
// 只做 WS 机制——accept、读泵（字节 → table.Handle）、写泵（Conn.Out → 字节）、关闭清理；
// 不懂协议内容、不懂座位、不懂规则。背压与挂断政策在 table 侧（有界出站通道）。
package transport

import (
	"log"
	"net/http"

	"github.com/coder/websocket"

	"talgent-work/game-x/server/internal/table"
)

// Handler 构建 WebSocket 端点：每条 WS 连接 = table 的一条 Conn。
func Handler(tb *table.Table) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			log.Printf("websocket 接受失败：%v", err)
			return
		}
		ctx := r.Context()
		conn := tb.Attach()

		writeDone := make(chan struct{})
		go func() {
			defer close(writeDone)
			for raw := range conn.Out() { // 通道关闭＝服务端挂断（终局/违规/慢消费者）
				if err := ws.Write(ctx, websocket.MessageText, raw); err != nil {
					break
				}
			}
			_ = ws.Close(websocket.StatusNormalClosure, "对局结束")
		}()

		for {
			_, raw, err := ws.Read(ctx)
			if err != nil {
				break // 客户端断开或连接关闭
			}
			if err := tb.Handle(conn, raw); err != nil {
				break // 生命周期错误：连接已分离
			}
		}
		tb.Detach(conn)
		<-writeDone
	}
}
