批 5 步骤 1（CPU 竖条）· 门禁账（2026-11-05）
==========================================

本轮改动面（`git status` 实数）：修改 12 文件、新增 5 路径。

- 渲染层新增：`render/internal/filter/bloom_float.go`（核本体，internal）、
  `render/hdr/bloom.go`（`BloomParams`/`Validate`/`IsIdentity`/`MaxBloomSigma`/`MaxBloomIntensity`/`FloatTarget.Bloom`）、
  `render/hdr/bloom_test.go`（12 枚手算钉）。
- 引擎层：`engine/fx/bloom.go`（薄转调 + `Radius int`→`Sigma float64`）、`engine/fx/doc.go`（收回「render 永不 import」旧契约）、
  `engine/fx/p37_5_bloom_test.go`（7 枚手算钉）、`engine/fx/bloom_lut_test.go` + `testdata/bloom_lut_cases.json`（词汇与金图再生）。
- 示例：`examples/engine/fx/main.go`（σ 词汇 + 定帧停机 + 定格 + 跑不满显式失败 + 满窗接回胶片链）、
  `examples/engine/fx/README.md`（`Radius=2`→`Sigma=2`）、`testdata/fx_grade_golden.png`/`fx_final_base.png`/`fx_final.png` 重基线。
- 文档：`docs/RENDER_API_CATALOG.md`、`batch5-design.md`（§7 拍板修订）、`batch5-assessment.md`（勘误 C1/C2）、
  本目录 `negatives.txt`/`golden-recheck.txt`/`window-golden.md`。

跑过的门禁（逐条实测，不凭印象）
--------------------------------
| 门 | 结果 |
|---|---|
| `go build ./...` | rc=0 |
| `go vet ./render/hdr ./render/internal/filter ./engine/fx ./examples/engine/fx` | rc=0 |
| `gofmt -l`（触及目录） | 净（首轮报 `engine/fx/bloom_lut_test.go` 字段对齐，已 `-w` 修） |
| 逐文件回归 `tools/per-file-tests.sh`（触及 3 包：`render/hdr`、`render/internal/filter`、`engine/fx`） | **15 文件 = 14 PASS + 1 SKIP_TAGGED**（`render/internal/filter/helpers_test.go` 无 Test 函数，存量即如此），`FAIL_TOTAL=0` |
| `CGO_ENABLED=1 go test -race`（同三包） | ok 1.263s / 1.263→1.391s / 33.816s（末次复跑全 ok） |
| `go run ./scripts/apidoc` | **rc=0**（hdr 新增三符号入 §0 行；§7 的 hdr 行从「无生产消费者」表移入 §7.1 生产段；§13 P3.7 行补批 4 与批 5 步骤 1，并把 3e-3/mip 从「⬜ 未开工」摘掉） |
| 负向 | 12 枚变异（跨层重复 1 枚 ⇒ 独立 11）**逐条 rc=1 行为红**、还原逐字节一致，见 `negatives.txt` |
| 金图非自证 | 3×3 用例按独立算式（含 float32 逐步舍入）复算到**逐位一致**，见 `golden-recheck.txt` |
| 真窗 `examples/engine/fx -auto-only` | 4 连跑 `golden=0.0000% win_golden=0.0000%`，探针四轮的数值逐字一致；σ 2→3 变异 → 离屏 10.1562% **且窗口 3.9265%**（接线前窗口零敏感，已修），见 `window-golden.md`；Xwayland 跑前跑后存活，一轮一窗、自退、无 kill |

本轮**未跑**的门禁与理由（挂账不隐）
---------------------------------
- **五目标交叉构建（默认/nogpu 10 段）与 64 包全量 sweep**：按批 3 收尾登记的门禁口径
  「逐文件 sweep 对基线只减不增、**五目标交叉构建按「批」为单位跑**，子步提交时只跑触及包 + 真窗」，
  步骤 1 属批 5 子步，故不在本轮。我起初仍起了一轮交叉构建，为确保真窗那一轮不被编译负载污染而**主动停掉**；
  **批 5 收尾（步骤 2 之后）按「批」补跑 10/10 + 64 包全量并回填 §7**。
- **conformance 双后端**：本轮零 GPU 代码改动（核在 CPU float 域，WGSL 一条没碰），四 pass 与双后端一致性属步骤 2；
  届时按 §5 的「两路同式对齐」容差钉 + conformance 一轮一起做。
- `engine/fx` 的**能力收窄**已在钉与目录双登记：单边 > `hdr.MaxFloatDimension(8192)` 的图现在具名 `InvalidArg`
  （原先 `MaxImagePixels=16M` 允许 16384×1024 这类形状）。这是薄转调换来的「一份算法」的代价，不是隐藏洞；
  若要复原形状支持，得给 `render/hdr` 提预算上限（另案，属 M5 float 面一并处理更合理）。
