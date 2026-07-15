# S3a — Render M0–M1 GPU 门禁

> 版本：1.0 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) S3a  
> 架构：`render.Context → accelerator → gpu/webgpu → gpu/rwgpu → libwgpu_native`

## 范围（M0–M1）

清屏、solid fill、path fill/stroke、circle、hairline、CTM（translate/scale/rotate/push-pop）、clip rect、AA、SourceOver（既有 P1.2）。

## 硬规则

1. `WGPU_NATIVE_PATH` 真库（或可发现的 native）。  
2. `FlushGPU` 后 **`GPUOps > 0`**；禁止 silent CPU-only 过门。  
3. 固定像素/区域语义检查。  
4. 性能数字不作为退出条件。

## 门禁命令

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache

# S3a 固定像素
go test -count=1 ./render -run 'TestS3a_|TestP12GPUFixedPixel'

# 可选：CPU/GPU 场景 STRICT
GPUI_BASIC_VISUAL=1 GPUI_BASIC_VISUAL_STRICT=1 \
  go test -count=1 ./render -run 'TestBasicCPUvsGPUVisualDiagnostic'
```

## 测试映射

| 能力 | 测试 |
|------|------|
| Clear / solid | `TestS3a_M0_ClearWithColor`, `TestS3a_M0_SolidFillRect` |
| SourceOver premul | `TestP12GPUFixedPixel_SourceOverPremul` |
| Stroke / hairline | `TestS3a_M1_StrokeRect`, `TestS3a_M1_Hairline`, `TestP12GPUFixedPixel_HairlineStroke` |
| Path / circle | `TestS3a_M1_PathTriangleFill`, `TestS3a_M1_CircleFill` |
| CTM | `TestS3a_M1_CTMTranslate`, `CTMScale`, `PushPopCTM`, `RotateAbout` |
| Clip | `TestS3a_M1_ClipRect`, `TestP12GPUFixedPixel_ClipOutsideClear` |
| AA | `TestS3a_M1_AntiAliasToggle` |
| 场景 diff | `TestBasicCPUvsGPUVisualDiagnostic` + STRICT |

## 退出检查

- [x] M0–M1 关键固定像素门禁绿（`TestS3a_*` + `TestP12*`）  
- [x] 每项 `GPUOps>0`  
- [x] BASIC visual STRICT 绿（circle/rect/stroke 区域）  
- [x] 能力表 M0–M1 相关 render 列回写  
- [x] 文档本表  

**S3a：✅ 关闭**

已知软点（不挡 S3a）：

- Hairline `width=0` 与 `width=1` 像素分布可能因 AA 不同；门禁接受邻域有墨迹。  
- AA fringe 计数为质量 soft 信号，非 bit-exact。  
- Cap/Join 细粒度几何差异 → 可在 S3b 加强。  

下一阶段：**S3b**（blend 全集、image、text、rrect、layer opacity、dash、gradient、MSAA）。

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | S3a 门禁落地：`s3a_m0m1_gpu_gate_test.go` |
