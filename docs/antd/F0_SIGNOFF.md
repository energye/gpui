# F0 六窗签字单（F0-6 关闭用）

> 状态只认本单 + `PROGRESS.md §1`。签字不过 F1 不开。

## 1. 六窗 `-auto-only` 全绿（2026-09-21，X11 真机）

| 窗 | 命令 | 脚本 | Golden | 调大小回基线 |
| --- | --- | --- | --- | --- |
| scope | `go run ./examples/kit_f0_scope -auto-only` | 3/3 | `showcase_scope_base.png` 差异 0 | 1200x800 ✓ |
| layout | `go run ./examples/kit_f0_layout -auto-only` | 3/3 | `showcase_layout_base.png` 差异 0 | 1200x800 ✓ |
| decor | `go run ./examples/kit_f0_decor -auto-only` | 3/3 | `showcase_decor_base.png` 差异 0 | 1200x800 ✓ |
| content | `go run ./examples/kit_f0_content -auto-only` | 6/6 | `showcase_content_base.png` 差异 0 | 1200x800 ✓ |
| interact | `go run ./examples/kit_f0_interact -auto-only` | 15/15 | `showcase_interact_base.png` 差异 0 | 1200x800 ✓ |
| motion | `go run ./examples/kit_f0_motion -auto-only` | 17/17 | `showcase_motion_base.png` 差异 0 | 1200x800 ✓（retained/damage 0.09/光栅 0.22ms） |

## 2. 主题三套 Golden（2026-09-21，X11 真机）

同一按钮横排三块，一次 `theme.Default.SetBase` 整树跟随；换肤改组件代码即 FAIL。

| 主题 | 命令 | 脚本 | Golden |
| --- | --- | --- | --- |
| default | `THEME=default go run ./examples/kit_f0_theme -auto-only` | 10/10 | `showcase_theme_default_base.png` 差异 0 |
| skinned | `THEME=skinned go run ./examples/kit_f0_theme -auto-only` | 10/10 | `showcase_theme_skinned_base.png` 差异 0 |
| compact | `THEME=compact go run ./examples/kit_f0_theme -auto-only` | 10/10 | `showcase_theme_compact_base.png` 差异 0 |

整树禁用盖住三块（点击全 0）、尺寸跟主题（default 32 / compact 24 + 切小恢复）已在脚本 10 项内。

## 3. Wayland 双证据（嵌套 GNOME 合成器，见 `ENGINE_WAYLAND_NESTED_TEST.md`）

2026-09-21 首跑结论：合成器立住了（套接字 `ALIVE`），窗也起来了（748×541，
嵌套缩放与基线 1200×800 不对齐），但三条门禁线证明**当前嵌套环境跑不出
Wayland 有效证据**，不是窗的问题：

| 现象 | 数 | 说明 |
| --- | --- | --- |
| 窗尺寸不对齐 | 快照 748×541 vs 基线 1200×800 | 嵌套缩放/装饰吃掉尺寸，Golden 掩码直接报 size mismatch |
| 像素全错位 | 两探针全 FAIL（取到底色） | 尺寸一错，探针坐标全落在底上 |
| 流畅门禁全红 | fps 9.65，p95 144ms，光栅 109ms，CPU 回退 18450 ops | 嵌套+软件栈代价（文档 §5.2 已预警：慢且 jank 正常），60Hz 门禁在此环境下无意义 |
| 后端身份 | JSON 无 `backend` 字段，`close (…)` 行未打印 | 窗在 `Run` 内 FAIL 退出，走不到 close 行；`platform.Open` 看到 `WAYLAND_DISPLAY` 自动选后端，但 JSON 不记录，无从验明正身 |

缺口（拦 F1 开工前补）：

1. [x] **JSON 记后端身份**：已补（`wrgate.Report.backend` + 七窗 `win.Backend().String()`；
   X11 跑出 `backend=x11`，嵌套跑出 `backend=wayland`，验明正身可用）。
2. **Wayland 门禁口径**：嵌套环境需单独的通过线（去 fps 门禁、去像素探针/Golden，
   只留 presents>0 + 脚本逻辑项），否则永远红。
3. **尺寸对齐**：嵌套缩放后的可绘制区需先标定，再谈 Golden 对齐。

2026-09-21 复验（scope 窗，嵌套 `gpui-nested`）：`backend=wayland` 验明正身通过；
fps 10.6 / p95 115ms / 光栅 90ms / presents 54 —— 嵌套+软件栈代价，门禁口径 2 未定前记 FAIL（非窗问题）。

| 窗 | X11（本机 :1） | Wayland（嵌套 `gpui-nested`） |
| --- | --- | --- |
| scope | §1 PASS | 首跑 FAIL（上表四行，非窗问题） |
| layout | §1 PASS | 待跑（等 1–3 补完再跑） |
| decor | §1 PASS | 待跑 |
| content | §1 PASS | 待跑 |
| interact | §1 PASS | 待跑 |
| motion | §1 PASS | 待跑 |
| theme×3 | §2 PASS | 待跑 |

## 4. 人工签字

| 项 | 结论 | 日期 |
| --- | --- | --- |
| 六窗常驻人工看过 | 通过（用户确认） | 2026-09-21 |
| 主题窗常驻人工看过 | 待用户确认 | — |
| motion 顺畅（省画后） | 通过（用户确认顺了很多） | 2026-09-21 |

## 5. 缺口记账（签字时已知，不拦 F0-6，拦 F1 开工前补）

- [x] 三套主题切换 Golden：已补（`examples/kit_f0_theme/`，三张基线入库）
- [x] 本签字文档：已建（本文件）
- [ ] Wayland 双证据：待跑（本单 §3）
