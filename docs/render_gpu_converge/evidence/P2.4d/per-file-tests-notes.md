# P2.4d 逐测试文件回归说明

- 日期：2026-10-30；HEAD 对照基准 `ac2e58b3`（P2.4d 工作区未提交）
- 口径：AGENTS「回归按测试文件逐个跑」——每个 `*_test.go` 单独 `go test -run '^(本文件用例名…)$'`，bench 不跑；无测试函数的文件记 compile-only；构建标签排掉的记 SKIP_TAGGED
- 本轮把范围从 P2.4c 的 4 包**扩到 9 包**：新增 `gpu/hal`、`gpu/hal/noop`、`gpu/webgpu`、`gpu/rwgpu`、`examples/wrgate`——本步动了 hal（`CompletionToken`）与两端实现，这几包必须进台账口径

## 1 总数对账

| 口径 | 文件数 | PASS | compile-only | SKIP_TAGGED | FAIL |
|---|---|---|---|---|---|
| 当前树（9 包） | 421 | 375 | 17 | 1 | **28** |
| 其中 P2.4c 已钉的 4 包 | 349 | 309 | 12 | 1 | 27（与 P2.4c 逐同名） |
| 其中本轮新增 5 包 | 72 | 66 | 5 | 0 | 1 |
| 干净树同 5 包（`per-file-tests-ab-clean.txt`） | 70 | 63 | 5 | 0 | **2** |

- 28 红按名字与基线逐条对齐（`comm` 差分）：只有一条不在 P2.4c 的 27 里——`gpu/webgpu/swapchain_x11_linux_test.go`，因为这条所在包本轮才进口径。
- 该条在干净树 `ac2e58b3`（不含 P2.4d 任何改动）**同样 FAIL**，同断言行 `:219`、同日志 `reconfig=0`；当前树另跑一次单用例复核，`acquires=12 presents=12 reconfig=0` 逐字一致 → **既有红，零新增红**。
- 干净树那次跑还留下两条环境性 setup failed 的教训，已写进脚本注释：worktree 放 `/tmp` 会撞 `go.mod` 的 `replace ../purego` 相对路径；放兄弟目录后又撞父级 `go.work`（worktree 不在模块列表）。对照必须 `GOWORK=off` + 兄弟目录 worktree，否则 5 包会整包假红。

## 2 本轮把一条红改成绿（D1/T1 关闭）

| 文件 | 干净树 | 当前树 | 原因 |
|---|---|---|---|
| `gpu/rwgpu/queue_workdone_test.go` | FAIL | PASS | hal token 落地时顺带查实 v29 绑定四处不齐（回调结构体缺 `mode`、签名缺 `WStringView`、状态枚举旧值、native 返回空 future），修完 `TestQueue_OnSubmittedWorkDone` 转绿；原先失败后 cleanup 的 SIGSEGV（待办池 D1）一并消失。详见 `hal_token.md` |

## 3 新增包的两向对账（逐包）

| 包 | 干净树 | 当前树 | 差异 |
|---|---|---|---|
| `gpu/hal` | 7 PASS / 2 compile-only / 0 FAIL | 同 | 无 |
| `gpu/hal/noop` | 3 PASS / 1 compile-only / 0 FAIL | 4 PASS / 1 / 0 | +`queue_token_test.go`（新） |
| `gpu/webgpu` | 18 PASS / 1 FAIL | 19 PASS / 1 FAIL | +`queue_token_test.go`（新）；那条 FAIL 两边同名同断言 |
| `gpu/rwgpu` | 34 PASS / 2 compile-only / 1 FAIL | 35 PASS / 2 / 0 FAIL | `queue_workdone_test.go` 红→绿 |
| `examples/wrgate` | 1 PASS | 1 PASS | 同一文件内新增 pool 段两条用例（用例数涨、文件数不变） |

## 4 归位（AGENTS「新问题归位」）

`swapchain_x11_linux_test.go` 的断言与产品语义相撞，不在 P2.4d 射程，已落台账待办池 **I2**：
- 断言侧：`gpu/webgpu/swapchain_x11_linux_test.go:219` 期望第 6 帧的 `sc.Resize(160, 120)`（与原尺寸同值）能抬 `reconfigures`，用例由 `8e4c0e99`（S6）引入。
- 产品侧：`gpu/webgpu/swapchain_configure.go:213` 的 `Resize` 在 `34d9785e`（T1.2 swapchain 拆刀）改为「同尺寸调用是 no-op」，注释写明理由是避免 `Surface.Configure` 抖动导致黑闪。
- 结论：不是显存/池的问题，是「用例期望」与「现语义」二选一，属 P2.4e 呈现入口收敛射程（同 I1 一条线），解剖前不动。
