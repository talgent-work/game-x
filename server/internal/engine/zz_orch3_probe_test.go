package engine

import "testing"

func TestZZOrchSeatRangePanics(t *testing.T) {
	// ViewFor with out-of-range seat
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("PANIC ViewFor(99): %v", r)
			}
		}()
		_ = NewGame(1, 0).ViewFor(99)
	}()
	// Actions with out-of-range seat during claim window
	hands := [Seats][]Tile{
		{wan(3), wan(1), wan(1), wan(5), wan(5), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(2), wan(4), wan(6), wan(6), wan(6), tong(7), tong(8), tong(9), tiao(1), tiao(2), tiao(3), tiao(5), tiao(9)},
		{wan(7), wan(7), wan(2), wan(3), wan(4), tong(1), tong(2), tong(3), tong(4), tong(5), tong(6), tiao(9), tiao(9)},
		{wan(9), wan(9), wan(9), tong(5), tong(6), tong(7), tong(8), tong(9), tiao(1), tiao(2), tiao(3), tiao(4), tiao(4)},
	}
	g := fixedGame(hands, bigWall(), 0)
	if _, err := g.Apply(0, Action{Kind: ActDiscard, Tile: wan(3)}); err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("PANIC Actions(99) in claim window: %v", r)
			}
		}()
		_ = g.Actions(99)
	}()
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("PANIC Apply(99, ActPass) in claim window: %v", r)
			}
		}()
		_, err := g.Apply(99, Action{Kind: ActPass})
		t.Logf("Apply(99, Pass) err: %v", err)
	}()
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("PANIC Apply(-1, ActPass) in claim window: %v", r)
			}
		}()
		_, err := g.Apply(-1, Action{Kind: ActPass})
		t.Logf("Apply(-1, Pass) err: %v", err)
	}()
}
