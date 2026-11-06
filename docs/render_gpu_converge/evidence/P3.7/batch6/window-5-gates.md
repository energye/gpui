批 6 ⑤ · 门禁账（2026-11-06，一律「先落文件再取 `$?`」）
====================================================================================

| 门 | 命令 | 结果 |
|---|---|---|
| 窗体构建 | `go build ./examples/engine/p37_bloom/` | rc=0 |
| 全仓构建 | `go build ./...` | rc=0（收尾轮复跑） |
| 窗体 vet | `go vet ./examples/engine/p37_bloom` | rc=0 |
| gofmt | `gofmt -l` 窗目录 | 0 行 |
| 门禁相 native | `RUN_SECONDS=8 go run ./examples/engine/p37_bloom -auto-only` | rc=0，`OK presents=241 probes=11/11 golden=0.0000% (921600 px)` |
| 门禁相 go | `GPUI_BACKEND=go RUN_SECONDS=8 … -auto-only` | rc=0，同数全绿 |
| 两基线互比 | `cmp native.png go.png` | 逐位相同 |
| fail-closed 反面 | 缺基线跑 / `GPUI_ACCEPT_GOLDEN=1` 播种跑 | 均判红（rc=1），播种不算过 |
| H15 六轮 | 六次 `-auto-only` | 金图恒 0.0000%（presents 240/241 两形态） |
| 常驻人工相 | `-manual-seconds 6` | rc=0，354 presents，探针/像素 true，金图 0.0000%（静态内容窗，如实记） |
| H16 两变异 | 窗参一行（refIntensity / refThreshold）→跑→还原→`git diff` 零命中→终验复绿 | 8.4939% / 8.1050%，各带探针报警 |
| apidoc | `go run ./scripts/apidoc` | rc=0（零新增公开 API） |

GPU 安全口径：全程一轮一窗、进程自终止（RunFor/MaxFrames 到期 → 正常关闭链）、
每轮前后 Xwayland 存活核对，无 kill/pkill。

fps/p95 只报数不判绿（§3-5，性能门重定标未做）；浮动计时门未作收口证据。

批级门禁（五目标交叉构建 {默认,nogpu} 10 段、64 包全量 sweep、conformance
双后端一轮、整包 `-race ./render` 回填）按登记口径留批 6 收尾统一跑。
