#!/usr/bin/env python3
# GX-5 规则对照回放核对器：对 --dump JSON 逐局检查流转不变式。
# 用法: check_dump.py <dump.json> [dump.json ...]
import json, sys
from collections import Counter

SEATS = 4
SUIT_NAMES = {0: "万", 1: "饼", 2: "条", 3: "东", 4: "南", 5: "西", 6: "北", 7: "中", 8: "发", 9: "白"}
WINSRC = {0: "自摸", 1: "点和", 2: "抢杠和"}

def tkey(t):
    return (t["suit"], t["rank"])

def tname(t):
    s, r = t["suit"], t["rank"]
    return f"{r}{SUIT_NAMES[s]}" if s <= 2 else SUIT_NAMES[s]

def is_run(tiles):
    if len(tiles) != 3 or any(t["suit"] != tiles[0]["suit"] for t in tiles):
        return False
    if tiles[0]["suit"] > 2:
        return False
    rs = sorted(t["rank"] for t in tiles)
    return rs[1] == rs[0] + 1 and rs[2] == rs[0] + 2

def is_triplet(tiles):
    return len(tiles) == 3 and len({tkey(t) for t in tiles}) == 1

def is_quad(tiles):
    return len(tiles) == 4 and len({tkey(t) for t in tiles}) == 1

def prev(seat):
    return (seat + 3) % 4

def check(path):
    d = json.load(open(path))
    problems = []
    warn = problems.append
    ev = d["events"]
    r = d["result"]
    dealer = d["dealer"]

    # ---- 1. 首事件：庄家弃牌，或庄家开局暗杠（摸第 14 张后）----
    first_ok = ev[0]["kind"] == "discarded" and ev[0]["seat"] == dealer
    if not first_ok and not (ev[0]["kind"] == "meld" and ev[0]["seat"] == dealer
                             and ev[0]["meld"]["kind"] == 2 and ev[0]["meld"].get("kong") == 1):
        warn(f"首事件非庄家弃牌/开局暗杠：{ev[0]}")

    # ---- 2. 期望状态机回放 ----
    # state: ('after_discard', last_discarder) | ('after_meld', seat) |
    #        ('after_kong', seat) 等补牌 | ('after_draw', seat)
    state = ("begin", None)
    last_disc = {}      # seat -> 最后一张未声明的弃牌（声明后移除）
    river = Counter()   # 重建的河（未被声明取走的弃牌）
    drawn = Counter()   # seat -> 摸牌次数（不含庄家起手）
    drawn_tiles = []
    melds_seen = []
    for i, e in enumerate(ev):
        k = e["kind"]
        if k == "discarded":
            s = e["seat"]
            if state[0] in ("after_discard",):
                pass  # 有声明取走了弃牌，声明者打牌
            elif state[0] == "after_meld":
                if s != state[1]:
                    warn(f"副子后应由声明者 {state[1]} 打牌，实际 {s}（#{i}）")
            elif state[0] == "after_draw":
                if s != state[1]:
                    warn(f"摸牌后应由 {state[1]} 打牌，实际 {s}（#{i}）")
            elif state[0] == "after_kong_draw":
                if s != state[1]:
                    warn(f"杠后补牌后应由 {state[1]} 打牌，实际 {s}（#{i}）")
            elif state[0] == "begin":
                pass
            last_disc[s] = e["tile"]
            river[tkey(e["tile"])] += 1
            state = ("after_discard", s)
        elif k == "drawn":
            s = e["seat"]
            if state[0] == "after_discard":
                expect = (state[1] + 1) % 4
                if s != expect:
                    warn(f"轮转错：应由 {expect} 摸，实际 {s}（#{i}）")
            elif state[0] == "after_kong":
                if s != state[1]:
                    warn(f"杠后应由 {state[1]} 补牌，实际 {s}（#{i}）")
                if e.get("source") != "kong":
                    warn(f"杠后补牌来源非 kong（牌尾）：{e.get('source')}（#{i}）")
            else:
                warn(f"意外摸牌：座位 {s}，前置状态 {state}（#{i}）")
            drawn[s] += 1
            drawn_tiles.append(e["tile"])
            state = ("after_draw", s)
        elif k == "meld":
            m = e["meld"]
            s, frm, tiles, mk = e["seat"], m["from"], m["tiles"], m["kind"]
            kongfrom = m.get("kong", 0)
            if mk in (0, 1):
                # 吃/碰只能声明他人弃牌
                if state[0] != "after_discard":
                    warn(f"吃/碰时机错：前置状态 {state}（#{i}）")
            else:  # 杠
                if kongfrom == 0:  # 明杠：他人弃牌
                    if state[0] != "after_discard":
                        warn(f"明杠时机错：前置状态 {state}（#{i}）")
                elif kongfrom == 1:  # 暗杠：自己回合内（含庄家开局摸后）
                    if state[0] not in ("after_draw", "begin"):
                        warn(f"暗杠时机错：前置状态 {state}（#{i}）")
                    if state[0] == "begin" and s != dealer:
                        warn(f"开局暗杠者非庄家：{s}（#{i}）")
                    if frm != -1:
                        warn(f"暗杠来源应为 -1，实际 {frm}（#{i}）")
                elif kongfrom == 2:  # 补杠：自己回合内，且此前应有 kongPromoted 事件
                    if state[0] not in ("after_promote",):
                        warn(f"补杠时机错：前置状态 {state}（#{i}）")
                if not is_quad(tiles):
                    warn(f"杠的牌不成四同：{[tname(t) for t in tiles]}（#{i}）")
            if mk == 0:  # 吃
                if frm != prev(s):
                    warn(f"吃非上家：座位 {s} 吃座位 {frm}（#{i}）")
                if not is_run(tiles):
                    warn(f"吃的牌不成顺子：{[tname(t) for t in tiles]}（#{i}）")
            elif mk == 1:  # 碰
                if not is_triplet(tiles):
                    warn(f"碰的牌不成刻子：{[tname(t) for t in tiles]}（#{i}）")
            # 从河里取走被声明的弃牌（吃/碰/明杠）
            if mk in (0, 1) or (mk == 2 and kongfrom == 0):
                claim = last_disc.get(frm)
                if claim is None:
                    warn(f"声明来源无弃牌记录：座位 {frm}（#{i}）")
                else:
                    if tkey(claim) not in {tkey(t) for t in tiles}:
                        warn(f"声明牌与来源弃牌不符：弃 {tname(claim)}，副子 {[tname(t) for t in tiles]}（#{i}）")
                    river[tkey(claim)] -= 1
                    last_disc[frm] = None
            melds_seen.append((s, mk, frm, kongfrom))
            state = ("after_meld", s) if mk in (0, 1) else ("after_kong", s)
        elif k == "kongPromoted":
            s = e["seat"]
            if state[0] != "after_draw":
                warn(f"补杠时机错：前置状态 {state}（#{i}）")
            state = ("after_promote", s)
        elif k == "win":
            if state[0] == "after_promote" and not r.get("exhaustive") and r.get("source") != 2:
                warn(f"声明窗口为补杠却被普通和牌收束（#{i}）")
            state = ("end", None)
        elif k == "exhaustive":
            state = ("end", None)
        if state[0] == "end":
            if i != len(ev) - 1:
                warn(f"终局事件后仍有事件（#{i}/{len(ev)-1}）")
            break

    # ---- 3. 终局一致性 ----
    if r.get("exhaustive"):
        if ev[-1]["kind"] != "exhaustive":
            warn(f"流局终事件非 exhaustive：{ev[-1]['kind']}")
        wall_left = 84 - 1 - sum(drawn.values())  # 84 为发牌后牌墙，-1 庄家起手摸
        if wall_left != 0:
            warn(f"流局但牌墙余 {wall_left} 张")
    else:
        if ev[-1]["kind"] != "win":
            warn(f"和牌终事件非 win：{ev[-1]['kind']}")
        wall_left = 84 - 1 - sum(drawn.values())
        if wall_left < 0:
            warn(f"牌墙摸超（余 {wall_left}）")

    # ---- 4. 牌张守恒：终局 hands+melds+river(+和牌张去向) + 牌墙余 = 136 ----
    total = Counter()
    for s in range(SEATS):
        for t in (r["hands"][s] or []):
            total[tkey(t)] += 1
        for m in (r["melds"][s] or []):
            for t in m["tiles"]:
                total[tkey(t)] += 1
    for t, c in river.items():
        total[t] += c
    if not r.get("exhaustive") and r.get("source") != 0:
        # 点和/抢杠：和牌张取自他人弃牌/被抢杠张，已计入 river 或由 winner 持有——
        # winnerHand 已含和牌张且 hands[winner] 应等于 winnerHand，避免重复计
        pass
    wall_left = 84 - 1 - sum(drawn.values())
    total_count = sum(total.values()) + max(wall_left, 0)
    if total_count != 136:
        warn(f"牌张守恒不符：清点 {sum(total.values())} + 牌墙 {wall_left} = {total_count} ≠ 136")
    over = {t: c for t, c in total.items() if c > 4}
    if over:
        warn(f"某牌超过 4 张：{ {tname({'suit': t[0], 'rank': t[1]}): c for t, c in over.items()} }")

    # ---- 5. 点和放铳者 = 最后弃牌者 ----
    last_discarder = None
    for e in ev:
        if e["kind"] == "discarded":
            last_discarder = e["seat"]
    if not r.get("exhaustive") and r.get("source") == 1:
        if r.get("feeder") != last_discarder:
            warn(f"点和放铳者 {r.get('feeder')} != 最后弃牌者 {last_discarder}")

    # ---- 6. winnerHand 语义：自摸时＝hands[winner]；点和/抢杠时＝hands[winner]＋和牌张 ----
    if not r.get("exhaustive") and r.get("winner") is not None and r.get("winner") >= 0:
        w = r.get("winner")
        wh = sorted(map(tkey, r.get("winnerHand") or []))
        hw = sorted(map(tkey, r["hands"][w] or []))
        if r.get("source") == 0:
            if wh != hw:
                warn("自摸 winnerHand 与 hands[winner] 不一致")
        else:
            wt = tkey(r["winningTile"])
            if sorted(hw + [wt]) != wh:
                warn("点和/抢杠 winnerHand ≠ hands[winner]+和牌张（语义核对）")
        wm = sorted(map(tkey, [t for m in (r.get("winnerMelds") or []) for t in m["tiles"]]))
        ml = sorted(map(tkey, [t for m in (r["melds"][w] or []) for t in m["tiles"]]))
        if wm != ml:
            warn("winnerMelds 与 melds[winner] 不一致")

    n_rej = sum(1 for a in d["actions"] if a.get("error"))
    return problems, dict(
        seed=d["seed"], actions=len(d["actions"]), rejected=n_rej,
        events=len(ev), melds=len(melds_seen),
        outcome="流局" if r.get("exhaustive") else f"座位{r['winner']}和({WINSRC[r.get('source')]})",
    )

if __name__ == "__main__":
    ok = bad = 0
    for path in sys.argv[1:]:
        try:
            problems, info = check(path)
        except Exception as ex:
            print(f"{path}: 解析失败 {ex}")
            bad += 1
            continue
        if problems:
            bad += 1
            print(f"种子 {info['seed']}: {info['outcome']} | {info['actions']}步/{info['rejected']}拒 | 问题 {len(problems)} 条")
            for p in problems[:12]:
                print(f"   - {p}")
        else:
            ok += 1
    print(f"\n== 通过 {ok} 局，异常 {bad} 局 ==")
