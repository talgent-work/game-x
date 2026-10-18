package protocol

import (
	"fmt"

	"talgent-work/game-x/server/internal/engine"
)

// ToEngine 把线上动作翻译为引擎动作。只做「形状校验」：kind 已知、必填字段齐备、牌值合法；
// 规则合法性（回合、手牌、时机、上家、对子……）一律留给 engine.Apply 判定，表层不复制规则。
func (a Action) ToEngine() (engine.Action, error) {
	var out engine.Action
	switch a.Kind {
	case KindDiscard:
		if a.Tile == nil || !a.Tile.Valid() {
			return out, fmt.Errorf("discard 需要合法的 tile")
		}
		out = engine.Action{Kind: engine.ActDiscard, Tile: *a.Tile}
	case KindChi:
		if len(a.With) != 2 || !a.With[0].Valid() || !a.With[1].Valid() {
			return out, fmt.Errorf("chi 需要恰两张合法的 with 手牌")
		}
		out = engine.Action{Kind: engine.ActChi, With: append([]engine.Tile(nil), a.With...)}
	case KindPong:
		out = engine.Action{Kind: engine.ActPong}
	case KindKongDiscard:
		out = engine.Action{Kind: engine.ActKongDiscard}
	case KindKongConcealed:
		if a.Tile == nil || !a.Tile.Valid() {
			return out, fmt.Errorf("kongConcealed 需要合法的 tile")
		}
		out = engine.Action{Kind: engine.ActKongConcealed, Tile: *a.Tile}
	case KindKongPromote:
		if a.Tile == nil || !a.Tile.Valid() {
			return out, fmt.Errorf("kongPromote 需要合法的 tile")
		}
		out = engine.Action{Kind: engine.ActKongPromote, Tile: *a.Tile}
	case KindWin:
		out = engine.Action{Kind: engine.ActWin}
	case KindPass:
		out = engine.Action{Kind: engine.ActPass}
	default:
		return out, fmt.Errorf("未知动作 %q", a.Kind)
	}
	return out, nil
}

// FromEngine 把引擎动作翻译为线上动作（state.actions 用）。
func FromEngine(a engine.Action) Action {
	out := Action{}
	switch a.Kind {
	case engine.ActDiscard:
		t := a.Tile
		out = Action{Kind: KindDiscard, Tile: &t}
	case engine.ActChi:
		out = Action{Kind: KindChi, With: append([]engine.Tile(nil), a.With...)}
	case engine.ActPong:
		out = Action{Kind: KindPong}
	case engine.ActKongDiscard:
		out = Action{Kind: KindKongDiscard}
	case engine.ActKongConcealed:
		t := a.Tile
		out = Action{Kind: KindKongConcealed, Tile: &t}
	case engine.ActKongPromote:
		t := a.Tile
		out = Action{Kind: KindKongPromote, Tile: &t}
	case engine.ActWin:
		out = Action{Kind: KindWin}
	case engine.ActPass:
		out = Action{Kind: KindPass}
	}
	return out
}

// EventKindString 事件 kind 的线上拼写；未知返回空串（调用方据此丢弃，不外发）。
func EventKindString(k engine.EventKind) string {
	switch k {
	case engine.EvDiscarded:
		return EvDiscarded
	case engine.EvDrawn:
		return EvDrawn
	case engine.EvMeld:
		return EvMeld
	case engine.EvKongPromoted:
		return EvKongPromoted
	case engine.EvWin:
		return EvWin
	case engine.EvExhaustive:
		return EvExhaustive
	}
	return ""
}

// DrawSourceString 摸牌来源的线上拼写；未知返回空串。
func DrawSourceString(s engine.DrawSource) string {
	switch s {
	case engine.DrawInitial:
		return DrawInitial
	case engine.DrawWall:
		return DrawWall
	case engine.DrawKong:
		return DrawKong
	}
	return ""
}
