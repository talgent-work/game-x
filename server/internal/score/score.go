package score

import (
	"sort"

	"talgent-work/game-x/server/internal/engine"
)

// Context 计分所需的对局上下文（引擎 Result 之外的输入）。
type Context struct {
	// RoundWind 圈风（1=东、2=南、3=西、4=北）。圈风是跨局概念，由上层传入；
	// 座位风（门风）按「庄家东、下家南、对家西、上家北」由 Result.Context.Dealer 推导。
	RoundWind uint8
}

// ScoreResult 一局计分与结算的完整结果（JSON 形状供 GX-4 协议层复用）。
type ScoreResult struct {
	Exhaustive bool // 流局（引擎 Result.Exhaustive）
	// Best 取最优后的计分：胜者全部合法分解中番数最大者。
	Best struct {
		Fans  []FanHit `json:"fans"`
		Total int      `json:"total"`
	} `json:"best"`
	Eligible   bool       `json:"eligible"`   // 达到起和线（番种分≥8；无番和 8 分兜底）
	Settlement Settlement `json:"settlement"` // 四家得分（流局为零和零分）
}

// Score 对引擎终局结果计分并结算。
// result 必须为和牌终局（Exhaustive=false）时计分；流局返回流局结算。
// 胜者全部合法分解中取番数最大者；番种分≥8 或「无番和」时 Eligible。
func Score(result *engine.Result, ctx Context) ScoreResult {
	out := ScoreResult{Exhaustive: result.Exhaustive}
	if result.Exhaustive || result.Winner < 0 {
		out.Settlement = Settlement{Winner: -1, Kind: PayoutExhaustive, Feeder: -1, Base: 8}
		return out
	}

	var best FanScore
	var bestTotal = -1
	for _, wh := range result.Decompositions {
		s := scoreDecomposition(wh, result, ctx)
		if s.Total > bestTotal {
			best, bestTotal = s, s.Total
		}
	}
	out.Best.Fans = best.Fans
	out.Best.Total = best.Total
	out.Eligible = best.Total >= MinFanScore
	out.Settlement = settle(result, best.Total)
	return out
}

// MinFanScore 起和线：番种分 8 分。
const MinFanScore = 8

// BasePoints 底分：8 分（官方 MCR，Extra Points）。
const BasePoints = 8

// settle 按官方 MCR 公式结算：
//   - 点炮（含抢杠和）：放铳者付 番种分＋8 底分，其余两家各付 8 底分；
//   - 自摸：三家各付 番种分＋8 底分；
//   - 零和：胜者所得＝三家所付之和。
func settle(result *engine.Result, fanScore int) Settlement {
	s := Settlement{
		Winner:   result.Winner,
		Feeder:   result.Feeder,
		FanScore: fanScore,
		Base:     BasePoints,
	}
	switch result.Source {
	case engine.WinSelfDrawn:
		s.Kind = PayoutSelfDrawn
		s.Feeder = -1
		each := fanScore + BasePoints
		for i := 0; i < engine.Seats; i++ {
			if i == result.Winner {
				s.Deltas[i] = each * (engine.Seats - 1)
			} else {
				s.Deltas[i] = -each
			}
		}
	default: // WinByDiscard / WinByRobbedKong：均为点和口径
		s.Kind = PayoutDiscard
		for i := 0; i < engine.Seats; i++ {
			if i == result.Winner {
				continue
			}
			if i == result.Feeder {
				s.Deltas[i] = -(fanScore + BasePoints)
			} else {
				s.Deltas[i] = -BasePoints
			}
			s.Deltas[result.Winner] -= s.Deltas[i]
		}
	}
	return s
}

// scoreDecomposition 对一个合法分解计分：调用番种识别，再在冲突图中取番值最大的组合。
func scoreDecomposition(wh engine.WinHand, result *engine.Result, ctx Context) FanScore {
	fe := buildFeatures(wh, result, ctx)
	hits := fanHits(fe)
	return bestCombination(hits)
}

// --- 分解特征（番种识别的公共输入） ---

// groupedSet 分解中的一副面子（吃/碰/杠/组合龙），隐/明在分解内无关，由 meld 结构决定。
type groupedSet struct {
	kind  groupKind
	tiles []engine.Tile
}

type groupKind uint8

const (
	grpChow groupKind = iota
	grpPung
	grpKong
	grpKnitted // 组合龙（9 张，计 3 副特殊顺子）
)

// features 一个分解的全部派生特征，供番种识别函数共享。
type features struct {
	dec    engine.WinHand
	res    *engine.Result
	ctx    Context
	dealer int // 庄家座位

	concT []engine.Tile // 暗牌（含胡牌张）
	conc  [34]int       // 暗牌直方图
	full  [34]int       // 全牌（暗牌＋全部副子）直方图
	meld  [34]int       // 明牌副子直方图（不含暗杠）

	concealedGroups []groupedSet // 分解出的暗面子（组合龙为一副 grpKnitted）
	meldSets        []groupedSet // 已亮出的副子（吃/碰/明杠/补杠；暗杠不计入「明」但计入面子）

	allKongSets    []groupedSet // 全部杠（暗＋明）
	nConcealedKong int
	nMeldedKong    int

	kongs       int  // 杠数（面子数）
	pungs       int  // 刻子数（含杠，作为面子）
	chows       int  // 顺子数（含组合龙折算 3）
	allPungs    bool // 4 副面子全为刻/杠（标准型/组合龙型之外为 false）
	allChows    bool // 4 副面子全为顺子（含组合龙折算）
	knitted     bool // 含组合龙
	pureKnitted bool // 组合龙型且无吃碰杠副子

	hasHonors bool
	hasTerms  bool // 含幺九（1/9）或字牌

	fullFlush bool // 清一色：单一数牌花色，无字牌
	halfFlush bool // 混一色：单一数牌花色＋字牌
	hasSuit   [3]bool
	fiveGates bool // 五门齐：万筒条风箭齐

	lackSuits int // 缺门数（数牌中未出现者）

	winTile  engine.Tile
	winOrd   int
	winGroup int // 含胡牌张的分解组下标（-1 = 将）
}

var ordSuit = func() [34]engine.Suit {
	var a [34]engine.Suit
	for i := range a {
		a[i] = suitOfOrd(i)
	}
	return a
}()

func suitOfOrd(o int) engine.Suit {
	switch {
	case o < 9:
		return engine.SuitWan
	case o < 18:
		return engine.SuitTong
	case o < 27:
		return engine.SuitTiao
	case o < 31:
		return engine.SuitFeng
	default:
		return engine.SuitJian
	}
}

func rankOfOrd(o int) uint8 {
	switch {
	case o < 9:
		return uint8(o + 1)
	case o < 18:
		return uint8(o - 9 + 1)
	case o < 27:
		return uint8(o - 18 + 1)
	case o < 31:
		return uint8(o - 27 + 1)
	default:
		return uint8(o - 31 + 1)
	}
}

func tileOfOrd(o int) engine.Tile { return engine.Tile{Suit: suitOfOrd(o), Rank: rankOfOrd(o)} }

func ordOfTile(t engine.Tile) int {
	base := [5]int{0, 9, 18, 27, 31}
	return base[t.Suit] + int(t.Rank) - 1
}

// buildFeatures 从分解、终局结果与上下文派生全部特征。
func buildFeatures(wh engine.WinHand, result *engine.Result, ctx Context) features {
	fe := features{
		dec:      wh,
		res:      result,
		ctx:      ctx,
		dealer:   result.Context.Dealer,
		winTile:  result.WinningTile,
		winOrd:   ordOfTile(result.WinningTile),
		winGroup: -1,
	}
	fe.concT = append([]engine.Tile(nil), result.WinnerHand...)
	fe.conc = hist(fe.concT)

	// 已亮出的副子：吃/碰/杠（暗杠对「明」不贡献，但仍占面子位）——
	// 引擎 WinnerMelds 给出全部副子，KongFrom 区分明暗。
	for _, m := range result.WinnerMelds {
		tiles := append([]engine.Tile(nil), m.Tiles...)
		gk := grpChow
		switch m.Kind {
		case engine.MeldChi:
			gk = grpChow
		case engine.MeldPong:
			gk = grpPung
		case engine.MeldKong:
			gk = grpKong
		}
		fe.meldSets = append(fe.meldSets, groupedSet{kind: gk, tiles: tiles})
		if m.Kind == engine.MeldKong {
			fe.allKongSets = append(fe.allKongSets, groupedSet{kind: grpKong, tiles: tiles})
			if m.HideTile() {
				fe.nConcealedKong++
			} else {
				fe.nMeldedKong++
				for _, t := range tiles {
					fe.meld[ordOfTile(t)]++
				}
			}
		} else {
			for _, t := range tiles {
				fe.meld[ordOfTile(t)]++
			}
		}
	}

	// 暗副子（分解所得）。
	for _, s := range wh.Sets {
		tiles := append([]engine.Tile(nil), s.Tiles...)
		switch s.Kind {
		case engine.SetKnitted:
			fe.concealedGroups = append(fe.concealedGroups, groupedSet{kind: grpKnitted, tiles: tiles})
			fe.knitted = true
			fe.chows += 3
		case engine.SetChow:
			fe.concealedGroups = append(fe.concealedGroups, groupedSet{kind: grpChow, tiles: tiles})
			fe.chows++
		case engine.SetPung:
			fe.concealedGroups = append(fe.concealedGroups, groupedSet{kind: grpPung, tiles: tiles})
			fe.pungs++
		}
	}
	for _, g := range fe.meldSets {
		switch g.kind {
		case grpChow:
			fe.chows++
		case grpPung:
			fe.pungs++
		case grpKong:
			fe.kongs++
			fe.pungs++
			fe.allKongSets = append(fe.allKongSets, g)
		}
	}
	fe.allPungs = fe.pungs == 4 && fe.chows == 0
	fe.allChows = fe.chows == 4

	// 全牌直方图：暗牌（含组合龙 9 张）＋ 全部副子（含暗杠，暗杠也算面子）。
	fe.full = fe.conc
	for _, m := range result.WinnerMelds {
		for _, t := range m.Tiles {
			fe.full[ordOfTile(t)]++
		}
	}
	fe.pureKnitted = fe.knitted && len(fe.meldSets) == 0

	// 花色分布与字牌/幺九。
	for o := 0; o < 34; o++ {
		if fe.full[o] == 0 {
			continue
		}
		s := ordSuit[o]
		if s < engine.NumericSuitCount {
			fe.hasSuit[s] = true
		} else {
			fe.hasHonors = true
		}
		if s >= engine.NumericSuitCount || rankOfOrd(o) == 1 || rankOfOrd(o) == 9 {
			fe.hasTerms = true
		}
	}
	suits := 0
	for _, b := range fe.hasSuit {
		if b {
			suits++
		}
	}
	fe.lackSuits = 3 - suits
	fe.fullFlush = suits == 1 && !fe.hasHonors
	fe.halfFlush = suits == 1 && fe.hasHonors
	// 五门齐：万筒条风箭齐（风与箭分别计数）。
	hasFeng, hasJian := false, false
	for o := 0; o < 34; o++ {
		if fe.full[o] == 0 {
			continue
		}
		switch ordSuit[o] {
		case engine.SuitFeng:
			hasFeng = true
		case engine.SuitJian:
			hasJian = true
		}
	}
	fe.fiveGates = suits == 3 && hasFeng && hasJian

	// 定位含胡牌张的组（将优先），供边/坎/钓与和绝张类识别。
	if wh.Pair != nil && ordOfTile(*wh.Pair) == fe.winOrd {
		fe.winGroup = 99 // 将
	} else {
		for i, g := range fe.concealedGroups {
			if groupContains(g, fe.winOrd) {
				fe.winGroup = i
				break
			}
		}
	}
	return fe
}

func hist(tiles []engine.Tile) [34]int {
	var c [34]int
	for _, t := range tiles {
		c[ordOfTile(t)]++
	}
	return c
}

func groupContains(g groupedSet, o int) bool {
	for _, t := range g.tiles {
		if ordOfTile(t) == o {
			return true
		}
	}
	return false
}

// --- 最优组合（不重复原则下的取最优） ---

// excludes 番种冲突表：key 出现时，value 中的番种不得同计（国标「不计」条款，
// 依 docs/score-rules.md 逐条列明）。计数型番种以单条计。
var excludes = map[FanKind][]FanKind{
	// 88 分档
	BigFourWinds:     {BigThreeWinds, PrevalentWind, SeatWind, AllPungs, LittleFourWinds},
	BigThreeDragons:  {DragonPung, TwoDragons},
	AllGreen:         {HalfFlush},
	NineGates:        {FullFlush, AllChows},
	FourKongs:        {ThreeKongs, TwoConcealedKongs, TwoMeldedKongs, MeldedKongs, ConcealedKong, MeldedKong, FourConcealedPungs},
	SevenShiftedPair: {SevenPairs, SingleWait},
	ThirteenOrphans:  {AllTypes, OutsideHand, ConcealedHand, FullyConcealedHand, SingleWait, SelfDrawn},

	// 64 分档
	AllTerminals:       {AllPungs, OutsideHand, PungOfTerminalsOrHons, AllChows, NoHonors},
	LittleFourWinds:    {BigThreeWinds, PrevalentWind, SeatWind, BigFourWinds},
	LittleThreeDragons: {DragonPung},
	AllHonors:          {AllPungs, OutsideHand, AllChows, NoHonors, OneVoidedSuit, AllSimples, AllTypes},
	// 四暗刻：不计三暗刻/双暗刻（同源低番）；碰碰和照计（牌例：四暗刻＋碰碰和可并计）。
	FourConcealedPungs: {ThreeConcealedPungs, TwoConcealedPung},
	PureTerminalChows:  {PureDoubleChow, TwoTerminalChows, NoHonors, AllChows},

	// 48 分档
	QuadrupleChow:     {PureTripleChow, PureDoubleChow, AllChows},
	FourPureShiftPung: {PureShiftedPungs, DoublePung, AllPungs},

	// 32 分档
	FourShiftedChows:       {PureShiftedChows, MixedShiftedChows, ShortStraight},
	ThreeKongs:             {TwoConcealedKongs, TwoMeldedKongs, MeldedKongs, ConcealedKong, MeldedKong, DoublePung},
	AllTerminalsAndHonours: {AllPungs, OutsideHand, PungOfTerminalsOrHons, AllChows},

	// 24 分档
	SevenPairs:              {SingleWait, ConcealedHand, FullyConcealedHand, AllChows, TileHog, SelfDrawn, PureDoubleChow},
	GreaterHonorsAndKnitted: {SevenPairs, AllChows, ConcealedHand, SingleWait, SelfDrawn, LesserHonorsAndKnitted},
	AllEvenPungs:            {AllPungs, AllSimples, NoHonors, DoublePung},
	FullFlush:               {NoHonors, OneVoidedSuit, AllTypes},
	PureTripleChow:          {PureDoubleChow, AllChows},
	PureShiftedPungs:        {DoublePung, AllPungs},
	UpperTiles:              {UpperFour, NoHonors},
	MiddleTiles:             {AllSimples, NoHonors},
	LowerTiles:              {LowerFour, NoHonors},

	// 16 分档
	PureStraight:             {ShortStraight, TwoTerminalChows, AllChows},
	ThreeSuitedTerminalChows: {TwoTerminalChows, NoHonors, AllChows, MixedDoubleChow},
	PureShiftedChows:         {ShortStraight, AllChows},
	AllFives:                 {AllSimples, NoHonors, OutsideHand},
	TriplePung:               {DoublePung, AllPungs},
	ThreeConcealedPungs:      {TwoConcealedPung},

	// 12 分档
	LesserHonorsAndKnitted: {AllChows, ConcealedHand, SingleWait, SelfDrawn, AllTypes},
	UpperFour:              {NoHonors},
	KnittedStraight:        {AllChows, MixedStraight},
	LowerFour:              {NoHonors},
	BigThreeWinds:          {PrevalentWind, SeatWind},

	// 8 分档
	MixedStraight:     {ShortStraight, AllChows},
	ReversibleTiles:   {AllSimples},
	MixedTripleChow:   {AllChows, MixedDoubleChow},
	MixedShiftedPungs: {DoublePung, AllPungs},
	ChickenHand:       {}, // 由识别侧保证：仅当无其他番种时成立
	TwoConcealedKongs: {ConcealedKong},

	// 6 分档
	AllPungs:          {AllChows},
	HalfFlush:         {NoHonors},
	MixedShiftedChows: {ShortStraight, AllChows},
	AllTypes:          {AllTypes},   // 反向：五门齐出现时排除；此处亦排
	MeldedHand:        {SingleWait}, // 98「全求人……不计单钓（将）」
	TwoDragons:        {DragonPung},
	MeldedKongs:       {MeldedKong},

	// 4 分档
	TwoMeldedKongs: {MeldedKong},

	// 2 分档
	ConcealedHand:    {},
	AllChows:         {PungOfTerminalsOrHons},
	TwoConcealedPung: {ConcealedKong},
	ConcealedKong:    {},
}

// bestCombination 在冲突图上取番值合计最大的番种组合（取最优计分）。
//
// 国标「五大计分原则」：从最高番值番种起选，不重复、不拆移；同一声明二义时
// 择高。番种冲突图很稀疏（多数番种彼此兼容），以「选/不选首条」分支穷举，
// 剪枝掉与已选冲突者，规模有界（识别出的候选番种通常 ≤ 20 条）。
func bestCombination(hits []FanHit) FanScore {
	// 展开计数型番种为单条（每条独立参与冲突判定）。
	var items []FanHit
	for _, h := range hits {
		n := h.Count
		if n < 1 {
			n = 1
		}
		for i := 0; i < n; i++ {
			items = append(items, FanHit{Kind: h.Kind, Count: 1, Value: h.Value / n})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Value > items[j].Value })

	// 冲突关系对称化：conflict[a][b] 与 conflict[b][a] 同步建立。
	n := len(items)
	if n > 64 {
		// 候选条数理论上远小于 64（81 番种中至多一二十条适用）；防御性截断。
		n = 64
		items = items[:64]
	}
	conflict := make([][]bool, n)
	for i := range conflict {
		conflict[i] = make([]bool, n)
	}
	idx := map[FanKind][]int{}
	for i, it := range items {
		idx[it.Kind] = append(idx[it.Kind], i)
	}
	for i, it := range items {
		for _, ex := range excludes[it.Kind] {
			for _, j := range idx[ex] {
				if i != j {
					conflict[i][j] = true
					conflict[j][i] = true
				}
			}
		}
	}

	var bestMask, curMask uint64
	bestSum, curSum := 0, 0
	var search func(i int)
	search = func(i int) {
		if curSum > bestSum {
			bestSum, bestMask = curSum, curMask
		}
		if i >= n {
			return
		}
		// 剪枝：剩余全选也超不过当前最优时停止。
		rem := 0
		for j := i; j < n; j++ {
			rem += items[j].Value
		}
		if curSum+rem <= bestSum {
			return
		}
		// 选第 i 条（若与已选不冲突）。
		conflictWithCur := false
		for j := 0; j < i; j++ {
			if curMask&(1<<uint(j)) != 0 && conflict[i][j] {
				conflictWithCur = true
				break
			}
		}
		if !conflictWithCur && i < 64 {
			curMask |= 1 << uint(i)
			curSum += items[i].Value
			search(i + 1)
			curSum -= items[i].Value
			curMask &^= 1 << uint(i)
		}
		// 不选第 i 条。
		search(i + 1)
	}
	search(0)
	var chosen []FanHit
	for i, it := range items {
		if bestMask&(1<<uint(i)) != 0 {
			chosen = append(chosen, it)
		}
	}
	// 合并同番种为计数型。
	return FanScore{Fans: mergeHits(chosen), Total: bestSum}
}

func mergeHits(items []FanHit) []FanHit {
	var order []FanKind
	counts := map[FanKind]int{}
	values := map[FanKind]int{}
	for _, it := range items {
		if counts[it.Kind] == 0 {
			order = append(order, it.Kind)
		}
		counts[it.Kind]++
		values[it.Kind] = it.Value
	}
	sort.SliceStable(order, func(i, j int) bool { return values[order[i]] > values[order[j]] })
	out := make([]FanHit, 0, len(order))
	for _, k := range order {
		out = append(out, FanHit{Kind: k, Count: counts[k], Value: values[k] * counts[k]})
	}
	return out
}
