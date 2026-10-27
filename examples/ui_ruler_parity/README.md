# ui_ruler_parity — gpui 侧同语义对照真窗

P3 对照尺的 gpui 臂。一个二进制里放四个场景，按 `RULER_SCENE`（或 `-scene`）选择：

| 场景 | 对位参考尺 | 客户区 | 内容 |
|---|---|---|---|
| `spin` / `flutter_spin` | Flutter `RULER_SCENE=spin` | 480×320 | 单 64px 白色旋转图标，2s 一圈，无文本 |
| `list` / `flutter_list` | Flutter `RULER_SCENE=list` | 1200×800 | 48 header + 12 body + 260 legend + 620 列表 + 16 缝 + 268 静态右栏 + 72 HUD；1200 项变高行（44/92/60/76），24 张 96×56 图，Steady/Spike/Recover 160→2600 px/s |
| `text` / `flutter_text` | Flutter `RULER_SCENE=text` | 1200×800 | 560px 文本列；m5 的 multiscript/emoji/richtext 语料；首帧后静止，无 ticker |
| `godot` / `godot_bullets` | Godot 4.5.1 `2d/bullet_shower` | 1200×800 | 500 发 32×32 子弹，20–80 px/s，x<-16 回绕；27px 玩家圆跟随鼠标，碰 8px 子弹换哭脸 |

这些窗只做“同尺寸、同布局、同内容、同动画、同负载”的对照；**不宣称跨引擎逐像素一致**。差异见下文，实测与比值见 `docs/RENDER_BASE_RESOURCE_MATRIX.md` §5.5。

## 运行

> **安全（硬）**：批量测量和长循环压测必须在隔离显示里跑（Wayland/XWayland、独立 Xorg 或嵌套 X server）；单窗验收/测量可走当前桌面，但必须先通过隔离验证，按“一轮一窗、跑完查 Xorg”执行。下面以嵌套 Xephyr 为例；结束测量时 `measure.py` 先发 WM_DELETE_WINDOW，应用走 `EventCloseRequested` → `Quit()` → `app.Close()` → `win.Close()` 正常关闭；SIGTERM/SIGINT 只是兜底。

```bash
# 先启动隔离显示；例如：
# Xephyr :2 -ac -screen 1280x1024 &
export DISPLAY=:2
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# live：跑 60s 后自动关；不设 RUN_SECONDS 则常驻等人工关窗
RULER_SCENE=list RUN_SECONDS=60 go run ./examples/ui_ruler_parity

# 冻结帧三证据门禁（逻辑探针 + 像素断言 + 零容差 Golden）
RULER_SCENE=list RUN_SECONDS=5 go run ./examples/ui_ruler_parity -auto-only

# 首次建 Golden 基线：seed 运行只建基线，不算 PASS。
# 更新基线必须先把 testdata/golden_*.png 备份/移走；有旧文件时只比较、不覆盖。
GPUI_ACCEPT_GOLDEN=1 RULER_SCENE=list RUN_SECONDS=5 \
  go run ./examples/ui_ruler_parity -auto-only

# 换场景
RULER_SCENE=spin  RUN_SECONDS=60 go run ./examples/ui_ruler_parity
RULER_SCENE=text  RUN_SECONDS=60 go run ./examples/ui_ruler_parity
RULER_SCENE=godot RUN_SECONDS=60 go run ./examples/ui_ruler_parity
```

`RUN_SECONDS` 是 live 时长；`RULER_SECONDS` 与 Flutter 的 `measure.py --scene` 契约相同（采数时长 + 预热 + 余量）。`RULER_SNAP` 可指定 live 快照路径；`RULER_BULLETS` 只改 Godot 场景的发数，默认 500（改它后 Golden 不再适用）。

## 三证据与冻结点

`-auto-only` 不走 ticker，按固定状态出 snapshot，再做：

1. **逻辑探针**：真实首帧 present、`warmup=true`、`cpu_fallback_ops=0`；静态场景不要求 `paint_count>0`（首帧不在调度器 paint 计数里），用快照像素做绘制证据。
2. **像素断言**：header/body/panel/右栏格子/HUD/玩家/背景等语义点位；另加 spin 字形墨迹、text 正文墨迹、Godot 子弹云墨迹计数。
3. **Golden**：零容差逐位对比。`spin`/`godot` 全窗；`list`/`text` 比 header + body，排除底部动态 HUD。seed 运行输出 `golden_seeded=true`，必须无 seed 再跑一次 `golden_diff_pct=0` 才算绿。

冻结点：list 为 `y=1234`；Godot 为 30s 物理步进后的固定弹云；spin 为初始旋转相；text 本身静止。冻结仅用于门禁，live 60s 资源测量另行进行。

## 与参考尺的差异（必须随数字一起读）

- **跨引擎像素**：字体整形/栅格化、AA、emoji 字体版本、GPU 后端都不同，逐像素全等不是目标；对齐的是客户区、区块几何、语料、颜色、字号、动画相位和负载。
- **spin**：Flutter 用 Material `sync`，gpui 用 kit `sync`，轮廓不同；尺寸/中心/颜色/2s 周期相同。spin 场景不加载 CJK 字体链，避免给无文本臂虚增 RSS。
- **list**：Flutter 用 lazy `ListView.builder`，gpui 用 retained `VariableVirtualList`；gpui 在开窗前加载 24 张源图（共享给可见行），Flutter 走 `Image.asset` 的异步解码。滚动条为 6px、无轨道的近似，位置/比例对齐。两边都不是同一实现，CPU 差不能归因到单一 API。
- **text**：gpui 和 Flutter 的 shaper/rasterizer 不同，字形轮廓不保证一致；语料、560px 列宽、字号/颜色/行高对齐。Flutter 每行是一个 `TextSpan`；gpui 也按行建 `RenderText` 节点，行高显式取 `1.2`（避免把跨行混合字号塞进一个最小 paragraph 的父行高）。这是行导向语料的显示等价，不是文本整形吞吐对照。
- **Godot**：这是官方 demo 的绘制/回绕/玩家碰撞语义，不是 Godot PhysicsServer2D 的 1:1 性能复制。gpui 侧用 27px/8px 圆做玩家-子弹碰撞，没有逐对象物理 body；500 发可用于绘制负载对照，5k/10k 的物理过载数字不能拿来和 gpui atlas 图元直接比值。随机弹云用固定种子，分布与上游同规则但具体落点不同。
- **HUD**：list 的 HUD 是动态文本，不进入 Golden；text 的 HUD 静态但同样排除，避免把状态文案冻结进画面基线。

## 资产

- `c3_src_*.png`：与 `examples/ui_wr_c3_list_scroll/testdata` 同源的 24 张 checkerboard。
- `multiscript.txt` / `emoji.txt` / `richtext.txt`：来自 `examples/ui_text_m5_anycase/testdata` 的 m5 语料。
- `bullet.png` / `face_happy.png` / `face_sad.png`：来自 `godot-demo-projects` 的 `2d/bullet_shower`（上游 MIT，见其 `LICENSE.md`）。
- `golden_*.png` / `showcase_*.png`：本窗自己的冻结基线与 live/auto 快照。

## 与旧 C3/m5/spin_min 的关系

旧窗仍保留：`spin_min` 是心跳税地板（无 Golden 豁免），C3 是 retained 列表能力窗，m5 是任意场景文本能力窗。它们与参考尺的布局/状态机并不完全同形（C3 右栏、异步图不同；m5 有 260px legend 和 HUD tick；spin_min 只到 `spin_min` 语义）。本包是**对照专用**臂，不替代能力窗，也不把旧窗改成对照窗。
