// Package protocol 定义 GX-4 本地原型的线上消息集合（JSON over WebSocket）。
//
// Go 结构体是本协议的唯一源头：tygo 据此生成 client/src/protocol/（勿手改）。
// 约定（依 GX-1《技术选型报告》§5.2 与 GX-4 设计定稿）：
//   - 动作与事件的 kind 用可读字符串；engine 载荷（快照/副子/结果/牌）原样复用其 JSON 形状；
//   - state 载荷只含该座位可见信息（快照直接取自 engine.ViewFor）；
//   - rejected 的 reason 是中文人话，直接展示，客户端不得解析（演进以加字段为主）。
package protocol

import (
	"talgent-work/game-x/server/internal/engine"
	"talgent-work/game-x/server/internal/score"
)

// Version 协议版本；hello 必须携带且相等，否则拒绝并挂断。
const Version = 1

// 消息类型（Msg.Type）。
const (
	TypeHello    = "hello"    // C→S：绑定座位
	TypeAction   = "action"   // C→S：提交动作意图
	TypeState    = "state"    // S→C：该座位可见快照 + 合法动作
	TypeEvent    = "event"    // S→C：按可见性分发的事件
	TypeRejected = "rejected" // S→C：拒绝（只回发起连接）
)

// 动作 kind（与 engine.ActionKind 一一映射，见 codec.go）。
const (
	KindDiscard       = "discard"
	KindChi           = "chi"
	KindPong          = "pong"
	KindKongDiscard   = "kongDiscard"   // 明杠
	KindKongConcealed = "kongConcealed" // 暗杠
	KindKongPromote   = "kongPromote"   // 补杠（开抢杠窗口）
	KindWin           = "win"
	KindPass          = "pass" // 声明窗口：过
)

// 事件 kind（与 engine.EventKind 一一映射，见 codec.go）。
const (
	EvDiscarded    = "discarded"
	EvDrawn        = "drawn"
	EvMeld         = "meld"
	EvKongPromoted = "kongPromoted"
	EvWin          = "win"
	EvExhaustive   = "exhaustive"
)

// 摸牌来源（Event.Source，drawn 专用）。
const (
	DrawInitial = "initial"
	DrawWall    = "wall"
	DrawKong    = "kong"
)

// 拒绝代码（Msg.Code）。
const (
	CodeBadJSON       = "bad_json"
	CodeUnknownType   = "unknown_type"
	CodeBadVersion    = "bad_version"
	CodeBadSeat       = "bad_seat"
	CodeAlreadyBound  = "already_bound"
	CodeSeatTaken     = "seat_taken"
	CodeNotBound      = "not_bound"
	CodeBadAction     = "bad_action"
	CodeIllegalAction = "illegal_action" // engine.Apply 拒绝（reason 为引擎原文）
	CodeGameOver      = "game_over"
)

// Msg 线上信封（C→S 与 S→C 共用；平铺字段，TS 端一个 interface）。
// 未知字段一律忽略；Type 与载荷一一对应（state→State、event→Event、rejected→Code/Reason）。
type Msg struct {
	Type    string  `json:"type"`
	Version int     `json:"version,omitempty"` // hello
	Seat    *int    `json:"seat,omitempty"`    // hello：请求座位（指针区分「缺省」与座位 0）
	Action  *Action `json:"action,omitempty"`  // action
	State   *State  `json:"state,omitempty"`   // state
	Event   *Event  `json:"event,omitempty"`   // event
	Code    string  `json:"code,omitempty"`    // rejected
	Reason  string  `json:"reason,omitempty"`  // rejected
}

// Action 动作意图（state.actions 与 C→S 的 action 同形）。
type Action struct {
	Kind string        `json:"kind"`
	Tile *engine.Tile  `json:"tile,omitempty"` // discard / kongConcealed / kongPromote
	With []engine.Tile `json:"with,omitempty"` // chi：与被吃之牌合成顺子的两张手牌
}

// State 某座位可见的完整局面与该座位当前全部合法动作。
// 构造函数保证 Actions 恒为数组（空为 []，不为 null）。
type State struct {
	Snapshot engine.Snapshot    `json:"snapshot"`
	Actions  []Action           `json:"actions"`
	Score    *score.ScoreResult `json:"score,omitempty"` // 终局（和牌/流局）state 携带 GX-3 计分与结算（东风圈常量）
}

// Event 引擎事件的「某座位可见形状」。
type Event struct {
	Kind   string         `json:"kind"`
	Seat   int            `json:"seat"`             // 事件主角（摸牌/打牌/吃碰杠/和牌）
	Tile   *engine.Tile   `json:"tile,omitempty"`   // drawn：仅摸牌者本人可见；discarded/kongPromoted：公开
	Source string         `json:"source,omitempty"` // drawn：initial|wall|kong
	Meld   *engine.Meld   `json:"meld,omitempty"`   // meld：他人暗杠的牌面已抹去
	Result *engine.Result `json:"result,omitempty"` // win|exhaustive：全揭示
}

// StateMsg 构造 state 消息（Actions 恒为数组）。
func StateMsg(snap engine.Snapshot, acts []Action) Msg {
	if acts == nil {
		acts = []Action{}
	}
	return Msg{Type: TypeState, State: &State{Snapshot: snap, Actions: acts}}
}

// EventMsg 构造 event 消息。
func EventMsg(ev Event) Msg { return Msg{Type: TypeEvent, Event: &ev} }

// Rejected 构造拒绝消息。
func Rejected(code, reason string) Msg { return Msg{Type: TypeRejected, Code: code, Reason: reason} }
