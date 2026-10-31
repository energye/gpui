# C7 修复拆半记录：Destroy 自死锁修复落地、开窗投递 hub 暂缓（池命中回归未根因）

- 日期：2026-10-31；P2.4e e4 内
- C7 三形态与处置：
  1. **开窗竞争 0x3002（X11 机制）**：修复方案 = `ui/embedder.PipelineApp.Open` 整段 `raster.RunGLSync`（hub job 串行化）。**隔离验证通过**：go 档 l1_blank 单窗 rc=0（853 帧 56.8fps，此前开窗 rc=1）、multiwin go 8 窗 rc=0（n=8 fps 56.5，此前第 8 窗必挂）。
  2. **关窗自死锁**：真因抓到——`gles/resource_linux.go Surface.Destroy` 的 `TryLock` 用**函数级 defer Unlock**，而函数体内 Wayland ownsContext 路径 `s.ctx.Destroy()` 再次加同一把锁 → 同 goroutine 非重入自死锁（dump：hub goroutine `[sync.Mutex.Lock, locked to thread]` 卡 `adapter_context_linux.go:214`，全进程无其他持锁者=自身持有）。X11 共享路径 ownsContext=false 不触发。**修复=Unlock 收紧到作用域内**，已落地；Windows twin 无此洞（Context 归 Instance，Destroy 内不再加锁）。
  3. **Wayland 第 N 窗第二机制**：随开窗修复在 go 档 8 窗上未复现（rc=0），按「未复现≠已修」继续挂在 C7 下。

## 暂缓原因（A/B 实测）

| 轮 | 树 | multiwin native -auto-only | device-surface 池 |
|---|---|---|---|
| A | e3 干净树（`e1605d51`） | rc=0 | hits=14 misses=32 evict=8 purge=24 peak=20326528 |
| B | e4 全部改动（含 hub 开窗） | **rc=1**（pool 门禁 0 hits） | **hits=0**，其余逐项相同 |
| C | e4 撤回 hub 开窗（其余保留） | rc=0 | hits=10 misses=30 evict=7 purge=23（正常） |

机制线索（未定论）：hub 上开窗让 `GetAdapterInfo`/caps 探测从「绑定失败静默空值（fail-soft）」变为真实成功，下游 present 模式/帧节奏随之改变（B 轮 56.5fps vs A 轮 35.2fps），池复用事件（疑似同尺寸重复 resize 的 release→reacquire）全部消失。**候选根因**：caps 成功路径改变了 swapchain/呈现配置选择链；需先钉住「命中事件的真实来源」（怀疑同尺寸重复 configure 的 session 重建）再定修复形态。

## 处置

- 落地：`gles/resource_linux.go` Destroy 锁作用域修复（独立正确，纯 bug 修）；`ui/raster/glsync.go` 导出 `RunGLSync`（复落载体）；C3 分支；I2 测试改绑 no-op+MarkNeedsReconfigure。
- 暂缓：`PipelineApp.Open` 的 RunGLSync 包裹（代码与验证记录留本档，根因闭合后重落）。
- 复跑证据：`/tmp/mwab.stderr`（A）、`/tmp/mwn.stderr`（B）、`/tmp/mwnoc7.stderr`（C）——关键数字已抄录本档。
