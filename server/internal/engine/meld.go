package engine

// MeldKind 副子（面子）种类：吃、碰、杠。
type MeldKind uint8

const (
	MeldChi  MeldKind = iota // 吃：以上家打出的牌组成的顺子
	MeldPong                 // 碰：以别人打出的牌组成的刻子
	MeldKong                 // 杠：明杠 / 暗杠 / 补杠（出处见 Kong）
)

// KongFrom 杠的出处：明杠与暗杠对计分（GX-3）含义不同，引擎如实记录。
type KongFrom uint8

const (
	KongFromDiscard   KongFrom = iota // 明杠（直杠）：别人打出的牌 + 手中 3 张
	KongFromConcealed                 // 暗杠：手中 4 张自行开杠
	KongFromPromote                   // 补杠（加杠）：向已碰的刻子补第 4 张
)

// Meld 一副已亮出的副子。公开信息——但暗杠的牌面除外：他人视图中牌面被抹去
// （见 ViewFor），局面终了后在 Result 中全部揭示。
type Meld struct {
	Kind  MeldKind `json:"kind"`
	Tiles []Tile   `json:"tiles"` // 吃/碰：3 张；杠：4 张。吃按点数升序、碰/杠同值。
	From  int      `json:"from"`  // 供牌座位：吃/碰/明杠＝出牌者；暗杠＝-1（自行）；补杠沿用原碰的供牌座位
	Kong  KongFrom `json:"kong"`  // 仅 Kind=MeldKong 时有意义
}

// HideTile 面子的牌面是否对外隐藏（暗杠）。
func (m Meld) HideTile() bool {
	return m.Kind == MeldKong && m.Kong == KongFromConcealed
}

// cloneMeld 深拷贝一副面子（牌面切片独立），供快照/事件/结果对外输出——
// 调用方改动拷贝不得影响引擎内部状态。
func cloneMeld(m Meld) Meld {
	c := m
	c.Tiles = append([]Tile(nil), m.Tiles...)
	return c
}

// meldPtr 取一副面子的堆上指针（事件字段用）。
func meldPtr(m Meld) *Meld { return &m }

// cloneMelds 深拷贝一副子列表。
func cloneMelds(ms []Meld) []Meld {
	if len(ms) == 0 {
		return nil
	}
	out := make([]Meld, len(ms))
	for i, m := range ms {
		out[i] = cloneMeld(m)
	}
	return out
}
