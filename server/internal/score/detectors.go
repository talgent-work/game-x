package score

import (
	"sort"

	"talgent-work/game-x/server/internal/engine"
)

// fanHits 识别一个分解上全部适用的番种（尚未做冲突消解——由 bestCombination 完成）。
// 组合龙（花龙）类番种由 features.knitted 标记；此处对计数型番种直接给出计数。
func fanHits(fe features) []FanHit {
	var hits []FanHit
	add := func(k FanKind, ok bool) {
		if ok {
			hits = append(hits, FanHit{Kind: k, Count: 1, Value: fanValue[k]})
		}
	}
	addN := func(k FanKind, n int) {
		if n > 0 {
			hits = append(hits, FanHit{Kind: k, Count: n, Value: fanValue[k] * n})
		}
	}

	// --- 大牌型（按分解形状） ---
	switch fe.dec.Shape {
	case engine.ShapeSevenPairs:
		add(SevenPairs, true)
		add(SevenShiftedPair, sevenShiftedPairs(fe.dec.Pairs))
	case engine.ShapeAllSingles:
		if allSevenHonors(fe.dec.Singles) {
			add(GreaterHonorsAndKnitted, true)
		} else {
			add(LesserHonorsAndKnitted, true)
		}
	case engine.ShapeThirteenOrphans:
		add(ThirteenOrphans, true)
	}
	add(KnittedStraight, fe.knitted)

	// --- 字牌、风箭 ---
	add(AllHonors, allHonors(fe))
	windPungs := countByTier(fe, engine.SuitFeng)
	dragonPungs := countByTier(fe, engine.SuitJian)
	switch {
	case windPungs == 4:
		add(BigFourWinds, true)
	case windPungs == 3 && pairInTier(fe, engine.SuitFeng):
		add(LittleFourWinds, true)
	case windPungs == 3:
		add(BigThreeWinds, true)
	}
	switch {
	case dragonPungs == 3:
		add(BigThreeDragons, true)
	case dragonPungs == 2 && pairInTier(fe, engine.SuitJian):
		add(LittleThreeDragons, true)
	}
	add(TwoDragons, dragonPungs == 2)
	addN(DragonPung, dragonPungs)

	// 圈风刻 / 门风刻：圈风由上层传入；座位风＝庄家东、下家南、对家西、上家北。
	// 两者同风（如东风圈庄家刻东）按两条各计一次。贪心取最优已保证与三风刻/小四喜
	// 等的冲突（后者番值更高时优先）。
	seatWind := windRankOfSeat(fe.res.Winner, fe.dealer)
	if fe.full[27+int(seatWind)-1] >= 3 {
		add(SeatWind, true)
	}
	if fe.ctx.RoundWind >= 1 && fe.ctx.RoundWind <= 4 && fe.full[27+int(fe.ctx.RoundWind)-1] >= 3 {
		add(PrevalentWind, true)
	}

	// --- 幺九/碰碰/清混一色/五门齐/平和 ---
	add(AllTerminals, allTerminals(fe))
	add(AllTerminalsAndHonours, allTerminalsAndHonours(fe))
	add(AllPungs, fe.allPungs && !fe.knitted)
	add(FullFlush, fe.fullFlush)
	add(HalfFlush, fe.halfFlush && !allHonors(fe))
	add(AllTypes, fe.fiveGates)
	add(AllChows, fe.allChows && pairNotValued(fe) && !fe.knitted)

	// --- 暗刻/暗杠/明杠 ---
	switch pungCount(fe) {
	case 4:
		add(FourConcealedPungs, true)
	case 3:
		add(ThreeConcealedPungs, true)
	case 2:
		add(TwoConcealedPung, true)
	}
	addN(ConcealedKong, fe.nConcealedKong)
	addN(MeldedKong, fe.nMeldedKong)
	switch {
	case fe.nConcealedKong == 2:
		add(TwoConcealedKongs, true)
	case fe.nConcealedKong == 1 && fe.nMeldedKong == 1:
		add(MeldedKongs, true)
	case fe.nMeldedKong == 2:
		add(TwoMeldedKongs, true)
	}
	switch fe.kongs {
	case 3:
		add(ThreeKongs, true)
	case 4:
		add(FourKongs, true)
	}

	// --- 数牌组合（同顺/节高/步高/龙） ---
	addN(PureDoubleChow, C(sameChowPairs(fe, sameSuitSameRank), 2))
	addN(MixedDoubleChow, C(sameChowPairs(fe, sameRankDiffSuit), 2))
	addN(ShortStraight, sameChowPairs(fe, spanSix))
	addN(TwoTerminalChows, sameChowPairs(fe, termChowPair))
	add(PureStraight, hasPureStraight(fe))
	add(MixedStraight, mixedStraight(fe))
	if shiftedChowsRunLen(fe, 1) >= 3 || shiftedChowsRunLen(fe, 2) >= 3 {
		add(PureShiftedChows, true)
	}
	add(MixedShiftedChows, mixedShiftedChowsN(fe) > 0)
	add(MixedShiftedPungs, mixedShiftedPungs(fe))
	add(TriplePung, triplePung(fe))
	addN(DoublePung, doublePungPairs(fe))
	add(QuadrupleChow, sameChowsInOneSuit(fe) == 4)
	add(PureTripleChow, sameChowsInOneSuit(fe) == 3)
	switch pureShiftedPungsRun(fe) {
	case 4:
		add(FourPureShiftPung, true)
	case 3:
		add(PureShiftedPungs, true)
	}
	if shiftedChowsRunLen(fe, 1) >= 4 || shiftedChowsRunLen(fe, 2) >= 4 {
		add(FourShiftedChows, true)
	}
	add(PureTerminalChows, pureTerminalChows(fe))
	add(ThreeSuitedTerminalChows, threeSuitedTerminalChows(fe))

	// --- 全域范围类 ---
	add(UpperTiles, rankRange(fe, 7, 9))
	add(MiddleTiles, rankRange(fe, 4, 6))
	add(LowerTiles, rankRange(fe, 1, 3))
	add(UpperFour, rankRangeNoHonors(fe, 6, 9))
	add(LowerFour, rankRangeNoHonors(fe, 1, 4))
	add(AllEvenPungs, allEvenPungs(fe))
	add(AllFives, allContainsFive(fe))

	// --- 幺九与全带 ---
	add(OutsideHand, allContainsTermOrHonor(fe))
	addN(PungOfTerminalsOrHons, pungTerminalHonorCount(fe))
	add(NoHonors, !fe.hasHonors)
	add(OneVoidedSuit, fe.lackSuits == 1 && !allHonors(fe))
	add(AllSimples, !fe.hasTerms)

	// --- 绿一色/九莲/推不倒/四归一 ---
	add(AllGreen, allGreen(fe))
	add(NineGates, nineGates(fe))
	add(ReversibleTiles, reversible(fe))
	addN(TileHog, tileHogCount(fe))

	// --- 门清/自摸/全求人 ---
	if fullyConcealed(fe) {
		add(ConcealedHand, true)
		if fe.res.Source == engine.WinSelfDrawn {
			add(FullyConcealedHand, true)
		}
	}
	// 全求人：98「全靠吃牌、碰牌、单钓别人打出的牌和牌」——和牌张须来自他人
	// （点炮/抢杠和）；自摸成和不计全求人。
	add(MeldedHand, allMelded(fe) && fe.res.Source != engine.WinSelfDrawn)
	add(SelfDrawn, fe.res.Source == engine.WinSelfDrawn)

	// --- 和牌方式上下文 ---
	ctx := fe.res.Context
	add(OutWithReplacement, ctx.KongBloom)
	add(LastTileDraw, ctx.LastTile)
	add(LastTileClaim, ctx.LastDiscard)
	add(RobbingTheKong, fe.res.Source == engine.WinByRobbedKong)
	if wk := waitKind(fe); wk != "" {
		add(wk, true)
	}
	add(LastTile, fe.res.Source != engine.WinByRobbedKong && lastOfKind(fe))

	// --- 无番和兜底：仅在无其他任何番种时成立 ---
	if len(hits) == 0 {
		add(ChickenHand, true)
	}
	return hits
}

// --- 基础判定 ---

// windRankOfSeat 座位风：庄家东(1)、下家南(2)、对家西(3)、上家北(4)。
func windRankOfSeat(seat, dealer int) uint8 {
	// 行牌按座次逆时针：座位 s 的顺序 = (s - dealer) mod 4。
	return uint8((seat-dealer+engine.Seats)%engine.Seats) + 1
}

func countByTier(fe features, suit engine.Suit) int {
	n := 0
	for o := 0; o < 34; o++ {
		if fe.full[o] >= 3 && ordSuit[o] == suit {
			n++
		}
	}
	return n
}

// pairInTier 将牌是否属于该字牌花色（风/箭）。
func pairInTier(fe features, suit engine.Suit) bool {
	return fe.dec.Pair != nil && fe.dec.Pair.Suit == suit
}

// pungCount 暗刻数（分解出的暗刻面子；暗杠单列于杠类）。
func pungCount(fe features) int {
	n := 0
	for _, g := range fe.concealedGroups {
		if g.kind == grpPung {
			n++
		}
	}
	return n
}

// meldPungs 全部（含明）刻/杠面子的牌值（去重；一副一面子）。
func meldPungs(fe features) []int {
	var out []int
	for _, g := range fe.concealedGroups {
		if g.kind == grpPung {
			out = append(out, ordOfTile(g.tiles[0]))
		}
	}
	for _, m := range fe.res.WinnerMelds {
		if m.Kind == engine.MeldPong || m.Kind == engine.MeldKong {
			out = append(out, ordOfTile(m.Tiles[0]))
		}
	}
	sort.Ints(out)
	return out
}

// meldChows 全部顺子面子的牌值三元组（明＋暗；组合龙不计入——由取用方单独处理）。
func meldChows(fe features) [][3]int {
	var out [][3]int
	for _, g := range fe.concealedGroups {
		if g.kind == grpChow {
			out = append(out, chowOrd(g.tiles))
		}
	}
	for _, m := range fe.res.WinnerMelds {
		if m.Kind == engine.MeldChi {
			out = append(out, chowOrd(m.Tiles))
		}
	}
	return out
}

func chowOrd(tiles []engine.Tile) [3]int {
	var a [3]int
	for i := 0; i < 3 && i < len(tiles); i++ {
		a[i] = ordOfTile(tiles[i])
	}
	sort.Ints(a[:])
	return a
}

// --- 字牌/幺九类 ---

func allHonors(fe features) bool {
	total := 0
	for o := 0; o < 34; o++ {
		if fe.full[o] == 0 {
			continue
		}
		if ordSuit[o] < engine.NumericSuitCount {
			return false
		}
		total += fe.full[o]
	}
	return total > 0
}

func allTerminals(fe features) bool {
	if len(meldPungs(fe)) != 4 || fe.dec.Pair == nil {
		return false
	}
	for o := 0; o < 34; o++ {
		if fe.full[o] == 0 {
			continue
		}
		if ordSuit[o] >= engine.NumericSuitCount {
			return false
		}
		if r := rankOfOrd(o); r != 1 && r != 9 {
			return false
		}
	}
	p := *fe.dec.Pair
	return p.Numeric() && (p.Rank == 1 || p.Rank == 9)
}

func allTerminalsAndHonours(fe features) bool {
	if len(meldPungs(fe)) != 4 || fe.dec.Pair == nil {
		return false
	}
	for o := 0; o < 34; o++ {
		if fe.full[o] == 0 {
			continue
		}
		if ordSuit[o] >= engine.NumericSuitCount {
			continue
		}
		if r := rankOfOrd(o); r != 1 && r != 9 {
			return false
		}
	}
	return termOrHonorTile(*fe.dec.Pair)
}

func termOrHonorTile(t engine.Tile) bool {
	if !t.Numeric() {
		return true
	}
	return t.Rank == 1 || t.Rank == 9
}

func pairNotValued(fe features) bool {
	if fe.dec.Pair == nil {
		return false
	}
	p := *fe.dec.Pair
	return p.Suit != engine.SuitJian
}

// allContainsTermOrHonor 每个面子（含将）都含幺九或字牌。
func allContainsTermOrHonor(fe features) bool {
	if fe.dec.Pair == nil || !termOrHonorTile(*fe.dec.Pair) {
		return false
	}
	for _, g := range fe.concealedGroups {
		if !groupHasTermOrHonor(g) {
			return false
		}
	}
	for _, m := range fe.res.WinnerMelds {
		if !groupHasTermOrHonor(groupedSet{kind: grpKindOf(m), tiles: m.Tiles}) {
			return false
		}
	}
	return true
}

func grpKindOf(m engine.Meld) groupKind {
	switch m.Kind {
	case engine.MeldChi:
		return grpChow
	case engine.MeldPong:
		return grpPung
	default:
		return grpKong
	}
}

func groupHasTermOrHonor(g groupedSet) bool {
	for _, t := range g.tiles {
		if termOrHonorTile(t) {
			return true
		}
	}
	return false
}

// pungTerminalHonorCount 幺九刻：幺九/字牌刻子（含杠）个数。
func pungTerminalHonorCount(fe features) int {
	n := 0
	for _, o := range meldPungs(fe) {
		if termOrHonorTile(tileOfOrd(o)) {
			n++
		}
	}
	return n
}

// --- 数牌组合 ---

// sameSuitSameRank 同花色同顺（花色即基准点数后缀，ord 区间内 %9 相同不代表同花色）。
func sameSuitSameRank(a, b [3]int) bool {
	return a[0]/9 == b[0]/9 && a[0]%9 == b[0]%9
}

func sameRankDiffSuit(a, b [3]int) bool {
	return a[0]%9 == b[0]%9 && a[0]/9 != b[0]/9
}

func spanSix(a, b [3]int) bool {
	if ordSuit[a[0]] != ordSuit[b[0]] {
		return false
	}
	d := a[0] - b[0]
	return d == 6 || d == -6
}

func termChowPair(a, b [3]int) bool {
	if ordSuit[a[0]] != ordSuit[b[0]] {
		return false
	}
	x, y := a[0]%9, b[0]%9
	return (x == 0 && y == 6) || (x == 6 && y == 0)
}

// sameChowPairs 满足 key 的顺子对数。
func sameChowPairs(fe features, key func(a, b [3]int) bool) int {
	chows := meldChows(fe)
	n := 0
	for i := 0; i < len(chows); i++ {
		for j := i + 1; j < len(chows); j++ {
			if key(chows[i], chows[j]) {
				n++
			}
		}
	}
	return n
}

// hasPureStraight 清龙：同花色 123、456、789 各一。
func hasPureStraight(fe features) bool {
	chows := meldChows(fe)
	for s := 0; s < 3; s++ {
		base := s * 9
		have := [3]bool{}
		for _, c := range chows {
			switch c[0] {
			case base:
				have[0] = true
			case base + 3:
				have[1] = true
			case base + 6:
				have[2] = true
			}
		}
		if have[0] && have[1] && have[2] {
			return true
		}
	}
	return false
}

// mixedStraight 花龙：三花色的 123、456、789（任意一种配法）。
func mixedStraight(fe features) bool {
	chows := meldChows(fe)
	for _, a := range chows {
		if a[0]%9 != 0 {
			continue
		}
		for _, b := range chows {
			if b[0]%9 != 3 || ordSuit[b[0]] == ordSuit[a[0]] {
				continue
			}
			for _, c := range chows {
				if c[0]%9 != 6 || ordSuit[c[0]] == ordSuit[a[0]] || ordSuit[c[0]] == ordSuit[b[0]] {
					continue
				}
				return true
			}
		}
	}
	return false
}

// shiftedChowsRunLen 同花色中起始点数等差 d 的最长顺子链长度（0 无）。
func shiftedChowsRunLen(fe features, d int) int {
	best := 0
	for s := 0; s < 3; s++ {
		starts := []int{}
		for _, c := range meldChows(fe) {
			if c[0] >= s*9 && c[0] < s*9+9 && c[0]%9 <= 9-3 {
				starts = append(starts, c[0]%9)
			}
		}
		if n := longestRunBy(starts, d); n > best {
			best = n
		}
	}
	return best
}

func longestRunBy(vals []int, d int) int {
	if len(vals) == 0 {
		return 0
	}
	set := map[int]bool{}
	for _, v := range vals {
		set[v] = true
	}
	best := 1
	for v := range set {
		n := 1
		for x := v + d; set[x]; x += d {
			n++
		}
		if n > best {
			best = n
		}
	}
	return best
}

// mixedShiftedChowsN 三色三步高（连差与隔差两形式）。
func mixedShiftedChowsN(fe features) int {
	for _, d := range []int{1, 2} {
		type sc struct {
			suit engine.Suit
			rank int
		}
		var list []sc
		for _, c := range meldChows(fe) {
			if c[0]%9 <= 9-3 {
				list = append(list, sc{ordSuit[c[0]], c[0] % 9})
			}
		}
		for _, a := range list {
			for _, b := range list {
				if b.suit == a.suit || b.rank != a.rank+d {
					continue
				}
				for _, c := range list {
					if c.suit != a.suit && c.suit != b.suit && c.rank == b.rank+d {
						return 1
					}
				}
			}
		}
	}
	return 0
}

// mixedShiftedPungs 三色三节高：三花色的刻子点数依次 +1。
func mixedShiftedPungs(fe features) bool {
	pungs := meldPungs(fe)
	for _, a := range pungs {
		if ordSuit[a] >= engine.NumericSuitCount {
			continue
		}
		for _, b := range pungs {
			if ordSuit[b] >= engine.NumericSuitCount || ordSuit[b] == ordSuit[a] || rankOfOrd(b) != rankOfOrd(a)+1 {
				continue
			}
			for _, c := range pungs {
				if ordSuit[c] >= engine.NumericSuitCount || ordSuit[c] == ordSuit[a] || ordSuit[c] == ordSuit[b] {
					continue
				}
				if rankOfOrd(c) == rankOfOrd(b)+1 {
					return true
				}
			}
		}
	}
	return false
}

// triplePung 三同刻：三花色的同点数刻子。
func triplePung(fe features) bool {
	pungs := meldPungs(fe)
	for _, a := range pungs {
		if ordSuit[a] >= engine.NumericSuitCount {
			continue
		}
		n := 1
		for _, b := range pungs {
			if b != a && ordSuit[b] < engine.NumericSuitCount && rankOfOrd(b) == rankOfOrd(a) {
				n++
			}
		}
		if n >= 3 {
			return true
		}
	}
	return false
}

// doublePungPairs 双同刻：同点数的刻子对数（须不同花色）。
func doublePungPairs(fe features) int {
	pungs := meldPungs(fe)
	n := 0
	for i := 0; i < len(pungs); i++ {
		for j := i + 1; j < len(pungs); j++ {
			if ordSuit[pungs[i]] < engine.NumericSuitCount &&
				ordSuit[pungs[j]] < engine.NumericSuitCount &&
				rankOfOrd(pungs[i]) == rankOfOrd(pungs[j]) {
				n++
			}
		}
	}
	return n
}

// sameChowsInOneSuit 完全相同的顺子的最大重数（含花色）。
func sameChowsInOneSuit(fe features) int {
	counts := map[[3]int]int{}
	best := 0
	for _, c := range meldChows(fe) {
		counts[c]++
		if counts[c] > best {
			best = counts[c]
		}
	}
	return best
}

// pureShiftedPungsRun 同花色刻子点数依次 +1 的最长链长（3=一色三节高、4=一色四节高）。
func pureShiftedPungsRun(fe features) int {
	pungs := meldPungs(fe)
	best := 0
	for s := 0; s < 3; s++ {
		ranks := []int{}
		for _, o := range pungs {
			if ordSuit[o] == engine.Suit(s) {
				ranks = append(ranks, int(rankOfOrd(o)))
			}
		}
		if n := longestRunBy(ranks, 1); n > best {
			best = n
		}
	}
	return best
}

// pureTerminalChows 一色双龙会：同花色 123、123、789、789 ＋ 5 的将。
func pureTerminalChows(fe features) bool {
	if fe.dec.Pair == nil || !fe.dec.Pair.Numeric() || fe.dec.Pair.Rank != 5 {
		return false
	}
	for s := 0; s < 3; s++ {
		c123, c789 := 0, 0
		for _, c := range meldChows(fe) {
			if c[0] == s*9 {
				c123++
			}
			if c[0] == s*9+6 {
				c789++
			}
		}
		if c123 == 2 && c789 == 2 {
			return true
		}
	}
	return false
}

// threeSuitedTerminalChows 三色双龙会：三花色中一方双 123、一方双 789、一方各一 ＋ 5 的将。
func threeSuitedTerminalChows(fe features) bool {
	if fe.dec.Pair == nil || !fe.dec.Pair.Numeric() || fe.dec.Pair.Rank != 5 {
		return false
	}
	var dbl123, dbl789, both int
	for s := 0; s < 3; s++ {
		n123, n789 := 0, 0
		for _, c := range meldChows(fe) {
			if c[0] == s*9 {
				n123++
			}
			if c[0] == s*9+6 {
				n789++
			}
		}
		switch {
		case n123 >= 1 && n789 >= 1:
			both++
		case n123 >= 2:
			dbl123++
		case n789 >= 2:
			dbl789++
		}
	}
	return both == 1 && dbl123 == 1 && dbl789 == 1
}

// --- 全域范围类 ---

func rankRange(fe features, lo, hi uint8) bool {
	total := 0
	for o := 0; o < 34; o++ {
		if fe.full[o] == 0 {
			continue
		}
		if ordSuit[o] >= engine.NumericSuitCount {
			return false
		}
		r := rankOfOrd(o)
		if r < lo || r > hi {
			return false
		}
		total += fe.full[o]
	}
	return total > 0
}

func rankRangeNoHonors(fe features, lo, hi uint8) bool { return rankRange(fe, lo, hi) }

func allEvenPungs(fe features) bool {
	if len(meldPungs(fe)) != 4 {
		return false
	}
	if fe.dec.Pair == nil || !fe.dec.Pair.Numeric() || fe.dec.Pair.Rank%2 != 0 {
		return false
	}
	for _, o := range meldPungs(fe) {
		if ordSuit[o] >= engine.NumericSuitCount || rankOfOrd(o)%2 != 0 {
			return false
		}
	}
	return true
}

func allContainsFive(fe features) bool {
	if fe.dec.Pair == nil || !fe.dec.Pair.Numeric() || fe.dec.Pair.Rank != 5 {
		return false
	}
	for _, g := range fe.concealedGroups {
		if !groupHasRank(g, 5) {
			return false
		}
	}
	for _, m := range fe.res.WinnerMelds {
		if !groupHasRank(groupedSet{kind: grpKindOf(m), tiles: m.Tiles}, 5) {
			return false
		}
	}
	return true
}

func groupHasRank(g groupedSet, r uint8) bool {
	for _, t := range g.tiles {
		if t.Numeric() && t.Rank == r {
			return true
		}
	}
	return false
}

// allGreen 绿一色：仅由 23468 条与发组成，必含发。
func allGreen(fe features) bool {
	total := 0
	for o := 0; o < 34; o++ {
		if fe.full[o] == 0 {
			continue
		}
		total += fe.full[o]
		if isGreenTile(ordSuit[o], rankOfOrd(o)) {
			continue
		}
		return false
	}
	if total == 0 {
		return false
	}
	return fe.full[ordOfTile(engine.Tile{Suit: engine.SuitJian, Rank: 2})] > 0
}

func isGreenTile(s engine.Suit, r uint8) bool {
	switch s {
	case engine.SuitTiao:
		return r == 2 || r == 3 || r == 4 || r == 6 || r == 8
	case engine.SuitJian:
		return r == 2
	}
	return false
}

// nineGates 九莲宝灯：清一色 1112345678999 ＋ 任一同色数牌（14 张）。
func nineGates(fe features) bool {
	if fe.dec.Shape != engine.ShapeStandard || !fe.fullFlush || fe.knitted {
		return false
	}
	suit := -1
	for s := 0; s < 3; s++ {
		if fe.hasSuit[s] {
			if suit >= 0 {
				return false
			}
			suit = s
		}
	}
	if suit < 0 {
		return false
	}
	base := suit * 9
	need := [9]int{3, 1, 1, 1, 1, 1, 1, 1, 3}
	total := 0
	for r := 0; r < 9; r++ {
		if fe.full[base+r] < need[r] {
			return false
		}
		total += fe.full[base+r]
	}
	return total == 14
}

// reversible 推不倒：仅由推不倒牌面组成（筒 1234589、条 245689、白）。
func reversible(fe features) bool {
	total := 0
	for o := 0; o < 34; o++ {
		if fe.full[o] == 0 {
			continue
		}
		total += fe.full[o]
		s := ordSuit[o]
		r := rankOfOrd(o)
		ok := false
		switch s {
		case engine.SuitTong:
			ok = r == 1 || r == 2 || r == 3 || r == 4 || r == 5 || r == 8 || r == 9
		case engine.SuitTiao:
			ok = r == 2 || r == 4 || r == 5 || r == 6 || r == 8 || r == 9
		case engine.SuitJian:
			ok = r == 3
		}
		if !ok {
			return false
		}
	}
	return total > 0
}

// tileHogCount 四归一：未成杠的、在手牌各组中共用满四张的牌值个数。
func tileHogCount(fe features) int {
	var kongSlots [34]bool
	for _, g := range fe.allKongSets {
		kongSlots[ordOfTile(g.tiles[0])] = true
	}
	n := 0
	for o := 0; o < 34; o++ {
		if fe.full[o] == 4 && !kongSlots[o] {
			n++
		}
	}
	return n
}

// --- 门清/全求人 ---

func fullyConcealed(fe features) bool {
	for _, m := range fe.res.WinnerMelds {
		if !m.HideTile() {
			return false
		}
	}
	return true
}

// allMelded 全求人：四副面子全为明副子，仅将牌在暗。
func allMelded(fe features) bool {
	if len(fe.res.WinnerMelds) != 4 || fe.dec.Pair == nil {
		return false
	}
	for _, m := range fe.res.WinnerMelds {
		if m.HideTile() {
			return false
		}
	}
	return len(fe.concealedGroups) == 0
}

// --- 和绝张/听型 ---

// lastOfKind 和绝张：所和之牌的第 4 张，其余 3 张均已亮明（他人亮明副子）。
// 引擎终局结果不携带牌河，故「已亮明」按他人副子（暗杠除外）保守判定；
// 见 docs/score-rules.md「已知偏差」。
func lastOfKind(fe features) bool {
	w := fe.winOrd
	visible := 0
	for s, melds := range fe.res.Melds {
		if s == fe.res.Winner {
			continue
		}
		for _, m := range melds {
			if m.Kind == engine.MeldKong && m.HideTile() {
				continue
			}
			for _, t := range m.Tiles {
				if ordOfTile(t) == w {
					visible++
				}
			}
		}
	}
	return visible == 3
}

// waitKind 听牌型：坎张/边张/单钓将（至多一条，符合不重复原则）。空串表示未命中。
//
// 口径（98 番种表）：「单钓将：钓单张牌作将成和（缺将牌）」——胡牌张若可成为某
// 顺子（暗）的中张/边端，计坎张/边张；否则以将位参与（与将同值）即计单钓将。
// 单钓按暗牌形状判定：同一牌值在明组（他人弃牌所成副子）或另副面子中另有出现
// 不影响本条（各为不同实体牌；混入立牌摆放等摆法差异见 docs/score-rules.md 已知偏差）。
func waitKind(fe features) FanKind {
	o := fe.winOrd
	// 坎张：胡牌张为某顺子（暗）的中张。
	for _, g := range fe.concealedGroups {
		if g.kind != grpChow || len(g.tiles) != 3 {
			continue
		}
		c := chowOrd(g.tiles)
		if c[1] == o {
			return ClosedWait
		}
		// 边张：123 和 3、789 和 7。
		if (c[0]%9 == 0 && c[2] == o) || (c[0]%9 == 6 && c[0] == o) {
			return EdgeWait
		}
	}
	// 单钓：胡牌张以将位参与（钓单张作将成和）。
	if fe.dec.Pair != nil && ordOfTile(*fe.dec.Pair) == o {
		return SingleWait
	}
	return ""
}

// --- 七对/十三幺 ---

func sevenShiftedPairs(pairs []engine.Tile) bool {
	if len(pairs) != 7 {
		return false
	}
	suit := pairs[0].Suit
	if !numericSuit(suit) {
		return false
	}
	ranks := make([]int, 0, 7)
	for _, t := range pairs {
		if t.Suit != suit {
			return false
		}
		ranks = append(ranks, int(t.Rank))
	}
	sort.Ints(ranks)
	for i := 1; i < len(ranks); i++ {
		if ranks[i] != ranks[i-1]+1 {
			return false
		}
	}
	return true
}

func allSevenHonors(singles []engine.Tile) bool {
	honors := map[int]bool{}
	for _, t := range singles {
		if t.Suit == engine.SuitFeng || t.Suit == engine.SuitJian {
			honors[ordOfTile(t)] = true
		}
	}
	return len(honors) == 7
}

// --- 小工具 ---

// C 组合数（小 n）。
func C(n, k int) int {
	if n < k || k < 0 {
		return 0
	}
	res := 1
	for i := 0; i < k; i++ {
		res = res * (n - i) / (i + 1)
	}
	return res
}
func numericSuit(s engine.Suit) bool { return s < engine.NumericSuitCount }
