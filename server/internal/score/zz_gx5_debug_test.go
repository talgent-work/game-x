package score

// GX-5 验证调试：复现种子 1067 的胜者牌（东东东碰＋4饼补杠＋两组吃＋7万将，自摸）
// 检查风刻番种（门风刻/圈风刻）是否命中。
// 本文件为 GX-5 验证工具，不属产品代码。

import (
	"testing"

	"talgent-work/game-x/server/internal/engine"
)

func TestGX5WindFansSeed1067(t *testing.T) {
	w := func(r int) engine.Tile { return engine.Tile{Suit: 0, Rank: uint8(r)} }
	p := func(r int) engine.Tile { return engine.Tile{Suit: 1, Rank: uint8(r)} }
	ti := func(r int) engine.Tile { return engine.Tile{Suit: 2, Rank: uint8(r)} }
	east := engine.Tile{Suit: 3, Rank: 1}
	pong := func(x engine.Tile) engine.Meld {
		return engine.Meld{Kind: 1, Tiles: []engine.Tile{x, x, x}, From: 3}
	}
	chi := func(a, b, c engine.Tile) engine.Meld {
		return engine.Meld{Kind: 0, Tiles: []engine.Tile{a, b, c}, From: 3}
	}
	kong := engine.Meld{Kind: 2, Tiles: []engine.Tile{p(4), p(4), p(4), p(4)}, From: 1, Kong: 2}
	res := &engine.Result{
		Exhaustive:  false,
		Winner:      0,
		WinningTile: w(7),
		Source:      engine.WinSelfDrawn,
		Feeder:      -1,
		WinnerHand:  []engine.Tile{w(7), w(7)},
		WinnerMelds: []engine.Meld{chi(ti(4), ti(5), ti(6)), pong(east), chi(ti(1), ti(2), ti(3)), kong},
		Context: engine.WinContext{Dealer: 0},
		Decompositions: []engine.WinHand{{
			Shape: engine.ShapeStandard,
			Pair:  &[]engine.Tile{w(7)}[0],
			Sets:  []engine.Set{},
		}},
	}
	s := Score(res, Context{RoundWind: 1})
	for _, f := range s.Best.Fans {
		t.Logf("fan: %s ×%d = %d", f.Kind, f.Count, f.Value)
	}
	t.Logf("total=%d eligible=%v", s.Best.Total, s.Eligible)
	found := map[string]int{}
	for _, f := range s.Best.Fans {
		found[string(f.Kind)] = f.Count
	}
	if found["门风刻"] == 0 {
		t.Errorf("门风刻（庄家东＋东刻）未计入")
	}
	if found["圈风刻"] == 0 {
		t.Errorf("圈风刻（东风圈＋东刻）未计入")
	}
}
