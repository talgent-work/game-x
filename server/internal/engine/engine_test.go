package engine

import (
	"encoding/json"
	"math/rand"
	"testing"
)

// --- 测试辅助：直接拼装确定局面（白盒） ---

// fixedGame 直接构造局面：dealer 为当前行动者，其手牌按已摸牌处理
// （14-3×副子数 张、可自摸）；wallTiles 为剩余牌墙（牌头在前、牌尾在后）。
// hands 须满足：dealer 座位 14 张，其余座位各 13 张。
func fixedGame(hands [Seats][]Tile, wallTiles []Tile, dealer int) *Game {
	g := &Game{dealer: dealer, turn: dealer, phase: PhaseDiscard}
	g.wall.tiles = append([]Tile(nil), wallTiles...)
	for s := 0; s < Seats; s++ {
		g.hands[s] = append([]Tile(nil), hands[s]...)
		sortTiles(g.hands[s])
	}
	g.canSelfWin = true
	return g
}

// applyErr 断言动作被拒绝。
func applyErr(t *testing.T, g *Game, seat int, act Action) {
	t.Helper()
	if _, err := g.Apply(seat, act); err == nil {
		t.Fatalf("座位 %d 动作 %v 应被拒绝", seat, act)
	}
}

// applyOk 断言动作成功。
func applyOk(t *testing.T, g *Game, seat int, act Action) []Event {
	t.Helper()
	evs, err := g.Apply(seat, act)
	if err != nil {
		t.Fatalf("座位 %d 动作 %v 意外被拒：%v", seat, act, err)
	}
	return evs
}

// passAll 把当前窗口其余三家全部声明过（窗口已自动收束则无动作）。
func passAll(t *testing.T, g *Game) {
	t.Helper()
	for g.Phase() == PhaseClaim {
		progressed := false
		for i := 1; i <= Seats-1 && g.Phase() == PhaseClaim; i++ {
			s := (g.pending.Seat + i) % Seats
			if !g.declared[s] {
				applyOk(t, g, s, Action{Kind: ActPass})
				progressed = true
			}
		}
		if g.Phase() != PhaseClaim {
			break
		}
		if !progressed {
			t.Fatalf("窗口无法推进：pending=%v decls=%v", g.pending, g.decls)
		}
	}
}

// mustDiscard 打牌并把窗口全部过掉。
func mustDiscard(t *testing.T, g *Game, seat int, tile Tile) {
	t.Helper()
	applyOk(t, g, seat, Action{Kind: ActDiscard, Tile: tile})
	passAll(t, g)
}

func wan(r uint8) Tile  { return Tile{Suit: SuitWan, Rank: r} }
func tong(r uint8) Tile { return Tile{Suit: SuitTong, Rank: r} }
func tiao(r uint8) Tile { return Tile{Suit: SuitTiao, Rank: r} }

// bigWall 构造一张物理合法的牌墙（16 风＋12 箭＝28 张；固定局面测试的手牌均不含字牌）。
func bigWall() []Tile {
	var w []Tile
	for rank := uint8(1); rank <= 4; rank++ {
		for i := 0; i < 4; i++ {
			w = append(w, Tile{Suit: SuitFeng, Rank: rank})
		}
	}
	for rank := uint8(1); rank <= 3; rank++ {
		for i := 0; i < 4; i++ {
			w = append(w, Tile{Suit: SuitJian, Rank: rank})
		}
	}
	return w
}

// missingTile 找一个 hand 中没有的牌值。
func missingTile(hand []Tile) Tile {
	counts := countsOf(hand)
	for o := 0; o < 34; o++ {
		if counts[o] == 0 {
			return tileFromOrd(o)
		}
	}
	return Tile{}
}

// --- 发牌与视图 ---

func TestDeal(t *testing.T) {
	for seed := int64(1); seed <= 20; seed++ {
		dealer := int(seed) % Seats
		g := NewGame(seed, dealer)
		for s := 0; s < Seats; s++ {
			want := HandSize
			if s == dealer { // 庄家已先摸成 14
				want = HandSize + 1
			}
			if got := len(g.ViewFor(s).Hand); got != want {
				t.Fatalf("种子 %d 座位 %d 起手应有 %d 张，实得 %d", seed, s, want, got)
			}
		}
		want := TotalTiles - Seats*HandSize - 1 // 庄家已先摸一张
		if got := g.ViewFor(0).WallCount; got != want {
			t.Fatalf("种子 %d 发牌后牌墙应余 %d 张，实得 %d", seed, want, got)
		}
		if g.Turn() != dealer {
			t.Fatalf("种子 %d 应轮到庄家 %d", seed, dealer)
		}
		if g.Phase() != PhaseDiscard {
			t.Fatal("开局应处打牌阶段")
		}
	}
}

// 可见性守恒：任一座位视图中可见张数＋隐藏张数（别家手牌＋牌墙＋别家暗杠牌面）＝136。
// 视图可见＝自己的手牌＋各家面子牌（他人暗杠抹面）＋牌河中未被吃碰杠和走的张。
func TestViewHidesOtherHands(t *testing.T) {
	for seed := int64(1); seed <= 200; seed++ {
		g := NewGame(seed, 0)
		rng := rand.New(rand.NewSource(seed * 3))
		driveSome(t, g, rng, 30, 0.4)
		passAll(t, g)
		if g.Over() {
			continue
		}
		for s := 0; s < Seats; s++ {
			snap := g.ViewFor(s)
			visible := len(snap.Hand)
			for _, melds := range snap.Melds {
				for _, m := range melds {
					visible += len(m.Tiles)
				}
			}
			for _, river := range snap.Discards {
				for _, e := range river {
					if !e.Claimed {
						visible++
					}
				}
			}
			hidden := g.wall.remaining()
			for o := 0; o < Seats; o++ {
				if o != s {
					hidden += len(g.hands[o])
					for _, m := range g.melds[o] {
						if m.HideTile() {
							hidden += len(m.Tiles) // 别家暗杠牌面不可见
						}
					}
				}
			}
			if visible+hidden != TotalTiles {
				t.Fatalf("种子 %d 座位 %d 视图守恒失败：可见 %d＋隐藏 %d ≠ %d",
					seed, s, visible, hidden, TotalTiles)
			}
		}
	}
}

// 他人视图中暗杠牌面必须被抹去；自己的视图完整可见。
func TestConcealedKongHiddenInView(t *testing.T) {
	hands := [Seats][]Tile{
		{wan(1), wan(1), wan(1), wan(1), wan(2), wan(3), wan(4), tong(5), tong(6), tong(7), tiao(5), tiao(6), tiao(7), tiao(9)},
		{wan(2), wan(3), wan(4), wan(5), wan(6), wan(7), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(9)},
		{wan(5), wan(6), wan(7), wan(8), wan(9), tong(4), tong(5), tong(6), tiao(4), tiao(5), tiao(6), tong(9), tiao(9)},
		{wan(8), wan(9), tong(1), tong(2), tong(3), tong(4), tong(5), tong(6), tong(7), tong(8), tong(9), tiao(9), wan(9)},
	}
	g := fixedGame(hands, bigWall(), 0)
	applyOk(t, g, 0, Action{Kind: ActKongConcealed, Tile: wan(1)})

	own := g.ViewFor(0)
	if got := own.Melds[0][0].Tiles; len(got) != 4 || got[0] != wan(1) {
		t.Fatalf("自己的视图应看到暗杠牌面，实得 %v", got)
	}
	for s := 1; s < Seats; s++ {
		m := g.ViewFor(s).Melds[0][0]
		if len(m.Tiles) != 0 {
			t.Fatalf("座位 %d 不应看到暗杠牌面，实得 %v", s, m.Tiles)
		}
		if m.Kind != MeldKong || m.Kong != KongFromConcealed {
			t.Fatalf("座位 %d 应看到暗杠的种类与出处，实得 %+v", s, m)
		}
	}
	if g.Phase() != PhaseDiscard || g.Turn() != 0 {
		t.Fatalf("暗杠后应轮到座位 0 打牌，实得 phase=%v turn=%d", g.Phase(), g.Turn())
	}
	if len(g.hands[0]) != 11 || len(g.melds[0]) != 1 {
		t.Fatalf("暗杠后手牌应为 11 张＋1 副杠，实得 %d/%d", len(g.hands[0]), len(g.melds[0]))
	}
}

// --- 动作合法性与回合流转 ---

func TestDiscardRotatesTurn(t *testing.T) {
	g := NewGame(9, 0)
	tile := g.ViewFor(0).Hand[0]
	mustDiscard(t, g, 0, tile)
	if g.Turn() != 1 || g.Phase() != PhaseDiscard {
		t.Fatalf("打出后应轮到座位 1，实得 turn=%d phase=%v", g.Turn(), g.Phase())
	}
	snap := g.ViewFor(0)
	if len(snap.Discards[0]) != 1 || snap.Discards[0][0].Tile != tile {
		t.Fatalf("座位 0 的牌河应为 [%v]", tile)
	}
	if len(snap.Hand) != HandSize {
		t.Fatalf("打出后手牌应为 %d 张", HandSize)
	}
}

func TestIllegalActions(t *testing.T) {
	g := NewGame(7, 0)
	applyErr(t, g, 1, Action{Kind: ActDiscard, Tile: g.ViewFor(1).Hand[0]})    // 非本座
	applyErr(t, g, 0, Action{Kind: ActDiscard, Tile: missingTile(g.hands[0])}) // 手中没有
	applyErr(t, g, 0, Action{Kind: ActPass})                                   // 打牌阶段不能过
	applyErr(t, g, 0, Action{Kind: ActPong})                                   // 不在窗口
	applyErr(t, g, 1, Action{Kind: ActChi, With: []Tile{wan(1), wan(2)}})      // 不在窗口
	if !winExists(g.hands[0], 0) {
		applyErr(t, g, 0, Action{Kind: ActWin}) // 未成和不能报
	}
	applyErr(t, g, 0, Action{Kind: ActKongConcealed, Tile: g.ViewFor(0).Hand[0]}) // 手中不足四张

	// 窗口部分：座位 0 打 3万，座位 1（下家）可吃。
	hands := [Seats][]Tile{
		{wan(3), wan(1), wan(1), wan(5), wan(5), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(2), wan(4), wan(6), wan(6), wan(6), tong(7), tong(8), tong(9), tiao(1), tiao(2), tiao(3), tiao(5), tiao(9)},
		{wan(7), wan(7), wan(2), wan(3), wan(4), tong(1), tong(2), tong(3), tong(4), tong(5), tong(6), tiao(9), tiao(9)},
		{wan(9), wan(9), wan(9), tong(5), tong(6), tong(7), tong(8), tong(9), tiao(1), tiao(2), tiao(3), tiao(4), tiao(4)},
	}
	g2 := fixedGame(hands, bigWall(), 0)
	applyOk(t, g2, 0, Action{Kind: ActDiscard, Tile: wan(3)})
	if g2.Phase() != PhaseClaim {
		t.Fatal("有人可吃时打出后应进入声明窗口")
	}
	applyErr(t, g2, 0, Action{Kind: ActPass})                              // 供牌者不能声明
	applyErr(t, g2, 1, Action{Kind: ActPong})                              // 无对子
	applyErr(t, g2, 1, Action{Kind: ActKongDiscard})                       // 不足三张
	applyErr(t, g2, 1, Action{Kind: ActChi, With: []Tile{wan(2), wan(5)}}) // 不成顺子
	applyErr(t, g2, 1, Action{Kind: ActChi, With: []Tile{wan(3), wan(4)}}) // 不能用被吃之牌本身
	applyErr(t, g2, 2, Action{Kind: ActChi, With: []Tile{wan(2), wan(4)}}) // 不是上家
	applyErr(t, g2, 1, Action{Kind: ActWin})                               // 不成和牌
	applyErr(t, g2, 1, Action{Kind: ActDiscard, Tile: wan(2)})             // 窗口内不能打牌
	applyOk(t, g2, 2, Action{Kind: ActPass})
	applyErr(t, g2, 2, Action{Kind: ActPass}) // 本窗口已声明
	applyOk(t, g2, 3, Action{Kind: ActPass})
	applyOk(t, g2, 1, Action{Kind: ActChi, With: []Tile{wan(2), wan(4)}})
	if g2.Phase() != PhaseDiscard || g2.Turn() != 1 {
		t.Fatalf("吃后应轮到座位 1 打牌，实得 phase=%v turn=%d", g2.Phase(), g2.Turn())
	}
}

// 吃：只能吃上家打出的牌。
func TestChiOnlyFromPreviousSeat(t *testing.T) {
	hands := [Seats][]Tile{
		{wan(1), wan(1), wan(2), wan(3), wan(4), wan(5), wan(6), wan(7), wan(8), wan(9), tong(1), tong(2), tong(3), tong(9)},
		{wan(2), wan(4), wan(5), wan(6), wan(7), wan(8), wan(9), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3)},
		{wan(2), wan(4), wan(5), wan(6), wan(7), wan(8), wan(9), tong(1), tong(2), tong(3), tong(4), tong(5), tong(6)},
		{wan(5), wan(6), wan(7), wan(8), wan(9), tong(1), tong(2), tong(3), tong(4), tong(5), tong(6), tong(7), tong(8)},
	}
	g := fixedGame(hands, bigWall(), 0)
	applyOk(t, g, 0, Action{Kind: ActDiscard, Tile: wan(3)})
	if g.Phase() != PhaseClaim {
		t.Fatal("下家可吃时应进入声明窗口")
	}
	applyErr(t, g, 2, Action{Kind: ActChi, With: []Tile{wan(2), wan(4)}}) // 非上家
	applyErr(t, g, 3, Action{Kind: ActChi, With: []Tile{wan(2), wan(4)}}) // 非上家
	applyOk(t, g, 1, Action{Kind: ActChi, With: []Tile{wan(2), wan(4)}})
	if g.Phase() != PhaseDiscard || g.Turn() != 1 {
		t.Fatalf("吃后应轮到座位 1 打牌")
	}
	melds := g.ViewFor(1).Melds[1]
	if len(melds) != 1 || melds[0].Kind != MeldChi || melds[0].From != 0 {
		t.Fatalf("吃后应有来自座位 0 的顺子，实得 %+v", melds)
	}
	run := melds[0].Tiles
	if run[0] != wan(2) || run[1] != wan(3) || run[2] != wan(4) {
		t.Fatalf("顺子应为 2万3万4万，实得 %v", run)
	}
	if len(g.hands[1]) != 11 { // 13 张手牌中两张入副子
		t.Fatalf("吃后手牌应为 11 张，实得 %d", len(g.hands[1]))
	}
}

// 优先级：和 > 碰 > 吃。
func TestClaimPriorityWinBeatsPong(t *testing.T) {
	// 座位 0 打 5万：座位 1 可吃（456），座位 2 可碰（55），座位 3 可和（单钓 5万）。
	hands := [Seats][]Tile{
		{wan(1), wan(1), wan(2), wan(3), wan(5), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(4), wan(6), wan(9), wan(9), wan(9), tong(1), tong(2), tong(3), tiao(1), tong(4), tong(4), tong(4), tiao(9)},
		{wan(5), wan(5), wan(7), wan(8), wan(9), tong(1), tong(2), tong(3), tong(6), tong(6), tong(6), tiao(2), tiao(2)},
		{wan(5), wan(1), wan(1), wan(1), tong(2), tong(2), tong(2), tiao(3), tiao(3), tiao(3), tong(6), tong(7), tong(8)},
	}
	g := fixedGame(hands, bigWall(), 0)
	applyOk(t, g, 0, Action{Kind: ActDiscard, Tile: wan(5)})
	// 碰、吃先声明，和最后声明——和仍优先。
	applyOk(t, g, 2, Action{Kind: ActPong})
	applyOk(t, g, 1, Action{Kind: ActChi, With: []Tile{wan(4), wan(6)}})
	applyOk(t, g, 3, Action{Kind: ActWin})
	if !g.Over() {
		t.Fatal("和牌应立即终局")
	}
	if g.Result().Winner != 3 || g.Result().Source != WinByDiscard || g.Result().Feeder != 0 {
		t.Fatalf("座位 3 应点和，实得 %+v", g.Result())
	}
}

func TestClaimPriorityPongBeatsChi(t *testing.T) {
	// 座位 0 打 3万：座位 1 可吃（234），座位 2 可碰（33）。
	hands := [Seats][]Tile{
		{wan(1), wan(1), wan(2), wan(3), wan(3), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(2), wan(2), wan(2), wan(4), wan(4), wan(5), wan(6), wan(7), wan(8), wan(9), tong(1), tong(2), tong(3)},
		{wan(3), wan(3), wan(5), wan(5), wan(5), tong(1), tong(2), tong(3), tong(6), tong(6), tong(6), tiao(2), tiao(2)},
		{wan(9), wan(9), wan(9), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(4), tong(4)},
	}
	g := fixedGame(hands, bigWall(), 0)
	applyOk(t, g, 0, Action{Kind: ActDiscard, Tile: wan(3)})
	applyOk(t, g, 1, Action{Kind: ActChi, With: []Tile{wan(2), wan(4)}})
	applyOk(t, g, 2, Action{Kind: ActPong})
	if g.Phase() != PhaseDiscard || g.Turn() != 2 {
		t.Fatalf("碰应压过吃：轮到座位 2 打牌，实得 phase=%v turn=%d", g.Phase(), g.Turn())
	}
	if m := g.ViewFor(2).Melds[2]; len(m) != 1 || m[0].Kind != MeldPong {
		t.Fatalf("座位 2 应成碰刻，实得 %+v", m)
	}
	if m := g.ViewFor(1).Melds[1]; len(m) != 0 {
		t.Fatalf("座位 1 的吃不应落定，实得 %+v", m)
	}
	if len(g.hands[1]) != 13 {
		t.Fatalf("座位 1 手牌应保持 13 张，实得 %d", len(g.hands[1]))
	}
}

// 截和（头跳）：多家报和取离放铳者最近者（下家 > 对家 > 上家）。
func TestHeadJump(t *testing.T) {
	// 座位 0 打 5万：座位 1 与座位 3 都单钓 5万。
	hands := [Seats][]Tile{
		{wan(1), wan(1), wan(2), wan(3), wan(5), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(5), wan(1), wan(1), wan(1), tong(2), tong(2), tong(2), tiao(3), tiao(3), tiao(3), tong(6), tong(7), tong(8)},
		{wan(9), wan(9), wan(9), wan(2), wan(3), wan(4), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tong(4)},
		{wan(5), wan(1), wan(1), wan(1), tong(2), tong(2), tong(2), tiao(3), tiao(3), tiao(3), tong(6), tong(7), tong(8)},
	}
	g := fixedGame(hands, bigWall(), 0)
	applyOk(t, g, 0, Action{Kind: ActDiscard, Tile: wan(5)})
	applyOk(t, g, 3, Action{Kind: ActWin}) // 上家先声明
	applyOk(t, g, 1, Action{Kind: ActWin}) // 下家后声明
	if !g.Over() || g.Result().Winner != 1 {
		t.Fatalf("截和（头跳）应取座位 1，实得 %+v", g.Result())
	}
}

// --- 杠 ---

// 杠后补牌从牌尾摸；普通摸牌从牌头摸。
func TestKongReplacementFromTail(t *testing.T) {
	hands := [Seats][]Tile{
		{wan(1), wan(1), wan(1), wan(1), wan(2), wan(3), wan(4), tong(5), tong(6), tong(7), tiao(5), tiao(6), tiao(7), tiao(9)},
		{wan(2), wan(3), wan(4), wan(5), wan(6), wan(7), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(9)},
		{wan(5), wan(6), wan(7), wan(8), wan(9), tong(4), tong(5), tong(6), tiao(4), tiao(5), tiao(6), tong(9), tiao(9)},
		{wan(8), wan(9), tong(1), tong(2), tong(3), tong(4), tong(5), tong(6), tong(7), tong(8), tong(9), tiao(9), wan(9)},
	}
	// 牌头 1筒、牌尾 8条，区分两头。
	w := []Tile{tong(1), tong(2), tong(3), tong(4), tong(5), tiao(8)}
	g := fixedGame(hands, w, 0)
	applyOk(t, g, 0, Action{Kind: ActKongConcealed, Tile: wan(1)})
	if !containsTile(g.hands[0], tiao(8)) {
		t.Fatalf("杠后应从牌尾补 8条，手牌=%v", g.hands[0])
	}
	if g.wall.remaining() != 5 || g.drawSource != DrawKong {
		t.Fatalf("杠后应从牌尾补牌（剩 5 张，来源杠），实得 %d/%v", g.wall.remaining(), g.drawSource)
	}
	mustDiscard(t, g, 0, tiao(8))
	if !containsTile(g.hands[1], tong(1)) {
		t.Fatalf("普通摸牌应从牌头摸 1筒，手牌=%v", g.hands[1])
	}
	if g.drawSource != DrawWall {
		t.Fatalf("普通摸牌来源应为牌头，实得 %v", g.drawSource)
	}
}

// 明杠：别人打出的牌＋手中三张成杠，牌尾补牌。
func TestKongFromDiscard(t *testing.T) {
	hands := [Seats][]Tile{
		{wan(5), wan(1), wan(1), wan(2), wan(3), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(5), wan(5), wan(5), wan(9), wan(9), tong(1), tong(2), tong(3), tong(4), tiao(1), tiao(2), tiao(3), tiao(4)},
		{wan(9), wan(9), wan(2), wan(3), wan(4), tong(1), tong(2), tong(3), tiao(5), tiao(5), tiao(5), tong(9), tiao(9)},
		{wan(6), wan(7), wan(8), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tong(4), tong(4), tong(4), tiao(9)},
	}
	g := fixedGame(hands, bigWall(), 0)
	applyOk(t, g, 0, Action{Kind: ActDiscard, Tile: wan(5)})
	applyOk(t, g, 1, Action{Kind: ActKongDiscard})
	if g.Phase() != PhaseDiscard || g.Turn() != 1 {
		t.Fatalf("明杠后应轮到座位 1 打牌")
	}
	m := g.ViewFor(1).Melds[1][0]
	if m.Kind != MeldKong || m.Kong != KongFromDiscard || m.From != 0 || len(m.Tiles) != 4 {
		t.Fatalf("应成来自座位 0 的明杠，实得 %+v", m)
	}
	if len(g.hands[1]) != 11 { // 13 - 3 + 1（补牌）
		t.Fatalf("明杠后手牌应为 11 张，实得 %d", len(g.hands[1]))
	}
	if e := g.ViewFor(1).Discards[0][0]; !e.Claimed {
		t.Fatal("被杠走的牌河张应标记 claimed")
	}
}

// 补杠被抢：刻子保持碰，牌归和牌者。
func TestPromoteKongRobbed(t *testing.T) {
	// 座位 1（庄家）已碰 2万（来自座位 0），现持第四张补杠；
	// 座位 3 持 1万、3万（坎张听 2万）可抢杠和。
	hands := [Seats][]Tile{
		{wan(3), wan(3), wan(3), wan(4), wan(5), wan(6), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4)},
		{wan(2), wan(7), wan(7), wan(7), wan(8), wan(8), wan(8), tong(1), tong(2), tong(3), tiao(1)},
		{wan(4), wan(4), wan(4), wan(9), wan(9), wan(9), tong(4), tong(4), tong(4), tiao(1), tiao(2), tiao(3), tiao(9)},
		{wan(1), wan(3), wan(9), wan(9), wan(9), tong(2), tong(2), tong(2), tiao(5), tiao(5), tiao(5), tiao(9), tiao(9)},
	}
	g := fixedGame(hands, bigWall(), 1)
	g.melds[1] = []Meld{{Kind: MeldPong, Tiles: []Tile{wan(2), wan(2), wan(2)}, From: 0}}
	applyOk(t, g, 1, Action{Kind: ActKongPromote, Tile: wan(2)})
	if g.Phase() != PhaseClaim || !g.pending.Rob {
		t.Fatal("补杠后应开抢杠窗口")
	}
	applyOk(t, g, 3, Action{Kind: ActWin})
	if !g.Over() {
		t.Fatal("抢杠和应终局")
	}
	r := g.Result()
	if r.Winner != 3 || r.Source != WinByRobbedKong || r.Feeder != 1 || r.WinningTile != wan(2) {
		t.Fatalf("座位 3 应抢杠和，实得 %+v", r)
	}
	if m := g.melds[1][0]; m.Kind != MeldPong {
		t.Fatalf("被抢杠者的刻子应保持碰，实得 %+v", m)
	}
	if !containsTile(r.WinnerHand, wan(2)) {
		t.Fatal("胜者手牌应含抢来的 2万")
	}
	if !setContains(r.Decompositions, wan(2)) {
		t.Fatal("分解中应含被抢的 2万")
	}
}

func setContains(whs []WinHand, tile Tile) bool {
	for _, wh := range whs {
		for _, s := range wh.Sets {
			for _, x := range s.Tiles {
				if x == tile {
					return true
				}
			}
		}
		if wh.Pair != nil && *wh.Pair == tile {
			return true
		}
	}
	return false
}

// 补杠无人抢：刻子升杠，牌尾补牌，轮本人打牌。
func TestPromoteKongNoRob(t *testing.T) {
	hands := [Seats][]Tile{
		{wan(3), wan(3), wan(3), wan(4), wan(5), wan(6), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4)},
		{wan(2), wan(7), wan(7), wan(7), wan(8), wan(8), wan(8), tong(1), tong(2), tong(3), tiao(1)},
		{wan(4), wan(4), wan(4), wan(9), wan(9), wan(9), tong(4), tong(4), tong(4), tiao(1), tiao(2), tiao(3), tiao(9)},
		{wan(6), wan(6), wan(6), tong(5), tong(5), tong(5), tiao(6), tiao(6), tiao(6), tong(6), tong(7), tong(8), tong(9)},
	}
	g := fixedGame(hands, bigWall(), 1)
	g.melds[1] = []Meld{{Kind: MeldPong, Tiles: []Tile{wan(2), wan(2), wan(2)}, From: 0}}
	applyOk(t, g, 1, Action{Kind: ActKongPromote, Tile: wan(2)})
	if g.Phase() != PhaseDiscard || g.Turn() != 1 {
		t.Fatalf("无人抢杠后应轮到座位 1 打牌，实得 phase=%v turn=%d", g.Phase(), g.Turn())
	}
	m := g.melds[1][0]
	if m.Kind != MeldKong || m.Kong != KongFromPromote || len(m.Tiles) != 4 {
		t.Fatalf("刻子应升为补杠，实得 %+v", m)
	}
	if len(g.hands[1]) != 11 { // 11 - 1（入杠）+ 1（补牌）
		t.Fatalf("补杠补牌后手牌应为 11 张，实得 %d", len(g.hands[1]))
	}
}

// --- 流局与终局 ---

func TestExhaustive(t *testing.T) {
	// 手牌互不成和、无可声明；牌墙只留 2 张。
	hands := [Seats][]Tile{
		{wan(1), wan(2), wan(4), wan(5), wan(7), wan(8), tong(1), tong(2), tong(4), tong(5), tiao(1), tiao(2), tiao(4), tiao(9)},
		{wan(1), wan(2), wan(4), wan(5), wan(7), wan(8), tong(1), tong(2), tong(4), tong(5), tiao(1), tong(7), tong(8)},
		{wan(2), wan(4), wan(5), wan(7), wan(8), wan(9), tong(2), tong(4), tong(5), tong(7), tiao(2), tiao(4), tiao(5)},
		{wan(4), wan(5), wan(7), wan(8), wan(9), tong(4), tong(5), tong(7), tong(8), tong(9), tiao(4), tiao(5), tiao(7)},
	}
	w := []Tile{tiao(9), wan(9)}
	g := fixedGame(hands, w, 0)
	steps := 0
	for !g.Over() {
		seat := g.Turn()
		tile := g.hands[seat][0]
		mustDiscard(t, g, seat, tile)
		steps++
		if steps > 20 {
			t.Fatal("对局未在有限步内收束")
		}
	}
	if !g.Result().Exhaustive || g.Result().Winner != -1 {
		t.Fatalf("应判流局，实得 %+v", g.Result())
	}
	applyErr(t, g, 0, Action{Kind: ActDiscard, Tile: wan(1)}) // 终局后拒动作
}

// 天和：庄家开局第一张即成和牌型。
func TestInitialWin(t *testing.T) {
	hands := [Seats][]Tile{
		{wan(1), wan(2), wan(3), wan(4), wan(5), wan(6), wan(7), wan(8), wan(9), tong(1), tong(2), tong(3), tiao(1), tiao(1)},
		{wan(1), wan(2), wan(4), wan(5), wan(7), wan(8), tong(1), tong(2), tong(4), tong(5), tiao(1), tiao(2), tiao(4)},
		{wan(2), wan(4), wan(5), wan(7), wan(8), wan(9), tong(2), tong(4), tong(5), tong(7), tiao(2), tiao(4), tiao(5)},
		{wan(4), wan(5), wan(7), wan(8), wan(9), tong(4), tong(5), tong(7), tong(8), tong(9), tiao(4), tiao(5), tiao(7)},
	}
	g := fixedGame(hands, bigWall(), 0)
	g.drawSource = DrawInitial
	applyOk(t, g, 0, Action{Kind: ActWin})
	if !g.Over() || g.Result().Source != WinSelfDrawn || !g.Result().Context.Initial {
		t.Fatalf("庄家开局和应记为天和上下文，实得 %+v", g.Result())
	}
}

// 自摸：报和后给出全部和牌分解。
func TestSelfDrawnWin(t *testing.T) {
	hands := [Seats][]Tile{
		{wan(1), wan(2), wan(4), wan(5), wan(7), wan(8), tong(1), tong(2), tong(4), tong(5), tiao(1), tiao(2), tiao(4)},
		{wan(1), wan(2), wan(3), wan(4), wan(5), wan(6), wan(7), wan(8), wan(9), tong(7), tong(8), tong(9), tiao(1), tiao(1)},
		{wan(2), wan(4), wan(5), wan(7), wan(8), wan(9), tong(2), tong(4), tong(5), tong(7), tiao(2), tiao(4), tiao(5)},
		{wan(4), wan(5), wan(7), wan(8), wan(9), tong(4), tong(5), tong(7), tong(8), tong(9), tiao(4), tiao(5), tiao(7)},
	}
	g := fixedGame(hands, []Tile{wan(1), tong(9)}, 1)
	g.lastDraw = tong(9)
	applyOk(t, g, 1, Action{Kind: ActWin})
	r := g.Result()
	if r.Winner != 1 || r.Source != WinSelfDrawn || r.WinningTile != tong(9) {
		t.Fatalf("座位 1 应自摸 9筒，实得 %+v", r)
	}
	if len(r.Decompositions) == 0 {
		t.Fatal("自摸应给出和牌分解")
	}
	// 123/456/789万＋789筒＋11条 是唯一分解。
	if len(r.Decompositions) != 1 {
		t.Fatalf("该手牌应有唯一分解，实得 %d 种", len(r.Decompositions))
	}
}

// --- 确定性 ---

func TestDeterminism(t *testing.T) {
	// 事件日志以 JSON 序列化比较（值比较，不含指针地址）。
	run := func(seed int64) string {
		g := NewGame(seed, 0)
		rng := rand.New(rand.NewSource(seed))
		driveSome(nil, g, rng, 200, 0.5)
		b, err := json.Marshal(g.Log())
		if err != nil {
			t.Fatalf("序列化事件失败：%v", err)
		}
		return string(b)
	}
	for seed := int64(1); seed <= 30; seed++ {
		if a, b := run(seed), run(seed); a != b {
			t.Fatalf("种子 %d 两次对局不一致", seed)
		}
	}
}

// Snapshot 的 JSON 形状即协议载荷：序列化/反序列化必须无错且稳定。
func TestSnapshotJSON(t *testing.T) {
	g := NewGame(2026, 0)
	rng := rand.New(rand.NewSource(2026))
	driveSome(t, g, rng, 40, 0.5)
	passAll(t, g)
	for s := 0; s < Seats; s++ {
		b, err := json.Marshal(g.ViewFor(s))
		if err != nil {
			t.Fatalf("座位 %d 快照序列化失败：%v", s, err)
		}
		var back map[string]any
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatalf("座位 %d 快照反序列化失败：%v", s, err)
		}
		if back["seat"].(float64) != float64(s) {
			t.Fatalf("座位 %d 快照回读座位不符", s)
		}
	}
	// 终局结果同样可序列化（含全部分解）。
	driveSome(t, g, rng, 400, 0.5)
	if !g.Over() {
		passAll(t, g)
	}
	if g.Over() {
		if _, err := json.Marshal(g.Result()); err != nil {
			t.Fatalf("终局结果序列化失败：%v", err)
		}
	}
}

// --- 随机推进（辅助，也被可见性/确定性测试复用） ---

// driveSome 随机推进至多 n 步。claimProb 为吃/碰/杠声明概率（和牌始终声明）。
func driveSome(t *testing.T, g *Game, rng *rand.Rand, n int, claimProb float64) {
	for i := 0; i < n && !g.Over(); i++ {
		step(t, g, rng, claimProb)
	}
}

func step(t *testing.T, g *Game, rng *rand.Rand, claimProb float64) {
	switch g.Phase() {
	case PhaseClaim:
		for i := 1; i <= Seats-1 && g.Phase() == PhaseClaim; i++ {
			s := (g.pending.Seat + i) % Seats
			if g.declared[s] {
				continue
			}
			acts := g.Actions(s)
			pick := Action{Kind: ActPass}
			for _, a := range acts {
				if a.Kind == ActWin { // 和牌必报
					pick = a
					break
				}
			}
			if pick.Kind == ActPass && rng.Float64() < claimProb {
				var others []Action
				for _, a := range acts {
					if a.Kind != ActPass {
						others = append(others, a)
					}
				}
				if len(others) > 0 {
					pick = others[rng.Intn(len(others))]
				}
			}
			if t != nil {
				applyOk(t, g, s, pick)
			} else if _, err := g.Apply(s, pick); err != nil {
				panic(err)
			}
		}
	case PhaseDiscard:
		acts := g.Actions(g.Turn())
		if len(acts) == 0 {
			panic("无动作可用")
		}
		var pick Action
		for _, a := range acts {
			if a.Kind == ActWin {
				pick = a
				break
			}
		}
		if pick.Kind != ActWin {
			pick = acts[rng.Intn(len(acts))]
		}
		if t != nil {
			applyOk(t, g, g.Turn(), pick)
		} else if _, err := g.Apply(g.Turn(), pick); err != nil {
			panic(err)
		}
	}
}
