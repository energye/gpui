# P1.4 全部真窗基线采集

> 生成时间：2026-10-29T06:20:37；仓库 `/home/yanghy/app/projects/gogpu/gpui`；HEAD `73df408c`；运行 worktree `/home/yanghy/app/projects/gogpu/gpui-p14`。

## 范围与口径

- `windows.md` 的 162 扇真窗，展开 183 个 case/状态变体；每变体 native + `GPUI_BACKEND=go` 两档。
- 不列入 28 个 headless 例子（`windows.md` §3.7）。
- 单进程串行、一轮一窗；隔离显示 `DISPLAY=:3`（独立 Xwayland，未使用当前 `:1`）；标准自关窗，M 探针先 WM_DELETE，未响应才按 AGENTS 兜底。批量窗（multiwin/vram/zz_soak）排在最后。
- 金图零容差；运行期 `0*` 首遍播种、次遍确认。`GPUI_BACKEND=go` 是**呈现层**切换；JSON `gpu_backend` 仍为 `integrated`，不能宣称纯 Go 加速器（P0.4 口径）。
- rootless Xwayland 的 x11grab 是黑图，已禁用屏幕抓图；视觉证据用窗内 GPU readback 快照或冻结金图，引擎窗归档冻结金图 + offscreen diff。

## 状态计数

| 状态 | 轮数 |
|---|---:|
| BUILD_FAIL | 2 |
| FAIL | 111 |
| NO_JSON | 28 |
| PASS | 219 |
| PROBE | 2 |
| SEED | 8 |
| SKIP | 4 |

| 家族 | BUILD_FAIL | FAIL | NO_JSON | PASS | PROBE | SEED | SKIP |
|---|---:|---:|---:|---:|---:|---:|---:|
| engine | 0 | 17 | 3 | 56 | 0 | 0 | 0 |
| render | 0 | 34 | 6 | 28 | 0 | 0 | 0 |
| ui_wr | 2 | 19 | 7 | 68 | 0 | 8 | 0 |
| base | 0 | 6 | 2 | 28 | 2 | 0 | 4 |
| kit | 0 | 10 | 1 | 21 | 0 | 0 | 0 |
| video | 0 | 25 | 7 | 12 | 0 | 0 | 0 |
| demo | 0 | 0 | 2 | 6 | 0 | 0 | 0 |

## FAIL / 构建失败 / 超时 / 无 JSON

| 窗 | 变体 | 后端 | 轮次 | 状态 | rc | 原因 |
|---|---|---|---|---|---:|---|
| anim | sk | native | run | FAIL | 1 | exit code 1; p99=22.71; vsync_source=true (not a source name); golden diff 74.9220%; hitch_rate_per_min=7.51 > 5; game_anim: selftest probe=true parity=true pixel=true golden=0.0000%(first=false) ok=true; game_anim: golden sk_final.png: diff=74.9220% over 921600 px; FAIL: win_golden_diff_pct=74.9220 want 0.0 over 921600 px |
| anim | sk | go | run | FAIL | 1 | exit code 1; p99=23.51; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 74.9220%; game_anim: selftest probe=true parity=true pixel=true golden=0.0000%(first=false) ok=true; game_anim: golden sk_final.png: diff=74.9220% over 921600 px; FAIL: win_golden_diff_pct=74.9220 want 0.0 over 921600 px |
| anim | fsm | native | run | FAIL | 1 | exit code 1; p99=20.66; vsync_source=true (not a source name); golden diff 73.6752%; game_anim: fsm selftest probe=true parity=true pixel=true golden=0.0000%(first=false) ok=true; game_anim: golden fsm_final.png: diff=73.6752% over 921600 px; FAIL: win_golden_diff_pct=73.6752 want 0.0 over 921600 px |
| anim | fsm | go | run | FAIL | 1 | exit code 1; p99=19.27; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 73.6752%; game_anim: fsm selftest probe=true parity=true pixel=true golden=0.0000%(first=false) ok=true; game_anim: golden fsm_final.png: diff=73.6752% over 921600 px; FAIL: win_golden_diff_pct=73.6752 want 0.0 over 921600 px |
| anim | tl | native | run | FAIL | 1 | JSON pass/ok=false; missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz; golden diff 25.7695%; game_anim: tl selftest probe=true parity=true pixel=true golden=25.7695%(first=false) ok=false; game_anim: tl selftest FAIL, not opening window |
| anim | tl | go | run | FAIL | 1 | JSON pass/ok=false; missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz; golden diff 25.7695%; game_anim: tl selftest probe=true parity=true pixel=true golden=25.7695%(first=false) ok=false; game_anim: tl selftest FAIL, not opening window |
| fx | all | native | run | FAIL | 1 | exit code 1; p99=19.58; vsync_source=true (not a source name); golden diff 100.0000%; game_fx: golden fx_final.png: diff=100.0000% over 921600 px; FAIL: win_golden_diff_pct=100.0000 want 0.0 over 921600 px |
| fx | all | go | run | FAIL | 1 | exit code 1; p99=19.67; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 100.0000%; game_fx: golden fx_final.png: diff=100.0000% over 921600 px; FAIL: win_golden_diff_pct=100.0000 want 0.0 over 921600 px |
| light | torch | native | run | FAIL | 1 | exit code 1; p99=20.56; vsync_source=true (not a source name); golden diff 99.8429%; hitch_rate_per_min=14.99 > 5; game_light: golden light_final.png: diff=99.8429% over 916800 px; FAIL: parity_changed_pct=1.1111 want <=1.00 (CPU/GPU diverge) |
| light | torch | go | run | FAIL | 1 | exit code 1; p99=18.93; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 99.8429%; game_light: golden light_final.png: diff=99.8429% over 916800 px; FAIL: parity_changed_pct=1.1111 want <=1.00 (CPU/GPU diverge) |
| light | shadow | native | run | FAIL | 1 | exit code 1; p99=20.63; vsync_source=true (not a source name); golden diff 99.8429%; game_light: golden shadow_final.png: diff=99.8429% over 916800 px; FAIL: parity_changed_pct=1.0069 want <=1.00 (CPU/GPU diverge) |
| light | shadow | go | run | FAIL | 1 | exit code 1; p99=18.93; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 99.8429%; game_light: golden shadow_final.png: diff=99.8429% over 916800 px; FAIL: parity_changed_pct=1.0069 want <=1.00 (CPU/GPU diverge) |
| light | normal | native | run | FAIL | 1 | exit code 1; p99=20.00; vsync_source=true (not a source name); golden diff 99.6451%; game_light: golden normal_final.png: diff=99.6451% over 916800 px; FAIL: parity_changed_pct=1.9618 want <=1.00 (CPU/GPU diverge) |
| light | normal | go | run | FAIL | 1 | exit code 1; p99=20.78; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 99.6451%; game_light: golden normal_final.png: diff=99.6451% over 916800 px; FAIL: parity_changed_pct=1.9618 want <=1.00 (CPU/GPU diverge) |
| light2d | shadow | native | run | FAIL | 1 | exit code 1; p99=21.90; vsync_source=true (not a source name); game_light2d: grade=PCF13 taps=13 mirror=true moved=252 gain=0.8007 lee=0.000000 golden=0.0000%(first=false) pixel=true; game_light2d: probes ok=true logic=true pixel=true golden=true off_is_night=true; game_light2d: golden light2d_final.png: diff=99.7670% over 916800 px; FAIL: win_golden_diff=99.7670 want 0.0 over 916800 px |
| light2d | shadow | go | run | NO_JSON | 1 | no parseable JSON on stdout; 2026/10/28 20:23:57 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002 |
| stage_chase | default | native | run | FAIL | 1 | exit code 1; p99=21.85; vsync_source=true (not a source name); golden diff 99.1429%; game_stage_chase: probes ok=true logic=true(cam=true chunk=true sort=true trail=true dirty=true pool=true detail=cam=(60,48) chunks=8x5 needed=2 sorted=far,near,fx,ui trail=12->2 dirty=1/full pool alive) pix=true parity=0.0000% golden=true(wrote=false changed=0) chunkA=(40,56,45) chunkB=(51,45,35) tree=(51,153,63) car=(229,38,30) trail=(255,165,38) tol=8; game_stage_chase: golden chase_final.png: diff=99.1429% over 240000 px; FAIL: p95=19.65 want <=16 (§5) |
| stage_chase | default | go | run | FAIL | 1 | exit code 1; p99=19.31; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 99.1429%; game_stage_chase: probes ok=true logic=true(cam=true chunk=true sort=true trail=true dirty=true pool=true detail=cam=(60,48) chunks=8x5 needed=2 sorted=far,near,fx,ui trail=12->2 dirty=1/full pool alive) pix=true parity=0.0000% golden=true(wrote=false changed=0) chunkA=(40,56,45) chunkB=(51,45,35) tree=(51,153,63) car=(229,38,30) trail=(255,165,38) tol=8; game_stage_chase: golden chase_final.png: diff=99.1429% over 240000 px; FAIL: p95=18.46 want <=16 (§5) |
| tilemap | map | go | run | NO_JSON | 1 | no parseable JSON on stdout; 2026/10/28 20:28:16 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002 |
| world | large | go | run | NO_JSON | 1 | no parseable JSON on stdout; game_world-large: probes ok=true logic=true pix=true golden=true(wrote=false changed=0) active=1655 total=20008 entities=20008 home_active=1655 attach=500 markers=8 / layer0@(28,28) marker@(420,246) bg@(100,52) tol=4; FAIL: open: render: CreateSurface(xlib): gles: failed to create shared X11 EGL context: eglCreateContext failed: error 0x3000 |
| ui_render_base_backdrop | msaa1 | native | run | FAIL | 1 | exit code 1; p95=137.18 > 22 (P1.7 metric); p99=146.14; vsync_source=true (not a source name); hitch_rate_per_min=509.27 > 5; golden showcase_backdrop.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_backdrop | msaa1 | go | run | FAIL | 1 | exit code 1; p95=151.58 > 22 (P1.7 metric); p99=161.51; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 1.2583%; hitch_rate_per_min=436.36 > 5; golden showcase_backdrop.png: diff=1.2583% over 627200 px; FAIL: pixel_golden_diff_pct=1.2583 want 0 over 627200 px |
| ui_render_base_backdrop | msaa4 | native | run | FAIL | 1 | exit code 1; p95=132.60 > 22 (P1.7 metric); p99=143.28; vsync_source=true (not a source name); hitch_rate_per_min=504.91 > 5; golden showcase_backdrop_msaa4.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_backdrop | msaa4 | go | run | FAIL | 1 | exit code 1; p95=154.91 > 22 (P1.7 metric); p99=161.80; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 52.7042%; hitch_rate_per_min=431.57 > 5; golden showcase_backdrop_msaa4.png: diff=52.7042% over 627200 px; FAIL: pixel probes 17/21 ok |
| ui_render_base_clipop | default | go | run | FAIL | 1 | exit code 1; p95=31.93 > 22 (P1.7 metric); p99=39.76; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0078%; hitch_rate_per_min=108.31 > 5; golden showcase_clipop.png: diff=0.0078% over 627200 px; FAIL: pixel_golden_diff_pct=0.0078 want 0 over 627200 px |
| ui_render_base_cliprrect | default | native | run | FAIL | 1 | exit code 1; p99=22.35; vsync_source=true (not a source name); hitch_rate_per_min=23.95 > 5; golden showcase_cliprrect.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_cliprrect | default | go | run | FAIL | 1 | exit code 1; p99=19.94; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden showcase_cliprrect.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_dither | default | native | run | FAIL | 1 | exit code 1; p99=20.82; vsync_source=true (not a source name); hitch_rate_per_min=23.98 > 5; golden showcase_dither.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_dither | default | go | run | FAIL | 1 | exit code 1; p99=20.98; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden showcase_dither.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_filterops | default | native | run | FAIL | 1 | exit code 1; p99=25.80; vsync_source=true (not a source name); hitch_rate_per_min=23.93 > 5; ui_render_base_filterops: logic filter_graph_skips_noop              pass=true; golden showcase_filterops.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=99.2653% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_filterops | default | go | run | FAIL | 1 | exit code 1; p99=20.29; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 9.4798%; ui_render_base_filterops: logic filter_graph_skips_noop              pass=true; golden showcase_filterops.png: diff=9.4798% over 627200 px; FAIL: pixel probes 18/26 ok |
| ui_render_base_image | default | go | run | FAIL | 1 | exit code 1; p99=19.60; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.2877%; ui_render_base_image: avg=17.33 p50=17.35 p99=19.60 hitches=0 vsync=true; ui_render_base_image: golden showcase_image.png: diff=0.2877% over 499200 px; FAIL: golden diff 0.2877% over 499200 px |
| ui_render_base_maskops | default | native | run | FAIL | 1 | exit code 1; p95=34.61 > 22 (P1.7 metric); p99=38.31; vsync_source=true (not a source name); hitch_rate_per_min=192.52 > 5; golden showcase_maskops.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_maskops | default | go | run | FAIL | 1 | exit code 1; p95=33.03 > 22 (P1.7 metric); p99=43.96; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=120.51 > 5; golden showcase_maskops.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_miscdraw | default | native | run | FAIL | 1 | exit code 1; p99=20.29; vsync_source=true (not a source name); hitch_rate_per_min=24.01 > 5; golden showcase_miscdraw.png: diff=0.0000% over 737760 px; FAIL: screen check: diff=100.0000% over 737760 px > 1.00% (grab vs snapshot) |
| ui_render_base_miscdraw | default | go | run | FAIL | 1 | exit code 1; p99=23.61; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0003%; hitch_rate_per_min=11.94 > 5; golden showcase_miscdraw.png: diff=0.0003% over 737760 px; FAIL: pixel_golden_diff_pct=0.0003 want 0 over 737760 px |
| ui_render_base_pathbool | default | native | run | FAIL | 1 | exit code 1; p99=25.03; vsync_source=true (not a source name); hitch_rate_per_min=11.98 > 5; golden showcase_pathbool.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_pathbool | default | go | run | FAIL | 1 | exit code 1; p99=22.66; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=11.97 > 5; golden showcase_pathbool.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_scene | default | go | run | FAIL | 1 | exit code 1; p99=21.37; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0032%; golden showcase_scene.png: diff=0.0032% over 627200 px; FAIL: pixel_golden_diff_pct=0.0032 want 0 over 627200 px |
| ui_render_base_svg | default | native | run | FAIL | 1 | exit code 1; p99=19.82; vsync_source=true (not a source name); golden showcase_svg.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_svg | default | go | run | FAIL | 1 | exit code 1; p99=19.59; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=11.97 > 5; golden showcase_svg.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_text | default | native | run | NO_JSON | 1 | no parseable JSON on stdout; ui_render_base_text: avg=16.69 p50=16.65 p99=25.69 hitches=7 vsync=true; ui_render_base_text: golden showcase_text.png: diff=0.0000% over 576000 px; FAIL: interval_p95_ms=25.05 > 22.00 |
| ui_render_base_textlow | default | native | run | FAIL | 1 | exit code 1; p95=38.97 > 22 (P1.7 metric); p99=45.43; vsync_source=true (not a source name); hitch_rate_per_min=335.96 > 5; golden showcase_textlow.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_textlow | default | go | run | FAIL | 1 | exit code 1; p95=48.44 > 22 (P1.7 metric); p99=51.22; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=995.95 > 5; golden showcase_textlow.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_textspacing | default | native | run | FAIL | 1 | exit code 1; p99=20.08; vsync_source=true (not a source name); golden showcase_textspacing.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_textspacing | default | go | run | FAIL | 1 | exit code 1; p99=27.21; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=11.97 > 5; golden showcase_textspacing.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_textstroke | default | native | run | FAIL | 1 | exit code 1; p99=20.77; vsync_source=true (not a source name); hitch_rate_per_min=23.96 > 5; golden showcase_textstroke.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_textstroke | default | go | run | NO_JSON | 1 | no parseable JSON on stdout; 2026/10/28 20:37:33 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002 |
| ui_render_base_variablefont | default | native | run | FAIL | 1 | exit code 1; p99=20.08; vsync_source=true (not a source name); golden diff 0.9517%; hitch_rate_per_min=11.98 > 5; golden showcase_variablefont.png: diff=0.9517% over 627200 px; FAIL: pixel_golden_diff_pct=0.9517 want 0 over 627200 px |
| ui_render_base_variablefont | default | go | run | FAIL | 1 | exit code 1; p99=21.19; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.9517%; hitch_rate_per_min=12.00 > 5; golden showcase_variablefont.png: diff=0.9517% over 627200 px; FAIL: pixel_golden_diff_pct=0.9517 want 0 over 627200 px |
| ui_render_base_verticesmesh | default | native | run | FAIL | 1 | exit code 1; p99=20.18; vsync_source=true (not a source name); ui_render_base_verticesmesh: logic mesh_short_indices_skip                pass=true; ui_render_base_verticesmesh: logic mesh_old_bad_index_skips               pass=true; golden showcase_verticesmesh.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_verticesmesh | default | go | run | FAIL | 1 | exit code 1; p99=21.73; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 5.6751%; hitch_rate_per_min=12.01 > 5; ui_render_base_verticesmesh: logic mesh_short_indices_skip                pass=true; ui_render_base_verticesmesh: logic mesh_old_bad_index_skips               pass=true; golden showcase_verticesmesh.png: diff=5.6751% over 627200 px; FAIL: pixel probes 20/29 ok |
| ui_render_base_webp | default | native | run | FAIL | 1 | exit code 1; p99=24.21; vsync_source=true (not a source name); hitch_rate_per_min=11.99 > 5; ui_render_base_webp: logic webp_empty_error                 pass=true; ui_render_base_webp: pixel pt_d_red_badge                 ok=true D red error badge got=(0.859,0.200,0.180) want=(0.860,0.200,0.180) Δ=0.0012 tol=0.0157; golden showcase_webp.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_base_webp | default | go | run | FAIL | 1 | exit code 1; p99=22.27; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=12.02 > 5; ui_render_base_webp: logic webp_empty_error                 pass=true; ui_render_base_webp: pixel pt_d_red_badge                 ok=true D red error badge got=(0.859,0.200,0.180) want=(0.860,0.200,0.180) Δ=0.0012 tol=0.0157; golden showcase_webp.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot) |
| ui_render_graphics/vector | default | native | run | FAIL | 1 | exit code 1; p99=21.09; vsync_source=true (not a source name); golden diff 0.0005%; vector: golden showcase_vector.png: diff=0.0005% over 821184 px; FAIL: golden diff 0.0005% over 821184 px |
| ui_render_graphics/vector | default | go | run | FAIL | 1 | exit code 1; p99=22.77; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0005%; vector: golden showcase_vector.png: diff=0.0005% over 821184 px; FAIL: golden diff 0.0005% over 821184 px |
| ui_ant_verify | msaa1 | native | run | NO_JSON | 0 | no parseable JSON on stdout; ui_ant_verify: OK mode=1x 几何AA presents=301 px=821184 golden=0.0000% |
| ui_ant_verify | msaa1 | go | run | NO_JSON | 0 | no parseable JSON on stdout; ui_ant_verify: OK mode=1x 几何AA presents=300 px=821184 golden=0.0000% |
| ui_ant_verify | msaa4 | native | run | NO_JSON | 0 | no parseable JSON on stdout; ui_ant_verify: OK mode=4x MSAA presents=295 px=821184 golden=0.0000% |
| ui_ant_verify | msaa4 | go | run | NO_JSON | 1 | no parseable JSON on stdout; FAIL: golden diff=100.0000% (821184/821184 px) baseline=examples/ui_ant_verify/testdata/ant_verify_msaa4_base.png |
| ui_wr_r1_layout | default | native | confirm | FAIL | 1 | exit code 1; p99=21.05; vsync_source=true (not a source name); hitch_rate_per_min=11.99 > 5; golden r1_final.png: diff=0.0000% over 421120 px; FAIL: hitch_rate_per_min=11.99 > 5 |
| ui_wr_r1_layout | default | go | confirm | FAIL | 1 | exit code 1; p99=18.84; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=8.00 > 5; golden r1_final.png: diff=0.0000% over 421120 px; FAIL: hitch_rate_per_min=8.00 > 5 |
| ui_wr_r5_picture | default | native | run | FAIL | 1 | exit code 1; p99=22.24; vsync_source=true (not a source name); golden diff 0.0217%; hitch_rate_per_min=12.02 > 5; golden showcase_r5_picture.png: diff=0.0217% over 821184 px; FAIL: logic probes 5/6 bad=[direct_replay_eq] |
| ui_wr_r5_picture | default | go | run | FAIL | 1 | exit code 1; p99=21.33; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0217%; hitch_rate_per_min=11.98 > 5; golden showcase_r5_picture.png: diff=0.0217% over 821184 px; FAIL: logic probes 5/6 bad=[direct_replay_eq] |
| ui_wr_r7_virtlist | default | native | run | FAIL | 1 | exit code 1; p95=24.89 > 22 (P1.7 metric); p99=25.18; vsync_source=true (not a source name); hitch_rate_per_min=16.02 > 5; FAIL: interval_p95_ms=24.9 > 22 |
| ui_wr_r7_virtlist | default | go | run | FAIL | 1 | exit code 1; p95=22.66 > 22 (P1.7 metric); p99=27.36; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=15.02 > 5; FAIL: interval_p95_ms=22.7 > 22 |
| ui_wr_r7b_scroll_reuse | default | native | run | FAIL | 1 | exit code 1; p95=22.42 > 22 (P1.7 metric); p99=28.12; vsync_source=true (not a source name); ui_wr_r7b_scroll_reuse: golden showcase_r7b_scroll_reuse.png: diff=0.0000% over 821184 px; FAIL: logic probes 9/10 bad=[p95_le_22] |
| ui_wr_r7b_scroll_reuse | default | go | run | NO_JSON | 1 | no parseable JSON on stdout; 2026/10/28 20:42:48 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002 |
| ui_wr_r8_overlay | open | go | run | FAIL | 1 | exit code 1; p95=22.24 > 22 (P1.7 metric); p99=25.14; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=8.04 > 5; ui_wr_r8_overlay: golden showcase_r8_overlay_open.png: diff=0.0000% over 821184 px; FAIL: FAIL: interval_p95_ms=22.24 > 22.00 |
| ui_wr_r8_overlay | closed | native | run | FAIL | 1 | exit code 1; p95=22.07 > 22 (P1.7 metric); p99=25.19; vsync_source=true (not a source name); ui_wr_r8_overlay: golden showcase_r8_overlay_closed.png: diff=0.0000% over 821184 px; FAIL: FAIL: interval_p95_ms=22.07 > 22.00 |
| ui_wr_r15_soak | default | go | confirm | NO_JSON | 1 | no parseable JSON on stdout; FAIL: open: render: CreateSurface(xlib): gles: failed to create shared X11 EGL context: eglCreateContext failed: error 0x3000 |
| ui_wr_r20_filter | default | native | confirm | FAIL | 1 | exit code 1; p99=18.60; vsync_source=true (not a source name); hitch_rate_per_min=17.98 > 5; golden r20_final.png: diff=0.0000% over 306280 px; FAIL: scripted=4/5 want 5/5 (像素断言) |
| ui_wr_r20_filter | default | go | confirm | FAIL | 1 | exit code 1; p99=20.11; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=12.03 > 5; golden r20_final.png: diff=0.0000% over 306280 px; FAIL: scripted=3/5 want 5/5 (像素断言) |
| ui_wr_r22_selection_stub | default | native | confirm | FAIL | 1 | exit code 1; p99=32.29; vsync_source=true (not a source name); hitch_rate_per_min=48.18 > 5; golden r22_final.png: diff=0.0000% over 318160 px; FAIL: hitch_rate_per_min=48.18 > 5 |
| ui_wr_r22_selection_stub | default | go | confirm | FAIL | 1 | exit code 1; p99=22.87; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=23.86 > 5; golden r22_final.png: diff=0.0000% over 318160 px; FAIL: hitch_rate_per_min=23.86 > 5 |
| ui_wr_c1_boundary_nest | default | go | run | NO_JSON | 1 | no parseable JSON on stdout; 2026/10/28 20:47:25 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002 |
| ui_wr_c3_list_scroll | default | native | run | FAIL | 1 | exit code 1; p95=26.43 > 22 (P1.7 metric); p99=32.04; vsync_source=true (not a source name); hitch_rate_per_min=52.00 > 5; ui_wr_c3_list_scroll: golden showcase_c3_list_scroll.png: diff=0.0000% over 821184 px; FAIL: logic probes 8/11 bad=[fps_interval_ge_55 p95_le_22 hitch_le_budget] |
| ui_wr_c6_savelayer_group | default | native | confirm | FAIL | 1 | exit code 1; p99=21.58; vsync_source=true (not a source name); hitch_rate_per_min=12.01 > 5; golden c6_final.png: diff=0.0000% over 529400 px; FAIL: hitch_rate_per_min=12.01 > 5 |
| ui_wr_c6_savelayer_group | default | go | confirm | NO_JSON | 1 | no parseable JSON on stdout; 2026/10/28 20:49:44 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002 |
| ui_wr_c11_policy_switch | default | go | run | FAIL | 1 | exit code 1; p99=20.59; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=7.99 > 5; golden c11_final.png: diff=0.0000% over 390176 px; FAIL: hitch_rate_per_min=7.99 > 5 |
| ui_wr_ime_r1_editor | default | native | run | FAIL | 1 | exit code 1; p99=20.19; vsync_source=true (not a source name); golden diff 0.0847%; hitch_rate_per_min=24.12 > 5; ui_wr_ime_r1_editor: golden showcase_ime_r1_editor.png: diff=0.0847% over 813312 px; FAIL: pixel_golden_diff_pct=0.0847 want 0 over 813312 px |
| ui_wr_ime_r1_editor | default | go | run | FAIL | 1 | exit code 1; p99=21.17; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0847%; hitch_rate_per_min=12.15 > 5; ui_wr_ime_r1_editor: golden showcase_ime_r1_editor.png: diff=0.0847% over 813312 px; FAIL: pixel_golden_diff_pct=0.0847 want 0 over 813312 px |
| ui_wr_ime_r2_textlayout | default | go | run | NO_JSON | 1 | no parseable JSON on stdout; FAIL: open: render: CreateSurface(xlib): gles: failed to create shared X11 EGL context: eglCreateContext failed: error 0x3000 |
| ui_wr_ime_r4_selection | default | go | run | NO_JSON | 1 | no parseable JSON on stdout; 2026/10/28 20:56:56 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002 |
| ui_wr_shadermask | default | native | run | FAIL | 1 | exit code 1; p95=24.34 > 22 (P1.7 metric); p99=27.31; vsync_source=true (not a source name); golden diff 4.2822%; hitch_rate_per_min=11.99 > 5; ui_wr_shadermask: logic mask_skips_zero          pass=true; ui_wr_shadermask: logic live_only_skips_zero     pass=true; ui_wr_shadermask: logic golden_zero              pass=false; FAIL: logic probes 10/11 bad=[golden_zero] |
| ui_wr_shadermask | default | go | run | NO_JSON | 1 | no parseable JSON on stdout; 2026/10/28 20:57:06 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002 |
| ui_wr_t_stress | default | native | run | BUILD_FAIL |  | binary missing: /tmp/p14/bin/ui_wr_t_stress.bin |
| ui_wr_t_stress | default | go | run | BUILD_FAIL |  | binary missing: /tmp/p14/bin/ui_wr_t_stress.bin |
| ui_base_batch_cap | default | native | run | FAIL | 1 | exit code 1; p95=53.11 > 22 (P1.7 metric); p99=56.38; vsync_source=true (not a source name); hitch_rate_per_min=244.72 > 5; ui_base_batch_cap: golden examples/ui_base_batch_cap/testdata/batch_cap_prims_base.png: diff=0.0000% over 883200 px; ui_base_batch_cap: golden examples/ui_base_batch_cap/testdata/batch_cap_vector_base.png: diff=0.0000% over 883200 px; FAIL: vector floor failed: max_60fps_paths=0 want >=100 (steps=prims/1000:57fps:19ms, prims/5000:57fps:20ms, prims/10000:57fps:22ms, vector/100:57fps:23ms, vector/300:34fps:33ms, vector/600:21fps:54ms) |
| ui_base_batch_cap | default | go | run | FAIL | 1 | exit code 1; p95=63.15 > 22 (P1.7 metric); p99=70.18; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0169%; hitch_rate_per_min=239.91 > 5; ui_base_batch_cap: golden examples/ui_base_batch_cap/testdata/batch_cap_prims_base.png: diff=0.0000% over 883200 px; ui_base_batch_cap: golden examples/ui_base_batch_cap/testdata/batch_cap_vector_base.png: diff=0.0169% over 883200 px; FAIL: vector/100 meets 60fps but cpu=90.3% > 85% health line |
| ui_base_vram_cap | ladder | native | run | NO_JSON | 1 | no parseable JSON on stdout; FAIL: BASE_VRAM_CAP gate needs RUN_SECONDS>=300 (3 open batches + 3 phases) |
| ui_base_vram_cap | ladder | go | run | NO_JSON | 1 | no parseable JSON on stdout; FAIL: BASE_VRAM_CAP gate needs RUN_SECONDS>=300 (3 open batches + 3 phases) |
| ui_l1_render_matrix | default | native | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz; ui_l1_render_matrix: avg=17.39ms max=27.64ms p50=17.38 p99=20.10 hitches=0 vsync=true; ui_l1_render_matrix: golden diff=1.5271% over 288000 px; FAIL: pixel_golden_diff_pct=1.5271 want 0 over 288000 px |
| ui_l1_render_matrix | default | go | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz; ui_l1_render_matrix: avg=17.29ms max=38.24ms p50=17.47 p99=21.24 hitches=1 vsync=true; ui_l1_render_matrix: golden diff=1.5271% over 288000 px; FAIL: pixel_golden_diff_pct=1.5271 want 0 over 288000 px |
| ui_text_m5_anycase | default | native | run | FAIL | 1 | exit code 1; p95=25.06 > 22 (P1.7 metric); p99=29.08; vsync_source=true (not a source name); golden diff 3.1630%; hitch_rate_per_min=48.06 > 5; ui_text_m5_anycase: golden showcase_text_m5.png: diff=3.1630% over 821184 px; FAIL: pixel_golden_diff_pct=3.1630 want 0 over 821184 px |
| ui_text_m5_anycase | default | go | run | FAIL | 1 | exit code 1; p95=24.97 > 22 (P1.7 metric); p99=29.86; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 3.1630%; hitch_rate_per_min=26.06 > 5; ui_text_m5_anycase: golden showcase_text_m5.png: diff=3.1630% over 821184 px; FAIL: pixel_golden_diff_pct=3.1630 want 0 over 821184 px |
| kit_f0_layout | default | native | run | FAIL | 1 | exit code 1; p95=25.28 > 22 (P1.7 metric); p99=34.69; vsync_source=true (not a source name); hitch_rate_per_min=59.90 > 5; golden showcase_layout.png: diff=0.0000% over 255040 px; FAIL: FAIL: interval_p95_ms=25.28 > 22.00 |
| kit_f0_scope | default | native | run | FAIL | 1 | exit code 1; p95=25.06 > 22 (P1.7 metric); p99=30.62; vsync_source=true (not a source name); hitch_rate_per_min=23.98 > 5; golden showcase_scope.png: diff=0.0000% over 311680 px; FAIL: FAIL: interval_p95_ms=25.06 > 22.00 |
| kit_f0_scope | default | go | run | NO_JSON | 1 | no parseable JSON on stdout; 2026/10/28 21:11:57 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002 |
| kit/color-model | default | native | run | FAIL | 1 | exit code 1; p95=25.63 > 22 (P1.7 metric); p99=27.10; vsync_source=true (not a source name); hitch_rate_per_min=11.96 > 5; golden showcase_color_model.png: diff=0.0000% over 353300 px; FAIL: FAIL: interval_p95_ms=25.63 > 22.00 |
| kit/date-engine | default | native | run | FAIL | 1 | exit code 1; p95=26.54 > 22 (P1.7 metric); p99=34.33; vsync_source=true (not a source name); hitch_rate_per_min=48.14 > 5; golden showcase_date_engine.png: diff=0.0000% over 303680 px; FAIL: FAIL: interval_p95_ms=26.54 > 22.00 |
| kit/icon | default | native | run | FAIL | 1 | exit code 1; p99=27.03; vsync_source=true (not a source name); hitch_rate_per_min=24.30 > 5; golden showcase_icon.png: diff=0.0000% over 1024 px; FAIL: FAIL: damage_ratio=1.0000 > max 0.3500 (retained must not full-screen damage) |
| kit/icon | default | go | run | FAIL | 1 | exit code 1; p99=21.76; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 3.3203%; hitch_rate_per_min=12.18 > 5; golden showcase_icon.png: diff=3.3203% over 1024 px; FAIL: FAIL: damage_ratio=1.0000 > max 0.3500 (retained must not full-screen damage) |
| kit/notice-queue | default | native | run | FAIL | 1 | exit code 1; p95=25.94 > 22 (P1.7 metric); p99=32.05; vsync_source=true (not a source name); hitch_rate_per_min=47.80 > 5; golden showcase_notice_queue.png: diff=0.0000% over 281960 px; FAIL: FAIL: interval_p95_ms=25.94 > 22.00 |
| kit/qrcode | default | native | run | FAIL | 1 | exit code 1; p95=24.89 > 22 (P1.7 metric); p99=28.98; vsync_source=true (not a source name); hitch_rate_per_min=23.97 > 5; golden showcase_qrcode.png: diff=0.0000% over 285296 px; FAIL: FAIL: interval_p95_ms=24.89 > 22.00 |
| kit/tour-mask | default | native | run | FAIL | 1 | exit code 1; p95=25.55 > 22 (P1.7 metric); p99=29.52; vsync_source=true (not a source name); hitch_rate_per_min=35.92 > 5; golden showcase_tour_mask.png: diff=0.0000% over 310064 px; FAIL: FAIL: interval_p95_ms=25.55 > 22.00 |
| kit/upload | default | native | run | FAIL | 1 | exit code 1; p95=25.79 > 22 (P1.7 metric); p99=33.73; vsync_source=true (not a source name); hitch_rate_per_min=48.01 > 5; golden showcase_upload.png: diff=0.0000% over 303160 px; FAIL: FAIL: interval_p95_ms=25.79 > 22.00 |
| video_retained_golden | default | go | run | FAIL | 1 | exit code 1; golden diff 78.9469%; video_retained_golden: pixel blue                   fg  @( 849, 224) got=(0.000,0.718,0.000) want=(0.001,0.000,1.000) ok=false; video_retained_golden: pixel magenta                fg  @( 849, 552) got=(0.412,0.412,0.412) want=(0.999,0.000,0.996) ok=false; video_retained_golden: golden video_retained_golden_a.png: diff=78.9469% over 589908 px; FAIL: 颜色探针 0/4 达标 |
| video_player | default | native | run | NO_JSON | 0 | no parseable JSON on stdout |
| video_player | default | go | run | NO_JSON | 0 | no parseable JSON on stdout; 2026/10/28 21:14:27 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002" |
| video_2k_play | default | native | run | NO_JSON | 1 | no parseable JSON on stdout; FAIL: 缺片 p3_2k_2560_1440_30fps.mp4：先跑 bash video/testdata/gen_p3_2k4k.sh（大二进制不进仓库，本地与 CI 现场生成） |
| video_2k_play | default | go | run | NO_JSON | 1 | no parseable JSON on stdout; FAIL: 缺片 p3_2k_2560_1440_30fps.mp4：先跑 bash video/testdata/gen_p3_2k4k.sh（大二进制不进仓库，本地与 CI 现场生成） |
| video_4k_play | default | native | run | NO_JSON | 1 | no parseable JSON on stdout; FAIL: 缺片 p3_4k_3840_2160_30fps.mp4：先跑 bash video/testdata/gen_p3_2k4k.sh（大二进制不进仓库，本地与 CI 现场生成） |
| video_4k_play | default | go | run | NO_JSON | 1 | no parseable JSON on stdout; FAIL: 缺片 p3_4k_3840_2160_30fps.mp4：先跑 bash video/testdata/gen_p3_2k4k.sh（大二进制不进仓库，本地与 CI 现场生成） |
| video_a2_sync | default | native | run | FAIL | 1 | JSON pass/ok=false; video_a2_sync: probes passed=6/8 failed=2 maxAV=0 err=identity:本该静音却有声 |
| video_a2_sync | default | go | run | FAIL | 1 | JSON pass/ok=false; video_a2_sync: probes passed=6/8 failed=2 maxAV=0 err=identity:本该静音却有声 |
| video_a4_sink | default | native | run | FAIL | 1 | JSON pass/ok=false; video_a4_sink: probes passed=14/15 failed=1 backend=paplay-pulse available=true err=identity:本该静音却有声 |
| video_a4_sink | default | go | run | FAIL | 1 | JSON pass/ok=false; video_a4_sink: probes passed=14/15 failed=1 backend=paplay-pulse available=true err=identity:本该静音却有声 |
| video_b1_frag | default | native | run | FAIL | 1 | JSON pass/ok=false; video_b1_frag: probes passed=113/114 failed=1 diff=0 err=play：b1_frag5.mp4：没播到尾 |
| video_b1_frag | default | go | run | FAIL | 1 | JSON pass/ok=false; video_b1_frag: probes passed=113/114 failed=1 diff=0 err=play：b1_frag5.mp4：没播到尾 |
| video_s8_index | default | native | run | FAIL | 1 | JSON pass/ok=false; video_s8_index: probes passed=16/18 failed=2 forwardMax=1 err=seek:vr_stream_long.mp4/100000:跳后黑屏: 落点100000没播出来 |
| video_s8_index | default | go | run | FAIL | 1 | JSON pass/ok=false; video_s8_index: probes passed=16/18 failed=2 forwardMax=1 err=seek:vr_stream_long.mp4/100000:跳后黑屏: 落点100000没播出来 |
| video_vc0_fullplay | default | native | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); FAIL: 全链路门禁: 黑屏嫌疑: 首帧MAD0.0 尾帧MAD0.0 |
| video_vc0_fullplay | default | go | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); 2026/10/28 21:15:07 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; 2026/10/28 21:15:16 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: 全链路门禁: 黑屏嫌疑: 首帧MAD0.0 尾帧MAD0.0 |
| video_vc1_seek_fault | default | native | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); [h264 @ 0x70a1d004a8c0] Error splitting the input into NAL units.; [h264 @ 0x70a1d076d0c0] Error splitting the input into NAL units.; [h264 @ 0x70a1d0087b00] Error splitting the input into NAL units.; FAIL: 跳进度门禁: 跳第二GOP：跳后黑屏: 目标1800 落点1800 没立刻播出来 |
| video_vc1_seek_fault | default | go | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); [h264 @ 0x3039de00] Error splitting the input into NAL units.; 2026/10/28 21:15:48 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: 跳进度门禁: 跳第二GOP：跳后黑屏: 目标1800 落点1800 没立刻播出来 |
| video_vc2_soak | default | native | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); FAIL: 长跑门禁: 黑屏嫌疑: 首帧MAD0.0 尾帧MAD0.0 |
| video_vc2_soak | default | go | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); 2026/10/28 21:17:57 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: 长跑门禁: 黑屏嫌疑: 首帧MAD0.0 尾帧MAD0.0 |
| video_vc3_embed_extend | default | native | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); FAIL: 组合门禁: 黑屏嫌疑: 首帧MAD0.0 尾帧MAD0.0 |
| video_vc3_embed_extend | default | go | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); 2026/10/28 21:18:27 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; 2026/10/28 21:18:29 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: 组合门禁: 黑屏嫌疑: 首帧MAD0.0 尾帧MAD0.0 |
| video_vr1_params | default | go | run | NO_JSON | 1 | no parseable JSON on stdout; FAIL: 打开失败: render: CreateSurface(xlib): gles: failed to create shared X11 EGL context: eglCreateContext failed: error 0x3000 |
| video_vr3_color | default | native | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); hitch_rate_per_min=12.08 > 5; FAIL: 转色失败: vr2_b_intra.mp4 播出1帧(要5) |
| video_vr3_color | default | go | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); hitch_rate_per_min=12.08 > 5; FAIL: 转色失败: vr2_b_intra.mp4 播出1帧(要5) |
| video_vr5_seek | default | native | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); hitch_rate_per_min=16.03 > 5; FAIL: 跳进度门禁: 双关键帧跳第二GOP：跳后黑屏: 目标1800 落点1800 没立刻播出来 |
| video_vr5_seek | default | go | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); hitch_rate_per_min=12.04 > 5; FAIL: 跳进度门禁: 双关键帧跳第二GOP：跳后黑屏: 目标1800 落点1800 没立刻播出来 |
| video_vr7_perf | default | native | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); FAIL: 极限门禁: 黑屏嫌疑: 首帧MAD0.0 尾帧MAD0.0 |
| video_vr7_perf | default | go | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); 2026/10/28 21:21:26 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: 极限门禁: 黑屏嫌疑: 首帧MAD0.0 尾帧MAD0.0 |
| video_vr9_registry | default | native | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); FAIL: 注册门禁: H265播出: 首帧未现 |
| video_vr9_registry | default | go | run | FAIL | 1 | exit code 1; vsync_source=true (not a source name); 2026/10/28 21:22:13 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: 注册门禁: H265播出: 首帧未现 |
| zz_soak_control | default | native | run | NO_JSON | 0 | no parseable JSON on stdout; CTRL win1 presents=10229 ~56.7 fps avg=17.60 p50=17.50 p95=20.04 hitches=61 raster=16.92 flush=0.48 acquire=1.95 presentWait=0.12 rerecord=15 skip=81800; CTRL win2 presents=10226 ~56.7 fps avg=17.61 p50=17.46 p95=19.91 hitches=66 raster=5.46 flush=0.35 acquire=1.40 presentWait=0.17 rerecord=15 skip=81776; CTRL win3 presents=10250 ~56.8 fps avg=17.57 p50=17.49 p95=19.71 hitches=33 raster=6.45 flush=0.41 acquire=0.25 presentWait=0.08 rerecord=15 skip=81968 |
| zz_soak_control | default | go | run | NO_JSON | 0 | no parseable JSON on stdout; CTRL win1 presents=10357 ~57.4 fps avg=17.38 p50=17.32 p95=18.09 hitches=4 raster=0.39 flush=0.24 acquire=0.00 presentWait=0.10 rerecord=15 skip=82824; CTRL win2 presents=10357 ~57.4 fps avg=17.38 p50=17.35 p95=18.20 hitches=6 raster=18.20 flush=0.17 acquire=0.00 presentWait=9.98 rerecord=15 skip=82824; CTRL win3 presents=10356 ~57.4 fps avg=17.39 p50=17.35 p95=18.02 hitches=3 raster=2.20 flush=0.33 acquire=0.01 presentWait=6.07 rerecord=15 skip=82816 |

## SKIP / 人工探针

### SKIP

| 窗 | 变体 | 后端 | 原因 |
|---|---|---|---|
| ui_pf_win32 | default | native | window reports SKIP |
| ui_pf_win32 | default | go | window reports SKIP |
| ui_pf_appkit | default | native | window reports SKIP |
| ui_pf_appkit | default | go | window reports SKIP |

### PROBE

| 窗 | 变体 | 后端 | 原因 |
|---|---|---|---|
| ui_ime_probe | default | native | manual probe: WM_DELETE not honored; SIGTERM fallback used; process did not self-close; WM_DELETE/SIGTERM fallback used; manual probe: classified PROBE (no JSON gate) |
| ui_ime_probe | default | go | manual probe: WM_DELETE not honored; SIGTERM fallback used; process did not self-close; WM_DELETE/SIGTERM fallback used; manual probe: classified PROBE (no JSON gate) |

## 指标诚实性观测（不替代窗自身门禁）

### p95 > 22ms：33

| 窗 | 变体 | 后端 | 详情 |
|---|---|---|---|
| tex | far | native | p95=25.13 |
| tex | far | go | p95=25.31 |
| ui_render_base_backdrop | msaa1 | native | p95=137.18 |
| ui_render_base_backdrop | msaa1 | go | p95=151.58 |
| ui_render_base_backdrop | msaa4 | native | p95=132.60 |
| ui_render_base_backdrop | msaa4 | go | p95=154.91 |
| ui_render_base_clipop | default | native | p95=29.90 |
| ui_render_base_clipop | default | go | p95=31.93 |
| ui_render_base_maskops | default | native | p95=34.61 |
| ui_render_base_maskops | default | go | p95=33.03 |
| ui_render_base_textlow | default | native | p95=38.97 |
| ui_render_base_textlow | default | go | p95=48.44 |
| ui_wr_r7_virtlist | default | native | p95=24.89 |
| ui_wr_r7_virtlist | default | go | p95=22.66 |
| ui_wr_r7b_scroll_reuse | default | native | p95=22.42 |
| ui_wr_r8_overlay | open | go | p95=22.24 |
| ui_wr_r8_overlay | closed | native | p95=22.07 |
| ui_wr_c3_list_scroll | default | native | p95=26.43 |
| ui_wr_shadermask | default | native | p95=24.34 |
| ui_base_batch_cap | default | native | p95=53.11 |
| ui_base_batch_cap | default | go | p95=63.15 |
| ui_base_multiwin_cap | default | native | p95=26.85 |
| ui_text_m5_anycase | default | native | p95=25.06 |
| ui_text_m5_anycase | default | go | p95=24.97 |
| kit_f0_decor | default | native | p95=25.15 |
| kit_f0_layout | default | native | p95=25.28 |
| kit_f0_scope | default | native | p95=25.06 |
| kit/color-model | default | native | p95=25.63 |
| kit/date-engine | default | native | p95=26.54 |
| kit/notice-queue | default | native | p95=25.94 |
| kit/qrcode | default | native | p95=24.89 |
| kit/tour-mask | default | native | p95=25.55 |
| kit/upload | default | native | p95=25.79 |

### vsync_source 是布尔串：308

| 窗 | 变体 | 后端 | 详情 |
|---|---|---|---|
| 25d_basis | default | native | vsync_source=true |
| 25d_basis | default | go | vsync_source=true |
| anim | sk | native | vsync_source=true |
| anim | sk | go | vsync_source=true |
| anim | fsm | native | vsync_source=true |
| anim | fsm | go | vsync_source=true |
| anim | lod | native | vsync_source=true |
| anim | lod | go | vsync_source=true |
| asset | reload | native | vsync_source=true |
| asset | reload | go | vsync_source=true |
| asset | large | native | vsync_source=true |
| asset | large | go | vsync_source=true |
| camera | follow | native | vsync_source=true |
| camera | follow | go | vsync_source=true |
| camera | voices | native | vsync_source=true |
| camera | voices | go | vsync_source=true |
| dodge | default | native | vsync_source=true |
| dodge | default | go | vsync_source=true |
| fx | all | native | vsync_source=true |
| fx | all | go | vsync_source=true |
| input | remap | native | vsync_source=true |
| input | remap | go | vsync_source=true |
| input | combo | native | vsync_source=true |
| input | combo | go | vsync_source=true |
| light | torch | native | vsync_source=true |
| light | torch | go | vsync_source=true |
| light | shadow | native | vsync_source=true |
| light | shadow | go | vsync_source=true |
| light | normal | native | vsync_source=true |
| light | normal | go | vsync_source=true |
| light2d | shadow | native | vsync_source=true |
| nav | default | native | vsync_source=true |
| nav | default | go | vsync_source=true |
| particle | fire | native | vsync_source=true |
| particle | fire | go | vsync_source=true |
| particle | trail | native | vsync_source=true |
| particle | trail | go | vsync_source=true |
| particles2d | default | native | vsync_source=true |
| particles2d | default | go | vsync_source=true |
| physics | hit | native | vsync_source=true |
| physics | hit | go | vsync_source=true |
| physics | jump | native | vsync_source=true |
| physics | jump | go | vsync_source=true |
| physics | large | native | vsync_source=true |
| physics | large | go | vsync_source=true |
| platformer | default | native | vsync_source=true |
| platformer | default | go | vsync_source=true |
| quad | default | native | vsync_source=true |
| quad | default | go | vsync_source=true |
| save | default | native | vsync_source=true |
| save | default | go | vsync_source=true |
| sprite | all | native | vsync_source=true |
| sprite | all | go | vsync_source=true |
| stage_chase | default | native | vsync_source=true |
| stage_chase | default | go | vsync_source=true |
| step | default | native | vsync_source=true |
| step | default | go | vsync_source=true |
| tex | far | native | vsync_source=true |
| tex | far | go | vsync_source=true |
| tex | stream | native | vsync_source=true |
| tex | stream | go | vsync_source=true |
| tilemap | map | native | vsync_source=true |
| tilemap | chunk | native | vsync_source=true |
| tilemap | chunk | go | vsync_source=true |
| tilemap | lod | native | vsync_source=true |
| tilemap | lod | go | vsync_source=true |
| tilemap | large | native | vsync_source=true |
| tilemap | large | go | vsync_source=true |
| world | open | native | vsync_source=true |
| world | open | go | vsync_source=true |
| world | large | native | vsync_source=true |
| ui_render_base_backdrop | msaa1 | native | vsync_source=true |
| ui_render_base_backdrop | msaa1 | go | vsync_source=true |
| ui_render_base_backdrop | msaa4 | native | vsync_source=true |
| ui_render_base_backdrop | msaa4 | go | vsync_source=true |
| ui_render_base_cliplayer | default | native | vsync_source=true |
| ui_render_base_cliplayer | default | go | vsync_source=true |
| ui_render_base_clipop | default | native | vsync_source=true |
| ui_render_base_clipop | default | go | vsync_source=true |
| ui_render_base_cliprrect | default | native | vsync_source=true |
| … | | | 其余 228 条见 summary.tsv |

### cpu_ui/cpu_raster 双 0：0

无。

### A 族字段缺失：105

| 窗 | 变体 | 后端 | 详情 |
|---|---|---|---|
| anim | tl | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| anim | tl | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| light2d | shadow | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| tilemap | map | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| world | large | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_render_base_perfsoak | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz |
| ui_render_base_perfsoak | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz |
| ui_render_base_text | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_render_base_textstroke | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_ant_verify | msaa1 | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_ant_verify | msaa1 | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_ant_verify | msaa4 | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_ant_verify | msaa4 | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_ruler_parity | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_ruler_parity | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_wr_r7b_scroll_reuse | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_wr_r15_soak | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_wr_c1_boundary_nest | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_wr_c6_savelayer_group | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_wr_ime_r2_textlayout | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_wr_ime_r4_selection | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_wr_shadermask | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_base_vram_cap | ladder | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_base_vram_cap | ladder | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_l1_blank | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz |
| ui_l1_blank | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz |
| ui_l1_render_matrix | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz |
| ui_l1_render_matrix | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz |
| ui_l1_scroll | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz |
| ui_l1_scroll | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz |
| ui_l1_spinner | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz |
| ui_l1_spinner | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz |
| ui_l2_shell | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_l2_shell | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_ime_probe | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_ime_probe | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_x11 | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_x11 | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_wayland | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_wayland | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_win32 | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_win32 | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_appkit | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_appkit | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_x11_device | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_x11_device | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_x11_dnd | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_pf_x11_dnd | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_textinput_ime | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| ui_textinput_ime | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| kit_f0_scope | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| kit/spin_min | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz |
| kit/spin_min | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz |
| video_retained_golden | default | native | interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_retained_golden | default | go | interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_player | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_player | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_2k_play | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_2k_play | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_4k_play | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_4k_play | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_a2_sync | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_a2_sync | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_a4_sink | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_a4_sink | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_b1_frag | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_b1_frag | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_s8_index | default | native | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_s8_index | default | go | fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| video_vc0_fullplay | default | native | interval_p99_ms |
| video_vc0_fullplay | default | go | interval_p99_ms |
| video_vc1_seek_fault | default | native | interval_p99_ms |
| video_vc1_seek_fault | default | go | interval_p99_ms |
| video_vc2_soak | default | native | interval_p99_ms |
| video_vc2_soak | default | go | interval_p99_ms |
| video_vc3_embed_extend | default | native | interval_p99_ms |
| video_vc3_embed_extend | default | go | interval_p99_ms |
| video_vr0_demux | default | native | interval_p99_ms |
| video_vr0_demux | default | go | interval_p99_ms |
| video_vr1_params | default | native | interval_p99_ms |
| … | | | 其余 25 条见 summary.tsv |

### 金图 diff 非 0：38

| 窗 | 变体 | 后端 | 详情 |
|---|---|---|---|
| anim | sk | native | 74.9220% |
| anim | sk | go | 74.9220% |
| anim | fsm | native | 73.6752% |
| anim | fsm | go | 73.6752% |
| anim | tl | native | 25.7695% |
| anim | tl | go | 25.7695% |
| fx | all | native | 100.0000% |
| fx | all | go | 100.0000% |
| light | torch | native | 99.8429% |
| light | torch | go | 99.8429% |
| light | shadow | native | 99.8429% |
| light | shadow | go | 99.8429% |
| light | normal | native | 99.6451% |
| light | normal | go | 99.6451% |
| stage_chase | default | native | 99.1429% |
| stage_chase | default | go | 99.1429% |
| ui_render_base_backdrop | msaa1 | go | 1.2583% |
| ui_render_base_backdrop | msaa4 | go | 52.7042% |
| ui_render_base_clipop | default | go | 0.0078% |
| ui_render_base_filterops | default | go | 9.4798% |
| ui_render_base_image | default | go | 0.2877% |
| ui_render_base_miscdraw | default | go | 0.0003% |
| ui_render_base_scene | default | go | 0.0032% |
| ui_render_base_variablefont | default | native | 0.9517% |
| ui_render_base_variablefont | default | go | 0.9517% |
| ui_render_base_verticesmesh | default | go | 5.6751% |
| ui_render_graphics/vector | default | native | 0.0005% |
| ui_render_graphics/vector | default | go | 0.0005% |
| ui_wr_r5_picture | default | native | 0.0217% |
| ui_wr_r5_picture | default | go | 0.0217% |
| ui_wr_ime_r1_editor | default | native | 0.0847% |
| ui_wr_ime_r1_editor | default | go | 0.0847% |
| ui_wr_shadermask | default | native | 4.2822% |
| ui_base_batch_cap | default | go | 0.0169% |
| ui_text_m5_anycase | default | native | 3.1630% |
| ui_text_m5_anycase | default | go | 3.1630% |
| kit/icon | default | go | 3.3203% |
| video_retained_golden | default | go | 78.9469% |

## 运行期金图（8 扇）

| 窗 | 基线归档 | 首遍 | 次遍 | 次遍 diff |
|---|---|---|---|---:|
| ui_wr_r1_layout | `docs/render_gpu_converge/baseline/windows/ui_wr_r1_layout/default/native/snapshots/r1_final_base.png` | SEED | FAIL | 0.0000% |
| ui_wr_r14_cache_budget | `docs/render_gpu_converge/baseline/windows/ui_wr_r14_cache_budget/default/native/snapshots/r14_final_base.png` | SEED | PASS | 0.0000% |
| ui_wr_r15_soak | `docs/render_gpu_converge/baseline/windows/ui_wr_r15_soak/default/native/snapshots/r15_final_base.png` | SEED | PASS | 0.0000% |
| ui_wr_r20_filter | `docs/render_gpu_converge/baseline/windows/ui_wr_r20_filter/default/native/snapshots/r20_final_base.png` | SEED | FAIL | 0.0000% |
| ui_wr_r22_selection_stub | `docs/render_gpu_converge/baseline/windows/ui_wr_r22_selection_stub/default/native/snapshots/r22_final_base.png` | SEED | FAIL | 0.0000% |
| ui_wr_c6_savelayer_group | `docs/render_gpu_converge/baseline/windows/ui_wr_c6_savelayer_group/default/native/snapshots/c6_final_base.png` | SEED | FAIL | 0.0000% |
| ui_wr_c9_stress_cache | `docs/render_gpu_converge/baseline/windows/ui_wr_c9_stress_cache/default/native/snapshots/c9_final_base.png` | SEED | PASS | 0.0000% |
| ui_wr_c10_soak | `docs/render_gpu_converge/baseline/windows/ui_wr_c10_soak/default/native/snapshots/c10_final_base.png` | SEED | PASS | 0.0000% |

## 无金图窗（40）

逐窗原因写在 `<窗名>/README.md`；共同口径：动态画面/纯性能/纯指标/平台探针不做固定点金图，替代判定用逻辑探针、像素断言、性能门禁或 JSON 字段完备性。

## 证据目录

- 逐窗：`docs/render_gpu_converge/baseline/windows/<窗名>/<变体>/<native|go>/`（`record.json`、`stdout.txt`、`stderr.log`、`snapshots/`、`golden/`）
- 汇总：`docs/render_gpu_converge/baseline/windows/summary.tsv`、`docs/render_gpu_converge/baseline/windows/summary.md`、`docs/render_gpu_converge/baseline/windows/README.md`

