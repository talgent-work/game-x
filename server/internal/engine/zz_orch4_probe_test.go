package engine

import "testing"

// 恶意/越界动作探测（回归）：吃碰杠声明必须按牌值严格校验，任何构造输入
// 不得被接受、不得引发 panic。源于一次外部探测发现跨花色吃可通过校验的缺陷。

func TestZZOrchCrossSuitChi(t *testing.T) {
	// seat 0 (dealer) discards 5筒; seat 1 (next) holds a legit chi (4筒+6筒) so the window stays open,
	// then submits a bogus cross-suit chi with 6条+7条 instead.
	hands := [Seats][]Tile{
		{wan(1), wan(1), wan(2), wan(3), wan(5), tong(5), tong(6), tong(7), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{tong(4), tong(6), tiao(6), tiao(7), wan(4), wan(5), wan(6), wan(7), wan(8), wan(9), tong(1), tong(2), tong(3)},
		{wan(2), wan(3), wan(4), tong(1), tong(2), tong(3), tiao(4), tiao(5), tiao(6), tiao(7), tiao(8), tiao(9), wan(9)},
		{wan(6), wan(7), wan(8), tong(4), tong(5), tong(6), tiao(1), tiao(2), tiao(3), tong(7), tong(8), tong(9), wan(9)},
	}
	g := fixedGame(hands, bigWall(), 0)
	if _, err := g.Apply(0, Action{Kind: ActDiscard, Tile: tong(5)}); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseClaim {
		t.Fatalf("expected claim window, got %v", g.Phase())
	}
	// 跨花色「顺」（5筒+6条+7条）必须被拒绝。
	if _, err := g.Apply(1, Action{Kind: ActChi, With: []Tile{tiao(6), tiao(7)}}); err == nil {
		m := g.melds[1][0]
		t.Fatalf("cross-suit chi accepted, meld tiles=%v", m.Tiles)
	}
	// 合法吃（4筒+6筒）仍应成立。
	applyOk(t, g, 1, Action{Kind: ActChi, With: []Tile{tong(4), tong(6)}})
	if m := g.melds[1][0]; m.Kind != MeldChi || m.Tiles[0] != tong(4) {
		t.Fatalf("legit chi should settle, got %+v", m)
	}

	// honor "chi": discard 1万; seat 1 holds 2万+3万 so the window stays open; submits 南+西
	// （1万+南+西 的「点数」恰为 1、2、3，同样必须被拒绝）。
	hands2 := [Seats][]Tile{
		{wan(1), wan(1), wan(1), wan(2), wan(3), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(2), wan(3), feng(2), feng(3), wan(4), wan(5), wan(6), wan(7), wan(8), wan(9), tong(1), tong(2), tong(3)},
		{wan(3), wan(3), wan(4), tong(1), tong(2), tong(3), tiao(4), tiao(5), tiao(6), tiao(7), tiao(8), tiao(9), wan(9)},
		{wan(6), wan(7), wan(8), tong(4), tong(5), tong(6), tiao(1), tiao(2), tiao(3), tong(7), tong(8), tong(9), wan(9)},
	}
	g2 := fixedGame(hands2, bigWall(), 0)
	if _, err := g2.Apply(0, Action{Kind: ActDiscard, Tile: wan(1)}); err != nil {
		t.Fatal(err)
	}
	if g2.Phase() != PhaseClaim {
		t.Fatalf("expected claim window, got %v", g2.Phase())
	}
	if _, err := g2.Apply(1, Action{Kind: ActChi, With: []Tile{feng(2), feng(3)}}); err == nil {
		t.Fatalf("honor chi accepted, meld tiles=%v", g2.melds[1][0].Tiles)
	}
}

func TestZZMalformedActions(t *testing.T) {
	g := NewGame(7, 0)
	// 越界座位不得 panic，一律拒绝/返回空。
	applyErr(t, g, -1, Action{Kind: ActDiscard, Tile: wan(1)})
	applyErr(t, g, Seats, Action{Kind: ActDiscard, Tile: wan(1)})
	if acts := g.Actions(-1); acts != nil {
		t.Fatalf("越界座位动作清单应为空")
	}
	if snap := g.ViewFor(9); snap.Seat != 0 || snap.Hand != nil {
		t.Fatalf("越界座位快照应为零值")
	}
	// 非法牌值：不得 panic。
	applyErr(t, g, 0, Action{Kind: ActKongConcealed, Tile: Tile{Suit: SuitWan, Rank: 0}})
	applyErr(t, g, 0, Action{Kind: ActKongConcealed, Tile: Tile{Suit: 99, Rank: 1}})
	applyErr(t, g, 0, Action{Kind: ActKongPromote, Tile: Tile{Suit: SuitTong, Rank: 10}})
	applyErr(t, g, 0, Action{Kind: ActDiscard, Tile: Tile{Suit: SuitJian, Rank: 4}})
	// 未知动作种类。
	applyErr(t, g, 0, Action{Kind: ActionKind(200)})
	// 吃声明带非法 With 数量/内容（窗口外直接拒）。
	applyErr(t, g, 0, Action{Kind: ActChi})
	applyErr(t, g, 0, Action{Kind: ActChi, With: []Tile{wan(1)}})
	applyErr(t, g, 0, Action{Kind: ActChi, With: []Tile{wan(1), wan(2), wan(3)}})
}
