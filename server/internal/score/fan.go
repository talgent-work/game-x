// Package score 实现国标麻将（GB /《中国麻将竞赛规则》1998 及 WMO《麻将竞赛规则》
// 通行裁定）的和牌计分与一局结算：番种识别、从全部合法分解中取最优计分、
// 8 番起和判定、按官方 MCR 公式的四家得分。
//
// 口径（见仓库 docs/score-rules.md 与 docs/engine-rules.md）：
//   - 本项目按 136 张无花牌口径（GX-2 既定项目裁定）；花牌类番种（花牌 1 分/张）
//     不适用，逐项记录；
//   - 座位风按「庄家东、下家南、对家西、上家北」推导；圈风为跨局概念，由调用方传入；
//   - 国标 81 番种中没有天和/地和；庄家开局第一张自摸按普通自摸处理；
//   - 结算公式（官方 MCR，无庄家加倍）：点炮＝放铳者付番种分＋三家各付 8 底分；
//     自摸＝三家各付（番种分＋8 底分）。
//
// 设计约束（依 GX-1《技术选型报告》§5.2 与 GX-2 引擎同构）：
//   - 零第三方依赖、纯逻辑、确定性（同一输入必得同一结果）；
//   - 只消费 engine 的导出类型（Result/WinHand/Meld/Tile 等），不修改引擎；
//   - 结算法结果的 JSON 形状可导出，供 GX-4 协议层（tygo 生成 TS 类型）直接复用。
package score

import "strconv"

// FanKind 番种标识（国标番种表 1998 条目名）。
type FanKind string

// 番种常量（按番值分档）。仅列本项目 136 张无花牌口径下适用的番种。
const (
	// 88 分
	BigFourWinds     FanKind = "大四喜"  // 88
	BigThreeDragons  FanKind = "大三元"  // 88
	AllGreen         FanKind = "绿一色"  // 88
	NineGates        FanKind = "九莲宝灯" // 88
	FourKongs        FanKind = "四杠"   // 88
	SevenShiftedPair FanKind = "连七对"  // 88
	ThirteenOrphans  FanKind = "十三幺"  // 88

	// 64 分
	AllTerminals       FanKind = "清幺九"   // 64
	LittleFourWinds    FanKind = "小四喜"   // 64
	LittleThreeDragons FanKind = "小三元"   // 64
	AllHonors          FanKind = "字一色"   // 64
	FourConcealedPungs FanKind = "四暗刻"   // 64
	PureTerminalChows  FanKind = "一色双龙会" // 64

	// 48 分
	QuadrupleChow     FanKind = "一色四同顺" // 48
	FourPureShiftPung FanKind = "一色四节高" // 48

	// 32 分
	FourShiftedChows       FanKind = "一色四步高" // 32
	ThreeKongs             FanKind = "三杠"    // 32
	AllTerminalsAndHonours FanKind = "混幺九"   // 32

	// 24 分
	SevenPairs              FanKind = "七对"    // 24
	GreaterHonorsAndKnitted FanKind = "七星不靠"  // 24
	AllEvenPungs            FanKind = "全双刻"   // 24
	FullFlush               FanKind = "清一色"   // 24
	PureTripleChow          FanKind = "一色三同顺" // 24
	PureShiftedPungs        FanKind = "一色三节高" // 24
	UpperTiles              FanKind = "全大"    // 24
	MiddleTiles             FanKind = "全中"    // 24
	LowerTiles              FanKind = "全小"    // 24

	// 16 分
	PureStraight             FanKind = "清龙"    // 16
	ThreeSuitedTerminalChows FanKind = "三色双龙会" // 16
	PureShiftedChows         FanKind = "一色三步高" // 16
	AllFives                 FanKind = "全带五"   // 16
	TriplePung               FanKind = "三同刻"   // 16
	ThreeConcealedPungs      FanKind = "三暗刻"   // 16

	// 12 分
	LesserHonorsAndKnitted FanKind = "全不靠" // 12
	UpperFour              FanKind = "大于五" // 12
	KnittedStraight        FanKind = "组合龙" // 12
	LowerFour              FanKind = "小于五" // 12
	BigThreeWinds          FanKind = "三风刻" // 12

	// 8 分
	MixedStraight      FanKind = "花龙"    // 8
	ReversibleTiles    FanKind = "推不倒"   // 8
	MixedTripleChow    FanKind = "三色三同顺" // 8
	MixedShiftedPungs  FanKind = "三色三节高" // 8
	ChickenHand        FanKind = "无番和"   // 8
	LastTileDraw       FanKind = "妙手回春"  // 8
	LastTileClaim      FanKind = "海底捞月"  // 8
	OutWithReplacement FanKind = "杠上开花"  // 8
	RobbingTheKong     FanKind = "抢杠和"   // 8
	TwoConcealedKongs  FanKind = "双暗杠"   // 8

	// 6 分
	AllPungs          FanKind = "碰碰和"   // 6
	HalfFlush         FanKind = "混一色"   // 6
	MixedShiftedChows FanKind = "三色三步高" // 6
	AllTypes          FanKind = "五门齐"   // 6
	MeldedHand        FanKind = "全求人"   // 6
	TwoDragons        FanKind = "双箭刻"   // 6
	MeldedKongs       FanKind = "一明一暗"  // 6

	// 4 分
	OutsideHand        FanKind = "全带幺" // 4
	FullyConcealedHand FanKind = "不求人" // 4
	TwoMeldedKongs     FanKind = "双明杠" // 4
	LastTile           FanKind = "和绝张" // 4

	// 2 分
	DragonPung       FanKind = "箭刻"  // 2
	PrevalentWind    FanKind = "圈风刻" // 2
	SeatWind         FanKind = "门风刻" // 2
	ConcealedHand    FanKind = "门前清" // 2
	AllChows         FanKind = "平和"  // 2
	TileHog          FanKind = "四归一" // 2
	DoublePung       FanKind = "双同刻" // 2
	TwoConcealedPung FanKind = "双暗刻" // 2
	ConcealedKong    FanKind = "暗杠"  // 2
	AllSimples       FanKind = "断幺"  // 2

	// 1 分
	PureDoubleChow        FanKind = "一般高" // 1
	MixedDoubleChow       FanKind = "喜相逢" // 1
	ShortStraight         FanKind = "连六"  // 1
	TwoTerminalChows      FanKind = "老少副" // 1
	PungOfTerminalsOrHons FanKind = "幺九刻" // 1
	MeldedKong            FanKind = "明杠"  // 1
	OneVoidedSuit         FanKind = "缺一门" // 1
	NoHonors              FanKind = "无字"  // 1
	EdgeWait              FanKind = "边张"  // 1
	ClosedWait            FanKind = "坎张"  // 1
	SingleWait            FanKind = "单钓将" // 1
	SelfDrawn             FanKind = "自摸"  // 1
)

// fanValue 各番种的番值（国标番种表 1998 口径）。
var fanValue = map[FanKind]int{
	BigFourWinds: 88, BigThreeDragons: 88, AllGreen: 88, NineGates: 88,
	FourKongs: 88, SevenShiftedPair: 88, ThirteenOrphans: 88,

	AllTerminals: 64, LittleFourWinds: 64, LittleThreeDragons: 64,
	AllHonors: 64, FourConcealedPungs: 64, PureTerminalChows: 64,

	QuadrupleChow: 48, FourPureShiftPung: 48,

	FourShiftedChows: 32, ThreeKongs: 32, AllTerminalsAndHonours: 32,

	SevenPairs: 24, GreaterHonorsAndKnitted: 24, AllEvenPungs: 24,
	FullFlush: 24, PureTripleChow: 24, PureShiftedPungs: 24,
	UpperTiles: 24, MiddleTiles: 24, LowerTiles: 24,

	PureStraight: 16, ThreeSuitedTerminalChows: 16, PureShiftedChows: 16,
	AllFives: 16, TriplePung: 16, ThreeConcealedPungs: 16,

	LesserHonorsAndKnitted: 12, UpperFour: 12, KnittedStraight: 12,
	LowerFour: 12, BigThreeWinds: 12,

	MixedStraight: 8, ReversibleTiles: 8, MixedTripleChow: 8,
	MixedShiftedPungs: 8, ChickenHand: 8, LastTileDraw: 8,
	LastTileClaim: 8, OutWithReplacement: 8, RobbingTheKong: 8,
	TwoConcealedKongs: 8,

	AllPungs: 6, HalfFlush: 6, MixedShiftedChows: 6, AllTypes: 6,
	MeldedHand: 6, TwoDragons: 6, MeldedKongs: 6,

	OutsideHand: 4, FullyConcealedHand: 4, TwoMeldedKongs: 4, LastTile: 4,

	DragonPung: 2, PrevalentWind: 2, SeatWind: 2, ConcealedHand: 2,
	AllChows: 2, TileHog: 2, DoublePung: 2, TwoConcealedPung: 2,
	ConcealedKong: 2, AllSimples: 2,

	PureDoubleChow: 1, MixedDoubleChow: 1, ShortStraight: 1,
	TwoTerminalChows: 1, PungOfTerminalsOrHons: 1, MeldedKong: 1,
	OneVoidedSuit: 1, NoHonors: 1, EdgeWait: 1, ClosedWait: 1,
	SingleWait: 1, SelfDrawn: 1,
}

// FanHit 一条计番结果：番种 + 命中数。
// 大多数番种命中数为 1；可累计番种（幺九刻、四归一、明杠、暗杠等）为实际个数，
// 番值为 番值 × 命中数。
type FanHit struct {
	Kind  FanKind `json:"kind"`
	Count int     `json:"count"`
	Value int     `json:"value"` // 番值合计（fanValue[Kind] × Count）
}

// String 例：「清一色」「幺九刻×3」。
func (h FanHit) String() string {
	if h.Count <= 1 {
		return string(h.Kind)
	}
	return string(h.Kind) + "×" + strconv.Itoa(h.Count)
}

// FanScore 一次计分的完整结果（某分解）。
type FanScore struct {
	Fans  []FanHit `json:"fans"`  // 选定组合的番种清单（按番值降序）
	Total int      `json:"total"` // 总番数
}

// PayoutKind 结算方式。
type PayoutKind string

const (
	// PayoutDiscard 点炮（含抢杠和）：放铳者付番种分＋8 底分，其余两家各付 8 底分。
	PayoutDiscard PayoutKind = "discard"
	// PayoutSelfDrawn 自摸：三家各付（番种分＋8 底分）。
	PayoutSelfDrawn PayoutKind = "selfDraw"
	// PayoutExhaustive 流局：无人付分。
	PayoutExhaustive PayoutKind = "exhaustive"
)

// Settlement 一局得分（四家，零和）。
type Settlement struct {
	Winner   int        `json:"winner"`   // 胜者座位；流局为 -1
	Kind     PayoutKind `json:"kind"`     // 结算方式
	Feeder   int        `json:"feeder"`   // 放铳者（自摸/流局为 -1）
	FanScore int        `json:"fanScore"` // 番种分（总番数）
	Base     int        `json:"base"`     // 底分（8）
	Deltas   [4]int     `json:"deltas"`   // 四家得分变化（零和）
}

// breakdown 各番种的番值合计（供展示/测试）。
func (s FanScore) breakdown() map[FanKind]int {
	m := make(map[FanKind]int, len(s.Fans))
	for _, f := range s.Fans {
		m[f.Kind] += f.Value
	}
	return m
}
