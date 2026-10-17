// Package engine 是最小实证用的"纯逻辑层"骨架：
// 不依赖界面、不依赖网络，可被本地原型（本机进程）与未来的权威服务端直接复用。
//
// 本包只演示结构：牌型、牌墙、发牌、摸打与"按座位出视图"。
// 吃碰杠、胡牌/流局判定属 GX-2，番种计分属 GX-3，不在本实证范围。
package engine

import "fmt"

// Suit 花色：万、筒、条、风（东南西北）、箭（中发白）。
type Suit uint8

const (
	SuitWan  Suit = iota // 万（rank 1-9）
	SuitTong             // 筒（rank 1-9）
	SuitTiao             // 条（rank 1-9）
	SuitFeng             // 风（rank 1-4 = 东南西北）
	SuitJian             // 箭（rank 1-3 = 中发白）
)

// Tile 一张牌。
type Tile struct {
	Suit Suit  `json:"suit"`
	Rank uint8 `json:"rank"`
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
