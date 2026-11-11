# N3：`ui_base_vram_cap` 首窗 snapshot 偶发未写出 —— 根因与修复（2026-11-12）

> 任务：§2.2 N3。现象：native 300s cascade 第一轮 stderr 只有 7 条 `snapshot:`（win1 缺），gate 读取陈旧 `vram_win1.png`，误报 `17.5273%`；第二轮 8/8 条、全 `0.0000%`。
> 结论：**根因是示例侧 `sync.WaitGroup` 的 Wait→Add 竞态：`runDone` 在窗口尚未全部 `wg.Add` 时就能关闭，主 goroutine 提前 `Close()`，把首窗 present target 置 nil；首窗快照闭包随后看到 target=nil，静默跳过。**不是渲染/呈现语义问题。

## 1 复现与定位

- 第一轮 300s native（证据 `i5-vram/verify-native.stderr`）：`snapshot:` 7/8，win1 缺；golden win1 17.5273%，其余 0。
- 第二轮 300s native（`i5-vram/verify-native-2.stderr`）：8/8，全 0.0000%。
- 诊断轮（临时把 `gateSeconds` 降到 60s，加 `SNAPDBG` 打印；原始日志 `repro-run1.stderr`）：
  - `SNAPDBG start path=vram_win1.png target_nil=false`
  - `SNAPDBG target nil in closure path=vram_win1.png`
  - `SNAPDBG Close path=vram_win1.png target_nil=false`（早于快照闭包执行）
  - 本轮 `snapshot:` 7/8，win1 缺。
- 代码定位：`examples/ui_base_vram_cap/main.go` 原先在控制器 goroutine 启动后立即 `go func(){ wg.Wait(); close(runDone) }()`；`wg.Add(1)` 在 `openBatch` 里、由控制器 goroutine 稍后执行。`wg.Wait()` 可以先看到计数 0 并返回，`runDone` 提前关闭；主 goroutine 在 gate 分支等到 `runDone` 后执行 `w.app.Close()`，与首窗 `Run` 末尾的快照闭包并发。`Close()` 把 `a.target` 置 nil，快照闭包里的 `a.target.Context()` 静默返回 nil，PNG 未写出。
- 首窗最容易中招：它的 `Run` 快照闭包通常排在其它窗之后，主 goroutine 的 `Close()` 先到。

## 2 修复

### 2.1 示例生命周期（根因）

`examples/ui_base_vram_cap/main.go`：

- 新增 `windowsOpened` 屏障 + `markOpened()`：控制器在最后一个 `openBatch(8)` 后关闭；`defer markOpened()` 覆盖提前退出。
- `runDone` 改为先 `<-windowsOpened` 再 `wg.Wait()`，保证所有 `wg.Add` 完成后才开始 Wait，消除 Wait→Add 竞态。
- gate 开始前删除上一轮 `vram_win1..8.png`；本轮没写出就必然缺文件。
- gate 读快照前先 `os.Stat` / `loadImage` 判空：缺文件记 `winN_snapshot_missing`，不可读记 `winN_snapshot_unreadable`，并把 `goldenMax` 置 100；不再拿旧图比较。
- 结束后复跑：native/go 各 300s 均 8/8 写出、8/8 `0.0000%`。

### 2.2 embedder 显式失败（防再静默）

`ui/embedder/pipeline_app.go` 的快照闭包：

- `a.target == nil` → 打印 `snapshot: target unavailable (window closed before snapshot): <path>`，不再静默返回。
- `a.target.Context() == nil` → 打印 `snapshot: context unavailable: <path>`。
- 正常路径不变。

### 2.3 不改什么

- 不改 present/swapchain/渲染语义；不改 `PipelineApp` 的正常快照时序；不改任何金图。
- `gateSeconds` 仍为 300；诊断期的 60s 仅用于复现，已还原。

## 3 验收

| 项 | 结果 | 证据 |
|---|---|---|
| 修复前 60s 复现 | 7/8 snapshot；`SNAPDBG target nil in closure`；win1 缺 | `repro-run1.stderr` |
| 修复后 60s 压测 ×5 | 每轮 8/8 snapshot、8/8 golden 0.0000% | `fixed-run1..5.stderr` |
| 修复后 native 300s | rc=0、8/8 snapshot、8/8 0.0000% | `accept-native.{stdout,stderr}` |
| 修复后 go/gles 300s | rc=0、8/8 snapshot、8/8 0.0000% | `accept-go.{stdout,stderr}` |
| 负向：testdata 只读 | 8/8 `FAIL: winN_snapshot missing`、rc=1；无 stale 17.5273% 比较 | `negative-nowrite.stderr` |

- 负向跑完已恢复 testdata 权限 0775，并用 `git checkout HEAD --` 恢复 `vram_win*_base.png` 与 `vram_win*.png`；工作区只保留两处代码改动。
- 验收跑使用隔离嵌套 Wayland `gpui-n3`，cascade 8 窗，标准 `Quit/Close`；跑完已清理合成器与套接字。

## 4 状态

- N3 ✅ 2026-11-12：根因修复 + 显式失败 + 双后端 300s 8/8 `0.0000%` + 负向红。
- 影响面：示例生命周期与 embedder 证据路径；不改变渲染输出，金图无需重种。
