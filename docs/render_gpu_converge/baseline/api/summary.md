# P1.5 每包公开 API 快照 + 测试计数快照

> 生成：2026-10-29；HEAD `bc18395c`；环境 go1.25.5 linux/amd64；主仓直接采集（纯静态，无 GPU、无窗口、不改代码）。

## 范围与口径

- **包集合**：与 P1.2 vet 完全同一把尺子——`docs/render_gpu_converge/evidence/P1.2b/packages-default.txt` 的 **352 包**（默认构建标签），副本存本目录 `packages.txt`。
  - 与 `go list ./...` 实测（353 行）差集仅 `examples/ui_wr_t_stress`：该包引用不存在的 `ui/kit/button`、`ui/kit/icon`，无法分析，P1.1 已登记（BUILD_FAIL），本快照同样排除。
- **API 快照**：逐包 `go doc -all <import>` 原样落盘 `doc/<包相对路径>.txt`。
  - 352 包产出 **351 个文件**；唯一例外 `ui/scene/internal/gpupixel`：目录里只有 `*_test.go`、没有非测试 Go 文件，`go doc` 无法产出（"no source-code package"），与 P1.2 vet 的纳入口径说明一致，按例外登记。
  - **16 个 `package main` 示例包无导出符号**，`go doc -all` 合法输出为空——文件照写（空文件），白名单在 `doc_empty_main.txt`；机器校验断言「空文件恰为白名单且都是 main 包」，非 main 包空输出会校验转红。
- **测试计数**：静态源码扫描 `^func (Test|Benchmark|Example|Fuzz)[A-Za-z0-9_]*\(`，逐包落盘 `test_counts.tsv`（完整 import 路径 + 5 列计数）。

## 数字

| 项 | 值 |
|---|---:|
| 包 | 352（1 例外：gpupixel 无 doc） |
| doc 文件 | 351（16 个为空 = main 示例包无导出符号） |
| 包内测试文件 | 1397 |
| 顶层测试函数合计 | 14388 = Test 13602 + Benchmark 753 + Example 18 + Fuzz 15 |

## 与 P1.3 / 磁盘的对账（零缺口）

- 磁盘 `*_test.go` 共 **1468** = **1397**（352 包目录内，入 `test_counts.tsv`）+ **1**（`render/tests/stress/analytic_stress_test.go`，`//go:build stress`，不进默认标签包集合；P1.3 已记 SKIP）+ **70**（`third_party/go-text/typesetting/**`，独立模块 `github.com/go-text/typesetting` 经 `replace` 引入，主模块 `go list` 不可见）。
- P1.3 的 **1398** = 1397 包内 + 1 stress 标签文件（P1.3 口径本就排除 third_party），两份基线对得上。
- P1.3 `summary.tsv` 的 `funcs` 列是**当次 linux 默认标签运行动态发现数**（平台/标签排除的文件记 0）；本文件是**静态源码声明数**（跨平台不变）。逐文件对比仅 17 个文件不同，全部是平台排除文件（gles/metal 等），两个口径各有用途：P1.3 管「这次跑出了什么」，P1.5 管「源码里声明了什么」。

## nogpu 说明

主快照按默认标签采集（按 P1.5 载体登记，不另采 nogpu）。nogpu 侧差异沿用 P1.2b 登记：348 包清单（`evidence/P1.2b/packages-nogpu.txt`）与默认差 4 个仅默认标签存在的 GPU 示例包；20 个测试文件带 `//go:build !nogpu`。

## 机器校验与再生成（P2 对账用）

```bash
# 生成（脚本随仓，路径自推导，任何 checkout 可跑）
python3 docs/render_gpu_converge/baseline/api/tools/gen_api.py
# 机器校验：非零退出 = 有缺失/多余/口径破坏
python3 docs/render_gpu_converge/baseline/api/tools/verify_api.py
```

校验断言（2026-10-29 实测 **ALL PASS**，连跑两次幂等）：

1. `packages.txt` 与 P1.2b 权威清单逐行一致（352 包）
2. doc 文件集 = 包集合 − gpupixel，零缺失零多余
3. 空 doc 文件恰为 `doc_empty_main.txt` 且都是 main 包
4. `test_counts.tsv` 352 行、包集合一致
5. 测试文件 1397、四类函数合计 14388
6. 磁盘对账：包外文件恰为 1 stress + 70 third_party，无其他

P2 收敛改 API 后：重跑 `gen_api.py`，与本次 doc/ 与 test_counts.tsv 逐字节 diff，变化即 API 变化清单，再对照各域总账文档核对同步情况。
