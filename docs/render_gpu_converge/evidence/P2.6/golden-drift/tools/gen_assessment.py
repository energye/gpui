#!/usr/bin/env python3
"""Generate assessment.md from analysis.jsonl + the per-window baseline map."""
from __future__ import annotations
import json
import os

root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
rows = [json.loads(l) for l in open(os.path.join(root, "analysis.jsonl"))]
native = {r["window"]: r for r in rows if r["backend"] == "native"}
go = {r["window"]: r for r in rows if r["backend"] == "go"}

# window: (baseline path, date, commit, reason)
META = {
    "ui_render_base_backdrop": ("examples/ui_render_base_backdrop/testdata/showcase_backdrop_base.png", "2026-10-19", "8ebf5698", "差异沿 backdrop/blur 卡边界与柔和渐变（edge 44.9%，内部 501/510 同），最大 21 在卡边过渡；基线早于 P4.2/P4.3 blur 对齐与 2026-11-11 SDF 覆盖 LUT。窗口 20/20 像素探针通过。"),
    "ui_render_base_cliprrect": ("examples/ui_render_base_cliprrect/testdata/showcase_cliprrect_base.png", "2026-10-21", "1bc7f803", "内部 510/510 同，差异仅在圆角/裁剪边沿（edge 23.0%），最大 17；基线早于 2026-11-11 SDF 覆盖 LUT。"),
    "ui_render_base_dither": ("examples/ui_render_base_dither/testdata/showcase_dither_base.png", "2026-10-25", "08d59211", "差异集中在扫掠/径向渐变卡：dither 把底层 1 LSB 覆盖精度变化放大成图案翻转；内部 399/510 同，差异点均为 1–3 LSB 邻域；卡 C 当前为连续平滑梯度、基线为旧量化带，最大 77 在渐变内部，不是缺内容。基线早于 SDF LUT。逻辑 16/16、像素 29/29 通过。"),
    "ui_render_base_filterops": ("examples/ui_render_base_filterops/testdata/showcase_filterops_base.png", "2026-10-20", "ab407ab0", "差异沿滤镜卡边缘与 diff 面板分布（diff 面板 ×64 放大 1 LSB；max 64 在 diff 面板），内部 502/510 同；基线早于 P4.2 blur 对齐、P4.3 filter 源清理与 SDF LUT。像素探针全过。"),
    "ui_render_base_maskops": ("examples/ui_render_base_maskops/testdata/showcase_maskops_base.png", "2026-10-19", "8db1592b", "内部 510/510 同，差异只在 mask 边沿（edge 32.6%），最大 11；基线早于 P4.3 mask 池化（应零行为变化）与 SDF LUT。"),
    "ui_render_base_miscdraw": ("examples/ui_render_base_miscdraw/testdata/showcase_miscdraw_base.png", "2026-10-25", "08d59211", "差异集中在 DrawShadow 卡与字形 AA 边沿：投影颜色整体变亮（P4.1 DrawShadow 设备空间/spot 模型变更），内部 458/510 同，最大 106 在投影内部但内容结构完整；基线早于 P4.1/P4.3。逻辑/像素探针全过。"),
    "ui_render_base_pathbool": ("examples/ui_render_base_pathbool/testdata/showcase_pathbool_base.png", "2026-11-02", "b93da030", "内部 509/510 同，差异沿布尔路径边沿（edge 25.6%），最大 10；基线早于 2026-11-11 SDF LUT。"),
    "ui_render_base_svg": ("examples/ui_render_base_svg/testdata/showcase_svg_base.png", "2026-11-11", "4a41c660", "0 差异；基线就是 2026-11-11 SDF LUT 批重种的图。对照组：同一引擎下唯一在 SDF LUT 后重种的窗正好 0。基线有效，不重种。"),
    "ui_render_base_textlow": ("examples/ui_render_base_textlow/testdata/showcase_textlow_base.png", "2026-10-25", "758d91f9", "差异沿字形 AA 边沿，内部 500/510 同，最大 13；基线早于 2026-11-11 SDF LUT。"),
    "ui_render_base_textspacing": ("examples/ui_render_base_textspacing/testdata/showcase_textspacing_base.png", "2026-10-25", "6a58f610", "差异沿字形边沿/字距边界，内部 489/510 同，最大 19；基线早于 2026-11-11 SDF LUT。"),
    "ui_render_base_textstroke": ("examples/ui_render_base_textstroke/testdata/showcase_textstroke_base.png", "2026-10-19", "b780dea7", "内部 510/510 同，差异只在描边边沿（edge 9.2% 因描边带宽），最大 22；基线早于 2026-11-11 SDF LUT。"),
    "ui_render_base_verticesmesh": ("examples/ui_render_base_verticesmesh/testdata/showcase_verticesmesh_base.png", "2026-10-20", "7a40b774", "内部 510/510 同，差异在网格三角形边沿（edge 52.8%），最大 8；基线早于 2026-11-11 SDF LUT。"),
    "ui_render_base_webp": ("examples/ui_render_base_webp/testdata/showcase_webp_base.png", "2026-10-20", "4f2f5fcd", "内部 504/510 同，差异在图片边缘/缩放采样（edge 65.1%），最大 12；基线早于 2026-11-11 SDF LUT。另有逻辑探针 format 期望过期（§6）。"),
    "ui_wr_shadermask": ("examples/ui_wr_shadermask/testdata/showcase_shadermask_base.png", "2026-10-27", "9327ff5c", "差异广泛但幅度极小（max 6，内部 487/510 同，差异多为 1 LSB），符合 P3.8 premul/alpha 精度变更 + SDF LUT；基线 2026-10-27。"),
    "ui_wr_p38_color_font": ("examples/ui_wr_p38_color_font/golden/showcase_color_font_base.png（go 用 _go_base）", "2026-11-10", "06573e5a", "内部 740/740 同，差异只在字形/卡片边沿（edge 18.4%），最大 10；基线在 P4.4 重种，但早于 2026-11-11 SDF LUT。"),
    "ui_wr_p38_picture": ("examples/ui_wr_p38_picture/golden/showcase_picture_base.png（go 用 _go_base）", "2026-11-07", "b028ae74", "内部 629/629 同，差异在图片/卡片边沿（edge 37.5%），最大 11；基线早于 P3.8 后续与 SDF LUT。"),
    "ui_wr_p38_svg_doc": ("examples/ui_wr_p38_svg_doc/golden/showcase_svg_doc_base.png", "2026-11-09", "972cc37c", "内部 629/629 同，差异在 SVG 图形边沿（edge 18.8%），最大 10；基线早于 2026-11-11 SDF LUT。"),
    "ui_wr_p38_svg_filter": ("examples/ui_wr_p38_svg_filter/golden/showcase_svg_filter_base.png", "2026-11-09", "972cc37c", "内部 629/629 同，差异在 SVG 滤镜/图形边沿（edge 19.0%），最大 10；基线早于 2026-11-11 SDF LUT。"),
    "ui_wr_p38_svg_paint": ("examples/ui_wr_p38_svg_paint/golden/showcase_svg_doc_base.png", "2026-11-09", "972cc37c", "内部 629/629 同，差异在 SVG paint-server 图形边沿（edge 19.0%），最大 10；基线早于 2026-11-11 SDF LUT。"),
    "ui_wr_p38_svg_showcase": ("examples/ui_wr_p38_svg_showcase/golden/showcase_showcase_base.png", "2026-11-09", "d5f9af60", "内部 629/629 同，差异在 SVG 图形边沿（edge 19.0%），最大 9；基线在 P3.8 premul 批重种，但早于 SDF LUT。"),
    "ui_wr_p38_svg_structure": ("examples/ui_wr_p38_svg_structure/golden/showcase_svg_structure_base.png", "2026-11-10", "66d08bd1", "内部 814/814 同，差异在 marker/结构图形边沿（edge 17.3%），最大 10；基线早于 2026-11-11 SDF LUT。"),
    "ui_wr_p38_svg_text": ("examples/ui_wr_p38_svg_text/golden/showcase_svg_text_base.png", "2026-11-09", "972cc37c", "内部 629/629 同，差异在 SVG 文字/图形边沿（edge 19.6%），最大 10；基线早于 2026-11-11 SDF LUT。"),
}

order = sorted(native, key=lambda w: (w.startswith("ui_wr_p38"), w.startswith("ui_wr_"), w))


def fmt_pct(v: float) -> str:
    return f"{v:.4f}%"


def fmt_px(v: int) -> str:
    return f"{v:,}"


lines = []
a = lines.append
a("# P2.6「22 窗金图漂移」逐窗判定（2026-11-12）")
a("")
a("> 结论：**21 个漂移窗判为「基线过期」，未发现渲染洞；`ui_render_base_svg` 金图有效（0 差异），不重种。**")
a("> 用户 2026-11-12 确认后已重种 21 窗：native/go 42 轮复跑全 `0.0000%`，窗口级 1px 负向 12.9901% 红→还原绿；webp 逻辑探针同步改为 `FormatRGBAPremul`（19/19 通过）。专项收口。")
a("> 依据：22 窗 × 双后端串行实跑（native webgpu / go gles，隔离 rootless Xwayland :3，标准自退）；每窗都有逻辑探针、像素探针、当前 PNG、基线 PNG、diff/compare 对照；代表窗金图 diff 与 C1 历史值逐字相同（dither 31.7164% / miscdraw 14.2041% / backdrop 6.3519% / shadermask 5.6476% / filterops 5.2101%），其余落在 C1 记录的 0.1880%–1.6722% 区间；native/go 的 golden pct 全部相等，内容区两后端逐字节相同。")
a("")
a("## 1 方法与口径")
a("")
a("- 采集：`tools/run_windows.sh`，22 窗 × {native, go} = 44 轮，单窗串行，`RUN_SECONDS=5 -auto-only`，窗口自行 `Quit/Close`；每轮前后 `xdpyinfo` 检查 :3，无重启、无硬杀。原始 stdout/stderr/rc 在 `runs/<窗>/<后端>/`。")
a("- 分析：`tools/analyze_golden.py`，逐窗输出 golden 门禁 diff（JSON `pixel_golden_diff_pct`）、内容区 diff 计数/最大通道差/bbox/边沿占比/内部采样点/±2px 平移搜索/通道差直方图；视觉对照 `analysis/<窗>/<后端>/{diff.png,compare.png}`。")
a("- 内容区定义：去掉 wrkit 顶栏 48px 与底部 HUD 72px；HUD 是 live FPS/phase 文本，本来就被各窗 golden mask 排除。")
a("- 内部采样：内容区每 40px 一个采样点（RGBA 逐字节比较）；“identical”计数即逐字节相同的采样点数。")
a("- 边沿占比：差异像素中，邻域 3×3 局部对比度 ≥16 的像素占比；1px AA 边沿漂移会得到高占比，渐变/阴影内部变化会得到低占比。")
a("- 平移搜索：把当前图按 (dx,dy)∈[-2,2] 平移后与基线比较，最优仍为 (0,0)、增益 0 = 没有整体错位。")
a("- 双后端：native 与 go 的内容区当前 PNG 逐字节相同（22/22，`content_diff=0`），差异只在 HUD/顶栏 live 文本；金图 pct 两档逐窗相等。")
a("")
a("## 2 汇总表")
a("")
a("| 窗 | 基线日期/提交 | golden diff（native/go） | 内容区 diff | 最大 Δ | 内部采样 identical | 平移最优 | 判定 |")
a("|---|---|---|---:|---:|---|---|---|---|")
for w in order:
    r = native[w]
    g = go[w]
    meta = META[w]
    s = r["interior_samples"]
    shift = r["shift_search"]
    verdict = "基线有效" if w == "ui_render_base_svg" else "基线过期"
    a(f"| `{w}` | {meta[1]} `{meta[2]}` | {fmt_pct(r['official_golden_pct'])} / {fmt_pct(g['official_golden_pct'])} | {fmt_px(r['content_diff'])} px（{100*r['content_diff']/r['content_px']:.3f}%） | {r['max_channel_delta']} | {s['identical']}/{s['samples']} | ({shift['best_dx']},{shift['best_dy']}) | {verdict} |")
a("")
a("## 3 逐窗证据")
a("")
for w in order:
    r = native[w]
    meta = META[w]
    s = r["interior_samples"]
    sh = r["shift_search"]
    a(f"### {w}")
    a("")
    a(f"- 基线：`{meta[0]}`（{meta[1]} `{meta[2]}`）")
    a(f"- golden：{fmt_pct(r['official_golden_pct'])} over {fmt_px(r['official_golden_px'])} px（native 与 go 同值；容差 0）")
    a(f"- 内容区：{fmt_px(r['content_diff'])} / {fmt_px(r['content_px'])} px 不同（{100*r['content_diff']/r['content_px']:.3f}%），最大通道 Δ={r['max_channel_delta']}，bbox={r['diff_bbox']}，边沿占比 {r['content_edge_diff_pct_of_diff']:.1f}%")
    a(f"- 内部采样：{s['identical']}/{s['samples']} 逐字节相同；平移最优 ({sh['best_dx']},{sh['best_dy']})，增益 {sh['best_gain_pct']:.2f}%")
    if s["first_diffs"]:
        pts = []
        for d in s["first_diffs"][:3]:
            pts.append(f"({d['x']},{d['y']}) base={tuple(d['base'])} current={tuple(d['current'])}")
        a("- 内部差异样点：" + "；".join(pts))
    else:
        a("- 内部差异样点：无（全部采样点逐字节相同）")
    a(f"- 视觉对照：`analysis/{w}/native/compare.png`（上=基线，下=当前）、`analysis/{w}/native/diff.png`（红=差异）")
    a(f"- 判定：{'基线有效，不重种' if w == 'ui_render_base_svg' else '基线过期'}。{meta[3]}")
    a("")

a("## 4 根因相关性与「渲染洞」排除")
a("")
a("1. **同一引擎下唯一 0 差异的窗正好是 SDF LUT 重种窗**：`ui_render_base_svg` 基线提交 `4a41c660`（2026-11-11「SDF 覆盖线实现批…6 扇金图重种」），本批 0.0000%；其余 21 窗基线均早于该提交（最晚 2026-11-10），全部出现 0.1880%–31.7164% 漂移。这个对照把「SDF 覆盖 LUT 改变 AA 边沿几个 LSB」与「基线过期」直接对上。")
a("2. **两后端内容逐字节相同**：22/22 窗 native 与 go 的内容区当前 PNG `content_diff=0`、max Δ=0；golden pct 逐窗相等。排除后端特有洞；也排除“某档跑错”的可能。")
a("3. **无整体错位**：22 窗平移搜索最优全部 (0,0)、增益 0；不是布局/相机/视口平移。")
a("4. **内容结构完整**：逐窗 compare 对照均为同一画面；差异集中在 AA 边沿、1px 过渡、渐变/阴影内部精度、diff 面板放大（filterops ×64、dither ×64）等位置。Dither 卡 C 当前是连续平滑梯度、基线是旧量化带，属于精度改善而非缺失。")
a("5. **像素探针全过**：21 个漂移窗的窗口内像素探针全绿（webp 的 24/24 像素探针也全绿；唯一红的是逻辑探针的格式期望，见 §6），说明当前画面满足各窗写死的颜色/几何/边界断言。")
a("6. **已知变更与基线日期吻合（未逐提交 bisect，按时间窗与失败形状归因）**：`4a41c660` SDF 覆盖 LUT（2026-11-11）、P4.1 DrawShadow/GLES（11-10）、P4.2 blur 对齐（11-10）、P4.3 阴影/滤镜源清理（11-10）、P3.8 premul 修复（`d5f9af60`）、M5 格式参数化（11-11，声明 8bit 行为零变化）。这些提交之后重种的窗（svg 11-11、color_font 11-10、p38 svg 11-09/10）漂移幅度正好落在 0–0.2% 的 SDF 边沿量级；更老的窗（10-19–10-27）叠加 P4.1/P4.2/P4.3，幅度更大。")
a("")
a("**结论：21 窗为基线过期，不是渲染洞。** 不建议改引擎；建议按 §7 清单重种基线。")
a("")
a("## 5 每窗理由（摘要）")
a("")
for w in order:
    meta = META[w]
    a(f"- `{w}`：{meta[3]}")
a("")
a("## 6 独立发现：webp 逻辑探针 format 期望过期（不是金图漂移）")
a("")
a("`ui_render_base_webp` 的 golden 漂移按 §3 判为基线过期；但该窗同时有一条逻辑探针红：")
a("")
a("```")
a("webp probe webp_lossless_format FAIL: format=RGBAPremul want RGBA8")
a("FAIL: logic probes 18/19 bad=[webp_lossless_format]")
a("```")
a("")
a("- P1.4 基线（2026-10-29）该探针为 pass；P3.8 `d5f9af60`「WebP 真修 FromStdImage NRGBA 漏预乘」把 `FromStdImage` 的存储格式统一为 `FormatRGBAPremul`（`render/internal/image/io.go:313-355`）。")
a("- 同窗的像素探针 24/24 全过：`webp_lossless_exact_vs_png` 逐字节 0 差、alpha 直方图 379/64/5701 全对；失败只来自“格式标签”期望仍是旧的 `FormatRGBA8`。")
a("- 定性：**测试 oracle 过期，不是渲染洞**。已按用户 2026-11-12 确认把探针期望改为 `FormatRGBAPremul`（`examples/ui_render_base_webp/probes.go`），重跑 logic 19/19、pixel 24/24、golden 0.0000%。")
a("")
a("## 7 重种清单与执行结果")
a("")
a("**重种 21 窗**（`ui_render_base_svg` 不重种）：")
a("")
for w in order:
    if w == "ui_render_base_svg":
        continue
    a(f"- `{w}`")
a("")
a("执行（2026-11-12，用户确认「全部重种 + 修 webp 探针」）：")
a("")
a("1. 21 窗当前图（`runs/<窗>/native|go/current.png`）复制到各自 `*_base.png`；旧基线归档 `old-bases/`，逐项哈希见 `reseed.txt`。`ui_wr_p38_color_font` / `picture` 另写 `_go_base.png`（go 快照名不同）。")
a("2. `ui_render_base_webp` 探针期望改为 `FormatRGBAPremul`（`examples/ui_render_base_webp/probes.go`），重跑 logic 19/19。")
a("3. 21 窗 × 双后端复跑 42 轮：**42/42 rc=0、42/42 `pixel_golden_diff_pct=0.0000%`、21/21 窗 native/go 内容区逐字节相同**；明细 `verify/SUMMARY.md`、`verify/run-index.tsv`。")
a("4. 窗口级 1px 负向（`ui_render_base_backdrop` + `SNAP_DIR` 临时目录）：平移基线 → `diff=12.9901%`、rc=1；还原 → `0.0000%`、rc=0；原文件未动。")
a("5. `ui_base_vram_cap` 按既有 i5-v2 证据同批处理：8 窗重种；native Mailbox + go/gles 各 300s cascade，8/8 `0.0000%`、像素 16/16、rc=0；见 `i5-vram/closeout.md`。首轮 win1 snapshot 未写出竞态登记 TASKS §2.2 N3。")
a("")
a("## 8 证据索引")
a("")
a("- 运行索引：`run-index.tsv`（44 行，窗/后端/rc/golden pct/total px/logic/backend/screen/snapshot/base）")
a("- 逐窗原始输出：`runs/<窗>/<后端>/{stdout.json,stderr.log,rc,elapsed_sec,current.png,base.png}`")
a("- 逐窗分析 JSON：`analysis.jsonl`（44 行）")
a("- 逐窗视觉对照：`analysis/<窗>/<后端>/{diff.png,compare.png}`")
a("- 分析器自检：`selftest.txt`（identical=0；1px 平移=非零且检出；单点改色=1 px；PASS）")
a("- 采集/分析脚本：`tools/run_windows.sh`、`tools/analyze_golden.py`、`tools/run_analysis.py`、`tools/selftest_analysis.py`、`tools/gen_assessment.py`；重种/复跑：`tools/reseed.py`、`verify/`、`reseed.txt`")
a("")
a("## 9 重种后验收汇总")
a("")
a("| 项 | 结果 |")
a("|---|---|")
a("| 21 窗 × 双后端复跑 | 42/42 rc=0；42/42 golden 0.0000% |")
a("| 跨后端内容 | 21/21 窗 native vs go 内容区 `content_diff=0`、max Δ=0 |")
a("| webp 逻辑探针 | `logic_pass=true`、19/19、pixel 24/24、golden 0.0000% |")
a("| 1px 窗口级负向 | 平移基线 `12.9901%` → rc=1；还原 `0.0000%` → rc=0 |")
a("| I5 `ui_base_vram_cap` | native 300s + go 300s 各 8/8 窗 `0.0000%`、像素 16/16、rc=0 |")
a("")
a("**状态：P2.6 22 窗金图漂移专项 ✅；I5 金图债 ✅。**")

with open(os.path.join(root, "assessment.md"), "w") as f:
    f.write("\n".join(lines) + "\n")
print("wrote assessment.md")
