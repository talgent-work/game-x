package engine

import (
	"fmt"
	"math/rand"
)

// Phase 阶段：轮到打牌 / 等待声明 / 终局。
type Phase uint8

const (
	PhaseDiscard Phase = iota // 待 g.turn 打牌（也可暗杠/补杠/自摸和）
	PhaseClaim                // 打牌或补杠后，等待其余三家声明（吃/碰/杠/和/过；抢杠窗口只可和/过）
	PhaseOver                 // 终局（和牌或流局）
)

// DrawSource 摸牌来源（计番上下文，GX-3 取用）。
type DrawSource uint8

const (
	DrawNone    DrawSource = iota
	DrawInitial            // 庄家开局第一张（发牌所得；按自摸类处理的上下文事实）
	DrawWall               // 普通摸牌（牌头）
	DrawKong               // 杠后补牌（牌尾，杠上开花判定用）
)

// WinSource 和牌方式。
type WinSource uint8

const (
	WinSelfDrawn    WinSource = iota // 自摸
	WinByDiscard                     // 和他人打出的牌
	WinByRobbedKong                  // 抢杠和（他人补杠的那张）
)

// --- 动作（上层提交的意图） ---

// ActionKind 动作种类。
type ActionKind uint8

const (
	ActPass          ActionKind = iota // 声明窗口：过
	ActDiscard                         // 打牌
	ActChi                             // 吃（上家打出的牌）
	ActPong                            // 碰
	ActKongDiscard                     // 明杠（别人打出的牌）
	ActKongConcealed                   // 暗杠（手中四张）
	ActKongPromote                     // 补杠（向已碰刻子加第四张，开抢杠窗口）
	ActWin                             // 和牌（自己回合＝自摸；声明窗口＝点和/抢杠）
)

// Action 一次动作意图。
type Action struct {
	Kind ActionKind `json:"kind"`
	Tile Tile       `json:"tile"` // ActDiscard / ActKongConcealed / ActKongPromote 的牌
	With []Tile     `json:"with"` // ActChi 的两张手牌（与被吃之牌合成顺子）
}

// --- 事件（引擎回执） ---

// EventKind 事件种类。
type EventKind uint8

const (
	EvDiscarded    EventKind = iota // 打出一张牌（随后开声明窗口，或直接推进）
	EvDrawn                         // 摸牌（普通摸牌或杠后补牌）
	EvMeld                          // 一副面子落定（吃/碰/杠）
	EvKongPromoted                  // 声明补杠，进入抢杠窗口（补杠尚未落定）
	EvWin                           // 和牌，对局结束
	EvExhaustive                    // 流局，对局结束
)

// Event 引擎回执事件。Tile 字段对上层不设限——上层（table 层）负责按座位分发可见性；
// EvDrawn 的牌只应发给摸牌者本人。
type Event struct {
	Kind   EventKind  `json:"kind"`
	Seat   int        `json:"seat"`
	Tile   *Tile      `json:"tile,omitempty"`
	Source DrawSource `json:"source,omitempty"` // 仅 EvDrawn
	Meld   *Meld      `json:"meld,omitempty"`   // 仅 EvMeld
	Result *Result    `json:"result,omitempty"` // 仅 EvWin / EvExhaustive
}

// --- 公开视图与终局结果 ---

// PendingClaim 声明窗口信息（公开）：谁供的哪张牌在等各家反应。
type PendingClaim struct {
	Seat int  `json:"seat"` // 供牌者（打牌者 / 补杠者）
	Tile Tile `json:"tile"` // 待声明之牌
	Rob  bool `json:"rob"`  // true＝补杠抢杠窗口（只可和牌或过）
}

// DiscardEntry 牌河中的一张。
type DiscardEntry struct {
	Tile    Tile `json:"tile"`
	Claimed bool `json:"claimed"` // 已被吃/碰/杠/和走
}

// Snapshot 「某座位可见」的视图，也正是将来经协议发给该客户端的载荷形状。
// 隐藏信息收口：无他人手牌；他人暗杠的牌面被抹去（只留种类与出处）。
type Snapshot struct {
	Seat       int                   `json:"seat"`
	Phase      Phase                 `json:"phase"`
	Turn       int                   `json:"turn"`   // PhaseDiscard：待行动座位；PhaseClaim：供牌座位
	Dealer     int                   `json:"dealer"` // 庄家（座位风：庄＝东，下家＝南…）
	Hand       []Tile                `json:"hand"`   // 视图持有者的暗牌（升序）
	Melds      [Seats][]Meld         `json:"melds"`  // 全桌副子（他人暗杠牌面被抹去）
	Discards   [Seats][]DiscardEntry `json:"discards"`
	WallCount  int                   `json:"wallCount"`
	Pending    *PendingClaim         `json:"pending,omitempty"` // 声明窗口
	Declared   [Seats]bool           `json:"declared"`          // 窗口中已声明的座位（公开）
	CanSelfWin bool                  `json:"canSelfWin"`        // 本座位当前可否报自摸（仅自己视图有意义）
	Result     *Result               `json:"result,omitempty"`  // 终局结果（全揭示）
}

// WinContext 和牌上下文（为 GX-3 计番记录；引擎只记事实，不定番种名）。
// 注意：国标（98/WMO）81 番种中没有天和、地和——Initial/FirstGoAround 只是
// 原始事实记录（庄家开局第一张自摸、头一轮无吃碰杠），并非番种判定输入。
type WinContext struct {
	Dealer        int  `json:"dealer"`
	KongBloom     bool `json:"kongBloom"`     // 杠上开花：杠后补牌自摸成和
	LastTile      bool `json:"lastTile"`      // 妙手回春：自摸牌墙最后一张
	LastDiscard   bool `json:"lastDiscard"`   // 海底捞月：和牌时牌墙已空
	Initial       bool `json:"initial"`       // 事实：庄家开局第一张即自摸成和（非天和番种）
	FirstGoAround bool `json:"firstGoAround"` // 事实：头一轮（无吃碰杠且各家至多打过一张）
	DiscardCount  int  `json:"discardCount"`
	MeldCount     int  `json:"meldCount"`
}

// Result 终局结果。对局终了一切公开（含各家暗牌与暗杠牌面——国标终局翻牌核对）。
type Result struct {
	Exhaustive     bool          `json:"exhaustive"` // 流局
	Winner         int           `json:"winner"`     // 流局时为 -1
	WinningTile    Tile          `json:"winningTile"`
	Source         WinSource     `json:"source"`
	Feeder         int           `json:"feeder"`                   // 放铳/被抢杠者（自摸为 -1）
	Decompositions []WinHand     `json:"decompositions,omitempty"` // 胜者全部合法和牌分解（计分取最优）
	WinnerHand     []Tile        `json:"winnerHand,omitempty"`     // 胜者暗牌（含胡牌张）
	WinnerMelds    []Meld        `json:"winnerMelds,omitempty"`
	Hands          [Seats][]Tile `json:"hands"` // 终局各家暗牌
	Melds          [Seats][]Meld `json:"melds"`
	Context        WinContext    `json:"context"`
}

// Game 一局国标麻将的完整状态与状态机。
// 状态只在包内；对外经 Apply 推进、ViewFor 读取、Result 取终局。
type Game struct {
	wall   wall
	hands  [Seats][]Tile
	melds  [Seats][]Meld
	rivers [Seats][]DiscardEntry

	phase Phase
	turn  int // PhaseDiscard：待行动座位；PhaseClaim：供牌座位

	pending  PendingClaim   // PhaseClaim 的窗口
	decls    [Seats]*Action // 窗口声明：nil＝未声明；ActPass＝过
	declared [Seats]bool

	lastDraw     Tile
	drawSource   DrawSource
	canSelfWin   bool // 当前行动者刚摸到牌、可报自摸
	kongBlocked  bool // 当前行动者本巡吃/碰过而未摸牌：暂不得开杠（WMO 3.6.8 同巡互斥）
	drewLastTile bool // 最近一次摸牌是牌墙最后一张（妙手回春判定用）

	over   *Result
	dealer int
	log    []Event
}

// NewGame 以给定随机种子开局：洗牌、发四家各 13 张、庄家先摸成 14。
// 同一种子对局完全确定（复现/回放/测试用）。
func NewGame(seed int64, dealer int) *Game {
	if dealer < 0 || dealer >= Seats {
		dealer = 0
	}
	r := rand.New(rand.NewSource(seed))
	g := &Game{dealer: dealer, turn: dealer, phase: PhaseDiscard}
	g.wall = newWall(r)
	for s := 0; s < Seats; s++ {
		g.hands[s] = g.wall.drawN(HandSize)
		sortTiles(g.hands[s])
	}
	if t, ok := g.wall.draw(); ok {
		g.hands[dealer] = insertSorted(g.hands[dealer], t)
		g.lastDraw = t
		g.drawSource = DrawInitial
		g.canSelfWin = true
		g.drewLastTile = g.wall.remaining() == 0
	}
	return g
}

// --- 只读访问 ---

// Phase 当前阶段。
func (g *Game) Phase() Phase { return g.phase }

// Turn 当前待行动（PhaseDiscard）或供牌（PhaseClaim）座位。
func (g *Game) Turn() int { return g.turn }

// Dealer 庄家座位。
func (g *Game) Dealer() int { return g.dealer }

// Over 对局是否结束。
func (g *Game) Over() bool { return g.phase == PhaseOver }

// Result 终局结果；未终局为 nil。
func (g *Game) Result() *Result { return g.over }

// Log 返回至今的完整事件序列（副本）。
func (g *Game) Log() []Event {
	out := make([]Event, len(g.log))
	copy(out, g.log)
	return out
}

// ViewFor 生成 seat 可见的快照（唯一对外读取通道）。seat 越界返回零值快照。
func (g *Game) ViewFor(seat int) Snapshot {
	if seat < 0 || seat >= Seats {
		return Snapshot{}
	}
	snap := Snapshot{
		Seat:       seat,
		Phase:      g.phase,
		Turn:       g.turn,
		Dealer:     g.dealer,
		Hand:       append([]Tile(nil), g.hands[seat]...),
		WallCount:  g.wall.remaining(),
		Declared:   g.declared,
		CanSelfWin: g.phase == PhaseDiscard && g.turn == seat && g.canSelfWin,
		Result:     g.over,
	}
	for s := 0; s < Seats; s++ {
		snap.Melds[s] = g.copyMeldsFor(s, seat)
		snap.Discards[s] = append([]DiscardEntry(nil), g.rivers[s]...)
	}
	if g.phase == PhaseClaim {
		p := g.pending
		snap.Pending = &p
	}
	return snap
}

// copyMeldsFor 深拷贝 s 的副子；viewSeat 之外看到的暗杠牌面被抹去。
func (g *Game) copyMeldsFor(s, viewSeat int) []Meld {
	out := make([]Meld, len(g.melds[s]))
	for i, m := range g.melds[s] {
		out[i] = cloneMeld(m)
		if s != viewSeat && out[i].HideTile() {
			out[i].Tiles = nil
		}
	}
	return out
}

// Actions 列出 seat 当前全部合法动作（驱动 UI 与模拟；空＝无可动作）。
func (g *Game) Actions(seat int) []Action {
	if seat < 0 || seat >= Seats {
		return nil
	}
	var acts []Action
	switch g.phase {
	case PhaseDiscard:
		if seat != g.turn {
			return nil
		}
		counts := countsOf(g.hands[seat])
		seen := map[Tile]bool{}
		for _, t := range g.hands[seat] {
			if !seen[t] {
				seen[t] = true
				acts = append(acts, Action{Kind: ActDiscard, Tile: t})
			}
		}
		for o, n := range counts {
			if n == 4 && g.wall.remaining() >= 1 && !g.kongBlocked {
				acts = append(acts, Action{Kind: ActKongConcealed, Tile: tileFromOrd(o)})
			}
		}
		for _, m := range g.melds[seat] {
			if m.Kind == MeldPong && counts[m.Tiles[0].ord()] > 0 && g.wall.remaining() >= 1 && !g.kongBlocked {
				acts = append(acts, Action{Kind: ActKongPromote, Tile: m.Tiles[0]})
			}
		}
		if g.canSelfWin && winExists(g.hands[seat], len(g.melds[seat])) {
			acts = append(acts, Action{Kind: ActWin})
		}
	case PhaseClaim:
		if seat == g.pending.Seat || g.declared[seat] {
			return nil
		}
		acts = append(acts, Action{Kind: ActPass})
		if g.canWinOn(seat, g.pending.Tile) {
			acts = append(acts, Action{Kind: ActWin})
		}
		if !g.pending.Rob {
			counts := countsOf(g.hands[seat])
			o := g.pending.Tile.ord()
			if counts[o] >= 2 {
				acts = append(acts, Action{Kind: ActPong})
			}
			if counts[o] >= 3 && g.wall.remaining() >= 1 {
				acts = append(acts, Action{Kind: ActKongDiscard})
			}
			if seat == nextSeat(g.pending.Seat) {
				acts = append(acts, g.chiOptions(seat, g.pending.Tile)...)
			}
		}
	}
	return acts
}

func nextSeat(s int) int { return (s + 1) % Seats }

// chiOptions 枚举吃 pending 的全部合法 With 组合。
func (g *Game) chiOptions(seat int, pending Tile) []Action {
	var acts []Action
	if !pending.Numeric() {
		return nil
	}
	counts := countsOf(g.hands[seat])
	try := func(a, b uint8) {
		ta, tb := Tile{Suit: pending.Suit, Rank: a}, Tile{Suit: pending.Suit, Rank: b}
		if counts[ta.ord()] > 0 && counts[tb.ord()] > 0 {
			acts = append(acts, Action{Kind: ActChi, With: []Tile{ta, tb}})
		}
	}
	r := pending.Rank
	if r >= 3 {
		try(r-2, r-1)
	}
	if r >= 2 && r <= 8 {
		try(r-1, r+1)
	}
	if r <= 7 {
		try(r+1, r+2)
	}
	return acts
}

// --- 动作提交 ---

// Apply 提交 seat 的动作，校验并推进状态，返回产生的事件。
// 非法动作（含座位越界、非法牌值、未知动作种类）返回错误且不改变状态，
// 任何输入不得引发 panic——权威服务端直连不可信输入。
func (g *Game) Apply(seat int, act Action) ([]Event, error) {
	if seat < 0 || seat >= Seats {
		return nil, fmt.Errorf("座位 %d 越界", seat)
	}
	if g.phase == PhaseOver {
		return nil, fmt.Errorf("对局已结束")
	}
	switch act.Kind {
	case ActDiscard:
		return g.applyDiscard(seat, act.Tile)
	case ActKongConcealed:
		return g.applyKongConcealed(seat, act.Tile)
	case ActKongPromote:
		return g.applyKongPromote(seat, act.Tile)
	case ActWin:
		if g.phase == PhaseDiscard {
			return g.applySelfWin(seat)
		}
		return g.applyDeclare(seat, act)
	case ActPass, ActChi, ActPong, ActKongDiscard:
		return g.applyDeclare(seat, act)
	}
	return nil, fmt.Errorf("未知动作 %v", act.Kind)
}

func (g *Game) applyDiscard(seat int, tile Tile) ([]Event, error) {
	if g.phase != PhaseDiscard {
		return nil, fmt.Errorf("当前不在打牌阶段")
	}
	if seat != g.turn {
		return nil, fmt.Errorf("还没轮到座位 %d（当前 %d）", seat, g.turn)
	}
	h := g.hands[seat]
	found := false
	for _, x := range h {
		if x == tile {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("手牌中没有 %v", tile)
	}

	g.hands[seat] = removeTile(h, tile)
	g.rivers[seat] = append(g.rivers[seat], DiscardEntry{Tile: tile})
	g.canSelfWin = false
	tileCopy := tile
	ev := g.emit(Event{Kind: EvDiscarded, Seat: seat, Tile: &tileCopy})

	// 开声明窗口；若其余三家对这张牌既无和也无碰杠吃的可能，直接结算推进。
	g.openWindow(PendingClaim{Seat: seat, Tile: tile})
	return append([]Event{ev}, g.maybeResolve()...), nil
}

func (g *Game) applyKongConcealed(seat int, tile Tile) ([]Event, error) {
	if g.phase != PhaseDiscard {
		return nil, fmt.Errorf("暗杠只能在打牌阶段声明")
	}
	if seat != g.turn {
		return nil, fmt.Errorf("还没轮到座位 %d（当前 %d）", seat, g.turn)
	}
	if g.kongBlocked {
		return nil, fmt.Errorf("本巡已吃/碰，须再拿牌后才能开杠（同巡不报杠）")
	}
	if g.wall.remaining() < 1 {
		return nil, fmt.Errorf("牌墙已空，无杠后补牌，不能开杠")
	}
	if !tile.Valid() {
		return nil, fmt.Errorf("非法牌值 %v", tile)
	}
	counts := countsOf(g.hands[seat])
	if counts[tile.ord()] != 4 {
		return nil, fmt.Errorf("暗杠需手中四张 %v", tile)
	}
	for i := 0; i < 4; i++ {
		g.hands[seat] = removeTile(g.hands[seat], tile)
	}
	meld := Meld{Kind: MeldKong, Tiles: []Tile{tile, tile, tile, tile}, From: -1, Kong: KongFromConcealed}
	g.melds[seat] = append(g.melds[seat], meld)
	meldCopy := cloneMeld(meld)
	ev := g.emit(Event{Kind: EvMeld, Seat: seat, Meld: &meldCopy})
	return append([]Event{ev}, g.drawTailFor(seat)...), nil
}

func (g *Game) applyKongPromote(seat int, tile Tile) ([]Event, error) {
	if g.phase != PhaseDiscard {
		return nil, fmt.Errorf("补杠只能在打牌阶段声明")
	}
	if seat != g.turn {
		return nil, fmt.Errorf("还没轮到座位 %d（当前 %d）", seat, g.turn)
	}
	if g.kongBlocked {
		return nil, fmt.Errorf("本巡已吃/碰，须再拿牌后才能开杠（同巡不报杠）")
	}
	if g.wall.remaining() < 1 {
		return nil, fmt.Errorf("牌墙已空，无杠后补牌，不能补杠")
	}
	if !tile.Valid() {
		return nil, fmt.Errorf("非法牌值 %v", tile)
	}
	// 找到该值的碰刻子。
	idx := -1
	for i, m := range g.melds[seat] {
		if m.Kind == MeldPong && m.Tiles[0] == tile {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("没有可补杠的 %v 刻子", tile)
	}
	if !containsTile(g.hands[seat], tile) {
		return nil, fmt.Errorf("手中没有 %v 可补杠", tile)
	}

	// 牌进托管：先离手，抢杠窗口中公开展示；无人抢则入杠，有人抢则归和牌者。
	g.hands[seat] = removeTile(g.hands[seat], tile)
	g.canSelfWin = false
	tileCopy := tile
	ev := g.emit(Event{Kind: EvKongPromoted, Seat: seat, Tile: &tileCopy})

	g.openWindow(PendingClaim{Seat: seat, Tile: tile, Rob: true})
	return append([]Event{ev}, g.maybeResolve()...), nil
}

func (g *Game) applySelfWin(seat int) ([]Event, error) {
	if seat != g.turn {
		return nil, fmt.Errorf("还没轮到座位 %d（当前 %d）", seat, g.turn)
	}
	if !g.canSelfWin {
		return nil, fmt.Errorf("当前未摸牌（吃碰后须打牌），不能报自摸")
	}
	if !winExists(g.hands[seat], len(g.melds[seat])) {
		return nil, fmt.Errorf("未成和牌牌型，不能报和")
	}
	g.finishWin(seat, g.lastDraw, WinSelfDrawn, -1)
	ev := g.emit(Event{Kind: EvWin, Seat: seat, Result: g.over})
	return []Event{ev}, nil
}

// applyDeclare 声明窗口中的声明：过 / 和 / 吃 / 碰 / 明杠。
func (g *Game) applyDeclare(seat int, act Action) ([]Event, error) {
	if g.phase != PhaseClaim {
		return nil, fmt.Errorf("当前不在声明窗口")
	}
	if seat == g.pending.Seat {
		return nil, fmt.Errorf("供牌者不能声明")
	}
	if g.declared[seat] {
		return nil, fmt.Errorf("座位 %d 本窗口已声明", seat)
	}
	tile := g.pending.Tile
	switch act.Kind {
	case ActPass:
		// 无需校验。
	case ActWin:
		if !g.canWinOn(seat, tile) {
			return nil, fmt.Errorf("%v 不能成和牌牌型", tile)
		}
	case ActPong:
		if g.pending.Rob {
			return nil, fmt.Errorf("抢杠窗口只可和牌或过")
		}
		if countsOf(g.hands[seat])[tile.ord()] < 2 {
			return nil, fmt.Errorf("手中没有 %v 对子，不能碰", tile)
		}
	case ActKongDiscard:
		if g.pending.Rob {
			return nil, fmt.Errorf("抢杠窗口只可和牌或过")
		}
		if g.wall.remaining() < 1 {
			return nil, fmt.Errorf("牌墙已空，无杠后补牌，不能明杠")
		}
		if countsOf(g.hands[seat])[tile.ord()] < 3 {
			return nil, fmt.Errorf("手中没有 %v 三张，不能明杠", tile)
		}
	case ActChi:
		if g.pending.Rob {
			return nil, fmt.Errorf("抢杠窗口只可和牌或过")
		}
		if seat != nextSeat(g.pending.Seat) {
			return nil, fmt.Errorf("只有上家打出的牌才能吃（座位 %d 不适用）", seat)
		}
		if !g.validChi(seat, tile, act.With) {
			return nil, fmt.Errorf("%v 与 %v 不成顺子", tile, act.With)
		}
	default:
		return nil, fmt.Errorf("该动作不属于声明窗口")
	}

	g.decls[seat] = &act
	g.declared[seat] = true
	return g.maybeResolve(), nil
}

// validChi 校验：与被吃之牌同花色数牌、三张同花色连续，且两张手牌在手。
func (g *Game) validChi(seat int, pending Tile, with []Tile) bool {
	if len(with) != 2 || !pending.Numeric() {
		return false
	}
	run := []Tile{with[0], with[1], pending}
	sortTiles(run)
	// 三张必须同花色（防跨花色/字牌拼「顺」）且数牌连续。
	for _, t := range run {
		if !t.Numeric() || t.Suit != pending.Suit {
			return false
		}
	}
	if run[0].Rank+1 != run[1].Rank || run[1].Rank+1 != run[2].Rank {
		return false
	}
	for _, t := range with {
		if t == pending || !containsTile(g.hands[seat], t) {
			return false
		}
	}
	return true
}

// --- 声明窗口 ---

// openWindow 重置并打开声明窗口。
func (g *Game) openWindow(p PendingClaim) {
	g.phase = PhaseClaim
	g.turn = p.Seat
	g.pending = p
	g.decls = [Seats]*Action{}
	g.declared = [Seats]bool{}
}

// maybeResolve 窗口若已收齐（或本就无人可声明），结算之；否则不动作。
// 返回结算产生的事件。
func (g *Game) maybeResolve() []Event {
	if g.phase != PhaseClaim {
		return nil
	}
	if !g.windowComplete() {
		return nil
	}
	return g.resolveWindow()
}

// windowComplete 窗口是否已收齐：全部候选座位已声明，或再无任何可行的声明
// （此时视作全体过——不改变结果：无声明可行者只能过）。
func (g *Game) windowComplete() bool {
	for i := 1; i <= Seats-1; i++ {
		s := (g.pending.Seat + i) % Seats
		if g.declared[s] {
			continue
		}
		if g.seatCanClaim(s) {
			return false
		}
	}
	return true
}

// seatCanClaim s 在当前窗口是否存在任何可行声明（和/碰/杠/吃）。
func (g *Game) seatCanClaim(s int) bool {
	tile := g.pending.Tile
	if g.canWinOn(s, tile) {
		return true
	}
	if g.pending.Rob {
		return false // 抢杠窗口其余声明一概不可
	}
	if countsOf(g.hands[s])[tile.ord()] >= 2 {
		return true // 碰（两张即够；杠为可选，碰可行即算「可声明」）
	}
	if s == nextSeat(g.pending.Seat) && len(g.chiOptions(s, tile)) > 0 {
		return true
	}
	return false
}

// resolveWindow 按优先级结算声明：和（截和） > 碰/杠 > 吃；均无则推进到下家摸牌（或流局）。
func (g *Game) resolveWindow() []Event {
	var events []Event

	// 1) 和牌声明：截和（头跳）——从供牌者起按下家、对家、上家取最先报和者。
	for i := 1; i <= Seats-1; i++ {
		s := (g.pending.Seat + i) % Seats
		if d := g.decls[s]; d != nil && d.Kind == ActWin {
			source := WinByDiscard
			if g.pending.Rob {
				source = WinByRobbedKong
			}
			g.finishWin(s, g.pending.Tile, source, g.pending.Seat)
			ev := g.emit(Event{Kind: EvWin, Seat: s, Result: g.over})
			return append(events, ev)
		}
	}

	if g.pending.Rob {
		// 2) 无人抢杠：补杠落定，杠后补牌。
		events = append(events, g.settlePromote()...)
		return events
	}

	// 2) 碰 / 明杠（同一张牌至多一家可能成功）。
	for i := 1; i <= Seats-1; i++ {
		s := (g.pending.Seat + i) % Seats
		d := g.decls[s]
		if d == nil {
			continue
		}
		switch d.Kind {
		case ActPong:
			events = append(events, g.settlePong(s)...)
			return events
		case ActKongDiscard:
			events = append(events, g.settleKongDiscard(s)...)
			return events
		}
	}

	// 3) 吃（只有供牌者下家可行）。
	if s := nextSeat(g.pending.Seat); g.decls[s] != nil && g.decls[s].Kind == ActChi {
		events = append(events, g.settleChi(s, g.decls[s].With)...)
		return events
	}

	// 4) 无人声明：下家摸牌；牌墙已空则流局。
	events = append(events, g.advanceTurn()...)
	return events
}

// settlePromote 抢杠窗口无人和：刻子升为杠，从牌尾补牌，轮补杠者打牌。
func (g *Game) settlePromote() []Event {
	seat, tile := g.pending.Seat, g.pending.Tile
	var events []Event
	for i := range g.melds[seat] {
		m := &g.melds[seat][i]
		if m.Kind == MeldPong && m.Tiles[0] == tile {
			m.Kind = MeldKong
			m.Kong = KongFromPromote
			m.Tiles = append(m.Tiles, tile)
			meldCopy := cloneMeld(*m)
			events = append(events, g.emit(Event{Kind: EvMeld, Seat: seat, Meld: &meldCopy}))
			break
		}
	}
	g.closeWindow(seat)
	events = append(events, g.drawTailFor(seat)...)
	return events
}

// settlePong 碰：碰牌离手入刻子，轮碰者打牌（无摸牌）。
// 吃/碰后未再摸牌前不得开杠（WMO 3.6.8 同巡互斥），置 kongBlocked。
func (g *Game) settlePong(seat int) []Event {
	tile := g.pending.Tile
	g.hands[seat] = removeTile(removeTile(g.hands[seat], tile), tile)
	g.markDiscardClaimed(g.pending.Seat, tile)
	meld := Meld{Kind: MeldPong, Tiles: []Tile{tile, tile, tile}, From: g.pending.Seat}
	g.melds[seat] = append(g.melds[seat], meld)
	ev := g.emit(Event{Kind: EvMeld, Seat: seat, Meld: meldPtr(cloneMeld(meld))})
	g.closeWindow(seat)
	g.kongBlocked = true
	return []Event{ev}
}

// settleKongDiscard 明杠：三张离手＋所碰之牌成杠，牌尾补牌，轮其打牌。
func (g *Game) settleKongDiscard(seat int) []Event {
	tile := g.pending.Tile
	for i := 0; i < 3; i++ {
		g.hands[seat] = removeTile(g.hands[seat], tile)
	}
	g.markDiscardClaimed(g.pending.Seat, tile)
	meld := Meld{Kind: MeldKong, Tiles: []Tile{tile, tile, tile, tile}, From: g.pending.Seat, Kong: KongFromDiscard}
	g.melds[seat] = append(g.melds[seat], meld)
	ev := g.emit(Event{Kind: EvMeld, Seat: seat, Meld: meldPtr(cloneMeld(meld))})
	g.closeWindow(seat)
	return append([]Event{ev}, g.drawTailFor(seat)...)
}

// settleChi 吃：两张手牌＋所吃之牌成顺子，轮吃者打牌（无摸牌）。
// 吃/碰后未再摸牌前不得开杠（WMO 3.6.8 同巡互斥），置 kongBlocked。
func (g *Game) settleChi(seat int, with []Tile) []Event {
	tile := g.pending.Tile
	g.hands[seat] = removeTile(removeTile(g.hands[seat], with[0]), with[1])
	g.markDiscardClaimed(g.pending.Seat, tile)
	run := []Tile{with[0], with[1], tile}
	sortTiles(run)
	meld := Meld{Kind: MeldChi, Tiles: run, From: g.pending.Seat}
	g.melds[seat] = append(g.melds[seat], meld)
	ev := g.emit(Event{Kind: EvMeld, Seat: seat, Meld: meldPtr(cloneMeld(meld))})
	g.closeWindow(seat)
	g.kongBlocked = true
	return []Event{ev}
}

// closeWindow 窗口收束，进入 seat 的打牌阶段。
func (g *Game) closeWindow(seat int) {
	g.phase = PhaseDiscard
	g.turn = seat
	g.pending = PendingClaim{}
	g.decls = [Seats]*Action{}
	g.declared = [Seats]bool{}
}

// advanceTurn 无人声明：下家从牌头摸牌；牌墙已空则流局。
func (g *Game) advanceTurn() []Event {
	next := nextSeat(g.pending.Seat)
	g.closeWindow(next)
	if g.wall.remaining() == 0 {
		g.finishExhaustive()
		return []Event{g.emit(Event{Kind: EvExhaustive, Result: g.over})}
	}
	t, _ := g.wall.draw()
	g.hands[next] = insertSorted(g.hands[next], t)
	g.lastDraw = t
	g.drawSource = DrawWall
	g.canSelfWin = true
	g.kongBlocked = false // 摸牌后同巡开杠限制解除
	g.drewLastTile = g.wall.remaining() == 0
	tileCopy := t
	return []Event{g.emit(Event{Kind: EvDrawn, Seat: next, Tile: &tileCopy, Source: DrawWall})}
}

// drawTailFor seat 杠后从牌尾补牌（调用方保证牌墙非空）。
func (g *Game) drawTailFor(seat int) []Event {
	t, _ := g.wall.drawTail()
	g.hands[seat] = insertSorted(g.hands[seat], t)
	g.lastDraw = t
	g.drawSource = DrawKong
	g.canSelfWin = true
	g.kongBlocked = false // 补牌即「再拿牌」，解除同巡开杠限制
	g.drewLastTile = g.wall.remaining() == 0
	tileCopy := t
	return []Event{g.emit(Event{Kind: EvDrawn, Seat: seat, Tile: &tileCopy, Source: DrawKong})}
}

func (g *Game) markDiscardClaimed(discarder int, tile Tile) {
	r := g.rivers[discarder]
	for i := len(r) - 1; i >= 0; i-- {
		if r[i].Tile == tile && !r[i].Claimed {
			r[i].Claimed = true
			return
		}
	}
}

func containsTile(tiles []Tile, t Tile) bool {
	for _, x := range tiles {
		if x == t {
			return true
		}
	}
	return false
}

// canWinOn s 以 tile 为胡牌张能否成和牌牌型（声明校验用）。
func (g *Game) canWinOn(s int, tile Tile) bool {
	if len(g.hands[s]) != 13-3*len(g.melds[s]) {
		return false // 声明者手数异常（理论不可达，防御）
	}
	return winExists(append(append([]Tile(nil), g.hands[s]...), tile), len(g.melds[s]))
}

// --- 终局 ---

// finishWin 记录和牌终局。
func (g *Game) finishWin(winner int, tile Tile, source WinSource, feeder int) {
	winnerHand := append(append([]Tile(nil), g.hands[winner]...), tile)
	if source == WinSelfDrawn {
		winnerHand = append([]Tile(nil), g.hands[winner]...) // 自摸：胡牌张已在手
	}
	meldCount, discardCount := 0, 0
	for s := 0; s < Seats; s++ {
		meldCount += len(g.melds[s])
		discardCount += len(g.rivers[s])
	}
	ctx := WinContext{
		Dealer:       g.dealer,
		LastDiscard:  source == WinByDiscard && g.wall.remaining() == 0,
		Initial:      source == WinSelfDrawn && g.drawSource == DrawInitial, // 事实记录，非天和番种
		KongBloom:    source == WinSelfDrawn && g.drawSource == DrawKong,
		LastTile:     source == WinSelfDrawn && g.drewLastTile,
		DiscardCount: discardCount,
		MeldCount:    meldCount,
	}
	ctx.FirstGoAround = meldCount == 0 && g.maxRiverLen() <= 1

	g.over = &Result{
		Winner:         winner,
		WinningTile:    tile,
		Source:         source,
		Feeder:         feeder,
		Decompositions: Decompose(winnerHand, len(g.melds[winner])),
		WinnerHand:     winnerHand,
		WinnerMelds:    cloneMelds(g.melds[winner]),
	}
	for s := 0; s < Seats; s++ {
		g.over.Hands[s] = append([]Tile(nil), g.hands[s]...)
		g.over.Melds[s] = cloneMelds(g.melds[s])
	}
	g.over.Context = ctx
	g.phase = PhaseOver
	if source == WinByDiscard {
		g.markDiscardClaimed(feeder, tile)
	}
	// 抢杠和：托管的第四张归和牌者（Result.WinnerHand 已含），被抢者的刻子保持碰不变。
}

// finishExhaustive 记录流局终局。
func (g *Game) finishExhaustive() {
	meldCount, discardCount := 0, 0
	for s := 0; s < Seats; s++ {
		meldCount += len(g.melds[s])
		discardCount += len(g.rivers[s])
	}
	g.over = &Result{
		Exhaustive: true,
		Winner:     -1,
		Feeder:     -1,
		Context: WinContext{
			Dealer:       g.dealer,
			DiscardCount: discardCount,
			MeldCount:    meldCount,
		},
	}
	for s := 0; s < Seats; s++ {
		g.over.Hands[s] = append([]Tile(nil), g.hands[s]...)
		g.over.Melds[s] = cloneMelds(g.melds[s])
	}
	g.phase = PhaseOver
}

func (g *Game) maxRiverLen() int {
	m := 0
	for s := 0; s < Seats; s++ {
		if len(g.rivers[s]) > m {
			m = len(g.rivers[s])
		}
	}
	return m
}

// emit 记入事件日志（内部封装，保证 Log 与 Apply 返回一致）。
func (g *Game) emit(ev Event) Event {
	g.log = append(g.log, ev)
	return ev
}
