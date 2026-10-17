// GX-1 最小实证 4b：Node↔Go 协议回环的服务端（Go，权威方）。
//
// 它复用 4a 的 engine 包（跨模块 replace 引用），通过 WebSocket 与 TS 客户端
// 交换 JSON 消息：hello → snapshot；action(discard) → discarded / rejected。
// 服务器是唯一的规则执行者：客户端只能"请求"，非法请求按权威规则拒绝。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"talgent-work/game-x/spikes/go-engine/engine"
)

type tileMsg = engine.Tile // 协议直接复用引擎牌型的 JSON 形状

type inAction struct {
	Kind string   `json:"kind"`
	Tile *tileMsg `json:"tile,omitempty"`
}

type inMsg struct {
	Type   string    `json:"type"`
	Seat   int       `json:"seat,omitempty"`
	Action *inAction `json:"action,omitempty"`
}

type outMsg struct {
	Type      string           `json:"type"`
	Seat      int              `json:"seat,omitempty"`
	Snapshot  *engine.Snapshot `json:"snapshot,omitempty"`
	Tile      *tileMsg         `json:"tile,omitempty"`
	Turn      int              `json:"turn,omitempty"`
	WallCount int              `json:"wallCount,omitempty"`
	Reason    string           `json:"reason,omitempty"`
}

func writeJSON(ctx context.Context, c *websocket.Conn, m outMsg) {
	b, err := json.Marshal(m)
	if err != nil {
		log.Printf("序列化失败: %v", err)
		return
	}
	log.Printf("→ %s", b)
	_ = c.Write(ctx, websocket.MessageText, b)
}

func main() {
	tb := engine.NewTable(20261017) // 与 4a 相同的确定性种子

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			log.Printf("accept 失败: %v", err)
			return
		}
		defer c.CloseNow()
		log.Printf("客户端已连接: %s", r.RemoteAddr)

		ctx := context.Background()
		connSeat := -1

		for {
			readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			typ, data, err := c.Read(readCtx)
			cancel()
			if err != nil {
				log.Printf("连接结束: %v", err)
				return
			}
			if typ != websocket.MessageText {
				continue
			}
			log.Printf("← %s", data)

			var m inMsg
			if err := json.Unmarshal(data, &m); err != nil {
				writeJSON(ctx, c, outMsg{Type: "rejected", Reason: "无效 JSON"})
				continue
			}

			switch m.Type {
			case "hello":
				if m.Seat < 0 || m.Seat >= engine.Seats {
					writeJSON(ctx, c, outMsg{Type: "rejected", Reason: "座位号无效"})
					continue
				}
				connSeat = m.Seat
				snap := tb.ViewFor(connSeat)
				writeJSON(ctx, c, outMsg{Type: "snapshot", Snapshot: &snap})

			case "action":
				if connSeat < 0 {
					writeJSON(ctx, c, outMsg{Type: "rejected", Reason: "请先发送 hello"})
					continue
				}
				if m.Action == nil || m.Action.Kind != "discard" || m.Action.Tile == nil {
					writeJSON(ctx, c, outMsg{Type: "rejected", Reason: "不支持的操作"})
					continue
				}
				if turn := tb.ViewFor(connSeat).Turn; turn != connSeat {
					writeJSON(ctx, c, outMsg{Type: "rejected", Reason: fmt.Sprintf("不是你的回合（当前轮到座位 %d）", turn)})
					continue
				}
				if err := tb.Discard(*m.Action.Tile); err != nil {
					writeJSON(ctx, c, outMsg{Type: "rejected", Reason: err.Error()})
					continue
				}
				snap := tb.ViewFor(connSeat)
				writeJSON(ctx, c, outMsg{Type: "discarded", Seat: connSeat, Tile: m.Action.Tile, Turn: snap.Turn, WallCount: snap.WallCount})

			default:
				writeJSON(ctx, c, outMsg{Type: "rejected", Reason: "未知消息类型"})
			}
		}
	})

	log.Println("权威服务端（最小样例）监听 127.0.0.1:8787，端点 /ws")
	if err := http.ListenAndServe("127.0.0.1:8787", nil); err != nil {
		log.Fatal(err)
	}
}
