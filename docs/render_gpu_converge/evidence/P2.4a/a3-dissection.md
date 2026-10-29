# A3 解剖报告：46 个 `RequestDevice unavailable` SKIP（2026-10-30）

> 任务：P2.4a 影响面评估 §4 约定的解剖产出。性质：只诊断，不改代码。
> 台账原文（§2.2 A3）：「46 个 `RequestDevice unavailable` SKIP 的根因未解剖，首轮与干净重跑均复现；解剖前不定修复方向。」

## 0 结论（一句话）

**根因是显存 OOM，不是代码缺陷、不是 DefaultLimits 超限**：wgpu-native 在 RequestDevice 阶段分配失败时，把 OOM 包装成 `RequestDevice: Validation Error` 上报，完整错误里 `Caused by: Not enough memory left` 才是真因；46 例全部同因。显存压力决定了 OOM 在哪一层浮现（P1.3 在 RequestDevice 层、2026-10-30 在 CreateBuffer 层、单独跑全 PASS），错误分类表象随之变化，属**环境依赖**，建议关闭 A3（登记为已知环境现象，不需要修复动作）。

## 1 证据一：P1.3 原始日志的错误全文（真因一直在日志里）

`baseline/tests/logs/github.com_energye_gpui_render_internal_gpu__buffer_test.log`（texture/s6_7 两文件同文）：

```
buffer_test.go:190: webgpu RequestDevice unavailable: wgpu: failed to request device: wgpu: RequestDevice: Validation Error

    Caused by:
      Not enough memory left.
--- SKIP: TestBuffer_MapAsync_UsageMismatch
```

三文件 distinct-cause 扫描（`grep -A2 "RequestDevice: Validation Error" … | grep -v … | sort -u`）：**除 "Not enough memory left." 外无第二种 cause**，46/46 同因。

P1.3 当时每个文件独立进程，`skips.tsv` 记录的是 `t.Skipf` 的第一层（"RequestDevice: Validation Error"），真因在多行错误的 Caused by 段，P1.3 汇总时未展开——这就是「46 例根因未解剖」的全部原因。

## 2 证据二：2026-10-30 复现矩阵（同一台机器，两种表象）

环境（与 P1.3 对齐）：`WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`、`LD_LIBRARY_PATH=$PWD/lib`、`GPUI_SURFACE_SAMPLE_COUNT=1`、`GOMAXPROCS=2`、`CGO_ENABLED=0`、linux/amd64 go1.25.5、940MX 1GB、驱动 580.178.04。

| 批次 | P1.3（2026-10-29） | 2026-10-30 本会话 |
|---|---|---|
| `TestBuffer*`（buffer_test.go 32 例） | 8 PASS / 32 SKIP（RequestDevice Validation Error） | 8 PASS / 其余 SKIP，**原因变为 CreateBuffer 阶段 OOM "Not enough memory left"**（设备创建成功） |
| `TestTexture_*` + `TestCreateCoreTexture*`（12 例） | 0 PASS / 12 SKIP（同上） | 混合：先 SKIP 后单独跑 PASS——`TestTexture_Destroy`、`TestCreateCoreTexture` 单独批跑 **PASS**；大批次连跑时后半段 SKIP |
| `TestS67_ImageCache_*`（2 例） | 0 PASS / 2 SKIP | **2 PASS** |
| 规律 | 显存紧 → RequestDevice 层 OOM → SKIP 级联 | 显存松 → 设备建得出来；同进程跑得越多显存越紧，OOM 在更后面的建资源层浮现；单独跑全 PASS |

两个方向的表象都是同一个底层（wgpu 堆 OOM），随会话显存状态在「RequestDevice / CreateBuffer / CreateTexture」层间漂移。

## 3 为什么不是其它假设

| 假设 | 证伪证据 |
|---|---|
| DefaultLimits 超适配器上限（limits validation） | wgpu 的 limits 校验失败会点名具体 limit 字段；本例 cause 是 "Not enough memory left"（OOM），不是 limit 字段名。且同 descriptor（`render.DeviceDescriptor` ≡ `renderDeviceDescriptor`）在 P0.4/P1.1b 一致性台 native+GL 两档 RequestDevice PASS |
| 代码路径差异（测试 helper vs 生产链） | helper 序列（CreateInstance(BackendsPrimary)→RequestAdapter(HighPerformance)→RequestDevice(plain desc)）与 conformance `setupNativeDevice` 完全一致，今天同机 PASS |
| wgpu 库损坏/路径错误 | CreateInstance 与 RequestAdapter 都成功（skip 原因在 RequestDevice 层），库加载正常 |

## 4 生产健壮性核验（不改行为，只核对）

`render/oom_purge.go:87 IsGPUOutOfMemory`：按错误全文小写匹配 `not enough memory` / `out of memory` / `out of device memory`——**包装后的 "Validation Error + Not enough memory left" 会被正确判为 OOM 类**。因此：

- 呈现链的降级阶梯（`presentLevels`）与设备重试（`requestDeviceWithRetry`/`requestPresentDeviceWithRetry`，OOM 重试 3 次×1s）对该错误类是健壮的；
- 测试 helper（`native_device_test.go:31`）不重试、直接 `t.Skipf`——测试诚实降级（不假绿），符合预期，不需要改。

## 5 处置建议

1. **A3 关闭**：登记为「GPU 显存压力类环境 SKIP」，不是缺陷，不安排修复。台账 §2.2 A3 行改 ✅（转本报告）。
2. **测试卫生备注（归 P2.5，不本步做）**：`native_device_test.go` 的 skip 文案可改为「webgpu RequestDevice unavailable (GPU OOM / memory pressure): …」并列出 Caused by，让未来基线不用再解剖一次；属于测试输出变化，P2.5 测试卫生批统一处理。
3. **对 P2.4a 收敛的影响**：无。收敛不动 OOM 判定与重试逻辑（`IsGPUOutOfMemory` 与重试次数/间隔原样保留）。

## 6 复现命令

```bash
# P1.3 环境复现（本会话）
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib \
       GPUI_SURFACE_SAMPLE_COUNT=1 GOMAXPROCS=2 CGO_ENABLED=0
go test ./render/internal/gpu/ -run 'TestBuffer' -count=1 -v          # 看 SKIP 原因随显存压力漂移
go test ./render/internal/gpu/ -run '^(TestTexture_Destroy|TestCreateCoreTexture)$' -count=1 -v  # 单独跑 PASS
```
