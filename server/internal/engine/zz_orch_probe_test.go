package engine

import "testing"

func TestZZOrchOrphans(t *testing.T) {
	orphans := tiles(
		wan(1), wan(9), tong(1), tong(9), tiao(1), tiao(9),
		feng(1), feng(2), feng(3), feng(4), jian(1), jian(2), jian(3),
		wan(1),
	)
	t.Logf("Decompose(13 orphans) = %d decompositions", len(Decompose(orphans, 0)))
}

func TestZZOrchInvalidTilePanic(t *testing.T) {
	g := NewGame(42, 0)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("PANIC KongConcealed rank=0: %v", r)
			}
		}()
		_, err := g.Apply(0, Action{Kind: ActKongConcealed, Tile: Tile{Suit: SuitWan, Rank: 0}})
		t.Logf("KongConcealed rank=0 err: %v", err)
	}()
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("PANIC KongConcealed suit=99: %v", r)
			}
		}()
		_, err := g.Apply(0, Action{Kind: ActKongConcealed, Tile: Tile{Suit: 99, Rank: 1}})
		t.Logf("KongConcealed suit=99 err: %v", err)
	}()
	// also invalid rank in promote path and discard path for comparison
	_, err := g.Apply(0, Action{Kind: ActKongPromote, Tile: Tile{Suit: 99, Rank: 1}})
	t.Logf("KongPromote suit=99 err: %v", err)
	_, err = g.Apply(0, Action{Kind: ActDiscard, Tile: Tile{Suit: 99, Rank: 1}})
	t.Logf("Discard suit=99 err: %v", err)
}

func TestZZOrchSnapshotAlias(t *testing.T) {
	// seat 1 pongs from seat 0, then check snapshot meld tile aliasing
	hands := [Seats][]Tile{
		{wan(5), wan(1), wan(1), wan(2), wan(3), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tiao(5), tong(9)},
		{wan(5), wan(5), wan(9), wan(9), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tiao(4), tong(7), tong(8)},
		{wan(9), wan(9), wan(2), wan(3), wan(4), tong(1), tong(2), tong(3), tiao(5), tiao(5), tiao(5), tong(9), tiao(9)},
		{wan(6), wan(7), wan(8), tong(1), tong(2), tong(3), tiao(1), tiao(2), tiao(3), tong(4), tong(4), tong(4), tiao(9)},
	}
	g := fixedGame(hands, bigWall(), 0)
	if _, err := g.Apply(0, Action{Kind: ActDiscard, Tile: wan(5)}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Apply(1, Action{Kind: ActPong}); err != nil {
		t.Fatal(err)
	}
	snap := g.ViewFor(2)
	m := snap.Melds[1][0]
	if len(m.Tiles) != 3 {
		t.Fatalf("meld tiles %d", len(m.Tiles))
	}
	m.Tiles[0] = wan(9) // caller mutates the snapshot
	inner := g.melds[1][0].Tiles[0]
	t.Logf("after mutating snapshot meld tile, engine meld tile = %v (aliased if not wan5)", inner)
}
