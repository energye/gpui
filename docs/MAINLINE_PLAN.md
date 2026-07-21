# GPUI 渲染栈主线状态（精简 · 现网）

> 版本：2.0 | 日期：2026-07-21 | **活文档**  
> 状态：S0–S6 **历史阶段已关闭**；当前以本页 + [`README.md`](./README.md) 所列活文档为准  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 能力：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)  
> 缺口：[`ENGINE_GAPS.md`](./ENGINE_GAPS.md)  
> 优化：[`PERF_ENGINE_FORWARD.md`](./PERF_ENGINE_FORWARD.md)

---

## 1. 目标（仍有效）

1. **rwgpu / webgpu / render** 按 Skia 2D 语义可测、可 present。  
2. GPU 优先（[`GPU_FIRST_ROUTING.md`](./GPU_FIRST_ROUTING.md)）。  
3. 真窗口 lifecycle / device lost 可恢复（[`SURFACE_LIFECYCLE_SKIA_FLUTTER.md`](./SURFACE_LIFECYCLE_SKIA_FLUTTER.md) · [`GPU_修复_device_lost.md`](./GPU_修复_device_lost.md)）。  
4. 内存与性能护栏（[`MEM_LEAK_PERF_GUARD_PLAN.md`](./MEM_LEAK_PERF_GUARD_PLAN.md)）。  
5. **不在本主线实现 Ant Design 控件层**；控件开工条件见 [`S5_WIDGET_ENTRY.md`](./S5_WIDGET_ENTRY.md)。

---

## 2. 阶段结论（摘要）

| 阶段 | 结论 | 现网证据 |
|------|------|----------|
| S0–S2 | ABI + facade 可用 | `gpu/rwgpu` · `gpu/webgpu` + 测试 |
| S3–S4 | 2D GPU + batch/atlas/damage | `go test ./render -run 'TestS3|TestP1_|TestS4|TestS43|TestS44'` |
| S5 | present / 帧模型 / 控件入口条件 | `PresentFrame*` · `S5_WIDGET_ENTRY` |
| S6 | 深性能与重场景 | `TestS6*` · particle / mem 护栏 |
| 生命周期 | purge / OOM 自适应 / ForceRecoverHealthy | `render/gpu/lifecycle_policy.go` · `exboot.SurfaceHost` |

实现细节以 **代码与测试** 为准，不再维护分阶段独立文档。

---

## 3. 当前工程焦点（2026-07-21）

| 优先级 | 内容 | 文档 |
|--------|------|------|
| P0 | 稳：lost / minimize / 多 RT / VRAM | SURFACE · device_lost · ENGINE_GAPS G3 |
| P0 | 引擎缺口 G1–G3 | [`ENGINE_GAPS.md`](./ENGINE_GAPS.md) |
| P1 | 正向优化 | [`PERF_ENGINE_FORWARD.md`](./PERF_ENGINE_FORWARD.md) |
| P2 | 控件层（另开） | S5_WIDGET_ENTRY |

---

## 4. 常用命令

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

go test -count=1 ./render -run 'TestP1_' -timeout 600s
go test -count=1 ./render/gpu -run 'Lifecycle|TextureOOM|AdapterPolicy' -timeout 60s
./scripts/run_mem_guard.sh
GPUI_COVERAGE_STRICT=1 GPUI_SELFTEST_LIFECYCLE=1 go run ./examples/api_coverage_app
```

---

## 5. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-21 | 2.0 | 只保留现网状态；删除历史分阶段卡正文与断链 |
