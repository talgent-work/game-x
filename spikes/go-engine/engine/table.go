package engine

import (
	"fmt"
	"math/rand"
)

// Snapshot 是"某个座位可见"的视图——也正是将来经协议发给该客户端的载荷形状。
// 关键点（服务器权威结构的雏形）：视图里没有别家手牌的位置，
// 隐藏信息只存在于 Table 内部，不随视图外流。
type Snapshot struct {
	Seat      int           `json:"seat"`
	Hand      []Tile        `json:"hand"`      // 仅视图持有者的手牌
	Discards  [Seats][]Tile `json:"discards"`  // 牌河：公开信息
	WallCount int           `json:"wallCount"` // 牌墙剩余：公开信息
	Turn      int           `json:"turn"`      // 当前轮到谁：公开信息
}

// Table 一局对局的最小状态骨架（完整状态只在逻辑层内部）。
type Table struct {
	wall     []Tile // 牌墙（剩余）
	hands    [Seats][]Tile
	discards [Seats][]Tile
	turn     int
}

// NewTable 发牌（四人各 13 张），余牌留作牌墙。
func NewTable(seed int64) *Table {
	r := rand.New(rand.NewSource(seed))
	t := &Table{wall: NewWall(r)}
	for s := 0; s < Seats; s++ {
		t.hands[s] = append(t.hands[s], t.drawN(HandSize)...)
		sortTiles(t.hands[s])
	}
	return t
}

func (t *Table) drawN(n int) []Tile {
	if n > len(t.wall) {
		n = len(t.wall)
	}
	out := make([]Tile, n)
	copy(out, t.wall[:n])
	t.wall = t.wall[n:]
	return out
}

// Draw 当前座位摸入一张（骨架：不含吃碰杠胡判定）。
func (t *Table) Draw() (Tile, error) {
	if len(t.wall) == 0 {
		return Tile{}, fmt.Errorf("牌墙已空")
	}
	tile := t.wall[0]
	t.wall = t.wall[1:]
	t.hands[t.turn] = append(t.hands[t.turn], tile)
	sortTiles(t.hands[t.turn])
	return tile, nil
}

// Discard 当前座位打出一张手牌，轮到下一家（骨架）。
func (t *Table) Discard(tile Tile) error {
	h := t.hands[t.turn]
	for i, x := range h {
		if x == tile {
			t.hands[t.turn] = append(h[:i:i], h[i+1:]...)
			t.discards[t.turn] = append(t.discards[t.turn], tile)
			t.turn = (t.turn + 1) % Seats
			return nil
		}
	}
	return fmt.Errorf("手牌中没有 %v", tile)
}

// ViewFor 唯一的对外读取通道：按座位生成快照。上层（本地原型 / 权威服务端）
// 只能拿到 Snapshot，拿不到别家手牌——服务器权威在类型层面就成立。
func (t *Table) ViewFor(seat int) Snapshot {
	snap := Snapshot{Seat: seat, WallCount: len(t.wall), Turn: t.turn}
	snap.Hand = append([]Tile(nil), t.hands[seat]...)
	snap.Discards = t.discards // 值拷贝（元素切片共享底层数组；骨架阶段可接受，正式实现再收口）
	return snap
}
