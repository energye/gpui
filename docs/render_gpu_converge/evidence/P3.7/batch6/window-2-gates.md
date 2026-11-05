批 6 ② · 门禁账（2026-11-06，一律「先落文件再取 `$?`」）
====================================================================================

| 门 | 命令 | 结果 |
|---|---|---|
| 全仓构建 | `go build ./...` | rc=0 |
| 窗体 vet | `go vet ./examples/engine/p37_height_zsort` | rc=0 |
| gofmt | `gofmt -l` 窗文件 | 0 行 |
| 门禁相 native | `RUN_SECONDS=8 go run ./examples/engine/p37_height_zsort -auto-only` | rc=0，`OK presents=241 probes=22/22 golden=0.0000% (921600 px) tie_frames=240` |
| 门禁相 go | `GPUI_BACKEND=go RUN_SECONDS=8 … -auto-only` | rc=0，同数全绿 |
| 两基线互比 | `cmp native.png go.png` | 逐位相同 |
| fail-closed 反面 | 缺基线跑 / `GPUI_ACCEPT_GOLDEN=1` 播种跑 | 均判红（rc=1），播种不算过 |
| H15 六轮 | 六次 `-auto-only` | 金图恒 0.0000%（presents 240/241 两形态） |
| 常驻人工相 | `-manual-seconds 8` | rc=0，473 presents，探针/像素 true，金图 0.0755% 只报数 |
| H16 四变异 | 改码→跑→取值→还原 | 0.25% / 0.3056% / 0.9566% / 1.1079%，各带探针+像素双报警；还原后终验复绿 |
| apidoc | `go run ./scripts/apidoc` | rc=0（零新增公开 API） |

浮动计时门（`engine/world cull`、`s6_9 heavy budget`）未作收口证据。

批级门禁（五目标交叉构建 {默认,nogpu} 10 段、64 包全量 sweep、conformance 双后端）
按登记口径留批 6 收尾统一补，不随单窗跑。
