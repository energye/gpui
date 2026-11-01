# 批 4 逐文件门禁两枚「新增红」归因（墙钟比率门漂移，非本批回归）

批 4 主逐文件门禁：693 PASS / 21 SKIP_TAGGED / 10 FAIL / 1 NO_TESTS（批 3 基线 692/21/8/1）。
10 枚红里 8 枚与批 3 基线名单逐名一致（`render/` 6 枚 + `ui/application/app_test.go` +
`ui/scene/internal/gpupixel/blit_rrect_corner_test.go`），**零例基线红转绿**；另 2 枚为
`ui/rendering/m1_noquadratic_test.go`、`ui/rendering/m5_keystroke_g1_test.go`。

## 定性证据（同机、同环境、同命令 A/B）

- `head-m1m5.txt`：clean HEAD `c892ba29`（批 3 提交）worktree `/tmp/p35b4_ab2`，
  `go test ./ui/rendering -run '^TestKeystrokeRatio_M5$|^TestKeystrokeRatio_M1$'` 隔离复跑 5 次 → **5/5 红**，
  失败档位 ×1.5 / ×1.8 / ×1.9 / ×2.1 / ×3.1（门禁 ≤1.5）。
- `worktree-m1m5.txt`：批 4 工作区同一命令 5 次 → **5/5 红**，档位 ×1.8 / ×1.9 / ×2.1 / ×2.5 同量级。
- 单测名单独跑 `^TestKeystrokeRatio_M1$`：两树各 3 次全 `ok` → m1 的红只在负载下出现。
- 批 3 记录同款：`m1/m5 老红转绿=只减不增`、`s6_9` 经 HEAD worktree A/B 定性为墙钟 p50 贴线漂移，
  且「两树红名互不一致」是已知现象——本轮正是同一类的反向（HEAD 也红）。

## 结论

两枚红是比率型性能门（文本排版耗时比）在当前机器负载下的漂移，批 4 代码路径（描边 path→path 效果）
不在其测量面上；HEAD 复现同量级红 → **本批零新增回归**。批 4 对 `ui/rendering` 的唯一改动是
`render.Stroke`/`render.Paint` 各多一个 nil 判定（`EffectivePathEffect`），非该门测量对象。

按 AGENTS「门禁红名逐项定性、只减不增」登记，不在此批动比率门阈值、不修该线（性能门归各自线立案）。
