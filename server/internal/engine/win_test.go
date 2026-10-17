package engine

import "testing"

// 和牌分解：全部国标和牌型逐一验证（随机模拟打不出七对/全不靠，必须定点测）。

func tiles(ts ...Tile) []Tile { return ts }

func TestDecomposeStandard(t *testing.T) {
	// 123万456万789万＋123筒＋11条：唯一分解。
	whs := Decompose(tiles(
		wan(1), wan(2), wan(3), wan(4), wan(5), wan(6), wan(7), wan(8), wan(9),
		tong(1), tong(2), tong(3), tiao(1), tiao(1),
	), 0)
	if len(whs) != 1 || whs[0].Shape != ShapeStandard || whs[0].Pair == nil || *whs[0].Pair != tiao(1) {
		t.Fatalf("标准型应唯一分解：实得 %v", whs)
	}
	if len(whs[0].Sets) != 4 {
		t.Fatalf("应有 4 副面子，实得 %d", len(whs[0].Sets))
	}
}

func TestDecomposeStandardMultiple(t *testing.T) {
	// 11122233344455万：至少两种分解（三刻＋55 与 三顺＋44455…）。
	whs := Decompose(tiles(
		wan(1), wan(1), wan(1), wan(2), wan(2), wan(2), wan(3), wan(3), wan(3),
		wan(4), wan(4), wan(4), wan(5), wan(5),
	), 0)
	if len(whs) < 2 {
		t.Fatalf("歧义手牌应枚举多种分解，实得 %d 种：%v", len(whs), whs)
	}
}

func TestDecomposeKnitted(t *testing.T) {
	// 组合龙：万147 筒258 条369 ＋ 234万 ＋ 55筒。
	hand := tiles(
		wan(1), wan(4), wan(7), tong(2), tong(5), tong(8), tiao(3), tiao(6), tiao(9),
		wan(2), wan(3), wan(4), tong(5), tong(5),
	)
	whs := Decompose(hand, 0)
	if len(whs) == 0 {
		t.Fatal("组合龙型应可和牌")
	}
	var knitted *WinHand
	for i := range whs {
		if whs[i].Shape == ShapeKnitted {
			knitted = &whs[i]
		}
	}
	if knitted == nil {
		t.Fatalf("应含组合龙分解，实得 %v", whs)
	}
	if len(knitted.Sets) != 2 { // 组合龙（9 张）＋一副顺子
		t.Fatalf("组合龙分解应为组合龙＋1 副面子，实得 %v", knitted.Sets)
	}
	if knitted.Sets[0].Kind != SetKnitted || len(knitted.Sets[0].Tiles) != 9 {
		t.Fatalf("首副应为 9 张组合龙，实得 %v", knitted.Sets[0])
	}
	if knitted.Pair == nil || *knitted.Pair != tong(5) {
		t.Fatalf("将应为 5筒，实得 %v", knitted.Pair)
	}

	// 同手带 1 副吃碰杠副子：暗牌 11 张＝组合龙＋将。
	handMeld1 := tiles(
		wan(1), wan(4), wan(7), tong(2), tong(5), tong(8), tiao(3), tiao(6), tiao(9),
		tong(5), tong(5),
	)
	if whs := Decompose(handMeld1, 1); len(whs) == 0 || whs[0].Shape != ShapeKnitted {
		t.Fatalf("组合龙＋1 副吃碰杠副子应可和牌，实得 %v", whs)
	}
	// 2 副吃碰杠副子（11-…＝暗牌 8 张）不可能配组合龙（9＋将＝11＞8）。
	handMeld2 := tiles(
		wan(1), wan(4), wan(7), tong(2), tong(5), tong(8), tiao(3), tiao(6),
	)
	if whs := Decompose(handMeld2, 2); len(whs) != 0 {
		t.Fatalf("组合龙至多配 1 副副子，实得 %v", whs)
	}
}

func TestDecomposeSevenPairs(t *testing.T) {
	hand := tiles(
		wan(1), wan(1), wan(2), wan(2), wan(3), wan(3), tong(4), tong(4),
		tong(5), tong(5), tiao(6), tiao(6), tiao(7), tiao(7),
	)
	whs := Decompose(hand, 0)
	if len(whs) != 1 || whs[0].Shape != ShapeSevenPairs || len(whs[0].Pairs) != 7 {
		t.Fatalf("七对应唯一分解，实得 %v", whs)
	}
	// 4 张同值当两对。
	hand4 := tiles(
		wan(1), wan(1), wan(1), wan(1), wan(2), wan(2), tong(4), tong(4),
		tong(5), tong(5), tiao(6), tiao(6), tiao(7), tiao(7),
	)
	if whs := Decompose(hand4, 0); len(whs) == 0 || whs[0].Shape != ShapeSevenPairs {
		t.Fatalf("4 张同值可当两对，实得 %v", whs)
	}
	// 奇数张不成七对。
	handOdd := tiles(
		wan(1), wan(1), wan(1), wan(2), wan(2), tong(4), tong(4),
		tong(5), tong(5), tiao(6), tiao(6), tiao(7), tiao(7), tiao(8),
	)
	if whs := Decompose(handOdd, 0); len(whs) != 0 {
		t.Fatalf("奇张不成七对，实得 %v", whs)
	}
}

func TestDecomposeAllSingles(t *testing.T) {
	// 全不靠：万147 筒258 条369＋东南西北中。
	hand := tiles(
		wan(1), wan(4), wan(7), tong(2), tong(5), tong(8), tiao(3), tiao(6), tiao(9),
		feng(1), feng(2), feng(3), feng(4), jian(1),
	)
	whs := Decompose(hand, 0)
	if len(whs) != 1 || whs[0].Shape != ShapeAllSingles || len(whs[0].Singles) != 14 {
		t.Fatalf("全不靠应唯一分解，实得 %v", whs)
	}
	// 部分数牌（花色分组内不齐 3 张）＋字牌补足也成立：万{1,4} 筒{2,5,8} 条{3,6,9}＋六字。
	hand2 := tiles(
		wan(1), wan(4), tong(2), tong(5), tong(8), tiao(3), tiao(6), tiao(9),
		feng(1), feng(2), feng(3), feng(4), jian(1), jian(2),
	)
	if whs := Decompose(hand2, 0); len(whs) != 1 || whs[0].Shape != ShapeAllSingles {
		t.Fatalf("部分数牌的全不靠应成立，实得 %v", whs)
	}
	// 错位（同一花色出现两组的点数）不成立：万{1,4,2}。
	handBad := tiles(
		wan(1), wan(4), wan(2), tong(5), tong(8), tiao(3), tiao(6), tiao(9),
		feng(1), feng(2), feng(3), feng(4), jian(1), jian(2),
	)
	if whs := Decompose(handBad, 0); len(whs) != 0 {
		t.Fatalf("错位排列不成全不靠，实得 %v", whs)
	}
	// 重复牌不成（单张）。
	handDup := tiles(
		wan(1), wan(4), wan(7), tong(2), tong(2), tong(8), tiao(3), tiao(6), tiao(9),
		feng(1), feng(2), feng(3), feng(4), jian(1),
	)
	if whs := Decompose(handDup, 0); len(whs) != 0 {
		t.Fatalf("重复牌不成全不靠，实得 %v", whs)
	}
}

func TestDecomposeWithMelds(t *testing.T) {
	// 4 副副子：暗牌只剩将。
	whs := Decompose(tiles(tiao(5), tiao(5)), 4)
	if len(whs) != 1 || whs[0].Shape != ShapeStandard || whs[0].Pair == nil || *whs[0].Pair != tiao(5) {
		t.Fatalf("四副子＋将应可和，实得 %v", whs)
	}
	// 2 副副子：暗牌 8 张＝2 副＋将。
	whs = Decompose(tiles(
		wan(1), wan(2), wan(3), wan(7), wan(8), wan(9), tiao(5), tiao(5),
	), 2)
	if len(whs) != 1 || len(whs[0].Sets) != 2 {
		t.Fatalf("两副副子＋两暗副＋将应可和，实得 %v", whs)
	}
}

func TestDecomposeInvalid(t *testing.T) {
	if whs := Decompose(tiles(wan(1), wan(2), wan(3)), 0); whs != nil {
		t.Fatalf("张数不符应无解，实得 %v", whs)
	}
	if whs := Decompose(tiles(
		wan(1), wan(2), wan(4), wan(5), wan(7), wan(8), tong(1), tong(2), tong(4),
		tong(5), tong(7), tong(8), tiao(1), tiao(2),
	), 0); len(whs) != 0 {
		t.Fatalf("不成和牌应无解，实得 %v", whs)
	}
	// 非法牌值。
	if whs := Decompose(tiles(wan(10), wan(2), wan(3), wan(4), wan(5), wan(6), wan(7), wan(8), wan(9),
		tong(1), tong(2), tong(3), tiao(1), tiao(1)), 0); len(whs) != 0 {
		t.Fatalf("非法牌值应无解，实得 %v", whs)
	}
}

func TestDecomposeThirteenOrphans(t *testing.T) {
	// 十三幺：13 种幺九字牌各一＋恰一种成对。
	hand := tiles(
		wan(1), wan(9), tong(1), tong(9), tiao(1), tiao(9),
		feng(1), feng(2), feng(3), feng(4), jian(1), jian(2), jian(3), wan(1),
	)
	whs := Decompose(hand, 0)
	if len(whs) != 1 || whs[0].Shape != ShapeThirteenOrphans {
		t.Fatalf("十三幺应唯一分解，实得 %v", whs)
	}
	if whs[0].Pair == nil || *whs[0].Pair != wan(1) || len(whs[0].Singles) != 12 {
		t.Fatalf("十三幺将应为 1万、单牌 12 张，实得 %+v", whs[0])
	}
	// 13 张单张（未成对）不成和牌型。
	noPair := tiles(
		wan(1), wan(9), tong(1), tong(9), tiao(1), tiao(9),
		feng(1), feng(2), feng(3), feng(4), jian(1), jian(2), jian(3),
	)
	if whs := Decompose(append(append([]Tile(nil), noPair...), wan(1)), 0); len(whs) == 0 {
		t.Fatal("缺一张补成对的十三幺应可和")
	}
	// 两种成对不成。
	twoPairs := tiles(
		wan(1), wan(1), wan(9), wan(9), tong(1), tong(9), tiao(1), tiao(9),
		feng(1), feng(2), feng(3), feng(4), jian(1), jian(2),
	)
	if whs := Decompose(twoPairs, 0); len(whs) != 0 {
		t.Fatalf("两种成对应不成十三幺，实得 %v", whs)
	}
	// 混入非幺九牌不成。
	mixed := tiles(
		wan(1), wan(9), tong(1), tong(9), tiao(1), tiao(9),
		feng(1), feng(2), feng(3), feng(4), jian(1), jian(2), jian(3), wan(5),
	)
	if whs := Decompose(mixed, 0); len(whs) != 0 {
		t.Fatalf("混入中张不应成和，实得 %v", whs)
	}
}

func feng(r uint8) Tile { return Tile{Suit: SuitFeng, Rank: r} }
func jian(r uint8) Tile { return Tile{Suit: SuitJian, Rank: r} }
