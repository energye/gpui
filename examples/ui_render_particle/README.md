# ui_render_particle — 粒子特效演示真窗

渲染层粒子能力演示：用 UI 渲染管线（RenderObject 树 + GPU Present）
模拟传奇类游戏三类法术特效 + 星空背景。

| 特效 | 位置 | 手法 |
|------|------|------|
| 火球术 | 右下椭圆轨道 | 三层同心盘立体球（亮心→橙→暗红边）+ 径向渐变光晕 + 拖尾弧线 + 26 溅射火花 |
| 冰霜新星 | 左上 (210,210) | 扩散冰环（双层）+ 8 放射冰晶线 + 24 环绕粒子 + 纯白冰核 |
| 魔法旋涡 | 中心 (600,400) | 90 螺旋粒子（紫→青 色相渐变）+ 径向渐变紫光 + 核心 |
| 星空 | 全屏 | 140 加色混合闪烁亮点 |

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 默认 20s（可调），结束打印指标 JSON
RUN_SECONDS=15 go run ./examples/ui_render_particle

# 结束时读回一帧 PNG（看单帧效果）
SNAPSHOT=tmp/particle_shot.png RUN_SECONDS=5 go run ./examples/ui_render_particle
```

## 指标与门禁

- JSON：fps / interval p50·p95·p99 / hitch / gpu_ops / cpu_fallback_ops / rss / cpu（A–J 精简族，源：Metrics().JSON()）。
- 门禁：有帧即过；`RUN_SECONDS>=10` 时 fps>=30（全屏粒子满负荷线）。

## 说明

- 粒子绘制走 `PaintContext` 公开绘制 API（FillCircle / StrokeLine / StrokeCircle / FillRadialGradient / StrokeArc），
  底层经 render.Context -> GPU（WebGPU）合批提交；每帧约 280 个 path 提交（星空 140 + 火花 26 + 环粒 24 + 旋涡 90）。
- 舞台无 RepaintBoundary：全屏动态特效整窗重绘是常态，保留边界缓存无收益（与被删的
  examples/particle_kitchen_sink 的 L0–L4 压测结论一致：粒子数量级在 3000 内由 mesh/图集合批保证 60fps）。
- 立体感手法：同心盘亮度分层（球体明暗）+ 径向渐变光晕（FillRadialGradient）。GPU 渲染器另有
  `DrawMesh` 逐顶点 Gouraud 着色、`NewRadialGradientBrush` 焦点渐变，可做更细的球体光照。