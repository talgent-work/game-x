package table

import (
	"encoding/json"
	"testing"

	"talgent-work/game-x/server/internal/engine"
	"talgent-work/game-x/server/internal/protocol"
)

// TestVisibleEventRedaction 单元级验证可见性投影与共享载荷隔离。
func TestVisibleEventRedaction(t *testing.T) {
	tile := engine.Tile{Suit: engine.SuitWan, Rank: 5}
	kong := engine.Meld{Kind: engine.MeldKong, Tiles: []engine.Tile{tile, tile, tile, tile}, From: -1, Kong: engine.KongFromConcealed}
	ev := engine.Event{Kind: engine.EvMeld, Seat: 2, Meld: &kong}

	// 他人：暗杠牌面抹去
	msg, ok := visibleEvent(ev, 0)
	if !ok || msg.Event == nil || msg.Event.Meld == nil {
		t.Fatal("meld 事件应发给他人")
	}
	if msg.Event.Meld.Tiles != nil {
		t.Fatal("他人视图暗杠牌面应抹去")
	}
	// 引擎共享载荷未被改动
	if len(kong.Tiles) != 4 {
		t.Fatal("共享载荷被就地改动")
	}
	// 本人：完整牌面
	msg2, _ := visibleEvent(ev, 2)
	if len(msg2.Event.Meld.Tiles) != 4 {
		t.Fatal("本人应看到完整暗杠牌面")
	}
	// 明杠（非暗杠）：对所有人公开
	pong := engine.Meld{Kind: engine.MeldPong, Tiles: []engine.Tile{tile, tile, tile}, From: 1}
	msg3, _ := visibleEvent(engine.Event{Kind: engine.EvMeld, Seat: 2, Meld: &pong}, 0)
	if len(msg3.Event.Meld.Tiles) != 3 {
		t.Fatal("碰应公开牌面")
	}
	// 摸牌：他人不收
	draw := engine.Event{Kind: engine.EvDrawn, Seat: 1, Tile: &tile, Source: engine.DrawWall}
	if _, ok := visibleEvent(draw, 2); ok {
		t.Fatal("他人摸牌不应外发")
	}
	if m, ok := visibleEvent(draw, 1); !ok || m.Event.Tile == nil || m.Event.Source != protocol.DrawWall {
		t.Fatal("本人摸牌应携带牌面与来源")
	}
}

// TestSlowConsumerHangup 慢消费者只丢它自己：缓冲（cap=1）灌满即挂断，座位释放，对局不受影响。
func TestSlowConsumerHangup(t *testing.T) {
	tb := New(Config{Seed: 3, Dealer: 0, Outbox: 1})
	defer tb.Close()

	seat0 := 0
	c0 := tb.Attach()
	b, _ := json.Marshal(protocol.Msg{Type: protocol.TypeHello, Version: protocol.Version, Seat: &seat0})
	if err := tb.Handle(c0, b); err != nil {
		t.Fatal(err)
	}
	// 读掉首推 state，取一张可打的牌
	var st protocol.Msg
	select {
	case bb := <-c0.Out():
		if err := json.Unmarshal(bb, &st); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("hello 后应有 state")
	}
	if st.State == nil || len(st.State.Actions) == 0 || st.State.Actions[0].Kind != protocol.KindDiscard {
		t.Fatalf("应有打牌动作，得 %+v", st.State)
	}

	// 打牌：自身的 EvDiscarded（1/1）之后，state 推送灌满缓冲 → 挂断
	act := st.State.Actions[0]
	ba, _ := json.Marshal(protocol.Msg{Type: protocol.TypeAction, Action: &act})
	if err := tb.Handle(c0, ba); err != nil {
		t.Fatal(err)
	}
	if _, ok := <-c0.Out(); !ok {
		t.Fatal("应先读到事件")
	}
	if _, ok := <-c0.Out(); ok {
		t.Fatal("慢消费者应被挂断（通道关闭）")
	}
	if tb.seat[0] != nil {
		t.Fatal("挂断后座位应释放")
	}
	if tb.Over() {
		t.Fatal("对局不应因单连接挂断而结束")
	}
}
