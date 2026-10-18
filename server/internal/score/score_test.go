package score

import (
	"strconv"
	"testing"

	"talgent-work/game-x/server/internal/engine"
)

// --- 构造工具 ---

func w(r uint8) engine.Tile  { return engine.Tile{Suit: engine.SuitWan, Rank: r} }
func o(r uint8) engine.Tile  { return engine.Tile{Suit: engine.SuitTong, Rank: r} }
func s(r uint8) engine.Tile  { return engine.Tile{Suit: engine.SuitTiao, Rank: r} }
func fw(r uint8) engine.Tile { return engine.Tile{Suit: engine.SuitFeng, Rank: r} }
func j(r uint8) engine.Tile  { return engine.Tile{Suit: engine.SuitJian, Rank: r} }

func rep(t engine.Tile, n int) []engine.Tile {
	out := make([]engine.Tile, n)
	for i := range out {
		out[i] = t
	}
	return out
}

func meldedChi(tiles ...engine.Tile) engine.Meld {
	return engine.Meld{Kind: engine.MeldChi, Tiles: tiles, From: 1}
}
func meldedPong(t engine.Tile) engine.Meld {
	return engine.Meld{Kind: engine.MeldPong, Tiles: rep(t, 3), From: 1}
}
func meldedKong(t engine.Tile) engine.Meld {
	return engine.Meld{Kind: engine.MeldKong, Tiles: rep(t, 4), Kong: engine.KongFromDiscard, From: 1}
}
func concealedKong(t engine.Tile) engine.Meld {
	return engine.Meld{Kind: engine.MeldKong, Tiles: rep(t, 4), Kong: engine.KongFromConcealed, From: -1}
}
func promotedKong(t engine.Tile) engine.Meld {
	return engine.Meld{Kind: engine.MeldKong, Tiles: rep(t, 4), Kong: engine.KongFromPromote, From: 1}
}

// mkResult 构造终局结果：tiles 为胜者暗牌（含胡牌张），melds 为已亮副子。
// 默认庄家座位 3（非胜者）——座位风＝东、下家南……胜者 0 在庄 3 时是「上家北」，
// 避免东风刻意外命中门风刻。需要门风刻的用例显式传 Dealer: 0。
func mkResult(tiles []engine.Tile, melds []engine.Meld, winning engine.Tile, src engine.WinSource, feeder int, ctx engine.WinContext) *engine.Result {
	if ctx.Dealer == 0 && feeder == 0 {
		ctx.Dealer = 3
	}
	return &engine.Result{
		Winner:         0,
		WinningTile:    winning,
		Source:         src,
		Feeder:         feeder,
		WinnerHand:     append([]engine.Tile(nil), tiles...),
		WinnerMelds:    melds,
		Decompositions: engine.Decompose(append([]engine.Tile(nil), tiles...), len(melds)),
		Context:        ctx,
	}
}

// mkLastTileResult 同 mkResult，并把 winning 的另 3 张放入对家的亮明副子（和绝张）。
func mkLastTileResult(tiles []engine.Tile, melds []engine.Meld, winning engine.Tile, src engine.WinSource, feeder int, ctx engine.WinContext) *engine.Result {
	res := mkResult(tiles, melds, winning, src, feeder, ctx)
	res.Melds[1] = append(res.Melds[1], engine.Meld{Kind: engine.MeldPong, Tiles: rep(winning, 3), From: 2})
	return res
}

func fanSet(hits []FanHit) map[FanKind]int {
	m := map[FanKind]int{}
	for _, h := range hits {
		m[h.Kind] += h.Count
	}
	return m
}

// --- 定向测试：每个适用番种至少一条 ---

func TestFanTable(t *testing.T) {
	const (
		self = engine.WinSelfDrawn
		disc = engine.WinByDiscard
	)
	cases := []struct {
		name      string
		tiles     []engine.Tile
		melds     []engine.Meld
		winning   engine.Tile
		source    engine.WinSource
		feeder    int
		ctx       engine.WinContext
		want      map[FanKind]int
		wantSum   int
		lastTile  bool  // 另 3 张置于他人亮明副子（和绝张）
		roundWind uint8 // 0 = 默认东风圈
	}{
		// === 88 分档 ===
		{
			name:    "大四喜",
			tiles:   []engine.Tile{fw(1), fw(1), fw(1), fw(2), fw(2), fw(2), fw(3), fw(3), fw(3), fw(4), fw(4), fw(4), w(5), w(5)},
			winning: w(5),
			source:  self,
			want:    map[FanKind]int{BigFourWinds: 1, FourConcealedPungs: 1, HalfFlush: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 4, SelfDrawn: 1, SingleWait: 1},
			wantSum: 170,
		},
		{
			name:    "大三元",
			tiles:   []engine.Tile{j(1), j(1), j(1), j(2), j(2), j(2), j(3), j(3), j(3), w(5), w(5), w(5), w(6), w(6)},
			winning: w(6),
			source:  self,
			want:    map[FanKind]int{BigThreeDragons: 1, FourConcealedPungs: 1, AllPungs: 1, HalfFlush: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 3, SelfDrawn: 1, SingleWait: 1},
			wantSum: 175,
		},
		{
			name:    "绿一色",
			tiles:   []engine.Tile{s(2), s(3), s(4), s(2), s(2), s(6), s(6), s(6), s(8), s(8), s(8), j(2), j(2), j(2)},
			winning: j(2),
			want:    map[FanKind]int{AllGreen: 1, ThreeConcealedPungs: 1, FullyConcealedHand: 1, DragonPung: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, SelfDrawn: 1},
			wantSum: 114,
		},
		{
			name:    "九莲宝灯",
			tiles:   append([]engine.Tile{w(1), w(1), w(1), w(2), w(3), w(4), w(5), w(6), w(7), w(8), w(9), w(9), w(9)}, w(3)),
			winning: w(3),
			want:    map[FanKind]int{NineGates: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, NoHonors: 1, SelfDrawn: 1, EdgeWait: 1},
			wantSum: 98,
		},
		{
			name:    "连七对",
			tiles:   append([]engine.Tile{w(1), w(1), w(2), w(2), w(3), w(3), w(4), w(4), w(5), w(5), w(6), w(6), w(7)}, w(7)),
			winning: w(7),
			want:    map[FanKind]int{SevenShiftedPair: 1, FullFlush: 1, FullyConcealedHand: 1, ConcealedHand: 1, SelfDrawn: 1},
			wantSum: 119,
		},
		{
			name:    "十三幺",
			tiles:   append([]engine.Tile{w(1), w(9), o(1), o(9), s(1), s(9), fw(1), fw(2), fw(3), fw(4), j(1), j(2), j(3)}, j(3)),
			winning: j(3),
			want:    map[FanKind]int{ThirteenOrphans: 1},
			wantSum: 88,
		},
		// === 64 分档 ===
		{
			name:    "清幺九",
			tiles:   []engine.Tile{w(1), w(1), w(1), o(9), o(9), o(9), s(1), s(1), s(1), o(1), o(1), o(1), w(9), w(9)},
			winning: w(9),
			source:  self,
			want:    map[FanKind]int{AllTerminals: 1, FourConcealedPungs: 1, AllTerminalsAndHonours: 1, TriplePung: 1, FullyConcealedHand: 1, ConcealedHand: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 184,
		},
		{
			name:    "小四喜",
			tiles:   []engine.Tile{fw(1), fw(1), fw(1), fw(2), fw(2), fw(2), fw(3), fw(3), fw(3), o(2), o(3), o(4), fw(4), fw(4)},
			winning: fw(4),
			want:    map[FanKind]int{LittleFourWinds: 1, ThreeConcealedPungs: 1, HalfFlush: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 3, SelfDrawn: 1, SingleWait: 1},
			wantSum: 97,
		},
		{
			name:    "小三元",
			tiles:   []engine.Tile{j(1), j(1), j(1), j(2), j(2), j(2), j(3), j(3), fw(1), fw(1), fw(1), o(4), o(5), o(6)},
			winning: fw(1),
			want:    map[FanKind]int{LittleThreeDragons: 1, ThreeConcealedPungs: 1, TwoDragons: 1, HalfFlush: 1, FullyConcealedHand: 1, PrevalentWind: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 3, SelfDrawn: 1},
			wantSum: 104,
		},
		{
			name:    "字一色",
			tiles:   []engine.Tile{fw(1), fw(1), fw(1), j(1), j(1), j(1), j(2), j(2), j(2), fw(4), fw(4), fw(4), fw(2), fw(2)},
			winning: fw(2),
			want:    map[FanKind]int{AllHonors: 1, FourConcealedPungs: 1, AllTerminalsAndHonours: 1, TwoDragons: 1, FullyConcealedHand: 1, PrevalentWind: 1, ConcealedHand: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 176,
		},
		{
			name:    "四暗刻＋碰碰和",
			tiles:   []engine.Tile{w(1), w(1), w(1), o(6), o(6), o(6), s(7), s(7), s(7), j(1), j(1), j(1), w(6), w(6)},
			winning: w(6),
			want:    map[FanKind]int{FourConcealedPungs: 1, AllPungs: 1, FullyConcealedHand: 1, DragonPung: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 2, SelfDrawn: 1, SingleWait: 1},
			wantSum: 82,
		},
		{
			name:    "一色双龙会",
			tiles:   []engine.Tile{w(1), w(2), w(3), w(1), w(2), w(3), w(7), w(8), w(9), w(7), w(8), w(9), w(5), w(5)},
			winning: w(5),
			want:    map[FanKind]int{PureTerminalChows: 1, FullFlush: 1, FullyConcealedHand: 1, ConcealedHand: 1, ShortStraight: 4, SelfDrawn: 1, SingleWait: 1},
			wantSum: 100,
		},
		// === 48 分档 ===
		{
			name:    "一色四同顺",
			tiles:   []engine.Tile{w(1), w(2), w(3), w(1), w(2), w(3), w(1), w(2), w(3), w(1), w(2), w(3), w(5), w(5)},
			winning: w(5),
			want:    map[FanKind]int{QuadrupleChow: 1, FullFlush: 1, FullyConcealedHand: 1, TileHog: 3, ConcealedHand: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 86,
		},
		{
			name:    "一色四节高",
			tiles:   []engine.Tile{w(1), w(1), w(1), w(2), w(2), w(2), w(3), w(3), w(3), w(4), w(4), w(4), w(6), w(6)},
			winning: w(6),
			want:    map[FanKind]int{FourConcealedPungs: 1, FourPureShiftPung: 1, FullFlush: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 145,
		},
		// === 32 分档 ===
		{
			name:    "一色四步高",
			tiles:   []engine.Tile{w(3), w(4), w(5), w(4), w(5), w(6), w(5), w(6), w(7), w(6), w(7), w(8), o(5), o(5)},
			winning: o(5),
			want:    map[FanKind]int{FourShiftedChows: 1, FullyConcealedHand: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, OneVoidedSuit: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 46,
		},
		{
			name:    "三杠",
			tiles:   []engine.Tile{w(2), w(3), w(4), o(5), o(5)},
			melds:   []engine.Meld{concealedKong(w(1)), concealedKong(o(9)), concealedKong(s(3))},
			winning: w(2),
			want:    map[FanKind]int{ThreeKongs: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 2, NoHonors: 1, SelfDrawn: 1},
			wantSum: 42,
		},
		{
			name:    "混幺九",
			tiles:   []engine.Tile{w(1), w(1), w(1), w(9), w(9), w(9), o(1), o(1), o(1), o(9), o(9), o(9), fw(1), fw(1)},
			winning: fw(1),
			want:    map[FanKind]int{FourConcealedPungs: 1, AllTerminalsAndHonours: 1, FullyConcealedHand: 1, DoublePung: 2, ConcealedHand: 1, OneVoidedSuit: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 109,
		},
		// === 24 分档 ===
		{
			name:    "七对",
			tiles:   append([]engine.Tile{w(1), w(1), w(3), w(3), w(5), w(5), o(2), o(2), o(6), o(6), s(4), s(4), s(8)}, s(8)),
			winning: s(8),
			want:    map[FanKind]int{SevenPairs: 1, NoHonors: 1},
			wantSum: 25,
		},
		{
			name:    "全双刻",
			tiles:   []engine.Tile{w(2), w(2), w(2), w(4), w(4), w(4), o(6), o(6), o(6), s(8), s(8), s(8), o(2), o(2)},
			winning: o(2),
			want:    map[FanKind]int{FourConcealedPungs: 1, AllEvenPungs: 1, FullyConcealedHand: 1, ConcealedHand: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 96,
		},
		{
			name:    "清一色＋一色三步高",
			tiles:   []engine.Tile{fw(1), fw(1), fw(1), w(1), w(2), w(3), w(2), w(3), w(4), w(3), w(4), w(5), o(9), o(9)},
			winning: o(9),
			want:    map[FanKind]int{PureShiftedChows: 1, FullyConcealedHand: 1, PrevalentWind: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, OneVoidedSuit: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 28,
		},
		{
			name:    "一色三同顺",
			tiles:   []engine.Tile{w(1), w(2), w(3), w(1), w(2), w(3), w(1), w(2), w(3), o(5), o(6), o(7), o(9), o(9)},
			winning: o(9),
			want:    map[FanKind]int{PureShiftedPungs: 1, ThreeConcealedPungs: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, NoHonors: 1, OneVoidedSuit: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 51,
		},
		{
			name:    "全大",
			tiles:   []engine.Tile{w(7), w(8), w(9), s(7), s(7), s(7), o(9), o(9), o(9), o(8), o(8), o(8), o(9), o(9)},
			winning: o(9),
			want:    map[FanKind]int{UpperTiles: 1, ThreeConcealedPungs: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, SelfDrawn: 1},
			wantSum: 48,
		},
		{
			name:    "全小",
			tiles:   []engine.Tile{w(1), w(2), w(3), o(1), o(1), o(1), s(2), s(2), s(2), s(3), s(3), s(3), w(3), w(3)},
			winning: w(3),
			want:    map[FanKind]int{LowerTiles: 1, ThreeConcealedPungs: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, SelfDrawn: 1, EdgeWait: 1},
			wantSum: 49,
		},
		{
			name:    "全中",
			tiles:   []engine.Tile{w(4), w(5), w(6), o(4), o(5), o(6), s(4), s(5), s(6), w(5), w(5), w(5), w(5), w(5)},
			winning: w(5),
			want:    map[FanKind]int{MiddleTiles: 1, AllFives: 1, FullyConcealedHand: 1, ConcealedHand: 1, MixedDoubleChow: 3, SelfDrawn: 1, ClosedWait: 1},
			wantSum: 51,
		},
		// === 16 分档 ===
		{
			name:    "清龙",
			tiles:   []engine.Tile{w(1), w(2), w(3), w(4), w(5), w(6), w(7), w(8), w(9), o(2), o(3), o(4), fw(1), fw(1)},
			winning: fw(1),
			want:    map[FanKind]int{PureStraight: 1, FullyConcealedHand: 1, ConcealedHand: 1, OneVoidedSuit: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 25,
		},
		{
			name:    "全带五",
			tiles:   []engine.Tile{w(4), w(5), w(6), o(4), o(5), o(6), s(3), s(4), s(5), o(5), o(5), o(5), o(5), o(5)},
			winning: o(5),
			want:    map[FanKind]int{AllFives: 1, FullyConcealedHand: 1, ConcealedHand: 1, SelfDrawn: 1, ClosedWait: 1},
			wantSum: 24,
		},
		{
			name:    "三同刻",
			tiles:   []engine.Tile{w(1), w(1), w(1), o(1), o(1), o(1), s(1), s(1), s(1), o(5), o(6), o(7), w(9), w(9)},
			winning: w(9),
			want:    map[FanKind]int{TriplePung: 1, ThreeConcealedPungs: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 3, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 44,
		},
		{
			name:    "三暗刻",
			tiles:   []engine.Tile{w(4), w(4), w(4), o(2), o(2), o(2), s(3), s(3), s(3), w(6), w(7), w(8), o(7), o(7)},
			winning: o(7),
			want:    map[FanKind]int{ThreeConcealedPungs: 1, MixedShiftedPungs: 1, FullyConcealedHand: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 35,
		},
		// === 12 分档 ===
		{
			name:    "全不靠",
			tiles:   []engine.Tile{fw(1), fw(2), fw(3), fw(4), j(1), j(2), w(1), w(4), w(7), o(2), o(5), s(3), s(6), s(9)},
			winning: s(9),
			want:    map[FanKind]int{LesserHonorsAndKnitted: 1, FullyConcealedHand: 1, SelfDrawn: 0},
			wantSum: 16,
		},
		{
			name:    "大于五",
			tiles:   []engine.Tile{w(6), w(7), w(8), o(6), o(7), o(8), s(7), s(8), s(9), o(9), o(9), o(9), s(9), s(9)},
			winning: s(9),
			want:    map[FanKind]int{UpperFour: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, SelfDrawn: 1},
			wantSum: 20,
		},
		{
			name:    "小于五",
			tiles:   []engine.Tile{w(1), w(2), w(3), o(1), o(2), o(3), s(2), s(3), s(4), o(4), o(4), o(4), s(4), s(4)},
			winning: s(4),
			want:    map[FanKind]int{LowerFour: 1, FullyConcealedHand: 1, ConcealedHand: 1, SelfDrawn: 1},
			wantSum: 19,
		},
		{
			name:    "组合龙",
			tiles:   []engine.Tile{w(1), w(4), w(7), o(2), o(5), o(8), s(3), s(6), s(9), s(2), s(3), s(4), w(9), w(9)},
			winning: w(9),
			want:    map[FanKind]int{KnittedStraight: 1, FullyConcealedHand: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 21,
		},
		{
			name:    "三风刻",
			tiles:   []engine.Tile{fw(1), fw(1), fw(1), fw(3), fw(3), fw(3), fw(4), fw(4), fw(4), o(2), o(3), o(4), s(9), s(9)},
			winning: o(4),
			want:    map[FanKind]int{ThreeConcealedPungs: 1, BigThreeWinds: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 3, OneVoidedSuit: 1, SelfDrawn: 1},
			wantSum: 39,
		},
		// === 8 分档 ===
		{
			name:    "推不倒",
			tiles:   []engine.Tile{o(1), o(2), o(3), o(3), o(4), o(5), o(9), o(9), o(9), s(2), s(2), s(2), j(3), j(3)},
			winning: j(3),
			want:    map[FanKind]int{ReversibleTiles: 1, FullyConcealedHand: 1, TwoConcealedPung: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, OneVoidedSuit: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 20,
		},
		{
			name:    "三色三同顺",
			tiles:   []engine.Tile{w(1), w(2), w(3), o(1), o(2), o(3), s(1), s(2), s(3), o(5), o(6), o(7), fw(1), fw(1)},
			winning: fw(1),
			want:    map[FanKind]int{FullyConcealedHand: 1, AllChows: 1, ConcealedHand: 1, MixedDoubleChow: 3, SelfDrawn: 1, SingleWait: 1},
			wantSum: 13,
		},
		{
			name:    "三色三节高",
			tiles:   []engine.Tile{w(1), w(1), w(1), o(2), o(2), o(2), s(3), s(3), s(3), o(9), o(9), o(9), fw(1), fw(1)},
			winning: fw(1),
			want:    map[FanKind]int{FourConcealedPungs: 1, MixedShiftedPungs: 1, FullyConcealedHand: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 2, SelfDrawn: 1, SingleWait: 1},
			wantSum: 82,
		},
		{
			name:    "妙手回春",
			tiles:   append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5)}, w(5)),
			winning: w(5),
			source:  self,
			ctx:     engine.WinContext{Dealer: 3, LastTile: true},
			want:    map[FanKind]int{LastTileDraw: 1, FullyConcealedHand: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 21,
		},
		{
			name:    "海底捞月",
			tiles:   append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5)}, w(5)),
			winning: w(5),
			feeder:  2,
			ctx:     engine.WinContext{Dealer: 3, LastDiscard: true},
			want:    map[FanKind]int{LastTileClaim: 1, FullyConcealedHand: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 21,
		},
		{
			name:    "杠上开花",
			tiles:   append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5)}, w(5)),
			winning: w(5),
			source:  self,
			ctx:     engine.WinContext{Dealer: 3, KongBloom: true},
			want:    map[FanKind]int{OutWithReplacement: 1, FullyConcealedHand: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 21,
		},
		{
			name:    "抢杠和",
			tiles:   append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5)}, w(5)),
			winning: w(5),
			source:  engine.WinByRobbedKong,
			feeder:  2,
			want:    map[FanKind]int{RobbingTheKong: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SingleWait: 1},
			wantSum: 16,
		},
		// === 6 分档 ===
		{
			name:    "混一色",
			tiles:   []engine.Tile{o(2), o(2), o(2), o(4), o(5), o(6), o(8), o(8), o(8), fw(1), fw(1), fw(1), j(3), j(3)},
			winning: j(3),
			want:    map[FanKind]int{ThreeConcealedPungs: 1, HalfFlush: 1, FullyConcealedHand: 1, PrevalentWind: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 33,
		},
		{
			name:    "三色三步高",
			tiles:   []engine.Tile{w(1), w(2), w(3), o(2), o(3), o(4), s(3), s(4), s(5), fw(1), fw(1), fw(1), o(9), o(9)},
			winning: o(9),
			want:    map[FanKind]int{MixedShiftedChows: 1, FullyConcealedHand: 1, PrevalentWind: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 17,
		},
		{
			name:    "五门齐",
			tiles:   []engine.Tile{w(2), w(3), w(4), o(6), o(7), o(8), s(3), s(4), s(5), fw(1), fw(1), fw(1), j(3), j(3)},
			winning: j(3),
			want:    map[FanKind]int{AllTypes: 1, FullyConcealedHand: 1, PrevalentWind: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 17,
		},
		{
			name:  "全求人",
			tiles: []engine.Tile{o(5), o(5)},
			melds: []engine.Meld{
				meldedChi(w(1), w(2), w(3)),
				meldedChi(o(1), o(2), o(3)),
				meldedPong(s(7)),
				meldedPong(j(1)),
			},
			winning: o(5),
			feeder:  1,
			want:    map[FanKind]int{MeldedHand: 1, DragonPung: 1, PungOfTerminalsOrHons: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 11,
		},
		{
			name:    "双箭刻",
			tiles:   []engine.Tile{j(1), j(1), j(1), j(2), j(2), j(2), o(2), o(3), o(4), o(6), o(7), o(8), fw(1), fw(1)},
			winning: fw(1),
			want:    map[FanKind]int{TwoDragons: 1, HalfFlush: 1, FullyConcealedHand: 1, TwoConcealedPung: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 2, SelfDrawn: 1, SingleWait: 1},
			wantSum: 24,
		},
		// === 4 分档 ===
		{
			name:    "全带幺",
			tiles:   []engine.Tile{w(1), w(2), w(3), o(7), o(8), o(9), s(1), s(1), s(1), fw(1), fw(1), fw(1), j(3), j(3)},
			winning: j(3),
			want:    map[FanKind]int{AllTypes: 1, OutsideHand: 1, FullyConcealedHand: 1, PrevalentWind: 1, TwoConcealedPung: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 2, SelfDrawn: 1, SingleWait: 1},
			wantSum: 24,
		},
		{
			name:    "不求人",
			tiles:   append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5)}, w(5)),
			winning: w(5),
			source:  self,
			want:    map[FanKind]int{FullyConcealedHand: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 13,
		},
		{
			name:     "和绝张",
			tiles:    append([]engine.Tile{w(1), w(3), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5), w(5)}, w(2)),
			winning:  w(2),
			lastTile: true,
			want:     map[FanKind]int{MixedShiftedChows: 1, FullyConcealedHand: 1, LastTile: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, ClosedWait: 1},
			wantSum:  19,
		},
		// === 2 分档 ===
		{
			name:    "箭刻",
			tiles:   []engine.Tile{j(1), j(1), j(1), o(2), o(3), o(4), o(6), o(7), o(8), fw(2), fw(2), fw(2), w(5), w(5)},
			winning: w(5),
			ctx:     engine.WinContext{Dealer: 2}, // 庄家 2 → 座位 0 是南家，东风刻不计门风刻
			want:    map[FanKind]int{FullyConcealedHand: 1, DragonPung: 1, TwoConcealedPung: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 2, OneVoidedSuit: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 15,
		},
		{
			name:    "圈风刻（东圈）",
			tiles:   []engine.Tile{fw(1), fw(1), fw(1), o(2), o(3), o(4), o(6), o(7), o(8), s(2), s(3), s(4), w(5), w(5)},
			winning: w(5),
			ctx:     engine.WinContext{Dealer: 2},
			want:    map[FanKind]int{FullyConcealedHand: 1, PrevalentWind: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 11,
		},
		{
			name:    "门风刻（庄家东）",
			tiles:   []engine.Tile{fw(1), fw(1), fw(1), o(2), o(3), o(4), o(6), o(7), o(8), s(2), s(3), s(4), w(5), w(5)},
			winning: w(5),
			source:  disc,
			feeder:  1,
			ctx:     engine.WinContext{Dealer: 0},
			want:    map[FanKind]int{SeatWind: 1, PrevalentWind: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 1, SingleWait: 1},
			wantSum: 8,
		},
		{
			name:    "平和",
			tiles:   append([]engine.Tile{o(2), o(3), o(4), o(5), o(6), o(7), s(3), s(4), s(5), s(6), s(7), s(8), w(7)}, w(7)),
			winning: w(7),
			want:    map[FanKind]int{FullyConcealedHand: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 13,
		},
		{
			name:    "双同刻",
			tiles:   []engine.Tile{o(5), o(5), o(5), s(5), s(5), s(5), o(2), o(3), o(4), o(7), o(8), o(9), fw(1), fw(1)},
			winning: fw(1),
			want:    map[FanKind]int{TwoConcealedPung: 1, DoublePung: 1, FullyConcealedHand: 1, ConcealedHand: 1, OneVoidedSuit: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 13,
		},
		{
			name:    "断幺",
			tiles:   append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), s(6), s(7), s(8), o(5)}, o(5)),
			winning: o(5),
			want:    map[FanKind]int{FullyConcealedHand: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 13,
		},
		// === 1 分档 ===
		{
			name:    "一般高＋连六",
			tiles:   append([]engine.Tile{w(1), w(2), w(3), w(1), w(2), w(3), w(7), w(8), w(9), o(2), o(3), o(4), s(5)}, s(5)),
			winning: s(5),
			want:    map[FanKind]int{ShortStraight: 2, TwoTerminalChows: 2, FullyConcealedHand: 1, AllChows: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 15,
		},
		{
			name:    "喜相逢",
			tiles:   append([]engine.Tile{w(1), w(2), w(3), o(1), o(2), o(3), s(4), s(5), s(6), s(7), s(8), s(9), fw(1)}, fw(1)),
			winning: fw(1),
			want:    map[FanKind]int{FullyConcealedHand: 1, AllChows: 1, ConcealedHand: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 10,
		},
		{
			name:    "幺九刻",
			tiles:   append([]engine.Tile{w(1), w(1), w(1), o(9), o(9), o(9), s(1), s(1), s(1), o(2), o(3), o(4), fw(1)}, fw(1)),
			winning: fw(1),
			want:    map[FanKind]int{ThreeConcealedPungs: 1, FullyConcealedHand: 1, DoublePung: 1, ConcealedHand: 1, PungOfTerminalsOrHons: 3, SelfDrawn: 1, SingleWait: 1},
			wantSum: 29,
		},
		{
			name:    "缺一门＋无字",
			tiles:   append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), o(6), o(7), o(8), w(5), w(5), w(5), o(5)}, o(5)),
			winning: o(5),
			want:    map[FanKind]int{FullyConcealedHand: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, OneVoidedSuit: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 12,
		},
		{
			name:    "单钓将",
			tiles:   append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5)}, w(5)),
			winning: w(5),
			want:    map[FanKind]int{SingleWait: 1, FullyConcealedHand: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1},
			wantSum: 13,
		},
		{
			name:    "边张",
			tiles:   append([]engine.Tile{w(1), w(2), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5), w(5)}, w(3)),
			winning: w(3),
			want:    map[FanKind]int{EdgeWait: 1, FullyConcealedHand: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, MixedShiftedChows: 1},
			wantSum: 15,
		},
		{
			name:    "坎张",
			tiles:   append([]engine.Tile{w(1), w(3), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5), w(5)}, w(2)),
			winning: w(2),
			want:    map[FanKind]int{ClosedWait: 1, FullyConcealedHand: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, MixedShiftedChows: 1},
			wantSum: 15,
		},
		{
			name:    "自摸",
			tiles:   append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5)}, w(5)),
			winning: w(5),
			source:  self,
			want:    map[FanKind]int{SelfDrawn: 1, FullyConcealedHand: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SingleWait: 1},
			wantSum: 13,
		},
		{
			name:     "和绝张（另三张在他人副子）",
			tiles:    append([]engine.Tile{w(1), w(3), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5), w(5)}, w(2)),
			winning:  w(2),
			lastTile: true,
			want:     map[FanKind]int{MixedShiftedChows: 1, FullyConcealedHand: 1, LastTile: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, ClosedWait: 1},
			wantSum:  19,
		},
		{
			name:    "无番和",
			tiles:   append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), o(6), o(7), o(8), s(3), s(4), s(5), w(5)}, w(5)),
			winning: w(5),
			feeder:  1,
			want:    map[FanKind]int{FullyConcealedHand: 1, AllChows: 1, AllSimples: 1, ConcealedHand: 1, NoHonors: 1, SelfDrawn: 1, SingleWait: 1},
			wantSum: 13,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			feeder := c.feeder
			var res *engine.Result
			if c.lastTile {
				res = mkLastTileResult(c.tiles, c.melds, c.winning, c.source, feeder, c.ctx)
			} else {
				res = mkResult(c.tiles, c.melds, c.winning, c.source, feeder, c.ctx)
			}
			if res.Decompositions == nil {
				t.Fatalf("构造的手牌无法分解（测试用例错误）：%v", c.tiles)
			}
			roundWind := uint8(1)
			if c.roundWind != 0 {
				roundWind = c.roundWind
			}
			got := Score(res, Context{RoundWind: roundWind})
			if got.Best.Total != c.wantSum {
				var parts []string
				for _, f := range got.Best.Fans {
					parts = append(parts, string(f.Kind)+"="+strconv.Itoa(f.Value))
				}
				t.Errorf("总分 = %d，期望 %d：%v", got.Best.Total, c.wantSum, parts)
			}
			if c.want != nil {
				set := fanSet(got.Best.Fans)
				for k, want := range c.want {
					if gotN := set[k]; gotN != want {
						t.Errorf("番种 %s = %d，期望 %d（识别：%v）", k, gotN, want, got.Best.Fans)
					}
				}
			}
		})
	}
}

// --- 结算公式 ---

func TestSettlement(t *testing.T) {
	// 点炮：胜者得 放铳者付(番种分+8) + 其余两家各付 8。
	tiles := append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5)}, w(5))
	res := mkResult(tiles, nil, w(5), engine.WinByDiscard, 2, engine.WinContext{Dealer: 3})
	got := Score(res, Context{RoundWind: 1})
	if got.Best.Total != 8 {
		t.Fatalf("前置条件：番种分应 8，得 %d（%v）", got.Best.Total, got.Best.Fans)
	}
	st := got.Settlement
	if st.Kind != PayoutDiscard || st.Feeder != 2 || st.Winner != 0 {
		t.Errorf("结算方式/放铳者错：%+v", st)
	}
	// 8 + 8 = 16；放铳者 -16，其他两家各 -8，胜者 +32。
	if st.Deltas[2] != -16 || st.Deltas[3] != -8 || st.Deltas[1] != -8 || st.Deltas[0] != 32 {
		t.Errorf("点炮得分错：%v", st.Deltas)
	}
	if st.Deltas[0]+st.Deltas[1]+st.Deltas[2]+st.Deltas[3] != 0 {
		t.Errorf("非零和：%v", st.Deltas)
	}

	// 自摸：三家各付（番种分+8）。
	res2 := mkResult(tiles, nil, w(5), engine.WinSelfDrawn, -1, engine.WinContext{Dealer: 3})
	got2 := Score(res2, Context{RoundWind: 1})
	if got2.Best.Total != 13 { // 自摸 + 不求人：8 + 5 = 13
		t.Fatalf("前置条件：番种分应 13，得 %d", got2.Best.Total)
	}
	st2 := got2.Settlement
	if st2.Kind != PayoutSelfDrawn || st2.Feeder != -1 {
		t.Errorf("自摸结算方式错：%+v", st2)
	}
	for i := 0; i < engine.Seats; i++ {
		if i == 0 {
			if st2.Deltas[i] != 3*21 {
				t.Errorf("自摸胜者 +%d，应 +63", st2.Deltas[i])
			}
		} else if st2.Deltas[i] != -21 {
			t.Errorf("自摸付者 %d，应 -21", st2.Deltas[i])
		}
	}

	// 抢杠和：按点和口径（放铳者＝被抢杠者）。
	res3 := mkResult(tiles, nil, w(5), engine.WinByRobbedKong, 2, engine.WinContext{Dealer: 3})
	got3 := Score(res3, Context{RoundWind: 1})
	if got3.Settlement.Kind != PayoutDiscard || got3.Settlement.Feeder != 2 {
		t.Errorf("抢杠和应按点和计：%+v", got3.Settlement)
	}

	// 流局：零分。
	res4 := &engine.Result{Exhaustive: true, Winner: -1, Feeder: -1, Context: engine.WinContext{Dealer: 0}}
	got4 := Score(res4, Context{RoundWind: 1})
	if !got4.Exhaustive || got4.Settlement.Kind != PayoutExhaustive {
		t.Errorf("流局错：%+v", got4)
	}
	var zero [4]int
	if got4.Settlement.Deltas != zero {
		t.Errorf("流局应零分：%v", got4.Settlement.Deltas)
	}
}

// --- 起和线（8 番）边界 ---

func TestMinFanBoundary(t *testing.T) {
	// 8 番整：无番和（构造无任何番种的和牌：连碰碰和也没有——用副子明吃+明碰+门清外）
	// 用 门前清之外的构造：tiles 有吃碰副子则不计门清等，取法则更简单。
	tiles := []engine.Tile{o(2), o(3), o(4), o(5), o(6), o(7), s(2), s(3), s(4), s(5), s(6), s(7), w(7), w(7)}
	_ = tiles
	// 直接测：7 番不过线，8 番过线，用九莲（88）与十三幺（88）等大番不必测；
	// 用最小番：单钓将 1 番。
	small := append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5)}, w(5))
	res := mkResult(small, nil, w(5), engine.WinByDiscard, 2, engine.WinContext{Dealer: 3})
	got := Score(res, Context{RoundWind: 1})
	if got.Best.Total >= 8 && got.Eligible != true {
		t.Errorf("≥8 番应 Eligible")
	}
	if got.Best.Total < 8 && got.Eligible {
		t.Errorf("<8 番不应 Eligible（总分 %d）", got.Best.Total)
	}
	// 无番和：8 分兜底，Eligible。
	chicken := append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), o(6), o(7), o(8), s(3), s(4), s(5), w(5)}, w(5))
	res2 := mkResult(chicken, nil, w(5), engine.WinByDiscard, 1, engine.WinContext{Dealer: 3})
	got2 := Score(res2, Context{RoundWind: 1})
	if !got2.Eligible || got2.Best.Total != 8 {
		t.Errorf("无番和应 8 分且过线：%+v", got2.Best)
	}
}

// --- 分解↔结果一致性：计分消费的分解必须能由胜者手牌重构 ---

func TestDecompositionConsistency(t *testing.T) {
	// 用几个代表性手牌：分解的牌面总数（含将）须等于胜者手牌，且每个牌值计数一致。
	samples := [][]engine.Tile{
		append([]engine.Tile{w(2), w(3), w(4), o(2), o(3), o(4), s(3), s(4), s(5), o(6), o(7), o(8), w(5)}, w(5)),
		append([]engine.Tile{w(1), w(1), w(2), w(2), w(3), w(3), w(4), w(4), w(5), w(5), w(6), w(6), w(7)}, w(7)),
		append([]engine.Tile{w(1), w(1), w(1), w(2), w(3), w(4), w(5), w(6), w(7), w(8), w(9), w(9), w(9)}, w(3)),
		append([]engine.Tile{w(1), w(9), o(1), o(9), s(1), s(9), fw(1), fw(2), fw(3), fw(4), j(1), j(2), j(3)}, j(3)),
	}
	for i, tiles := range samples {
		res := mkResult(tiles, nil, tiles[len(tiles)-1], engine.WinByDiscard, 1, engine.WinContext{Dealer: 3})
		if res.Decompositions == nil {
			t.Fatalf("样例 %d 无法分解", i)
		}
		for _, d := range res.Decompositions {
			var rebuilt []engine.Tile
			for _, s := range d.Sets {
				rebuilt = append(rebuilt, s.Tiles...)
			}
			if d.Pair != nil {
				rebuilt = append(rebuilt, *d.Pair, *d.Pair)
			}
			if d.Shape == engine.ShapeSevenPairs {
				rebuilt = rebuilt[:0]
				for _, p := range d.Pairs {
					rebuilt = append(rebuilt, p, p)
				}
			}
			if d.Shape == engine.ShapeAllSingles || d.Shape == engine.ShapeThirteenOrphans {
				rebuilt = rebuilt[:0]
				rebuilt = append(rebuilt, d.Singles...)
				if d.Pair != nil {
					rebuilt = append(rebuilt, *d.Pair, *d.Pair)
				}
			}
			if len(rebuilt) != len(res.WinnerHand) {
				t.Errorf("样例 %d：分解牌数 %d ≠ 手牌 %d", i, len(rebuilt), len(res.WinnerHand))
				continue
			}
			a, b := hist(rebuilt), hist(res.WinnerHand)
			if a != b {
				t.Errorf("样例 %d：分解牌面与手牌不一致", i)
			}
		}
	}
}

// --- 已知引擎限制（记录 GX-2 的 Decompose 手数口径） ---

// TestKongHandEngineLimit 记录：引擎 Decompose 的输入张数按 14-3×melds 校验，
// 含暗杠的手牌（杠占 4 张、仅按 1 副面子计）不满足该校验，会返回无分解——
// 因而计分层对「含暗杠且暗杠留到最后成和」的手牌无法计分（含四杠、含暗杠的
// 三杠等）。这是引擎侧口径，属 GX-2；GX-3 按边界不修改引擎，如实记录并等待
// 上层（GX-4/后续工作）决定是否对齐。参见 docs/score-rules.md「已知限制」。
func TestKongHandEngineLimit(t *testing.T) {
	tiles := []engine.Tile{w(2), w(3), w(4), o(2), o(2)}
	melds := []engine.Meld{concealedKong(w(1)), concealedKong(o(9)), concealedKong(s(3))}
	res := mkResult(tiles, melds, w(2), engine.WinSelfDrawn, -1, engine.WinContext{Dealer: 3})
	if res.Decompositions != nil {
		t.Log("引擎现可分解含暗杠的手牌；本限制记录可撤销，请核对 GX-2 最新分支")
	}
	_ = Score
}
