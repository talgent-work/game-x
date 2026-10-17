// Package engine 实现国标麻将（GB /《中国麻将竞赛规则》1998 及其通行裁定）
// 一局的完整纯逻辑状态机：发牌、摸打、吃碰杠、胡牌/流局判定。
//
// 设计约束（依 GX-1《技术选型报告》§5.2）：
//   - 零网络、零界面依赖；完整对局状态只在包内部；
//   - 对外唯一读取通道是 ViewFor(seat) Snapshot，Snapshot 的 JSON 形状即协议载荷，
//     「隐藏他人手牌」由类型收口（他人暗杠牌面同样不出视图）；
//   - 上层（本地原型 / 权威服务端）只以 Apply 提交动作意图，
//     引擎校验座位、时机与合法性，回事件或拒绝。
//
// 规则依据与歧义裁定见仓库 docs/engine.md。
package engine

import "fmt"

// 基本常量：四人一桌，起手 13 张，国标 136 张（万/筒/条 1-9、东南西北、中发白，各四张，无花牌）。
const (
	Seats      = 4
	HandSize   = 13
	TotalTiles = 136
)

// Suit 花色：万、筒、条、风（东南西北）、箭（中发白）。
// 前三种为数牌花色，序数值恰好为 0/1/2，与组合龙、全不靠的花色枚举对应。
type Suit uint8

const (
	SuitWan  Suit = iota // 万（rank 1-9）
	SuitTong             // 筒（rank 1-9）
	SuitTiao             // 条（rank 1-9）
	SuitFeng             // 风（rank 1-4 = 东南西北）
	SuitJian             // 箭（rank 1-3 = 中发白）
)

// 数牌花色种数（万/筒/条）。
const NumericSuitCount = 3

// rankMax 每种花色的最大点数：万/筒/条 9，风 4，箭 3。
var rankMax = [5]uint8{9, 9, 9, 4, 3}

// Tile 一张牌。
type Tile struct {
	Suit Suit  `json:"suit"`
	Rank uint8 `json:"rank"`
}

// Valid 牌值是否合法。
func (t Tile) Valid() bool {
	return int(t.Suit) < len(rankMax) && t.Rank >= 1 && t.Rank <= rankMax[t.Suit]
}

// Numeric 是否数牌（万/筒/条）。
func (t Tile) Numeric() bool { return t.Suit < NumericSuitCount }

// suitStart 各花色在 34 个牌值槽位中的起点：万 0-8、筒 9-17、条 18-26、风 27-30、箭 31-33。
// 末位 34 为终点哨兵：tileFromOrd 据此覆盖含箭牌在内的全部 34 个槽位。
var suitStart = [6]int{0, 9, 18, 27, 31, 34}

// ord 牌值槽位索引（0-33），全库直方图与顺序比较都用它。
func (t Tile) ord() int { return suitStart[t.Suit] + int(t.Rank) - 1 }

// Less 排序序：先花色后点数。
func (t Tile) Less(u Tile) bool {
	if t.Suit != u.Suit {
		return t.Suit < u.Suit
	}
	return t.Rank < u.Rank
}

// String 例：3万、7筒、1条、东、发。
func (t Tile) String() string {
	switch t.Suit {
	case SuitWan:
		return fmt.Sprintf("%d万", t.Rank)
	case SuitTong:
		return fmt.Sprintf("%d筒", t.Rank)
	case SuitTiao:
		return fmt.Sprintf("%d条", t.Rank)
	case SuitFeng:
		return [...]string{"东", "南", "西", "北"}[t.Rank-1]
	case SuitJian:
		return [...]string{"中", "发", "白"}[t.Rank-1]
	}
	return fmt.Sprintf("?(%d,%d)", t.Suit, t.Rank)
}

// countsOf 统计牌值直方图（34 槽）。
func countsOf(tiles []Tile) [34]int {
	var c [34]int
	for _, t := range tiles {
		c[t.ord()]++
	}
	return c
}

// sortTiles 原地排序（花色、点数）。
func sortTiles(tiles []Tile) {
	// 插入排序：手牌保持小规模有序，插入单张时成本 O(n)。
	for i := 1; i < len(tiles); i++ {
		t := tiles[i]
		j := i - 1
		for j >= 0 && t.Less(tiles[j]) {
			tiles[j+1] = tiles[j]
			j--
		}
		tiles[j+1] = t
	}
}

// insertSorted 把 t 插入 tiles 并保持有序。
func insertSorted(tiles []Tile, t Tile) []Tile {
	i := len(tiles)
	for tiles = append(tiles, t); i > 0 && t.Less(tiles[i-1]); i-- {
		tiles[i] = tiles[i-1]
	}
	tiles[i] = t
	return tiles
}

// removeTile 从 tiles 中去掉一张与 t 同值的牌（调用方保证存在）。
func removeTile(tiles []Tile, t Tile) []Tile {
	for i, x := range tiles {
		if x == t {
			return append(tiles[:i:i], tiles[i+1:]...)
		}
	}
	return tiles
}
