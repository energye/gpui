# race 组结果与 `gpu/hal` 红的归因（2026-11-02，批 2）

`-race -count=2` 逐组结果（`race/race.txt`，包级 rc 只作线索不作证据）：

| 组 | 结果 |
|---|---|
| `./render/internal/image` | ok 2.445s |
| `./render/recording` | ok 1.858s |
| `./ui/io` | ok 4.138s |
| `./gpu/hal` | **FAIL 0.034s** |
| `./engine/tex` | ok 6.053s |
| `./render/internal/gpu`（换代际逐出 + mip 钉） | ok 2.922s |
| `./render`（动图/导出/三档端到端钉） | ok 2.296s |

## `gpu/hal` 三条红的归因（两条独立验证）

失败项：
1. `TestSurfaceConfigureValidDimensions`（`error_test.go:132`）：`expected at least one adapter`
2. `TestGetBackend/unregistered_backend`（`registry_test.go:125`）：`GetBackend() ok = true, want false`
3. `TestGetBackend_NotRegistered`（`registry_test.go:141,144`）：同上

分离变量（同一棵本树）：

| 跑法 | 结果 |
|---|---|
| `-count=1`（带 `-race`） | **ok 1.093s** |
| `-count=1`（不带 race） | ok 0.033s |
| `-count=2`（不带 race） | **FAIL 三条同上** |

⇒ 与 `-race` 无关，是 **`-count=2` 第二轮的全局状态残留**：
- 2/3 的根因实测到行：`registry_test.go:66 TestRegisterBackend` 调 `hal.RegisterBackend(mock)` 写进**包级全局注册表**且不撤销，第二轮 `TestGetBackend*` 再断言 `BackendDX12`/`BackendGL` 未注册就必红。
- 1 是 wgpu-native 在同进程内二次 `CreateInstance/Release + EnumerateAdapters` 后枚举到 0 个适配器（驱动态敏感，非纯测试写法问题）。

干净树 A/B（`git worktree add` 到 `82ab30ef`，本批零代码状态）：`go test ./gpu/hal/ -count=2` 报**完全相同的三条红**（同测试、同消息、同行号）⇒ 非本批引入，也不是本批扩大 race 覆盖面造成的行为变化。

## 本批怎么处理

- 不把 `gpu/hal` 从覆盖面里删掉：它在批 2 起被纳入 race 观察，红也照记。
- 门禁该组的判据改为 **`-count=1`（race 下）**，并在门禁表里写明"该包测试非 `-count=2` 安全"的理由与上面两条根因，避免被误读成"race 过了"。
- 归位待办（不属批 2，不顺手改别线测试）：
  1. `gpu/hal/registry_test.go`：注册类测试要么用只属于自己的 variant、要么跑完撤销（需要 `UnregisterBackend` 之类的测试钩子，属 hal 线设计决定）；
  2. `gpu/hal/error_test.go`：适配器枚举用例应自带"枚举为空则跳过并写明环境原因"，或改走 `-count=1` 专用档；
  3. 门禁脚本记账口径缺陷（本批自己踩到，已改）：`progress.log` 里 `race rc=0` 一度是**块内最后一条命令的 rc**，被误当成组级汇总——逐组必须各记各的判定。
