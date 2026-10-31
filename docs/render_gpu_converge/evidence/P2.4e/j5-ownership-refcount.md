# J5 修复：AdapterContext 所有权引用计数（Wayland 共享设备上下文存活）

- 日期：2026-10-31；P2.4e e4 续（C7 转正后首次可达段）
- 真因（dump 证实）：Wayland Path B 每 surface 自有 EGL context（ownsContext=true），共享 Device 挂在首窗的 AdapterContext 上；首窗 `Surface.Destroy→AdapterContext.Destroy` 置空 eglCtx/gl 后，其余窗的快照/帧路径 `Device.CreateBuffer→GenBuffers` 打空指针 SIGSEGV（3 轮 2 挂）。X11 共享路径 ownsContext=false 不触发。

## 修复（引用计数）

- `AdapterContext` 增加 `refs`（创建者起始 1，mu 保护）：`retain()`（Device.Open 持一笔）+ `release()`（末笔且 owns 才 Destroy）；双端（linux/wgl twin）同步实现。
- 挂点：`Adapter.Open` 建设备 `retain()`——**必须在 Lock 之前调用**（Open 全程持 mu，锁内 retain = 自死锁；首轮验证抓到并修正，三轮 rc=124 教训在案）；`Device.Release` 先删 VAO 再 `release()` 并 `ctx=nil` 防双放；`Surface.Destroy`（ownsContext）`Destroy→release`；`Instance.Release`（linux ctx/x11.ctx/devCtxs + wgl）全部 `Destroy→release`。
- Windows twin 无 J5 症状（Context 归 Instance），但同款 API 落齐保语义一致（跨平台标记纪律）。

## 验证

| 轮 | 修复前 | 修复后 |
|---|---|---|
| multiwin go ×3 | rc=0 / **rc=2 SIGSEGV ×2**（关窗阶段） | **rc=0 ×3，hits=14/15/15，panic=0** |
| l1_blank go | rc=0（C7 转正后） | rc=0 |
| multiwin native | rc=0 | rc=0 |
| gles 单测 | — | 唯一 FAIL = `TestPresentDamageX11` = §2.2 G3 既有环境红（在册，与本改动无关） |
| 构建/交叉 | — | linux 绿 + windows/darwin 交叉绿 |

- 实验过程记录：首轮 retain 放在 Open 锁内 → 三轮 rc=124（开窗自死锁），修正后全绿——retain 必须先于 Open 的 Lock，已写入代码注释。
