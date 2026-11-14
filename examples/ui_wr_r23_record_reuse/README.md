# ui_wr_r23_record_reuse — O11 ① 层录制复用真窗

## 能力

retained 构建路径按 cacheKey 复用干净层的 `Recording`（`ui/rendering/layer_cache.go`，
`layer_build.go buildLayerRecording`）：干净层每帧指针复用零重录；脏层照常重录；
无指纹/无键类型保持 NeedsPaint 权威（与纹理腿同一信任契约，`scene/textured.go`
`changed := dirtyID || NeedsRaster`）；完整指纹类型（AbsoluteBox 背景、ColorBox）
不受祖先传播脏标记牵连（录音是其指纹输入的纯函数）。

## 场景

1200×800，retained 策略。24 个静态色格（各自 RepaintBoundary）+ 1 个每帧变色的
hot 边界 + 相位标签。auto-only 三相位：

| 相位 | 时窗 | 行为 | 证据 |
|---|---|---|---|
| SETTLE | 0–4s | 冷录 + 脏标记消费 | store=层总数 |
| STEADY | 4–10.5s | hot 每帧真变（唯一真重录源），其余全复用 | reuse/tick ≥ 12、store/tick ≤ 3 |
| FROZEN | 10.5–14s | hot 只 MarkNeedsPaint 不变内容 | storeΔ ≤ 2、帧照常 present |

## 跑法

```bash
# 自动门禁（隔离显示，串行）
env -u WAYLAND_DISPLAY DISPLAY=:3 LD_LIBRARY_PATH=$PWD/lib \
  WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so RUN_SECONDS=14 \
  timeout 60 ./bin/ui_wr_r23_record_reuse -auto-only

# 金图重种（只允许在逐位确认为基线问题时）
GPUI_ACCEPT_GOLDEN=1 <同上命令>
```

## 门禁（auto-only）

- `record_reuse_engaged=true`、steady 每帧复用 ≥12 层、每帧入库 ≤3（平均 ≤2）
- frozen 窗 store 增量 ≤2（内容未变零重录，端到端）
- present 非 full（retained damage 路径）、fps ≥ 55
- 金图 0.0000% 逐位 + 像素探针 3/3（复用不换像素）

## 单测

`ui/rendering/layer_cache_build_reuse_test.go`：指针恒等 / 脏帧重录后恢复 /
指纹漂移自愈 / 尺寸门 / 无键 fallback / Prune 释出 / 门单元（尺寸/指纹/零指纹跳过）。
