批 6 ⑥ · 门禁账（2026-11-06，一律「先落文件再取 `$?`」）
====================================================================================

| 门 | 命令 | 结果 |
|---|---|---|
| 全仓构建 | `go build ./...` | rc=0 |
| 窗体 vet | `go vet ./examples/ui_wr_p37_scene` | rc=0 |
| gofmt | `gofmt -l` 窗目录 | 0 行 |
| 门禁相 native | `RUN_SECONDS=8 go run ./examples/ui_wr_p37_scene -auto-only` | rc=0，`OK presents=240 probes=15/15 golden=0.0000% (921600 px) skips=239` |
| 门禁相 go | `GPUI_BACKEND=go RUN_SECONDS=8 … -auto-only` | rc=0，同数全绿 |
| 两基线互比 | `cmp`（字节层）不同 → 逐像素工具：差异全在 y<32 指标 HUD 行（presents 尾数文案），裁到比对区 y≥32 **像素差=0** | 如实登记（p36「两后端金图各锁各」既有口径族） |
| fail-closed 反面 | 缺基线跑 / `GPUI_ACCEPT_GOLDEN=1` 播种跑 | 均判红（rc=1），播种不算过 |
| H15 六轮 | 六次 `-auto-only` | 金图恒 0.0000%（presents 240/241 两形态） |
| 常驻人工相 | `-manual-seconds 6` | rc=0，358 presents，探针/像素 true，金图 0.0000%（静态内容窗，如实记） |
| H16 两变异 | 窗体场景各删一行发射（sceneProj SetProjection / sceneMaterial SetMaterial）→跑→还原→grep 核位→终验复绿 | 2.0477% / 3.3924%；变异 1 首跑 panic 退 2（探针 got 串无守卫索引，窗自身缺陷）修正后重跑取数；变异 2 带语义像素双咬 |
| apidoc | `go run ./scripts/apidoc` | rc=0（零新增公开 API） |

live_only_skips 口径：≈presents 只报数（Recorder 每帧合法跳过 live-only 回放，
`ui_render_base_scene` 同款）；scene 内容在呈现帧的存在性由四枚语义像素锚承担。
GPU 安全口径：一轮一窗、进程自终止、每轮前后 Xwayland 存活核对，无 kill/pkill；
一次性诊断件（包内临时测试）跑完即删并核 `git status` 无残留。

批级门禁（五目标交叉构建 {默认,nogpu} 10 段、64 包全量 sweep、conformance
双后端一轮、整包 `-race ./render` 回填）按登记口径留批 6 收尾统一跑——批 6
六扇至此全收，下一步=批级门禁收官。
