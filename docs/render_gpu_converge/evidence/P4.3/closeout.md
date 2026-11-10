# P4.3 收口（2026-11-10，用户拍板「两项都不改、P4.3 收口」）

## 0 拍板

二轮量测数据齐后，用户拍板：dither 客观判不修、backdrop 按正确性设计登记不优化，
**P4.3 收口**；「静止帧增量策略二轮」记为可选后续（不阻塞收口）。

## 1 逐项收口依据

| 子项 | 结论 | 依据 |
|---|---|---|
| DrawShadow 阴影模糊整画布成本 + 无跨帧缓存 | ✅ 已修（紧致界跨帧 tile 缓存） | `cache-impl.md`：native 58.91fps / go 58.46fps、双后端金图 0.0000%；no-cache 反证 3.40fps / p95 306ms |
| mask 临时纹理池化 | ✅ 已修（按尺寸平面池） | HEAD `c8f64d98`：稳态零分配、`ui_render_base_maskops` 金图 0.0000%（627200px）、`TestP43MaskPlaneStats` |
| 抖动路径优化 | **判不修**（客观证伪「破坏零回读 GPU 路」） | `measure/results-round2.md` §3.2：真 GPU 探针 `frame_flushes off=1 on=1` / `cpu_fallback off=0 on=0`；`dither_contract_test.go` A12 / `RENDER_FEATURE_COVERAGE §6`「dither 不进 `FlushGPUWithView/PresentWith` window-present 路径」；`render.Canvas` 接口无 `SetDither`、live present 帧为录制画布（P4.1 批 4 补正 1 实证） |
| backdrop 损伤区合成 | **判不优化**（正确性设计 + 量测未证裁剪收益） | `render/context_layer_stack.go:467-480` 注释自陈整层 blit 是「windowed present 尾段重放的正确性需要」、对齐 Skia `cropFilterOutputToLayerBounds`；A/B 量到净成本但 `damage_area_px` 两轮恒 960000 系满刷策略造成、裁剪可行性未判（不拿假证据硬判）；P4.3 阴影专项两次改层合成路径均 33% 像素缺失全回退（`impl-attempt.md` / `impl-attempt2.md`） |

## 2 二轮量测数据（本会话，正本 `measure/results-round2.md`）

- 环境：隔离 :3 headless、单窗、标准关闭、每轮前后 `xdpyinfo` 健康、无硬杀、无驱动崩溃。
- dither 探针 `render/p43_dither_readback_test.go` PASS（真 GPU 路，非 SKIP）。
- backdrop A/B（开/关）：`frame_flushes 3→1`、`frame_raster_ms 18.04→15.42`、
  `fps_interval 36.46→55.65`、`interval_p95 35.19→22.65`、`cpu_fallback 0/0`。
- **诚实局限**：两轮 `damage_area_px=960000` / `damage_ratio=1` / `present_mode=full`
  是测量窗「每帧 ScheduleFrame 全量重绘 + PresentPolicyFullPaint 标签」造成的，
  **不是 backdrop 层造成的**；方案 §4.1 的判定线在该策略下失效，故不据此判裁剪。
  这属于「如实说没判出来」，不是「没做」。

## 3 全程约束核验

- 零引擎改动、零公开 API、catalog/apidoc 不动、既有正确性金图不重种。
- 本会话新增仅：`examples/ui_wr_p43_measure/`（测量窗）、
  `render/p43_dither_readback_test.go`（探针）、`measure/*` 证据、台账与方案文档。
- 编译复验：`go build ./examples/ui_wr_p43_measure` rc=0；
  `go test ./render -run '^TestP43DitherReadbackCounters$'` PASS。
- GPU 安全：:3 隔离 + 标准关闭验证（`ui_l1_blank` 166 presents）+ 每轮 PRE/POST 健康。

## 4 可选后续（不阻塞收口，已登记 §2.2）

「静止帧 + retained/hybrid 策略」增量测量：改测量窗不每帧 `ScheduleFrame`，看
`damage_area_px` 是否收敛到 153600（卡片面积），再判 backdrop 损伤区裁剪是否可行。
当前「不优化」判定不因该后续改变，除非该轮拿出客观收益 + 双后端逐位 + 负向对照。

## 5 状态

P4.3 🟡 → ✅（用户拍板口径：阴影 + mask 已修、余两项量测定性不修）；
**§2 主表 40 项全部 ✅，render/gpu 收敛任务整表收口**。
本会话改动未提交，留工作区。
