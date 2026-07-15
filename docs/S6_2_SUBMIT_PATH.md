# S6.2 — 录制/提交 CPU 路径

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S6.2 关闭**  
> 依赖：S6.1 `docs/S6_1_FRAME_ENFORCE.md` + S6.0 回归锁  
> 架构：`Flush → RenderFrameGrouped → encodeSubmit* → Queue.Write*/Submit`

---

## 1. 目标

降低 **每帧 CPU 录制/提交** 成本（非改绘制算法）：

1. 去掉 Flush 内无意义的 **pending deep-copy**  
2. 单 scissor group **跳过 concat**（直接用 group 切片）  
3. multi-group / imageSeal / grpRes **session scratch 复用**  
4. Clip/SDF uniform **BytesInto + scratch**，避免热路径 `make([]byte)`  
5. CommandEncoderDescriptor **静态复用**  
6. `SubmitPathStats` 诊断：groups / encoders / submits / writeBuffers / singleGroupFast  

**非目标**：S6.3 batch 合并加深、S6.4 layer/filter、控件层。

---

## 2. 关键改动

| 位置 | 改动 |
|------|------|
| `gpu_render_context.Flush` | 不再 deep-copy scissor groups；atlas+encode 后再 `clear` pending |
| `RenderFrameGrouped` | `len(groups)==1` → `SingleGroupFast`；否则 scratch concat |
| `ClipParams.BytesInto` / `makeSDFRenderUniformInto` | 零分配热路径 |
| `queueWriteBuffer` + static encoder desc | 计数 + 少 alloc |
| `LastSubmitPathStats` | session / GPURenderContext 可观测 |

---

## 3. 诊断 API

```go
st := session.LastSubmitPathStats()
// Groups, EncodersCreated, Submits, WriteBuffers, WriteBytes, SingleGroupFast
```

典型稳态 UI（单脏区、单 group）：`SingleGroupFast=true`，`EncodersCreated≥1`，`WriteBuffers≥1`。

---

## 4. 测试

| 测试 | 作用 |
|------|------|
| `TestS62_ClipParams_BytesInto_NoAllocHot` | BytesInto 0 alloc |
| `TestS62_SDFUniformInto_NoAllocHot` | uniform Into 0 alloc |
| `TestS62_RenderSession_StatsAfterGrouped` | 真 GPU stats + SingleGroupFast |
| `TestS62_RenderSession_StatsAfterGrouped (2nd flush)` | 温路径 alloc 上界 |
| `TestS62_PresentMainPath_NoRegress` | present-only p50 ≤16.7ms |
| `TestS62_L0_S61HelpersStillGreen` | idle PresentFrameAuto 仍绿 |

```bash
export WGPU_NATIVE_PATH=.../lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=.../lib:$LD_LIBRARY_PATH

go test -count=1 ./render/internal/gpu -run 'TestS62_' -timeout 180s
go test -count=1 ./render -run 'TestS62_|TestS61_|TestS6_L0_|TestS52_|TestS53_' -timeout 300s
go test -count=1 ./render -run 'TestP1_Comp_(D01|D06|D08|D36|D63|D152)_' -timeout 300s
```

---

## 5. 退出条件

| 条件 | 状态 |
|------|------|
| Flush deep-copy 移除 + 生命周期正确 | ✅ |
| 单 group 快路径 + scratch | ✅ |
| uniform/clip 热路径 0 alloc | ✅ |
| SubmitPathStats 可观测 | ✅ |
| present 主路径不回退；L0/L1 抽样绿 | ✅ |
| 无 silent CPU / 无降语义 | ✅ |

**S6.2 关闭。** 下一：**S6.3 绘制合并加深**。
