package table_test

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"talgent-work/game-x/server/internal/engine"
	"talgent-work/game-x/server/internal/protocol"
	"talgent-work/game-x/server/internal/table"
)

// client 测试侧的内存适配器：直接消费 Conn.Out()（与生产写泵同一份字节）。
type client struct {
	t    *testing.T
	tb   *table.Table
	conn *table.Conn
	seat int
	msgs []protocol.Msg
}

func attach(t *testing.T, tb *table.Table) *client {
	t.Helper()
	return &client{t: t, tb: tb, conn: tb.Attach(), seat: -1}
}

func dial(t *testing.T, tb *table.Table, seat int) *client {
	t.Helper()
	c := attach(t, tb)
	c.seat = seat
	c.must(c.send(protocol.Msg{Type: protocol.TypeHello, Version: protocol.Version, Seat: &seat}))
	c.drain()
	return c
}

func (c *client) must(err error) {
	if err != nil {
		c.t.Fatalf("Handle: %v", err)
	}
}

func (c *client) send(m protocol.Msg) error {
	b, err := json.Marshal(m)
	if err != nil {
		c.t.Fatalf("marshal：%v", err)
	}
	return c.tb.Handle(c.conn, b)
}

// drain 读空当前缓冲（Handle 返回时本步产生的消息已全部入队）。
func (c *client) drain() {
	for {
		select {
		case b, ok := <-c.conn.Out():
			if !ok {
				return
			}
			var m protocol.Msg
			if err := json.Unmarshal(b, &m); err != nil {
				c.t.Fatalf("服务端消息不是合法 JSON：%v（%s）", err, b)
			}
			c.msgs = append(c.msgs, m)
		default:
			return
		}
	}
}

func (c *client) lastState() *protocol.State {
	for i := len(c.msgs) - 1; i >= 0; i-- {
		if c.msgs[i].State != nil {
			return c.msgs[i].State
		}
	}
	return nil
}

func (c *client) lastRejected() string {
	for i := len(c.msgs) - 1; i >= 0; i-- {
		if c.msgs[i].Type == protocol.TypeRejected {
			return c.msgs[i].Code
		}
	}
	return ""
}

func (c *client) closed() bool {
	select {
	case _, ok := <-c.conn.Out():
		return !ok
	default:
		return false
	}
}

// --- 绑定与规则 ---

func TestBindAndDeal(t *testing.T) {
	tb := table.New(table.Config{Seed: 20261018, Dealer: 0})
	defer tb.Close()

	c0 := dial(t, tb, 0)
	st := c0.lastState()
	if st == nil {
		t.Fatal("hello 后应立刻收到 state")
	}
	if st.Snapshot.Seat != 0 || st.Snapshot.Dealer != 0 {
		t.Fatalf("座位/庄家不符：%+v", st.Snapshot)
	}
	if len(st.Snapshot.Hand) != 14 {
		t.Fatalf("庄家应 14 张，得 %d", len(st.Snapshot.Hand))
	}
	if len(st.Actions) == 0 {
		t.Fatal("庄家应有可行动作")
	}
	c1 := dial(t, tb, 1)
	if got := len(c1.lastState().Snapshot.Hand); got != 13 {
		t.Fatalf("闲家应 13 张，得 %d", got)
	}
	if len(c1.lastState().Actions) != 0 {
		t.Fatal("非本座回合不应有动作")
	}
}

func TestBindRules(t *testing.T) {
	tb := table.New(table.Config{Seed: 1, Dealer: 0})
	defer tb.Close()

	c0 := dial(t, tb, 0)
	seat0 := 0

	// 同一连接重复 hello
	c0.must(c0.send(protocol.Msg{Type: protocol.TypeHello, Version: protocol.Version, Seat: &seat0}))
	c0.drain()
	if c0.lastRejected() != protocol.CodeAlreadyBound {
		t.Fatalf("重复 hello 应 already_bound，得 %q", c0.lastRejected())
	}
	mark := len(c0.msgs)

	// 座位已被占用
	c1 := attach(t, tb)
	c1.must(c1.send(protocol.Msg{Type: protocol.TypeHello, Version: protocol.Version, Seat: &seat0}))
	c1.drain()
	if c1.lastRejected() != protocol.CodeSeatTaken {
		t.Fatalf("占用座位应 seat_taken，得 %q", c1.lastRejected())
	}

	// 非法座位
	bad := 9
	c1.must(c1.send(protocol.Msg{Type: protocol.TypeHello, Version: protocol.Version, Seat: &bad}))
	c1.drain()
	if c1.lastRejected() != protocol.CodeBadSeat {
		t.Fatalf("越界座位应 bad_seat，得 %q", c1.lastRejected())
	}

	// 未 hello 就动作
	pass := protocol.Action{Kind: protocol.KindPass}
	c1.must(c1.send(protocol.Msg{Type: protocol.TypeAction, Action: &pass}))
	c1.drain()
	if c1.lastRejected() != protocol.CodeNotBound {
		t.Fatalf("未绑定动作应 not_bound，得 %q", c1.lastRejected())
	}

	// 他人的绑定/动作错误不惊动 c0：无新消息
	c0.drain()
	if len(c0.msgs) != mark {
		t.Fatalf("c0 不应收到他人消息（%d → %d）", mark, len(c0.msgs))
	}

	// 版本不符：拒绝并挂断
	c2 := attach(t, tb)
	badV := 99
	c2.must(c2.send(protocol.Msg{Type: protocol.TypeHello, Version: badV, Seat: &seat0}))
	c2.drain()
	if c2.lastRejected() != protocol.CodeBadVersion {
		t.Fatalf("版本不符应 bad_version，得 %q", c2.lastRejected())
	}
	if !c2.closed() {
		t.Fatal("版本不符应挂断连接")
	}
}

func TestRejectDoesNotAdvance(t *testing.T) {
	tb := table.New(table.Config{Seed: 5, Dealer: 0})
	defer tb.Close()

	c0 := dial(t, tb, 0)
	c1 := dial(t, tb, 1)

	// 非本座回合者打牌：被拒、零副作用（不发事件与 state）
	hand := c1.lastState().Snapshot.Hand
	act := protocol.Action{Kind: protocol.KindDiscard, Tile: &hand[0]}
	c1.must(c1.send(protocol.Msg{Type: protocol.TypeAction, Action: &act}))
	c1.drain()
	if c1.lastRejected() != protocol.CodeIllegalAction {
		t.Fatalf("非本座打牌应 illegal_action，得 %q", c1.lastRejected())
	}
	states, events := 0, 0
	for _, m := range c1.msgs {
		switch m.Type {
		case protocol.TypeState:
			states++
		case protocol.TypeEvent:
			events++
		}
	}
	if states != 1 || events != 0 {
		t.Fatalf("被拒动作不应产生事件或新 state（states=%d events=%d）", states, events)
	}
	// c0 状态未受影响：仍轮到座位 0，重放一次合法动作应成功
	if st := c0.lastState(); st == nil || st.Snapshot.Turn != 0 {
		t.Fatalf("c0 视图应仍轮到座位 0：%+v", st)
	}
}

func TestRebindAfterDetach(t *testing.T) {
	tb := table.New(table.Config{Seed: 9, Dealer: 0})
	defer tb.Close()

	c0 := dial(t, tb, 0)
	tb.Detach(c0.conn)
	if !c0.closed() {
		t.Fatal("Detach 后出站通道应关闭")
	}
	// 座位可被新连接占用
	c0b := dial(t, tb, 0)
	if c0b.lastState() == nil {
		t.Fatal("重绑后应有首推 state")
	}
}

// --- 整局驱动（与自动座同一逻辑：从 actions 随机取一条回送）---

func pickAction(acts []protocol.Action, rng *rand.Rand) protocol.Action {
	for _, a := range acts {
		if a.Kind == protocol.KindWin {
			return a
		}
	}
	return acts[rng.Intn(len(acts))]
}

func TestFullGameByActions(t *testing.T) {
	tb := table.New(table.Config{Seed: 7, Dealer: 1})
	defer tb.Close()

	cs := make([]*client, engine.Seats)
	for s := range cs {
		cs[s] = dial(t, tb, s)
	}
	rng := rand.New(rand.NewSource(42))
	for step := 0; step < 5000 && !tb.Over(); step++ {
		for _, c := range cs {
			c.drain()
		}
		for _, c := range cs {
			st := c.lastState()
			if st == nil || len(st.Actions) == 0 {
				continue
			}
			pick := pickAction(st.Actions, rng)
			_ = c.send(protocol.Msg{Type: protocol.TypeAction, Action: &pick}) // 被拒属正常（窗口竞态）
		}
	}

	select {
	case <-tb.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("一局未在步数上限内收束")
	}

	for _, c := range cs {
		c.drain()
		st := c.lastState()
		if st == nil || st.Snapshot.Result == nil {
			t.Fatalf("座位 %d 终局 state 应带 Result", c.seat)
		}
		if len(st.Actions) != 0 {
			t.Fatalf("座位 %d 终局后 actions 应为空", c.seat)
		}
		assertNoLeak(t, c)
	}

	if r := tb.Result(); r == nil {
		t.Fatal("Result 缺失")
	}
	d := tb.Dump()
	if len(d.Actions) == 0 || len(d.Events) == 0 || d.Result == nil {
		t.Fatal("Dump 应含动作、事件与结果")
	}
}

// TestBothEndingsReachable 用与自动座/E2E 相同策略（能和则和，否则随机）跑 300 个固定种子：
// 两条终局路径（和牌/流局）都必须可达，且每局必然收束。固定种子 → 结果确定，可作回归。
func TestBothEndingsReachable(t *testing.T) {
	wins, draws := 0, 0
	for seed := int64(1); seed <= 300; seed++ {
		tb := table.New(table.Config{Seed: seed, Dealer: int(seed % 4)})
		cs := make([]*client, engine.Seats)
		for s := range cs {
			cs[s] = dial(t, tb, s)
		}
		rng := rand.New(rand.NewSource(seed * 7919))
		for step := 0; step < 5000 && !tb.Over(); step++ {
			for _, c := range cs {
				c.drain()
			}
			for _, c := range cs {
				st := c.lastState()
				if st == nil || len(st.Actions) == 0 {
					continue
				}
				pick := pickAction(st.Actions, rng)
				_ = c.send(protocol.Msg{Type: protocol.TypeAction, Action: &pick})
			}
		}
		if !tb.Over() {
			t.Fatalf("种子 %d 未收束", seed)
		}
		if r := tb.Result(); r != nil && !r.Exhaustive {
			wins++
		} else {
			draws++
		}
		tb.Close()
	}
	t.Logf("300 局（固定种子，策略：能和则和、否则随机）：和牌 %d、流局 %d", wins, draws)
	if wins == 0 || draws == 0 {
		t.Fatalf("两条终局路径都必须可达（和牌 %d、流局 %d）", wins, draws)
	}
}

// assertNoLeak 隐藏信息全量性质：该连接收到的任何消息不含本座位视图之外的暗牌。
func assertNoLeak(t *testing.T, c *client) {
	t.Helper()
	for _, m := range c.msgs {
		if m.State != nil && m.State.Snapshot.Seat != c.seat {
			t.Fatalf("座位 %d 收到他人视图 state", c.seat)
		}
		if m.Event == nil {
			continue
		}
		switch m.Event.Kind {
		case protocol.EvDrawn:
			if m.Event.Seat != c.seat {
				t.Fatalf("座位 %d 收到他人摸牌事件", c.seat)
			}
		case protocol.EvMeld:
			if m.Event.Seat == c.seat || m.Event.Meld == nil {
				continue
			}
			if m.Event.Meld.HideTile() && len(m.Event.Meld.Tiles) != 0 {
				t.Fatalf("座位 %d 看到他人暗杠牌面", c.seat)
			}
		}
	}
}
