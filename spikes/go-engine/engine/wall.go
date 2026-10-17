package engine

import (
	"math/rand"
	"sort"
)

const (
	Seats      = 4   // 四人一桌
	HandSize   = 13  // 起手手牌
	TotalTiles = 136 // 国标麻将全牌数：万/筒/条 各 1-9 四张 + 东南西北各四张 + 中发白各四张
)

// NewWall 构造并洗好一副牌墙（136 张）。
func NewWall(r *rand.Rand) []Tile {
	wall := make([]Tile, 0, TotalTiles)
	for _, s := range []Suit{SuitWan, SuitTong, SuitTiao} {
		for rank := uint8(1); rank <= 9; rank++ {
			for i := 0; i < 4; i++ {
				wall = append(wall, Tile{Suit: s, Rank: rank})
			}
		}
	}
	for rank := uint8(1); rank <= 4; rank++ { // 东南西北
		for i := 0; i < 4; i++ {
			wall = append(wall, Tile{Suit: SuitFeng, Rank: rank})
		}
	}
	for rank := uint8(1); rank <= 3; rank++ { // 中发白
		for i := 0; i < 4; i++ {
			wall = append(wall, Tile{Suit: SuitJian, Rank: rank})
		}
	}
	r.Shuffle(len(wall), func(i, j int) { wall[i], wall[j] = wall[j], wall[i] })
	return wall
}

// sortTiles 按花色、点数排序（展示与测试友好）。
func sortTiles(ts []Tile) {
	sort.Slice(ts, func(i, j int) bool {
		if ts[i].Suit != ts[j].Suit {
			return ts[i].Suit < ts[j].Suit
		}
		return ts[i].Rank < ts[j].Rank
	})
}
