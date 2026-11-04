#!/usr/bin/env python3
"""P3.7 批 2 步 2b：sprite 排序改委托 camera 的逐位对拍语料生成器。

want_order 一律由「改委托之前」的旧判据离线复算：
  flat: layer 升序 → feetY 升序 → 输入位置（稳定）
  deep: depth 降序 → layer 升序 → feetY 升序 → 输入位置（稳定）
新实现（委托 camera 两级键）必须逐元素复现这份序，否则动的就是行为不是实现。

用法：python3 scripts/gen_p37_zsort_parity.py
输出：engine/sprite/testdata/p37_zsort_parity.json
"""
import json
import os

OUT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                   "engine", "sprite", "testdata", "p37_zsort_parity.json")


def legacy_flat_order(items):
    return [it["name"] for _, it in sorted(enumerate(items),
                                           key=lambda p: (p[1]["layer"], p[1]["feetY"]))]


def legacy_deep_order(items):
    return [it["name"] for _, it in sorted(enumerate(items),
                                           key=lambda p: (-p[1]["depth"], p[1]["layer"], p[1]["feetY"]))]


def case(name, mode, items):
    want = legacy_deep_order(items) if mode == "deep" else legacy_flat_order(items)
    return {"name": name, "mode": mode, "items": items, "want_order": want}


def flat(name, layer, feet_y, idx):
    return {"name": "%s%d" % (name, idx), "layer": layer, "feetY": feet_y}


def deep(name, depth, layer, feet_y, idx):
    return {"name": "%s%d" % (name, idx), "depth": depth, "layer": layer, "feetY": feet_y}


def lcg(seed):
    while True:
        seed = (seed * 6364136223846793005 + 1442695040888963407) & ((1 << 64) - 1)
        yield seed >> 33


cases = []

# 1) 基本带序与带内脚位序（stage_chase/main.go:222-225 的四元组同形）
cases.append(case("bands_basic", "flat", [
    flat("near", 0, 300.0, 0), flat("far", 0, 100.0, 1),
    flat("fx", 1, 50.0, 2), flat("ui", 2, 0.0, 3)]))
# 2) 同带同脚位：输入序必须原样保留
cases.append(case("same_band_same_feet", "flat", [
    flat("a", 0, 10.0, 0), flat("b", 0, 10.0, 1), flat("c", 0, 10.0, 2)]))
# 3) 负带与自定义带（整数带不受三档命名限制）
cases.append(case("custom_negative_bands", "flat", [
    flat("under", -3, 500.0, 0), flat("world", 0, 1.0, 1),
    flat("fx", 1, 999.0, 2), flat("over", 7, -2.0, 3)]))
# 4) 极端数值不许变 NaN/Inf
cases.append(case("extremes", "flat", [
    flat("hi", 0, 1e308, 0), flat("lo", 0, -1e308, 1), flat("mid", 0, 0.0, 2)]))
# 5) 亚像素级但仍在判等容差之外的脚位差（|Δy| 必须大于 1e-5·max(1,|y|)，
#    否则新判据会按平票处理 —— 那属于本批已登记的刻意行为差，不进对拍语料）
cases.append(case("gap_outside_tolerance", "flat", [
    flat("p", 0, 100.0, 0), flat("q", 0, 100.5, 1), flat("r", 0, 99.5, 2)]))

# 6) 深度家族：depth 压过 band
cases.append(case("depth_beats_band", "deep", [
    deep("ui_near", 100.0, 2, 5.0, 0), deep("world_far", 900.0, 0, 5.0, 1)]))
# 7) 同深度跨带
cases.append(case("same_depth_bands", "deep", [
    deep("u", 500.0, 2, 1.0, 0), deep("f", 500.0, 1, 1.0, 1), deep("w", 500.0, 0, 1.0, 2)]))
# 8) 同深度同带同脚位
cases.append(case("same_depth_full_tie", "deep", [
    deep("x", 700.0, 0, 42.0, 0), deep("y", 700.0, 0, 42.0, 1)]))
# 9) stage_chase 换算式：depth = 4000-(y+h)，feetY = y+h ⇒ depth 降序 ≡ feetY 升序
sc = []
for i, (y, h) in enumerate([(0, 60), (0, 60), (120, 40), (350, 20), (-80, 100), (600, 10)]):
    feet = y + h
    sc.append(deep("sc", 4000.0 - feet, 0, float(feet), i))
cases.append(case("stage_chase_conversion", "deep", sc))

# 10) 1000 prop 场景（stage_chase/main.go:899-910 同形：LayerWorld + depth=4000-(y+h)）
rng = lcg(20261104)
props = []
for i in range(1000):
    r1 = next(rng)
    r2 = next(rng)
    y = float(r1 % 1200) - 600.0
    h = float(8 + (r2 % 57))
    feet = y + h
    props.append(deep("p", 4000.0 - feet, 0, feet, i))
cases.append(case("props_1000", "deep", props))

# 11) 1000 prop 场景 + 三带混合（把 band 轴也压满）
rng = lcg(777)
mixed = []
for i in range(1000):
    r1 = next(rng)
    r2 = next(rng)
    band = [0, 1, 2][r2 % 3]
    y = float(r1 % 1200) - 600.0
    feet = y + float(8 + (r2 % 57))
    # 同带内故意让一部分深度落在整数网格上（同深度平票）
    depth = float(int(4000.0 - feet)) if (r2 % 7 == 0) else 4000.0 - feet
    mixed.append(deep("m", depth, band, feet, i))
cases.append(case("props_1000_mixed_bands", "deep", mixed))

payload = {
    "note": ("P3.7 批2 步2b 逐位对拍语料。want_order 由改委托前的旧判据离线复算"
             "（flat: layer→feetY→输入稳定序；deep: depth 降→layer→feetY→输入稳定序），"
             "新实现委托 camera 两级键后必须逐元素复现。生成脚本 scripts/gen_p37_zsort_parity.py。"),
    "cases": cases,
}

with open(OUT, "w", encoding="utf-8") as f:
    json.dump(payload, f, ensure_ascii=False, indent=1)
print("wrote %s (%d cases)" % (os.path.relpath(OUT), len(cases)))
