批 5 步骤 2（GPU 四 pass + CPU 8-bit 节点 + 参数真源归位）· 门禁账（2026-11-05）
================================================================================

交付枚 `0e5d2549`（代码 + 测试 + 目录 18 文件）；改动面（`git status` 实数，运行副产物已排除）：新增 8 文件、修改 11 文件。

- 新增：`render/bloom.go`（`BloomParams`/`Validate`/`IsIdentity`/`MaxBloomSigma`/`MaxBloomIntensity`）、
  `render/internal/filter/bloom.go`（`BloomFilter` = `BloomFloat` 的 8-bit 薄封装）、
  `render/internal/filter/bloom_pixmap_test.go`（3 枚）、`render/p37_5_bloom_test.go`（8 枚）、
  `render/p37_5_bloom_gpu_test.go`（8 枚）、`render/p37_5_bloom_gles_test.go`（1 枚）、
  `render/internal/gpu/bloom_passes_test.go`（1 枚）、本目录 `negatives-step2.txt`。
- 修改：`render/filter_ops.go`（`bloomApply` + `FilterOpImpls` + `snapshotFilterOps`）、
  `render/filter_ops_cpu.go`（`RegisterFilterOps` 收结构 + `Context.ApplyBloom`）、
  `render/filter_ops_gpu.go`（`ImageFilterBloom` kind + `ImageFilterNode.Bloom` + 三张支持/分派表）、
  `render/internal/gpu/filter_gpu_graph.go`（WGSL mode 13/14 + 五 pass 编排 + pass 槽估算 4→5）、
  `render/filters/register.go`、`render/paragraph_test.go`（注册面随迁）、
  `render/hdr/bloom.go` + `render/hdr/bloom_test.go` + `engine/fx/bloom.go` + `engine/fx/p37_5_bloom_test.go`
  （参数真源上移后的同笔随迁）、`docs/RENDER_API_CATALOG.md`、`batch5-design.md` §8、`doc-audit-20261105.md` ⑥。

一、跑过的门（全部实测，非引用）
--------------------------------
| 门 | 结果 |
|---|---|
| `go build ./...` | rc=0 |
| `go vet ./render ./render/hdr ./render/internal/filter ./render/internal/gpu ./render/filters ./engine/fx ./examples/engine/fx` | rc=0 |
| `gofmt -l`（触及目录） | 净 |
| **64 包全量逐文件 sweep**（`tools/per-file-tests.sh` 默认名单，带 `WGPU_NATIVE_PATH`） | **895 PASS / 12 FAIL / 24 SKIP_TAGGED / 1 NO_TESTS**；12 枚红逐一对上 `baseline/tests/failures.tsv` 在册名单（脚本核对：不在册的 = 空）⇒ **零新增红**；同轮 `video/playback_parallel_test.go` 转绿（在册抖动源，不计收益） |
| 触及两包逐文件复跑（新增测试文件之后） | 370 PASS / 6 FAIL / 14 SKIP_TAGGED；其中 5 枚为在册红，第 6 枚见下「抖动复核」 |
| `CGO_ENABLED=1 go test -race ./render`（整包，含 GL provider 切换那枚钉） | rc=0 |
| `-race ./render/hdr ./render/internal/filter ./engine/fx`（步骤 1 已跑，本轮改动后复跑） | ok |
| conformance 双后端 `TestConformanceDualBackend` | rc=0（输出到 `tmp/conformance-b5s2`，未覆盖基线目录） |
| **五目标交叉构建 {默认,nogpu}** | **10/10 rc=0**（linux/amd64、linux/arm64、windows/amd64、darwin/amd64、darwin/arm64）；新增测试文件之后再复核 linux/windows/darwin 三目标 default 段仍 rc=0 |
| `go run ./scripts/apidoc` | rc=0（目录同步见下第三节） |
| pass 计数探针（design §5「只加计数与探针」的计数侧） | 实测 **bloom=5 / blur=2 / bloom+blur=7** 个 pass 槽 |

二、真窗（当前桌面 Wayland+Xwayland，一轮一窗、`-auto-only` 自退、每轮查 Xwayland 存活）
------------------------------------------------------------------------------------
| 窗 | 结果 |
|---|---|
| `examples/ui_wr_p34_filters`（共用那份滤镜图 WGSL 的最大存量面） | rc=0：`presents=353 pixel=6/6 golden=0.0000% gate=true` ⇒ 本批改 WGSL 没动存量滤镜画面 |
| `examples/ui_render_base_filterops` | **golden 0.0000% / 627200 px**，但整窗 rc=1：卡在 `screen check grab failed: xwininfo -name "gpui ui_render_base_filterops — 内容滤镜": exit status 1` |

抓屏那一步不是本批引入：同一形态在 P3.5 批 5 的 **clean HEAD worktree** 里逐字复现过
（`evidence/P3.5/golden-batch5/head-window-ab.log:507-508`、`round2-seed.json`
`screen_check_ok:false` 而 `pixel_golden_diff_pct:0`）⇒ 登记 **H18**（环境性失败与像素性失败混成同一个
rc=1，修法方向也写在 H18）。本窗跑完留下的 `testdata/showcase_filterops.png` 是运行副产物
（比对区零差、字节差在排除掉的指标带上），**已复原到 HEAD，不进提交**。

抖动复核（本轮唯一的名单外红）：`render/mem_window_x11_linux_test.go` 的
`TestMem_T4_WindowComplex_ResizeChurn` 在触及两包 sweep 里红一次，报
`mem_window_x11_linux_test.go:154: BeginFrame: hal: surface outdated`；
隔离复跑 **3/3 全绿**（1.42s / 1.30s / 1.24s），且本批一行 present/surface 代码都没碰
⇒ 与 `s6_9`、`engine/world cull`、`playback_parallel` 同族（真 X11 窗 + 计时/显存压力浮动），
登记为**抖动源**，不登记为稳定红、也不当收口证据。跑序上它排在我连开两扇真窗之后，
显存与 WM churn 是最可能的诱因。

三、文档与目录同步
------------------
- `docs/RENDER_API_CATALOG.md`：§3 头（Context 225→226）、§3.6 新增泛光行（`ApplyBloom` +
  `BloomParams` + 两枚上限 + `FilterOpImpls`，含 LDR 声明第二处）、§3.6 kind 行补齐 14 枚 kind、
  §0 主包行改 156/195/226/18/206/41 并把滞后归因写清（a16c08ed 干净树实测 154/195/225/18/203/41，
  即「常量 201 · 变量 33」之后的 +2 常量/+8 变量非本批引入）、§0 hdr 行回到真实计数（常量 3、
  方法 12，并注明五枚泛光符号已上移）、§7.1 泛光条扩到三条腿、§13 P3.7 行补步骤 2 并摘掉「⬜ 未开工」里的泛光。
  计数口径是 go/ast 实测（脚本按符号名去重、接收者名解析 `*ast.Ident`/`*ast.SelectorExpr`）。
- `batch5-design.md` §8：参数真源归位的硬约束推导、三条腿同一份核的落点表、注册面收的隐患、
  实测出的三条性质、双后端与 Metal 待验。
- `doc-audit-20261105.md` ⑥：H18（抓屏与像素混门）与上面那枚抖动源的补记。

四、本步骤的"没做/挂账"清单
--------------------------
- **Metal 后端不可本机验**（跨平台标记纪律：写明待验，不猜）。GL 后端已有执行钉，native(webgpu) 有 8 枚。
- paint 级泛光（`Paint.ImageFilter` 挂 bloom 节点）与 `Canvas`/`Recorder` 录制面**未接**：
  P3.4 那批滤镜口也没进 Canvas，本批随它，接线要另拍。
- pass 数只有测试侧可读（`cache.lastPassUniformSlots`），未新增公开计数口；恒等/非法节点不占 pass 因此
  只能作「不断言的防护」（见 `negatives-step2.txt` 第二节）。
- 泛光的**画面级真窗证据**仍欠：本批只有离屏/原子级像素钉 + 存量窗零回退，收口批=批 6 ⑤（状态按 §4.1 记 ☑）。
- 性能门按 design §5 不在本批（只加计数与探针）；帧耗时与 p95 挂批 6 ⑤。
