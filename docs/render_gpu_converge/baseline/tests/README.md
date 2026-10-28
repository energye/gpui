# P1.3 逐测试文件运行基线

本目录是 P1.3 的原始证据，不是结论摘要。结论、失败分组和 P2 归属见
[`summary.md`](summary.md)；逐文件状态以 [`summary.tsv`](summary.tsv) 为准。

## 目录

| 文件/目录 | 内容 |
|---|---|
| `summary.md` | 主报告：方法、计数、失败分组、SKIP 分类、P2 交接 |
| `summary.tsv` | 1398 个测试文件的逐文件总账（status/pkg/file/funcs/pass/skip/fail/rc/duration/note） |
| `failures.tsv` | 失败文件清单及日志路径 |
| `skips.tsv` | 所有 SKIP 用例及 `t.Skipf` 原因（含静态跳过行） |
| `logs/` | 逐文件原始 `go test -v` 日志；文件名规则见下 |
| `logs/clean-rerun/` | 三个 `RequestDevice` 文件的干净重跑日志（buffer/texture/s6_7；结果与主日志相同） |
| `failures/` | 崩溃/panic 原始栈（T1、app、integration、opt24/28/29） |

## 环境

- linux/amd64，Go 1.25.5，`CGO_ENABLED=0`
- Wayland + XWayland，`DISPLAY=:1`
- NVIDIA 940MX，驱动 580.178.04，显存 1024 MiB
- 运行环境变量：
  - `GOWORK=off`
  - `WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`
  - `LD_LIBRARY_PATH=$PWD/lib`
  - `GPUI_SURFACE_SAMPLE_COUNT=1`
  - `GOMAXPROCS=2`

## 盘点口径

- 磁盘上 `*_test.go`：1398 个
- 默认构建（linux/amd64，无 tag）参与编译：1375 个，139 个包，13490 个 Test/Example
- 构建标签排除：9 个
- 不在默认包内：14 个
- 无 Test/Example（helpers/benchmarks/fuzz only）：47 个
- 上述静态行在 `summary.tsv` 中记 `SKIP`，原因写 `note`；它们没有运行日志。

## 执行方法

1. 每个包预编译一个测试二进制（`go test -c`），避免逐文件重复编译。
2. 每个测试文件用**独立进程**执行该文件内的 Test*/Example*：
   `-test.run '^(TestA|TestB|...)$' -test.v -test.count=1 -test.paniconexit0`。
3. 三阶段执行：
   - safe：普通单测，180 秒/文件上限；
   - window：涉及窗口/X11/呈现的测试，300 秒/文件上限，开窗前 5 秒沉降 + 显示健康检查；
   - long：长跑扫描/性能文件，3600 秒/文件上限。
4. 窗口测试串行执行，每轮检查 `xset q` 与 Xwayland；不使用 `kill -9`/`pkill`/`killall`。
5. 失败/超时/SKIP 均记录原始日志；`t.Skipf` 原因从 `--- SKIP` 前一条 `_test.go:N:` 回溯提取，避免抓到下一条测试的日志。

### 180 秒首轮与长跑批

首轮 safe 用 180 秒上限，在 `render/text/hint/zz_b1_scan_test.go` 被误杀；单独实测该文件 **270.7 秒 PASS**。因此长跑文件从 safe 批排除，改为 3600 秒上限单独执行；长跑批的
`render/text/zz_b1_ttf_scan_test.go` 实测 **704.2 秒 PASS**（CJK 3000×65 两轮 + Latin/Myanmar/Indic 全过），`render/text/zz_b2_bitmap_scan_test.go` 实测 **1256.2 秒 PASS**（CJK/Korean 11172 字×15 档 + Thai/TTF），证明 180 秒上限不能用于这些文件。

## 日志命名

`logs/<pkg 的路径字符替换为 _>__<文件名去掉 .go>.log`。例如：

```
logs/github.com_energye_gpui_render__context_clip_depth_test.log
```

崩溃/panic 原始栈另存一份到 `failures/`，便于直接打开：

- `queue_workdone_test.log`（T1，SIGSEGV，完整栈）
- `app_test.log`（StubHost 假 X11 句柄，SIGSEGV）
- `integration_test.log`（并发 RenderScene，wgpu-core panic，SIGABRT）
- `opt24_sticky_index_uniform_test.log`、`opt28_sticky_image_gputex_verts_test.log`、`opt29_image_uniform_slab_test.log`（nil ImageCache panic）

## 状态定义

- `PASS`：该文件内所有选中的 Test/Example 都通过（允许文件内有带原因的 SKIP；SKIP 另记 `skips.tsv`）。
- `FAIL`：至少一个测试失败、panic、崩溃或超时；`note` 写失败数或 crash marker。
- `SKIP`：该文件没有可运行测试（构建标签排除/不在默认包/无 Test/Example），或全部用例按原因跳过。

## 重要边界

- 本任务**不改代码**；所有 FAIL/SKIP 只登记，修复留给 P2 对应条目。
- 逐文件隔离会暴露依赖包级顺序/全局 GPU 状态的测试；这类失败本身是测试卫生问题，不能当成生产回归，也不能用“全量一起跑绿”掩盖。对 `ui/scene`、`video` 等包另做了整包交叉复核，结论写在 `summary.md`。
- 46 个 `RequestDevice unavailable` 首轮与干净重跑均复现（不是瞬跳）；`render/internal/gpu/buffer_test.go`、`texture_test.go`、`s6_7_resources_test.go` 文件级状态本来就是 PASS（分别 8/32、5/12、5/2 通过/跳过），46 个用例仍 SKIP；干净重跑日志在 `logs/clean-rerun/`，归 P2.5/P2.4a 设备创建环境。
