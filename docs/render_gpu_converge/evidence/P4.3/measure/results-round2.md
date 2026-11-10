# P4.3 补量测第二轮：backdrop 局部场景 A/B + dither 真 GPU 探针（2026-11-10）

## 0 口径与载体调整

用户拍板「造一扇专用量测窗」。实施中发现原方案 §4.2 的 dither 载体不成立
（现成 `ui_render_base_dither` 的 off/on 是离屏 `bakeImage` 贴图，窗主绘制不
`SetDither`，见 `measure-plan.md` §5.2），且 `render.Canvas` 接口无 `SetDither`、
live present 帧是录制画布（P4.1 批 4 补正 1 实证断言 `*Context` 会失败），
「窗主 DC 真开 dither」技术上做不到。故载体做最小调整：

- **backdrop**：新增 `examples/ui_wr_p43_measure`（满窗局部 backdrop 场景、
  零金图 / 零 screen-check / 零像素探针，只出 JSON 计数）
- **dither**：`render/p43_dither_readback_test.go` 离屏真 GPU 探针
  （`RenderPathStats` 计数，走 dither 真正生效的 `Context.Image()` 路径）

零引擎改动、零公开 API、既有正确性金图不重种、不碰任何现有窗。

## 1 环境与安全（GPU 真窗安全条）

- 隔离 :3：`evidence/P2.6/tools/start-xwayland.sh start` → `wayland gpui-p26: alive`
  / `X display :3: alive` / `check OK`。
- 标准关闭验证（前置）：`ui_l1_blank` `RUN_SECONDS=3` → `backend=x11 presents=166`、
  进程自行退出、无 kill/pkill、跑前 `PRE_OK` / 跑后 `POST_OK`（`measure-plan.md` §5.1）。
- 每轮单窗、`env -u WAYLAND_DISPLAY DISPLAY=:3` 强制 X11、`RUN_SECONDS=6 -auto-only`，
  应用走 `Run → Close` 标准关闭，脚本等进程自己退出。
- 二进制 `/tmp/p43_measure_bin`（不入库）；A/B 两轮串行，每轮前后
  `DISPLAY=:3 xdpyinfo` 健康检查均 `PRE_OK / POST_OK`；stderr 无
  `nvidia_drv.so` / Xorg 段错误 / `Invalid MIT-MAGIC-COOKIE-1 key`。
- 两轮 `RUN_EXIT=0`，JSON 有效，`gpu_ops>0`（假绿守卫通过）。

## 2 载体（新增，已编译验证）

- `examples/ui_wr_p43_measure/main.go`：`go build` rc=0。满窗 1200×800 条纹静态
  背景 + 中央 480×320（16% 面积）局部 backdrop 卡（`ClipRect` +
  `PushBackdropLayer` + 半透明压暗 + `PopLayer`），HUD 指标浮内容左上角；
  `wrgate.BuildReport` 出 `damage_area_px / frame_flushes / frame_raster_ms /
  gpu_ops / cpu_fallback_ops / present_count`；env `GPUI_P43_NO_BACKDROP=1`
  为「去掉 backdrop 层、其余完全相同」的 A/B 对照；`present_count<1` 或
  `gpu_ops=0` 直接 exit 1（防无 GPU 假绿）。
- `render/p43_dither_readback_test.go`：`go test ./render -run
  '^TestP43DitherReadbackCounters$' -count=1 -v` → **PASS**（0.37s）。复用现成
  `ditherGradientContext` helper，开/关 dither 各一次 `Image()`，读 `RenderPathStats()`。

## 3 实测数据（native / :3 / 6s / 标准关闭）

### 3.1 backdrop A/B（同二进制、同策略、串行）

| 字段 | A backdrop 开 | B backdrop 关（对照） | 差值 |
|---|---|---|---|
| `damage_area_px` | 960000 | 960000 | 0 |
| `damage_ratio` | 1 | 1 | 0 |
| `present_mode` | full | full | — |
| `frame_flushes` | 3 | 1 | **+2** |
| `frame_raster_ms` | 18.04 | 15.42 | **+2.61** |
| `gpu_ops` | 4423 | 6069 | 帧数差 |
| `cpu_fallback_ops` | 0 | 0 | 0 |
| `present_count` | 221 | 337 | −116 |
| `fps_interval` | 36.46 | 55.65 | −19.19 |
| `interval_p95_ms` | 35.19 | 22.65 | +12.54 |

原始 JSON：`backdrop-local-native.json` / `backdrop-local-native-nobackdrop.json`；
stderr 同名 `.stderr.err`（P4.3 既有惯例：`.log` 被 `.gitignore` 忽略，证据日志用 `.err` 后缀）。首轮满窗基线见 `results.md` §2
（`ui_render_base_backdrop`：damage 960000 / flush 11 / raster 153.2ms）。

### 3.2 dither 离屏真 GPU 探针

`TestP43DitherReadbackCounters`：`gpu_ops off=1 on=1`（**真 GPU 路，非 SKIP**）、
`frame_flushes off=1 on=1`、`cpu_fallback_ops off=0 on=0`。
⇒ 开 dither 不增加 GPU flush、不触发 CPU 回退。

## 4 诚实判读（客观判据，禁 mean 糊弄）

1. **dither 项可判定**：开 dither 不增加 `frame_flushes` / `cpu_fallback_ops`；
   且 `render.Canvas` 接口无 `SetDither`、live present 帧是录制画布，仓内
   `dither_contract_test.go` A12 / `RENDER_FEATURE_COVERAGE §6` 自述「dither 不影响
   `FlushGPUWithView/PresentWith` window-present 路径」。⇒ 「抖动破坏零回读 GPU 路」
   不成立，**无需改**。
2. **backdrop 的「损伤区裁剪可行性」本轮判不了**：A/B 两轮 `damage_area_px` 都恒为
   960000（整窗）、`damage_ratio=1`、`present_mode=full`——这是测量窗「每帧
   `ScheduleFrame` 全量重绘 + `PresentPolicyFullPaint` 标签」造成的，**不是 backdrop
   层造成的**。因此方案 §4.1「局部场景下 damage 仍整窗 ⇒ 判不可优化」的判定线在
   此策略下失效，不能据此下结论（那样等于拿假证据定性）。
3. **A/B 真正证到的是 backdrop 层的净成本**：+2 flush/帧、+2.6ms raster/帧、
   fps 36.5→55.7、p95 35.2→22.7ms。即 backdrop 层确实贵，但「贵」不等于「能按
   损伤区裁剪」——代码注释（`render/context_layer_stack.go:467-480`）自陈整层 blit
   是「windowed present 尾段重放的正确性需要」，P4.3 阴影专项两次改层合成路径均
   33% 像素缺失全回退（`impl-attempt.md` / `impl-attempt2.md`）。
4. **可选后续（本轮未做）**：若要真正判 damage 裁剪，需新增「静止帧 +
   retained/hybrid 策略」测量档（不每帧 `ScheduleFrame`），看 `damage_area_px`
   是否收敛到 153600（卡片面积）；在此之前不硬判、不硬改。

## 5 结论与建议

- dither：客观证伪「破坏零回读 GPU 路」，登记不修。
- backdrop：量到净成本，但裁剪可行性未判；结合「正确性设计 + 两次回退教训」，
  建议登记「不优化」，把「静止帧增量策略二轮」记为可选后续，本轮不硬改。
- **停在「停下报告」**：量测数据齐，拍板要不要改引擎（或补二轮）。
- 全程零引擎改动、零公开 API、既有金图不重种、前后显示健康、标准关闭。
