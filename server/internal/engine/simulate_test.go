package engine

import (
	"fmt"
	"math/rand"
	"testing"
)

// 大规模随机对局模拟：以随机合法动作驱动整局，
// 每一步之后校验不变式，终局后校验结果。
//
// 不变式：
//  1. 牌张守恒：全场 136 张（各家手牌＋副子＋牌墙＋未被吃碰杠和走的牌河＋托管中的补杠牌），
//     且每个牌值 ≤ 4 张；
//  2. 手牌形态：打牌阶段行动者 14-3×副子数，其余 13-3×副子数；声明窗口全员 13-3×副子数；
//  3. 全部对局必然终局（和牌或流局），步数有界；
//  4. 和牌结果：手牌形态正确、分解非空且可由胜者手牌重构；
//  5. 全程牌值合法。
func TestSimulationInvariants(t *testing.T) {
	type stats struct {
		games, steps                     int64
		wins, exhaustive                 int64
		selfDrawn, byDiscard, robbedKong int64
		chi, pong, kongDisc              int64
		kongCon, kongProm, robWindows    int64
		shapes                           map[WinShape]int64
		ctxKongBloom, ctxLastTile        int64
		ctxLastDiscard, ctxInitial       int64
		ctxFirstGoAround                 int64
		maxSteps                         int64
	}
	s := stats{shapes: map[WinShape]int64{}}

	run := func(policyName string, games int64, claimProb float64) {
		for seed := int64(1); seed <= games; seed++ {
			g := NewGame(seed+1_000_000, int(seed)%Seats)
			rng := rand.New(rand.NewSource(seed*7919 + 42))
			gameSteps := 0
			for !g.Over() {
				step(nil, g, rng, claimProb)
				s.steps++
				gameSteps++
				checkInvariants(t, g, seed)
				if gameSteps > 5000 {
					t.Fatalf("种子 %d：对局未在 5000 步内收束", seed)
				}
			}
			if int64(gameSteps) > s.maxSteps {
				s.maxSteps = int64(gameSteps)
			}
			s.games++
			validateResult(t, g, seed)
			for _, ev := range g.Log() {
				switch ev.Kind {
				case EvWin:
					s.wins++
					switch ev.Result.Source {
					case WinSelfDrawn:
						s.selfDrawn++
					case WinByDiscard:
						s.byDiscard++
					case WinByRobbedKong:
						s.robbedKong++
					}
					if ev.Result.Context.KongBloom {
						s.ctxKongBloom++
					}
					if ev.Result.Context.LastTile {
						s.ctxLastTile++
					}
					if ev.Result.Context.LastDiscard {
						s.ctxLastDiscard++
					}
					if ev.Result.Context.Initial {
						s.ctxInitial++
					}
					if ev.Result.Context.FirstGoAround {
						s.ctxFirstGoAround++
					}
					for _, wh := range ev.Result.Decompositions {
						s.shapes[wh.Shape]++
					}
				case EvExhaustive:
					s.exhaustive++
				case EvKongPromoted:
					s.kongProm++ // 声明补杠（开抢杠窗口）
					s.robWindows++
				case EvMeld:
					switch ev.Meld.Kind {
					case MeldChi:
						s.chi++
					case MeldPong:
						s.pong++
					case MeldKong:
						switch ev.Meld.Kong {
						case KongFromDiscard:
							s.kongDisc++
						case KongFromConcealed:
							s.kongCon++
						case KongFromPromote:
							// 已计入 kongProm（声明），落定不重复计
						}
					}
				}
			}
			_ = policyName
		}
	}

	run("greedy", 4000, 0.9) // 高声明率：充分走吃碰杠路径
	run("sparse", 4000, 0.2) // 低声明率：充分走摸打/流局路径

	t.Logf("模拟统计：%d 局 %d 步（单局最多 %d 步）", s.games, s.steps, s.maxSteps)
	t.Logf("终局：和牌 %d（自摸 %d 点和 %d 抢杠 %d）｜流局 %d",
		s.wins, s.selfDrawn, s.byDiscard, s.robbedKong, s.exhaustive)
	t.Logf("副子：吃 %d 碰 %d 明杠 %d 暗杠 %d 补杠 %d（抢杠窗口 %d）",
		s.chi, s.pong, s.kongDisc, s.kongCon, s.kongProm, s.robWindows)
	t.Logf("和牌型：%v", s.shapes)
	t.Logf("上下文：杠上开花 %d 妙手回春 %d 海底捞月 %d 天和 %d 头一轮 %d",
		s.ctxKongBloom, s.ctxLastTile, s.ctxLastDiscard, s.ctxInitial, s.ctxFirstGoAround)

	if s.wins == 0 || s.exhaustive == 0 {
		t.Fatalf("和牌与流局路径都应出现：%v/%v", s.wins, s.exhaustive)
	}
	if s.selfDrawn == 0 || s.byDiscard == 0 {
		t.Fatalf("自摸与点和都应出现：%v/%v", s.selfDrawn, s.byDiscard)
	}
	if s.chi == 0 || s.pong == 0 || s.kongCon == 0 || s.kongDisc == 0 {
		t.Fatalf("吃/碰/明杠/暗杠都应出现：吃 %d 碰 %d 明杠 %d 暗杠 %d", s.chi, s.pong, s.kongDisc, s.kongCon)
	}
	if s.ctxKongBloom == 0 {
		t.Fatalf("杠上开花应出现（暗/明杠已出现且补牌后可和）")
	}
}

// checkInvariants 每步校验（终局后的形态由 validateResult 单独校验）。
func checkInvariants(t *testing.T, g *Game, seed int64) {
	t.Helper()
	if g.phase == PhaseOver {
		return
	}
	var c [34]int
	add := func(tiles []Tile) {
		for _, x := range tiles {
			if !x.Valid() {
				t.Fatalf("种子 %d：非法牌值 %v", seed, x)
			}
			c[x.ord()]++
		}
	}
	for s := 0; s < Seats; s++ {
		add(g.hands[s])
		for _, m := range g.melds[s] {
			add(m.Tiles)
		}
	}
	add(g.wall.tiles)
	for s := 0; s < Seats; s++ {
		for _, e := range g.rivers[s] {
			if !e.Claimed { // 被吃碰杠和走的张已在其副子/胜者手中
				if !e.Tile.Valid() {
					t.Fatalf("种子 %d：牌河非法牌值 %v", seed, e.Tile)
				}
				c[e.Tile.ord()]++
			}
		}
	}
	if g.phase == PhaseClaim && g.pending.Rob {
		c[g.pending.Tile.ord()]++ // 托管中的补杠牌
	}
	total := 0
	for o, n := range c {
		if n > 4 {
			t.Fatalf("种子 %d：牌值 %s 出现 %d 张（>4）", seed, tileFromOrd(o), n)
		}
		total += n
	}
	if total != TotalTiles {
		t.Fatalf("种子 %d：牌张守恒失败，全场 %d 张 ≠ %d（phase=%v pending=%+v）",
			seed, total, TotalTiles, g.phase, g.pending)
	}

	// 手牌形态。
	for s := 0; s < Seats; s++ {
		want := 13 - 3*len(g.melds[s])
		if g.phase == PhaseDiscard && s == g.turn {
			want = 14 - 3*len(g.melds[s])
		}
		if got := len(g.hands[s]); got != want {
			t.Fatalf("种子 %d：座位 %d 手牌 %d 张，应为 %d（phase=%v turn=%d melds=%d）",
				seed, s, got, want, g.phase, g.turn, len(g.melds[s]))
		}
	}
}

// validateResult 终局结果校验。
func validateResult(t *testing.T, g *Game, seed int64) {
	t.Helper()
	r := g.Result()
	if r == nil {
		t.Fatalf("种子 %d：终局无结果", seed)
	}
	// 终局快照：Result 含全部暗牌（揭示）。
	for s := 0; s < Seats; s++ {
		if len(r.Hands[s]) != len(g.hands[s]) {
			t.Fatalf("种子 %d：终局暗牌与状态不一致", seed)
		}
	}
	if r.Exhaustive {
		if r.Winner != -1 {
			t.Fatalf("种子 %d：流局不应有胜者", seed)
		}
		return
	}
	if r.Winner < 0 || r.Winner >= Seats {
		t.Fatalf("种子 %d：胜者座位非法 %d", seed, r.Winner)
	}
	want := 14 - 3*len(r.WinnerMelds)
	if got := len(r.WinnerHand); got != want {
		t.Fatalf("种子 %d：胜者手牌 %d 张，应为 %d", seed, got, want)
	}
	if len(r.Decompositions) == 0 {
		t.Fatalf("种子 %d：胜者无和牌分解", seed)
	}
	// 分解只覆盖暗牌部分（含胡牌张）；吃碰杠副子不在分解里、由 WinnerMelds 充当。
	wantCounts := countsOf(r.WinnerHand)
	for _, wh := range r.Decompositions {
		var got [34]int
		for _, set := range wh.Sets {
			for _, x := range set.Tiles {
				got[x.ord()]++
			}
		}
		if wh.Pair != nil {
			got[wh.Pair.ord()] += 2
		}
		for _, p := range wh.Pairs {
			got[p.ord()] += 2
		}
		for _, x := range wh.Singles {
			got[x.ord()]++
		}
		if got != wantCounts {
			t.Fatalf("种子 %d：分解与胜者手牌不一致：分解 %v vs 手牌 %v", seed, got, wantCounts)
		}
	}
	// 集合数正确：分解的副子数＋副子数＝4（组合龙计 3 副）。
	for _, wh := range r.Decompositions {
		sets := len(r.WinnerMelds)
		for _, set := range wh.Sets {
			if set.Kind == SetKnitted {
				sets += 3
			} else {
				sets++
			}
		}
		if wh.Shape == ShapeSevenPairs || wh.Shape == ShapeAllSingles {
			if len(r.WinnerMelds) != 0 {
				t.Fatalf("种子 %d：七对/全不靠须门清", seed)
			}
			continue
		}
		if sets != 4 || wh.Pair == nil {
			t.Fatalf("种子 %d：分解副子数 %d ≠ 4 或缺将", seed, sets)
		}
	}
}

// 一局完整对局日志（-v 查看；交付说明引用）。
func TestFullGameLog(t *testing.T) {
	g := NewGame(20261017, 0)
	rng := rand.New(rand.NewSource(99))
	steps := 0
	for !g.Over() {
		step(nil, g, rng, 0.6)
		steps++
		if steps > 500 {
			t.Fatal("对局未收束")
		}
	}
	r := g.Result()
	t.Logf("种子 20261017，庄家 0，共 %d 步", steps)
	for _, ev := range g.Log() {
		switch ev.Kind {
		case EvDiscarded:
			t.Logf("  座位 %d 打出 %v", ev.Seat, ev.Tile)
		case EvDrawn:
			t.Logf("  座位 %d 摸牌（%v）", ev.Seat, drawSourceName(ev.Source))
		case EvMeld:
			t.Logf("  座位 %d 成副：%v（来源座位 %d）", ev.Seat, meldDesc(*ev.Meld), ev.Meld.From)
		case EvKongPromoted:
			t.Logf("  座位 %d 补杠 %v，开抢杠窗口", ev.Seat, ev.Tile)
		case EvWin:
			t.Logf("  座位 %d 和牌：%v（胡 %v，放铳/被抢 %d）",
				ev.Result.Winner, winSourceName(ev.Result.Source), ev.Result.WinningTile, ev.Result.Feeder)
		case EvExhaustive:
			t.Logf("  流局")
		}
	}
	if !r.Exhaustive {
		t.Logf("终局：胜者 %d，和牌型 %v，副子 %d 副",
			r.Winner, r.Decompositions[0].Shape, len(r.WinnerMelds))
	}
}

func drawSourceName(s DrawSource) string {
	switch s {
	case DrawInitial:
		return "开局第一张"
	case DrawWall:
		return "牌头"
	case DrawKong:
		return "杠后补牌"
	}
	return "?"
}

func meldDesc(m Meld) string {
	switch m.Kind {
	case MeldChi:
		return fmt.Sprintf("吃 %v", m.Tiles)
	case MeldPong:
		return fmt.Sprintf("碰 %v", m.Tiles)
	case MeldKong:
		switch m.Kong {
		case KongFromDiscard:
			return fmt.Sprintf("明杠 %v", m.Tiles)
		case KongFromConcealed:
			return fmt.Sprintf("暗杠（牌面隐藏）×4")
		case KongFromPromote:
			return fmt.Sprintf("补杠 %v", m.Tiles)
		}
	}
	return "?"
}

func winSourceName(s WinSource) string {
	switch s {
	case WinSelfDrawn:
		return "自摸"
	case WinByDiscard:
		return "点和"
	case WinByRobbedKong:
		return "抢杠和"
	}
	return "?"
}
