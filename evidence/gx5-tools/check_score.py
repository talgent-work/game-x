#!/usr/bin/env python3
# GX-5 计分机械一致性核对：对带 score 的 dump 检查番值/合计/起和线/结算公式/零和。
import json, sys, glob
from collections import Counter

FAN_VALUE = {
 "大四喜":88,"大三元":88,"绿一色":88,"九莲宝灯":88,"四杠":88,"连七对":88,"十三幺":88,
 "清幺九":64,"小四喜":64,"小三元":64,"字一色":64,"四暗刻":64,"一色双龙会":64,
 "一色四同顺":48,"一色四节高":48,
 "一色四步高":32,"三杠":32,"混幺九":32,
 "七对":24,"七星不靠":24,"全双刻":24,"清一色":24,"一色三同顺":24,"一色三节高":24,"全大":24,"全中":24,"全小":24,
 "清龙":16,"三色双龙会":16,"一色三步高":16,"全带五":16,"三同刻":16,"三暗刻":16,
 "全不靠":12,"大于五":12,"组合龙":12,"小于五":12,"三风刻":12,
 "花龙":8,"推不倒":8,"三色三同顺":8,"三色三节高":8,"无番和":8,
 "妙手回春":8,"海底捞月":8,"杠上开花":8,"抢杠和":8,"双暗杠":8,
 "碰碰和":6,"混一色":6,"三色三步高":6,"五门齐":6,"全求人":6,"双箭刻":6,"一明一暗":6,
 "全带幺":4,"不求人":4,"双明杠":4,"和绝张":4,
 "箭刻":2,"圈风刻":2,"门风刻":2,"门前清":2,"平和":2,"四归一":2,"双同刻":2,"双暗刻":2,"暗杠":2,"断幺":2,
 "一般高":1,"喜相逢":1,"连六":1,"老少副":1,"幺九刻":1,"明杠":1,"缺一门":1,"无字":1,
 "边张":1,"坎张":1,"单钓将":1,"自摸":1,
}

def check(path, issues, stats):
    d = json.load(open(path))
    s = d.get("score")
    if s is None:
        return
    r = d["result"]
    if s.get("exhaustive"):
        stats["draw"] += 1
        st = s.get("settlement") or {}
        if any(x != 0 for x in (st.get("deltas") or [])):
            issues.append(f"种子 {d['seed']}：流局但结算非零 {st.get('deltas')}")
        if r.get("winner") != -1:
            issues.append(f"种子 {d['seed']}：流局但 winner != -1")
        return
    stats["win"] += 1
    best = s.get("best") or {}
    fans = best.get("fans") or []
    total = best.get("total")
    # 1. 番值与合计
    sum_v = 0
    kinds = Counter()
    for f in fans:
        v = FAN_VALUE.get(f["kind"])
        if v is None:
            issues.append(f"种子 {d['seed']}：未知番种 {f['kind']}")
            continue
        cnt = f.get("count", 1)
        if cnt < 1:
            issues.append(f"种子 {d['seed']}：番种 {f['kind']} count<1")
        if f.get("value") != v * cnt:
            issues.append(f"种子 {d['seed']}：番种 {f['kind']} 分项 {f.get('value')} ≠ {v}×{cnt}")
        kinds[f["kind"]] += 1
        sum_v += f.get("value", v * cnt)
    if sum_v != total:
        issues.append(f"种子 {d['seed']}：总番 {total} ≠ 分项和 {sum_v}")
    # 2. 起和线（无番和 8 分兜底在 GX-3 内）
    if s.get("eligible") != (total >= 8):
        issues.append(f"种子 {d['seed']}：eligible={s.get('eligible')} 与总番 {total} 不符")
    if not s.get("eligible"):
        stats["ineligible_settled"] += 1
    # 3. 结算公式
    st = s.get("settlement") or {}
    deltas = st.get("deltas") or []
    w = st.get("winner")
    fanScore = st.get("fanScore")
    base = st.get("base", 8)
    kind = st.get("kind")
    src = r.get("source")
    if w != r.get("winner"):
        issues.append(f"种子 {d['seed']}：score.winner {w} != result.winner {r.get('winner')}")
    if fanScore != total:
        issues.append(f"种子 {d['seed']}：fanScore {fanScore} != total {total}")
    if len(deltas) != 4 or sum(deltas) != 0:
        issues.append(f"种子 {d['seed']}：deltas 非零和或长度错：{deltas}")
    expect_kind = {0: "selfDraw", 1: "discard", 2: "robkong"}.get(src)
    if expect_kind and kind != expect_kind and not (expect_kind == "robkong" and kind == "discard"):
        issues.append(f"种子 {d['seed']}：结算种类 {kind} 与和牌方式 {expect_kind} 不符")
    if kind == "selfDraw":
        exp = [-(fanScore + base)] * 4
        exp[w] = 3 * (fanScore + base)
        if deltas != exp:
            issues.append(f"种子 {d['seed']}：自摸结算公式不符：{deltas} ≠ {exp}")
    elif kind in ("discard", "robkong"):
        f_ = st.get("feeder")
        exp = [-base] * 4
        exp[w] = fanScore + 3 * base
        if f_ is not None and 0 <= f_ < 4:
            exp[f_] = -(fanScore + base)
        if deltas != exp:
            issues.append(f"种子 {d['seed']}：{kind} 结算公式不符：{deltas} ≠ {exp}")
        if r.get("feeder") != st.get("feeder"):
            issues.append(f"种子 {d['seed']}：feeder 不一致 {st.get('feeder')} vs {r.get('feeder')}")
    # 4. 番种互斥抽查：全求人×自摸 共现统计
    kk = set(kinds)
    if "全求人" in kk and "自摸" in kk:
        stats["meldedhand_selfdraw"] += 1
    if "不求人" in kk and "自摸" not in kk:
        issues.append(f"种子 {d['seed']}：不求人但非自摸")
    if "门前清" in kk and any(k in kk for k in ("全求人",)):
        issues.append(f"种子 {d['seed']}：门前清与全求人共现")
    # 单钓将与全求人共现统计（判定不一致线索）
    if "全求人" in kk and "单钓将" in kk:
        stats["meldedhand_singlewait"] += 1
    if "全求人" in kk and "单钓将" not in kk:
        stats["meldedhand_nosinglewait"] += 1

if __name__ == "__main__":
    issues = []
    stats = Counter()
    files = sys.argv[1:] or glob.glob("/workspace/gx5-evidence-rob/seed-*.json")
    for f in files:
        try:
            check(f, issues, stats)
        except Exception as e:
            issues.append(f"{f}: 解析异常 {e}")
    print("局数统计：", dict(stats))
    print(f"问题 {len(issues)} 条：")
    for i in issues[:40]:
        print("  -", i)
