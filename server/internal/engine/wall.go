package engine

import "math/rand"

// wall 活牌墙。
//
// 国标不保留王牌（无宝牌区）：牌墙抓完仍无人和牌即流局。
// 摸牌从牌头（正向）抓；杠的补牌从牌尾抓（国标规则「杠后从牌墙尾部补牌」，
// 参见 docs/engine.md 裁定记录）。
type wall struct {
	tiles []Tile
}

// newWall 构造并洗好一副 136 张牌。
func newWall(r *rand.Rand) wall {
	wallTiles := make([]Tile, 0, TotalTiles)
	for _, s := range []Suit{SuitWan, SuitTong, SuitTiao} {
		for rank := uint8(1); rank <= 9; rank++ {
			for i := 0; i < 4; i++ {
				wallTiles = append(wallTiles, Tile{Suit: s, Rank: rank})
			}
		}
	}
	for rank := uint8(1); rank <= 4; rank++ { // 东南西北
		for i := 0; i < 4; i++ {
			wallTiles = append(wallTiles, Tile{Suit: SuitFeng, Rank: rank})
		}
	}
	for rank := uint8(1); rank <= 3; rank++ { // 中发白
		for i := 0; i < 4; i++ {
			wallTiles = append(wallTiles, Tile{Suit: SuitJian, Rank: rank})
		}
	}
	r.Shuffle(len(wallTiles), func(i, j int) { wallTiles[i], wallTiles[j] = wallTiles[j], wallTiles[i] })
	return wall{tiles: wallTiles}
}

// remaining 牌墙剩余张数。
func (w *wall) remaining() int { return len(w.tiles) }

// draw 从牌头摸一张（普通摸牌）。空墙返回 false。
func (w *wall) draw() (Tile, bool) {
	if len(w.tiles) == 0 {
		return Tile{}, false
	}
	t := w.tiles[0]
	w.tiles = w.tiles[1:]
	return t, true
}

// drawN 从牌头连续摸 n 张（发牌用），不足 n 张时只摸到空为止。
func (w *wall) drawN(n int) []Tile {
	if n > len(w.tiles) {
		n = len(w.tiles)
	}
	out := make([]Tile, n)
	copy(out, w.tiles[:n])
	w.tiles = w.tiles[n:]
	return out
}

// drawTail 从牌尾摸一张（杠后补牌）。空墙返回 false。
func (w *wall) drawTail() (Tile, bool) {
	if len(w.tiles) == 0 {
		return Tile{}, false
	}
	last := len(w.tiles) - 1
	t := w.tiles[last]
	w.tiles = w.tiles[:last]
	return t, true
}
