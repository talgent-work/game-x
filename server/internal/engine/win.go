package engine

// 和牌牌型分解（胡牌判定的核心）。
//
// 国标认可的和牌牌型（规则依据与裁定见 docs/engine-rules.md）：
//   1. 标准型：4 副顺子/刻子（吃碰杠副子各充当 1 副）＋1 对将；
//   2. 组合龙型：组合龙（三花色 147/258/369 各一组、不能错位，计 3 副顺子）
//      ＋1 副面子＋1 对将（按 2006 竞赛规则问答的通行裁定，可配至多 1 副吃碰杠副子）；
//   3. 七对（须门清；4 张同值可当两对）；
//   4. 全不靠（须门清）。
//
// 引擎枚举全部合法分解交给计分（GX-3）取最优；计分不在本包。

// SetKind 分解中的一副面子种类。
type SetKind uint8

const (
	SetChow    SetKind = iota // 顺子
	SetPung                   // 刻子
	SetKnitted                // 组合龙（9 张，计 3 副顺子）
)

// Set 分解中的一副面子。
type Set struct {
	Kind  SetKind `json:"kind"`
	Tiles []Tile  `json:"tiles"`
}

// WinShape 和牌牌型。
type WinShape uint8

const (
	ShapeStandard   WinShape = iota // 4 副面子＋将
	ShapeKnitted                    // 组合龙＋1 副面子＋将
	ShapeSevenPairs                 // 七对
	ShapeAllSingles                 // 全不靠
)

// WinHand 一次合法的和牌分解。Sets 只含暗牌部分分解出的副子；
// 吃碰杠副子由玩家自己的副子列表充当，计分层（GX-3）自行组合。
type WinHand struct {
	Shape   WinShape `json:"shape"`
	Sets    []Set    `json:"sets,omitempty"`    // 暗牌分解出的副子；组合龙为一副 SetKnitted
	Pair    *Tile    `json:"pair,omitempty"`    // 将（七对/全不靠为空）
	Pairs   []Tile   `json:"pairs,omitempty"`   // 七对的 7 个对子牌
	Singles []Tile   `json:"singles,omitempty"` // 全不靠的 14 张单牌
}

// --- 存在性快查（声明/吃碰抢校验用：只判「能否和」，不枚举分解） ---

// winExists 判定 tiles（＝暗牌＋胡牌的那张，共 14-3×melds 张）能否构成和牌牌型。
func winExists(tiles []Tile, melds int) bool {
	if melds < 0 || melds > 4 || len(tiles) != 14-3*melds {
		return false
	}
	for _, t := range tiles {
		if !t.Valid() {
			return false
		}
	}
	counts := countsOf(tiles)
	if melds == 0 {
		if ok, _ := sevenPairs(counts); ok {
			return true
		}
		if ok, _ := allSingles(counts); ok {
			return true
		}
	}
	if melds <= 1 {
		for _, perm := range knittedPerms {
			knitted, ok := takeKnitted(&counts, perm)
			if !ok {
				continue
			}
			exists := setsPlusPairExists(&counts, 1-melds)
			restoreKnitted(&counts, knitted)
			if exists {
				return true
			}
		}
	}
	return setsPlusPairExists(&counts, 4-melds)
}

// takeKnitted 按排列 perm 取走一组组合龙（不足则原样返回 false）。
func takeKnitted(counts *[34]int, perm [3]int) ([]Tile, bool) {
	var knitted []Tile
	for s := 0; s < NumericSuitCount; s++ {
		for _, r := range knittedGroupRanks[perm[s]] {
			o := tileFromSuitRank(Suit(s), r).ord()
			if counts[o] < 1 {
				restoreKnitted(counts, knitted)
				return nil, false
			}
			knitted = append(knitted, tileFromOrd(o))
			counts[o]--
		}
	}
	return knitted, true
}

func restoreKnitted(counts *[34]int, knitted []Tile) {
	for _, t := range knitted {
		counts[t.ord()]++
	}
}

// setsPlusPairExists 存在性版：counts 的全部牌分为 nSets 副顺/刻子＋1 对将。
func setsPlusPairExists(counts *[34]int, nSets int) bool {
	for p := 0; p < 34; p++ {
		if counts[p] >= 2 {
			counts[p] -= 2
			if decomposeSetsExist(counts, nSets) {
				counts[p] += 2
				return true
			}
			counts[p] += 2
		}
	}
	return false
}

// decomposeSetsExist 存在性版 decomposeSets（约定同：最小槽位先取）。
func decomposeSetsExist(counts *[34]int, n int) bool {
	if n == 0 {
		return !anyLeft(*counts)
	}
	low := lowestSlot(*counts)
	if low < 0 {
		return false
	}
	if counts[low] >= 3 {
		counts[low] -= 3
		if decomposeSetsExist(counts, n-1) {
			counts[low] += 3
			return true
		}
		counts[low] += 3
	}
	t := tileFromOrd(low)
	if t.Numeric() && t.Rank <= 7 && counts[low+1] > 0 && counts[low+2] > 0 {
		counts[low]--
		counts[low+1]--
		counts[low+2]--
		if decomposeSetsExist(counts, n-1) {
			counts[low]++
			counts[low+1]++
			counts[low+2]++
			return true
		}
		counts[low]++
		counts[low+1]++
		counts[low+2]++
	}
	return false
}

// --- 槽位工具（34 个牌值槽：万 0-8、筒 9-17、条 18-26、风 27-30、箭 31-33） ---

func tileFromOrd(o int) Tile {
	for s := Suit(0); int(s) < len(suitStart)-1; s++ {
		if o >= suitStart[s] && o < suitStart[s+1] {
			return Tile{Suit: s, Rank: uint8(o - suitStart[s] + 1)}
		}
	}
	return Tile{} // 不可达：o 应在 [0,34)
}

func tileFromSuitRank(s Suit, r uint8) Tile { return Tile{Suit: s, Rank: r} }

func tripOfOrd(o int) []Tile {
	t := tileFromOrd(o)
	return []Tile{t, t, t}
}

func lowestSlot(c [34]int) int {
	for o, n := range c {
		if n > 0 {
			return o
		}
	}
	return -1
}

func anyLeft(c [34]int) bool { return lowestSlot(c) >= 0 }

// --- 组合龙 ---

// knittedGroupRanks 组合龙的三组点数：147 / 258 / 369。
var knittedGroupRanks = [3][3]uint8{{1, 4, 7}, {2, 5, 8}, {3, 6, 9}}

// knittedPerms 数牌花色（万=0 筒=1 条=2）到组号（0=147，1=258，2=369）的全部双射，
// 即「三种花色的 147、258、369 不能错位」的全部合法排列。
var knittedPerms = [6][3]int{
	{0, 1, 2}, {0, 2, 1},
	{1, 0, 2}, {1, 2, 0},
	{2, 0, 1}, {2, 1, 0},
}

// --- 分解入口 ---

// Decompose 枚举 tiles（＝暗牌＋胡牌的那张，共 14-3×melds 张）的全部合法和牌分解。
// melds 为该玩家已亮出的吃/碰/杠副子数（每副充当 1 副面子）。无解返回 nil。
func Decompose(tiles []Tile, melds int) []WinHand {
	if melds < 0 || melds > 4 || len(tiles) != 14-3*melds {
		return nil
	}
	for _, t := range tiles {
		if !t.Valid() {
			return nil
		}
	}
	counts := countsOf(tiles)

	var out []WinHand
	if melds == 0 {
		if ok, wh := sevenPairs(counts); ok {
			out = append(out, wh)
		}
		if ok, wh := allSingles(counts); ok {
			out = append(out, wh)
		}
	}
	if melds <= 1 {
		out = append(out, knittedHands(counts, melds)...)
	}
	out = append(out, setsPlusPairHands(&counts, 4-melds)...)
	return out
}

// --- 标准型（含组合龙型的剩余部分共用） ---

// setsPlusPairHands 枚举：counts 的全部牌分为恰好 nSets 副顺/刻子＋1 对将。
// 递归中会改动 counts 并恢复。
func setsPlusPairHands(counts *[34]int, nSets int) []WinHand {
	var out []WinHand
	for p := 0; p < 34; p++ {
		if counts[p] >= 2 {
			counts[p] -= 2
			for _, sets := range decomposeSets(counts, nSets) {
				pair := tileFromOrd(p)
				out = append(out, WinHand{Shape: ShapeStandard, Sets: sets, Pair: &pair})
			}
			counts[p] += 2
		}
	}
	return out
}

// decomposeSets 把 counts 的剩余牌分解为恰好 n 副顺子/刻子，枚举全部分法。
// 约定：每步取「现存最小槽位」的牌——含它的刻子即同值三张，含它的顺子必以它为
// 最低张（更小的槽位已空），因此枚举完备且无重复。
func decomposeSets(counts *[34]int, n int) [][]Set {
	if n == 0 {
		if anyLeft(*counts) {
			return nil
		}
		return [][]Set{nil} // 一种（空）分解
	}
	low := lowestSlot(*counts)
	if low < 0 {
		return nil
	}
	var out [][]Set
	if counts[low] >= 3 { // 刻子
		counts[low] -= 3
		for _, rest := range decomposeSets(counts, n-1) {
			out = append(out, append([]Set{{Kind: SetPung, Tiles: tripOfOrd(low)}}, rest...))
		}
		counts[low] += 3
	}
	t := tileFromOrd(low)
	if t.Numeric() && t.Rank <= 7 && counts[low+1] > 0 && counts[low+2] > 0 { // 顺子
		counts[low]--
		counts[low+1]--
		counts[low+2]--
		for _, rest := range decomposeSets(counts, n-1) {
			out = append(out, append([]Set{{Kind: SetChow, Tiles: []Tile{t, tileFromOrd(low + 1), tileFromOrd(low + 2)}}}, rest...))
		}
		counts[low]++
		counts[low+1]++
		counts[low+2]++
	}
	return out
}

// --- 组合龙型 ---

// knittedHands 枚举组合龙型：9 张组合龙＋(1-melds) 副顺/刻子＋将。
// 前提：melds ∈ {0,1}。
func knittedHands(counts [34]int, melds int) []WinHand {
	var out []WinHand
	for _, perm := range knittedPerms {
		knitted, ok := takeKnitted(&counts, perm)
		if !ok {
			continue
		}
		knittedSet := Set{Kind: SetKnitted, Tiles: knitted}
		for _, wh := range setsPlusPairHands(&counts, 1-melds) {
			out = append(out, WinHand{
				Shape: ShapeKnitted,
				Sets:  append([]Set{knittedSet}, wh.Sets...),
				Pair:  wh.Pair,
			})
		}
		restoreKnitted(&counts, knitted)
	}
	return out
}

// --- 七对（须门清） ---

func sevenPairs(counts [34]int) (bool, WinHand) {
	pairs := 0
	var pairTiles []Tile
	for o, n := range counts {
		if n%2 != 0 {
			return false, WinHand{}
		}
		if n == 4 { // 4 张同值计两对（见 docs/engine-rules.md 裁定）
			pairTiles = append(pairTiles, tileFromOrd(o), tileFromOrd(o))
			pairs += 2
		} else if n == 2 {
			pairTiles = append(pairTiles, tileFromOrd(o))
			pairs++
		}
	}
	if pairs != 7 {
		return false, WinHand{}
	}
	return true, WinHand{Shape: ShapeSevenPairs, Pairs: pairTiles}
}

// --- 全不靠（须门清） ---

// allSingles 判定「全不靠」：14 张互不成副的单牌；数牌须满足三花色 147/258/369
// 的不错位排列（各花色点数落在分派给它的组内），字牌不重复。
func allSingles(counts [34]int) (bool, WinHand) {
	total := 0
	for _, n := range counts {
		if n > 1 { // 单张：任何牌值不得重复
			return false, WinHand{}
		}
		total += n
	}
	if total != 14 {
		return false, WinHand{}
	}
	for _, perm := range knittedPerms {
		ok := true
		for s := 0; s < NumericSuitCount && ok; s++ {
			for r := uint8(1); r <= 9; r++ {
				o := tileFromSuitRank(Suit(s), r).ord()
				if counts[o] == 0 {
					continue
				}
				inGroup := false
				for _, gr := range knittedGroupRanks[perm[s]] {
					if r == gr {
						inGroup = true
						break
					}
				}
				if !inGroup {
					ok = false
					break
				}
			}
		}
		if ok {
			singles := make([]Tile, 0, 14)
			for o := 0; o < 34; o++ {
				if counts[o] > 0 {
					singles = append(singles, tileFromOrd(o))
				}
			}
			return true, WinHand{Shape: ShapeAllSingles, Singles: singles}
		}
	}
	return false, WinHand{}
}
