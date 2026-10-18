// Package table 实现 GX-4 的「一桌会话」深模块：engine 与 transport 之间的权威会话层。
//
// 职责（且仅此）：座位绑定（hello）、动作入口（交给 engine.Apply）、事件按座位可见性分发、
// 每次状态变化后向每个已绑定座位推送该座位可见的 state（快照＋合法动作）、终局收束。
// 规则判定一条不复制——全部归 engine。
//
// 并发模型（GX-1 §3.2「每桌一个 goroutine 串行处理动作」）：
//   - New 启动一个串行环 goroutine，它是 engine 状态的唯一接触者，桌内无锁；
//   - 入向方法（Attach/Handle/Detach/Close）可自任意 goroutine 调用，环内串行执行；
//   - 出向经每连接的有界通道 Conn.Out()（生产适配器＝WebSocket 写泵；测试＝直读通道），
//     慢消费者只丢它自己（挂断该连接），永不阻塞牌桌。
//
// 协议级失败（非法 JSON、未知类型、被拒动作）一律以 rejected 消息回该连接，不返回错误；
// 入向方法的 error 仅表示生命周期问题（ErrConnDetached）。
package table

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"talgent-work/game-x/server/internal/engine"
	"talgent-work/game-x/server/internal/protocol"
	"talgent-work/game-x/server/internal/score"
)

// ErrConnDetached 连接已分离（Detach/挂断/关卡后 Handle 的唯一错误）。
var ErrConnDetached = errors.New("连接已分离")

const defaultOutbox = 64

// Config 建桌参数。同 Seed+Dealer 的对局完全确定（复现/回放）。
type Config struct {
	Seed   int64 // 洗牌种子
	Dealer int   // 庄家座位（越界由引擎归 0）
	Outbox int   // 每连接出站缓冲容量；<=0 → 64
}

func (cfg Config) outbox() int {
	if cfg.Outbox <= 0 {
		return defaultOutbox
	}
	return cfg.Outbox
}

// ActionRecord 服务端视角的一次入站动作（含被拒），按序落盘供复现与人工核对。
type ActionRecord struct {
	Seat   int             `json:"seat"`
	Action protocol.Action `json:"action"`
	Error  string          `json:"error,omitempty"` // 空＝成功
}

// Dump 一局的完整记录（服务端视角，含隐藏信息与终局全揭示）。
// 供 --dump 落盘、复现与规则核对；绝不可下发客户端。
type Dump struct {
	Seed    int64              `json:"seed"`
	Dealer  int                `json:"dealer"`
	Actions []ActionRecord     `json:"actions"`
	Events  []protocol.Event   `json:"events"` // 引擎事件原文（不按可见性过滤）
	Result  *engine.Result     `json:"result,omitempty"`
	Score   *score.ScoreResult `json:"score,omitempty"` // GX-3 计分与结算（东风圈常量）：番种清单/总番/四家得分
}

// Conn 一条座位连接的句柄：表创建，运输层消费其出站通道。
type Conn struct {
	tbl    *Table
	out    chan []byte
	seat   atomic.Int32
	closed bool // 仅串行环读写
}

// Out 出站 JSON 消息流；写泵 for-range 消费，通道关闭＝服务端挂断该连接。
// 通道只由表关闭，运输层不得关闭。
func (c *Conn) Out() <-chan []byte { return c.out }

// Seat 已绑定座位；未绑定（hello 前）或已挂断为 -1。可随时安全调用。
func (c *Conn) Seat() int { return int(c.seat.Load()) }

// Table 一桌会话。
type Table struct {
	cfg   Config
	game  *engine.Game
	cmds  chan command
	close chan struct{}
	stop  chan struct{}
	done  chan struct{}

	doneOnce sync.Once

	// 以下字段仅串行环访问：
	conns   map[*Conn]struct{}
	seat    [engine.Seats]*Conn
	history []ActionRecord
	over    bool
	score   *score.ScoreResult // 终局计分与结算（GX-3），终局前为 nil
}

type cmdKind uint8

const (
	cmdAttach cmdKind = iota
	cmdHandle
	cmdDetach
)

type command struct {
	kind  cmdKind
	conn  *Conn
	raw   []byte
	reply chan error // 缓冲 1，环内非阻塞投递
}

// New 建桌并立即开局（发牌完成），启动串行环。Close 收束。
func New(cfg Config) *Table {
	t := &Table{
		game:  engine.NewGame(cfg.Seed, cfg.Dealer),
		cmds:  make(chan command, 64),
		close: make(chan struct{}),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
		conns: map[*Conn]struct{}{},
	}
	t.cfg = cfg
	go t.loop()
	return t
}

func (t *Table) outbox() int { return t.cfg.outbox() }

// Attach 挂一条新连接（尚未绑定座位），返回交给读/写泵的句柄。
func (t *Table) Attach() *Conn {
	c := &Conn{tbl: t, out: make(chan []byte, t.outbox())}
	c.seat.Store(-1)
	reply := make(chan error, 1)
	select {
	case t.cmds <- command{kind: cmdAttach, conn: c, reply: reply}:
		select {
		case <-reply:
		case <-t.stop:
		}
	case <-t.stop:
	}
	return c
}

// Handle 消化一条入站原始消息（唯一入站入口）：解码 → 路由（hello / action）。
// 同一 *Conn 上不得并发调用（各自读泵天然满足）；表内全局串行。
// 返回非 nil 仅表示生命周期问题（ErrConnDetached），读泵见此即退出。
func (t *Table) Handle(c *Conn, raw []byte) error {
	reply := make(chan error, 1)
	select {
	case t.cmds <- command{kind: cmdHandle, conn: c, raw: raw, reply: reply}:
		select {
		case err := <-reply:
			return err
		case <-t.stop:
			return ErrConnDetached
		}
	case <-t.stop:
		return ErrConnDetached
	}
}

// Detach 分离连接（幂等）：解绑座位、关闭出站通道（写泵随之退出）。
// 客户端先断开（读泵报错）或服务端挂断（终局/违规/慢消费者）之后调用皆安全。
func (t *Table) Detach(c *Conn) {
	if c == nil || c.tbl != t {
		return
	}
	reply := make(chan error, 1)
	select {
	case t.cmds <- command{kind: cmdDetach, conn: c, reply: reply}:
		select {
		case <-reply:
		case <-t.stop:
		}
	case <-t.stop:
	}
}

// Done 终局（和牌或流局）广播完成（最终事件与状态已入队、全部出站通道已关闭）后关闭。
// 此后 Result/Log/History/Dump 的内容不再变化，可安全读取。
func (t *Table) Done() <-chan struct{} { return t.done }

// Over 是否已终局。
func (t *Table) Over() bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

// Result 终局结果（未终局为 nil）。建议在 <-Done() 之后读取。
func (t *Table) Result() *engine.Result { return t.game.Result() }

// Log 引擎完整事件日志副本（含隐藏信息）。
func (t *Table) Log() []engine.Event { return t.game.Log() }

// History 服务端视角的入站动作记录（含被拒）副本。
func (t *Table) History() []ActionRecord {
	out := make([]ActionRecord, len(t.history))
	copy(out, t.history)
	return out
}

// Dump 组装一局的完整记录（含隐藏信息）。在 <-Done() 之后调用。
func (t *Table) Dump() Dump {
	evs := t.game.Log()
	wire := make([]protocol.Event, 0, len(evs))
	for _, ev := range evs {
		wire = append(wire, fullEvent(ev))
	}
	acts := t.History()
	if acts == nil {
		acts = []ActionRecord{}
	}
	return Dump{
		Seed:    t.cfg.Seed,
		Dealer:  t.game.Dealer(),
		Actions: acts,
		Events:  wire,
		Result:  t.game.Result(),
		Score:   t.score,
	}
}

// Close 收束会话：挂断全部连接、停止串行环。幂等。
func (t *Table) Close() {
	select {
	case <-t.close:
	default:
		close(t.close)
	}
	<-t.stop
}

// --- 串行环 ---

func (t *Table) loop() {
	defer close(t.stop)
	for {
		select {
		case <-t.close:
			t.hangupAll()
			return
		case cmd := <-t.cmds:
			t.exec(cmd)
		}
	}
}

func (t *Table) exec(cmd command) {
	switch cmd.kind {
	case cmdAttach:
		t.conns[cmd.conn] = struct{}{}
		cmd.reply <- nil
	case cmdDetach:
		t.hangup(cmd.conn)
		cmd.reply <- nil
	case cmdHandle:
		cmd.reply <- t.handle(cmd.conn, cmd.raw)
	}
}

func (t *Table) handle(c *Conn, raw []byte) error {
	if c.tbl != t || c.closed {
		return ErrConnDetached
	}
	var m protocol.Msg
	if err := json.Unmarshal(raw, &m); err != nil {
		t.reject(c, protocol.CodeBadJSON, "无效 JSON")
		return nil
	}
	if t.over {
		t.reject(c, protocol.CodeGameOver, "对局已结束")
		t.hangup(c)
		return nil
	}
	switch m.Type {
	case protocol.TypeHello:
		t.hello(c, m)
	case protocol.TypeAction:
		t.action(c, m)
	default:
		t.reject(c, protocol.CodeUnknownType, "未知消息类型 "+m.Type)
	}
	return nil
}

func (t *Table) hello(c *Conn, m protocol.Msg) {
	if m.Version != protocol.Version {
		t.reject(c, protocol.CodeBadVersion, fmt.Sprintf("协议版本不符：需要 %d，收到 %d", protocol.Version, m.Version))
		t.hangup(c) // 版本不符不可自愈，挂断
		return
	}
	if m.Seat == nil || *m.Seat < 0 || *m.Seat >= engine.Seats {
		t.reject(c, protocol.CodeBadSeat, "座位号非法（hello 需携带 seat 0..3）")
		return
	}
	seat := *m.Seat
	if c.seat.Load() >= 0 {
		t.reject(c, protocol.CodeAlreadyBound, "本连接已绑定座位")
		return
	}
	if t.seat[seat] != nil {
		t.reject(c, protocol.CodeSeatTaken, fmt.Sprintf("座位 %d 已被其他连接占用", seat))
		return
	}
	c.seat.Store(int32(seat))
	t.seat[seat] = c
	t.sendState(c) // 入座即拿到当前局面（迟到入座自然收敛）
}

func (t *Table) action(c *Conn, m protocol.Msg) {
	seat := int(c.seat.Load())
	if seat < 0 {
		t.reject(c, protocol.CodeNotBound, "请先发送 hello 绑定座位")
		return
	}
	if m.Action == nil {
		t.reject(c, protocol.CodeBadAction, "action 消息缺少 action 载荷")
		t.record(seat, protocol.Action{}, "action 消息缺少 action 载荷")
		return
	}
	act, err := m.Action.ToEngine()
	if err != nil {
		t.reject(c, protocol.CodeBadAction, err.Error())
		t.record(seat, *m.Action, err.Error())
		return
	}
	events, err := t.game.Apply(seat, act)
	if err != nil {
		t.reject(c, protocol.CodeIllegalAction, err.Error()) // 引擎中文原文直出
		t.record(seat, *m.Action, err.Error())
		return
	}
	t.record(seat, *m.Action, "")
	if t.game.Over() {
		// 终局：先计分（GX-3；圈风为跨局概念，本 Cycle 用东风圈常量），随终局 state 下发
		s := score.Score(t.game.Result(), score.Context{RoundWind: 1})
		t.score = &s
	}
	t.fanout(events)
	if t.game.Over() {
		t.finish()
	}
}

func (t *Table) record(seat int, a protocol.Action, errMsg string) {
	t.history = append(t.history, ActionRecord{Seat: seat, Action: a, Error: errMsg})
}

// fanout 一次成功推进的出站：按座位 0→3 升序，先该座位可见的事件（引擎顺序），后 state。
func (t *Table) fanout(events []engine.Event) {
	for _, ev := range events {
		for s := 0; s < engine.Seats; s++ {
			c := t.seat[s]
			if c == nil {
				continue
			}
			if msg, ok := visibleEvent(ev, s); ok {
				t.send(c, msg)
			}
		}
	}
	for s := 0; s < engine.Seats; s++ {
		if c := t.seat[s]; c != nil {
			t.sendState(c)
		}
	}
}

// visibleEvent 可见性投影：引擎事件 → 某座位可见形状。
//   - EvDrawn：只发摸牌者本人（其他人不发——快照的 turn/wallCount 已足够）；
//   - EvDiscarded / EvKongPromoted：公开；
//   - EvMeld：他人暗杠的牌面抹去（clone 后置空，绝不改共享载荷）；
//   - EvWin / EvExhaustive：全揭示广播。
func visibleEvent(ev engine.Event, viewer int) (protocol.Msg, bool) {
	kind := protocol.EventKindString(ev.Kind)
	switch ev.Kind {
	case engine.EvDrawn:
		if ev.Seat != viewer {
			return protocol.Msg{}, false
		}
		return protocol.EventMsg(protocol.Event{
			Kind: kind, Seat: ev.Seat, Tile: ev.Tile, Source: protocol.DrawSourceString(ev.Source),
		}), true
	case engine.EvDiscarded, engine.EvKongPromoted:
		return protocol.EventMsg(protocol.Event{Kind: kind, Seat: ev.Seat, Tile: ev.Tile}), true
	case engine.EvMeld:
		if ev.Meld == nil {
			return protocol.Msg{}, false
		}
		m := *ev.Meld
		m.Tiles = append([]engine.Tile(nil), ev.Meld.Tiles...)
		if m.HideTile() && ev.Seat != viewer {
			m.Tiles = nil // 他人暗杠：只留种类与出处
		}
		return protocol.EventMsg(protocol.Event{Kind: kind, Seat: ev.Seat, Meld: &m}), true
	case engine.EvWin, engine.EvExhaustive:
		return protocol.EventMsg(protocol.Event{Kind: kind, Seat: ev.Seat, Result: ev.Result}), true
	}
	return protocol.Msg{}, false
}

// fullEvent 事件原文（不按可见性过滤）——仅供 Dump（服务端本机文件）。
func fullEvent(ev engine.Event) protocol.Event {
	kind := protocol.EventKindString(ev.Kind)
	out := protocol.Event{Kind: kind, Seat: ev.Seat, Tile: ev.Tile, Result: ev.Result, Source: protocol.DrawSourceString(ev.Source)}
	if ev.Meld != nil {
		m := *ev.Meld
		m.Tiles = append([]engine.Tile(nil), ev.Meld.Tiles...)
		out.Meld = &m
	}
	return out
}

func (t *Table) sendState(c *Conn) {
	s := int(c.seat.Load())
	snap := t.game.ViewFor(s)
	normalizeSnapshot(&snap)
	acts := t.game.Actions(s)
	wire := make([]protocol.Action, 0, len(acts))
	for _, a := range acts {
		wire = append(wire, protocol.FromEngine(a))
	}
	msg := protocol.StateMsg(snap, wire)
	if t.score != nil {
		msg.State.Score = t.score
	}
	t.send(c, msg)
}

// normalizeSnapshot 线上不出现 null 数组：空的 discards 一律 []。
func normalizeSnapshot(s *engine.Snapshot) {
	for i := range s.Discards {
		if s.Discards[i] == nil {
			s.Discards[i] = []engine.DiscardEntry{}
		}
	}
}

func (t *Table) reject(c *Conn, code, reason string) {
	t.send(c, protocol.Rejected(code, reason))
}

func (t *Table) send(c *Conn, m protocol.Msg) {
	if c.closed {
		return
	}
	b, err := json.Marshal(m)
	if err != nil {
		return // 理论上不可达
	}
	select {
	case c.out <- b:
	default:
		t.hangup(c) // 慢消费者：挂断该连接，不阻塞整桌
	}
}

func (t *Table) hangup(c *Conn) {
	if c.closed {
		return
	}
	c.closed = true
	if s := int(c.seat.Load()); s >= 0 && t.seat[s] == c {
		t.seat[s] = nil
	}
	delete(t.conns, c)
	close(c.out)
}

func (t *Table) hangupAll() {
	for c := range t.conns {
		t.hangup(c)
	}
}

// finish 终局收束：挂断全部连接（写泵排空后退出），关闭 Done。
func (t *Table) finish() {
	t.over = true
	t.hangupAll()
	t.doneOnce.Do(func() { close(t.done) })
}
