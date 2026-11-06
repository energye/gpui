批 6 ⑤ · 泛光窗 收口账（2026-11-06；设计见同目录 `windows-3456-design.md` 四，窗 `examples/engine/p37_bloom`）
====================================================================================

一、收口对象
------------
台账批 5 步骤 1（CPU 竖条，`97ea98b8`）与步骤 2（GPU 四 pass + CPU 8-bit 节点 +
参数真源归位，`0e5d2549`）两行 ☑→✅。LDR 第三处声明（真窗探针）随本窗落，前两处
（文档+目录）批 5 已落——三处现在都咬得住：②③两处在窗内有实测探针。

二、臂判据与实测（11 枚探针全绿；容差显式：ROI 亮度关系 ≥+5/+8、逐位段 0、
语义像素 ±2、金图 0）
--------------------------------------------------------------------------------
| 臂 | 断言 | 实测 |
|---|---|---|
| A 扫参五面板 | 恒等/基准 T0.60 σ8 I0.60/阈值组 T0.15/半径组 σ16/强度组 I1.60：基准缘外 8px 光晕 ≥ 恒等+8；阈值组点亮基准下 ramp=0 的中灰块 ≥+8；半径组点亮基准≈零的 20px 远环 ≥+5；强度组近环比基准 ≥+8；五面板两两互异 ≥200 字节 | 10 对全过（p0↔p3=101004 最大、p1↔p4=38480 最小；native/go 两端**逐键全同**）；泛光腿 `gpu_ops=5 / cpu_fallback=0` 双端同值（四 pass 链真走 GPU） |
| B 恒等逐位 | 真调 `ApplyBloom{Intensity:0}` ≡ 完全不调用 逐位 | `bloom_intensity_zero_bitwise_vs_no_call` diff=0 worst=0 ✓（门口短路 `filter_ops_cpu.go:532` 语义窗内版） |
| C LDR 三处声明 | JSON `ldr_declarations` + 底带四行可见 | ①RGBA8 LDR 近似（声明+窗内画面即 8-bit 域）；②`hdr.FloatTarget.TextureView()==nil` 探针实测 ✓；③`fx` CPU 出口 clamp01 探针实测（R=2 输入出 ≤1）✓ |
| D 参数域 | Threshold>1 / σ>16 / I>8 / NaN 四枚具名拒 + 零值合法 + IsIdentity 语义 | `bloom_params_named_rejects` / `bloom_is_identity_semantics` ✓ |
| E 非空转 | 基准 vs 恒等画面差 ≥1000 字节 | `bloom_changes_picture_vs_identity` ✓（防「泛光没吃进来」的空转绿） |

三、口径如实（三件）
--------------------
1. **Canvas/Recorder 无 ApplyBloom 口**（批 5 余账登记「要接另拍」）：五面板在离屏
   `*render.Context` 画场景+真调 ApplyBloom（生产级 GPU 四 pass 被真调用），结果以
   1:1 整数位 DrawImage 贴回窗——scale=1 时采样点落纹素中心、任何采样核恒等，
   恒等面板四色块逐位锚同时验证贴回链路不改画面。泛光画面进金图；缺口本身已登记，
   不是绕洞。
2. **fps/p95 只报数不判绿**（批 6 硬要求 §3-5，性能门重定标未做）。
3. **半径组取值纠偏**：σ12 时远环实测差仅 +3.9（8-bit 尾太薄），抬到域顶 σ16
   （=MaxBloomSigma，kernel d=48）后判据 ≥+5 稳过——如实记。
4. 首版三轮纠偏留痕：ROI 距离账初版按 σ4 算（远环收不到光）、提取 ramp 初版
   用 (0.41−0.15)/0.85≈0.37 算错分母（软膝账已在 var 段重写）、中灰块初版
   L≈0.61 贴阈值线（改 100,105,115→L≈0.41 远离 0.6）。

四、三证据合证
--------------
1. 逻辑/像素离屏探针 11 枚（`probes=11/11`），extra 键 `panel_pair_diffs`（10 对）、
   `bloom_leg_stats_report_only`、`ldr_declarations`。
2. 语义像素 4 枚（恒等面板四色块精确色 ±2，坐标由场景几何同一份常数反推）。
3. Golden 逐位 0（fail-closed、mask 指标带 y<32、容差显式 0；native/go 两基线逐位相同；
   播种跑与缺基线跑均判红实测）。

五、H15/H16
-----------
- H15：native 门禁相六轮（播种后复核 + 五轮）金图恒 0.0000%（presents 240/241
  两形态）。⑤ 为静态内容窗：常驻相（-manual-seconds 6，354 presents）金图同 0
  ——如实记（无墙钟相位，对照 ② 动画窗 0.0755% 只报数）。
- H16 两变异（窗参一行：改码→跑→取值→还原→终验复绿；变异位为 var 段
  refIntensity/refThreshold，探针/面板/金图三者同吃这份常数）：
  1. `refIntensity 0.60→0.00`（泛光消失回恒等帧）：五枚探针红——
     `reference_halo_engages`（近环 15.51≡背景）/ `threshold_sweep_lights_mid_region`
     （阈值组中灰块 104.66 ≡ 基准 104.66——四臂增益全没了）/ `sigma_sweep_widens_halo`
     （远环 15.51≡基准）/ `five_panels_pairwise_differ`（p0↔p1=0 直证全帧复现恒等）
     / `bloom_changes_picture_vs_identity`（0 字节差）+ **金图 8.4939%**；
  2. `refThreshold 0.60→0.05`（全画面泛光）：`threshold_sweep_lights_mid_region`
     关系翻转红（基准中灰块 128.59 > 阈值组 123.80——基准把阈值组吃掉了）
     + **金图 8.1050%**。
  还原核对：两参回原位（grep 0.60/0.60）→ `git diff` 零命中 → 终验
  `OK backend=native presents=240 probes=11/11 golden=0.0000%`。

六、门禁账
----------
见 `window-5-gates.md`；原始跑数见 `window-5-runs.md`。

七、收口动作
------------
台账批 5 两行 ☑→✅（附本窗名+实测值）；§7 追加批 6 ⑤ 行；
`batch6-window-plan.md` §5 补交付进展。
