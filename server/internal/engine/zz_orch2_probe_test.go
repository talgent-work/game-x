package engine

import "testing"

// Can a player kong (concealed or promote) in the same turn they chi'd/ponged (no draw between)?
func TestZZOrchKongAfterMeld(t *testing.T) {
	// seat 1 chis 3万 from seat 0 (2万+4万 in hand), and holds 4x 9筒 for a concealed kong.
	hands := [Seats][]Tile{
		{wan(3), wan(1), wan(1), wan(5), wan(5), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(2), wan(4), tong(9), tong(9), tong(9), tong(9), wan(6), wan(7), wan(8), tiao(1), tiao(2), tiao(3), tong(7)},
		{wan(7), wan(7), wan(2), wan(3), wan(4), tong(1), tong(2), tong(3), tong(4), tong(5), tong(6), tiao(9), tiao(9)},
		{wan(9), wan(9), wan(9), tong(5), tong(6), tong(7), tong(8), tong(9), tiao(1), tiao(2), tiao(3), tiao(4), tiao(4)},
	}
	g := fixedGame(hands, bigWall(), 0)
	if _, err := g.Apply(0, Action{Kind: ActDiscard, Tile: wan(3)}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Apply(1, Action{Kind: ActChi, With: []Tile{wan(2), wan(4)}}); err != nil {
		t.Fatal(err)
	}
	acts := g.Actions(1)
	kongable := false
	for _, a := range acts {
		if a.Kind == ActKongConcealed {
			kongable = true
		}
	}
	t.Logf("immediately after chi, concealed kong offered: %v", kongable)
	if kongable {
		evs, err := g.Apply(1, Action{Kind: ActKongConcealed, Tile: tong(9)})
		t.Logf("chi-then-kong accepted: err=%v events=%d", err, len(evs))
	}
}

// Pong then immediately promote-kong the just-ponged set (no draw between).
func TestZZOrchPromoteAfterPong(t *testing.T) {
	// seat 1 holds 2x 5万; seat 0 discards 5万; seat 1 pongs; seat 1 kept... needs 4th in hand:
	// give seat 1 three 5万? pong uses 2 from hand + discard. For promote we need a 4th 5万 in hand after pong.
	hands := [Seats][]Tile{
		{wan(5), wan(1), wan(1), wan(2), wan(3), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(5), wan(5), wan(5), wan(9), wan(9), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tong(7), tong(8)},
		{wan(9), wan(9), wan(2), wan(3), wan(4), tong(1), tong(2), tong(3), tiao(5), tiao(5), tiao(5), tong(9), tiao(9)},
		{wan(6), wan(7), wan(8), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tong(4), tong(4), tong(4), tiao(9)},
	}
	// wall has one tile: 5万 at tail
	g := fixedGame(hands, []Tile{tong(6), wan(5)}, 0)
	if _, err := g.Apply(0, Action{Kind: ActDiscard, Tile: wan(5)}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Apply(1, Action{Kind: ActPong}); err != nil {
		t.Fatal(err)
	}
	acts := g.Actions(1)
	promote := false
	for _, a := range acts {
		if a.Kind == ActKongPromote {
			promote = true
		}
	}
	t.Logf("immediately after pong, promote-kong offered: %v", promote)
	if promote {
		evs, err := g.Apply(1, Action{Kind: ActKongPromote, Tile: wan(5)})
		t.Logf("pong-then-promote accepted: err=%v events=%d", err, len(evs))
	}
}

// Kong when wall is empty: engine.md ruling says forbidden; engine-rules.md §5.3 says kong proceeds without replacement.
func TestZZOrchKongEmptyWall(t *testing.T) {
	hands := [Seats][]Tile{
		{wan(1), wan(1), wan(1), wan(1), wan(2), wan(3), wan(4), tong(5), tong(6), tong(7), tiao(5), tiao(6), tiao(7), tiao(9)},
		{wan(2), wan(3), wan(4), wan(5), wan(6), wan(7), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(9)},
		{wan(5), wan(6), wan(7), wan(8), wan(9), tong(4), tong(5), tong(6), tiao(4), tiao(5), tiao(6), tong(9), tiao(9)},
		{wan(8), wan(9), tong(1), tong(2), tong(3), tong(4), tong(5), tong(6), tong(7), tong(8), tong(9), tiao(9), wan(9)},
	}
	g := fixedGame(hands, []Tile{}, 0) // empty wall
	_, err := g.Apply(0, Action{Kind: ActKongConcealed, Tile: wan(1)})
	t.Logf("concealed kong with empty wall: err=%v", err)
}

// Player draws last wall tile, wall becomes empty, discards; can the NEXT player pong (no draw needed)?
func TestZZOrchPongAfterWallEmpty(t *testing.T) {
	hands := [Seats][]Tile{
		{wan(5), wan(1), wan(1), wan(2), wan(3), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(5), wan(5), wan(9), wan(9), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tong(7), tong(8), tong(9)},
		{wan(9), wan(9), wan(2), wan(3), wan(4), tong(1), tong(2), tong(3), tiao(5), tiao(5), tiao(5), tong(9), tiao(9)},
		{wan(6), wan(7), wan(8), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tong(4), tong(4), tong(4), tiao(9)},
	}
	g := fixedGame(hands, []Tile{}, 0) // empty wall already
	if _, err := g.Apply(0, Action{Kind: ActDiscard, Tile: wan(5)}); err != nil {
		t.Fatal(err)
	}
	_, err := g.Apply(1, Action{Kind: ActPong})
	t.Logf("pong with empty wall: err=%v", err)
	// after pong, seat 1 discards; window; nobody claims → exhaustive?
	if err == nil {
		if _, err := g.Apply(1, Action{Kind: ActDiscard, Tile: tong(9)}); err != nil {
			t.Logf("seat1 discard after pong err: %v", err)
		}
		t.Logf("after all pass: phase=%v over=%v exhaustive=%v", g.Phase(), g.Over(), g.Result() != nil && g.Result().Exhaustive)
	}
}
