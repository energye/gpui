批 6 ④ · 门禁账（2026-11-06，一律「先落文件再取 `$?`」）
====================================================================================

| 门 | 命令 | 结果 |
|---|---|---|
| 全仓构建 | `go build ./...` | rc=0 |
| 窗体 vet | `go vet ./examples/engine/p37_mip_exit` | rc=0 |
| gofmt | `gofmt -l` 窗目录 + `engine/sprite`（H16 还原后） | 0 行 |
| 门禁相 native | `RUN_SECONDS=8 go run ./examples/engine/p37_mip_exit -auto-only` | rc=0，`OK presents=241 probes=10/10 golden=0.0000% (921600 px)` |
| 门禁相 go | `GPUI_BACKEND=go RUN_SECONDS=8 … -auto-only` | rc=0，`presents=240 probes=10/10 golden=0.0000%` |
| 两基线互比 | `cmp native.png go.png` | 逐位相同 |
| fail-closed 反面 | 缺基线跑 / `GPUI_ACCEPT_GOLDEN=1` 播种跑 | 均判红（rc=1），播种不算过 |
| H15 六轮 | 六次 `-auto-only` | 金图恒 0.0000%（presents 240/241 两形态） |
| 常驻人工相 | `-manual-seconds 6` | rc=0，352 presents，探针/像素 true，金图 0.0000%（静态内容窗，如实记） |
| H16 两变异 | 摘 ToRender 的 Mipmap/Filter 传递各一行（`engine/sprite/atlas.go`）→跑→`git checkout` 还原→`git diff` 零命中→终验复绿 | 1.9829% / 1.9669%，各带携带+画面双探针红；还原后终验 `probes=10/10 golden=0.0000%` |
| apidoc | `go run ./scripts/apidoc` | rc=0（零新增公开 API） |

GPU 安全口径：全程一轮一窗、进程自终止（WM_DELETE/RunFor 到期）、每轮前后
Xwayland PID 存活核对，无 kill/pkill。

浮动计时门（`engine/world cull`、`s6_9 heavy budget`）未作收口证据。

批级门禁（五目标交叉构建 {默认,nogpu} 10 段、64 包全量 sweep、conformance
双后端一轮、整包 `-race ./render` 回填）按登记口径留批 6 收尾统一跑。
