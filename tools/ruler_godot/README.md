# Godot 2D 对照尺（P3）

用官方 Godot 2D demo `bullet_shower` 做同机、同 1200×800 客户区的 2D 对照臂。
上游 demo 不被修改：`setup.sh` 把它复制到 `$GODOT_WORK/bullet_shower`，只在
副本上加 FPS 探针和 `GODOT_BULLETS` 容量开关。

## 固定版本

- Godot：`4.5.1-stable` 官方 Linux x86_64 预编译包
  - binary sha256：`db07cae7de644278a1884d4552bdf2bca3f5d30131b18faf3a0c4d730080b199`
  - 下载：`https://github.com/godotengine/godot/releases/download/4.5.1-stable/Godot_v4.5.1-stable_linux.x86_64.zip`
- demo：`godot-demo-projects` 的 `2d/bullet_shower`
  - 本机 checkout commit：`a3b5c113112f77291d5f3d1360f33a882fdc52f7`
  - 上游：`https://github.com/godotengine/godot-demo-projects`

本机 `project.godot` 的 `config/features` 写着 4.7（demo 仓用较新编辑器保存），
但 4.5.1 能正常 import 和出画；实际渲染用 `gl_compatibility` / OpenGL 3.3。

## 准备

```bash
export GODOT_DEMO_ROOT=/home/yanghy/app/projects/gogpu/godot-demo-projects
tools/ruler_godot/setup.sh
```

脚本会：

1. 下载/校验 Godot 4.5.1 到 `$GODOT_WORK`（默认 `/tmp/opencode/godot_ruler`）；
2. 复制 `2d/bullet_shower` 到 `$GODOT_WORK/bullet_shower`；
3. 加 `fps_probe.gd`（每 2s 打 `GODOT_FPS=... t=...`）；
4. 把 `BULLET_COUNT` 改成可由 `GODOT_BULLETS` 覆盖，默认仍是上游 500；
5. `--headless --import` 生成 `.godot/imported`。

可用 `GODOT_BIN=/path/to/godot` 复用已有二进制；`GODOT_WORK`、`GODOT_DEMO_ROOT`
都可覆盖。

## 运行/采数

```bash
# 默认 500 发，1200×800
tools/ruler_godot/run.sh

# 60s 测量（5s 预热），输出 report.json + shot.png
tools/ruler_godot/measure.sh /tmp/opencode/ruler_runs/godot_500

# 容量探针：只改每帧物理对象数，渲染路径不变
GODOT_BULLETS=1000 DURATION=10 tools/ruler_godot/measure.sh /tmp/opencode/ruler_runs/godot_1000
GODOT_BULLETS=5000 DURATION=10 tools/ruler_godot/measure.sh /tmp/opencode/ruler_runs/godot_5000
GODOT_BULLETS=10000 DURATION=10 tools/ruler_godot/measure.sh /tmp/opencode/ruler_runs/godot_10000
```

`measure.sh` 直接调用 Flutter 尺的 `scripts/measure.py`：等真实 X11 窗口、
按 `xwininfo` 客户区截屏、预热后从 `/proc` 采 CPU/RSS、读 `GODOT_FPS`。

## 口径与边界

- **500 发是上游默认语义**：每发一个 `PhysicsServer2D` body + 碰撞圆 +
  逐帧 `draw_texture`，画面持续动。
- **1k/5k/10k 是容量探针**：只改 `GODOT_BULLETS`，物理服务器仍逐对象更新；
  5k/10k 的 CPU 被单线程物理吃满，不能当成"纯图元 parity"去和 gpui
  `ui_base_batch_cap` 的 10k 图元直接划等号。矩阵里必须写明这一点。
- 帧率由 `Engine.get_frames_per_second()` 探针给出；采数器按 `t=` 丢掉预热前
  的启动样本。
- 截图只证明 1200×800 客户区非黑/内容可见，不是 Golden；对照尺不替代
  引擎真窗的像素门禁。
