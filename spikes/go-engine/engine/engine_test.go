package engine

import (
	"math/rand"
	"testing"
)

func TestWallSize(t *testing.T) {
	wall := NewWall(rand.New(rand.NewSource(1)))
	if len(wall) != TotalTiles {
		t.Fatalf("牌墙应有 %d 张，实得 %d", TotalTiles, len(wall))
	}
}

func TestTileString(t *testing.T) {
	cases := []struct {
		tile Tile
		want string
	}{
		{Tile{SuitWan, 3}, "3万"},
		{Tile{SuitTong, 7}, "7筒"},
		{Tile{SuitTiao, 1}, "1条"},
		{Tile{SuitFeng, 1}, "东"},
		{Tile{SuitFeng, 4}, "北"},
		{Tile{SuitJian, 2}, "发"},
	}
	for _, c := range cases {
		if got := c.tile.String(); got != c.want {
			t.Errorf("%v.String() = %q，期望 %q", c.tile, got, c.want)
		}
	}
}

func TestDeal(t *testing.T) {
	tb := NewTable(42)
	for s := 0; s < Seats; s++ {
		if got := len(tb.ViewFor(s).Hand); got != HandSize {
			t.Fatalf("座位 %d 起手应有 %d 张，实得 %d", s, HandSize, got)
		}
	}
	if got := tb.ViewFor(0).WallCount; got != TotalTiles-Seats*HandSize {
		t.Fatalf("发牌后牌墙应余 %d 张，实得 %d", TotalTiles-Seats*HandSize, got)
	}
}

// 关键断言：任一座位快照拿不到别家手牌。
// 发牌后，视图可见牌数 = 自己 13 + 牌墙 84 = 97；剩余 39 张（=3×13，别家手牌）不在视图里。
func TestViewHidesOtherHands(t *testing.T) {
	tb := NewTable(7)
	snap := tb.ViewFor(2)
	visible := len(snap.Hand) + snap.WallCount
	for _, d := range snap.Discards {
		visible += len(d)
	}
	if missing := TotalTiles - visible; missing != (Seats-1)*HandSize {
		t.Fatalf("视图应隐藏别家手牌 %d 张，实际隐藏 %d 张（可见 %d/%d）",
			(Seats-1)*HandSize, missing, visible, TotalTiles)
	}
}

func TestDrawDiscardRotatesTurn(t *testing.T) {
	tb := NewTable(9)
	if turn := tb.ViewFor(0).Turn; turn != 0 {
		t.Fatalf("开局应轮到座位 0，实得 %d", turn)
	}
	tile, err := tb.Draw()
	if err != nil {
		t.Fatalf("摸牌失败：%v", err)
	}
	if err := tb.Discard(tile); err != nil {
		t.Fatalf("打牌失败：%v", err)
	}
	snap := tb.ViewFor(0)
	if snap.Turn != 1 {
		t.Fatalf("打出后应轮到座位 1，实得 %d", snap.Turn)
	}
	if len(snap.Discards[0]) != 1 || snap.Discards[0][0] != tile {
		t.Fatalf("座位 0 的牌河应为 [%v]，实得 %v", tile, snap.Discards[0])
	}
	if got := len(snap.Hand); got != HandSize {
		t.Fatalf("打出后手牌应回到 %d 张，实得 %d", HandSize, got)
	}
}
