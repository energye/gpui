# P2.5 包组织收口 — 收口报告（2026-10-31）

> 任务：P2.5 包组织收口（go list 对比、分层约束测试、索引与总账同步）+ 池带入 A2/A4/D2/D3/H1/J4。
> 评估：`assessment.md`（D1–D7 已拍板：登记+修订 PLAN 零搬移 / gpu_shared rwgpu 收敛 hal / 三个 depcheck 守卫 / ui 全 25 包进清单 / A2 本步独立提交 / gpu/hal 加节 / 小项按推荐）。
> 本步提交拆四笔：a 文档对账 / b 分层守卫+hal 收敛+潜伏红 / c A2+E2 指标契约 / d 总账+小项。

## 1 结果总览

| 子步 | 内容 | 结果 |
|---|---|---|
| a 文档对账 | `golist-compare.md`（目标条目零缺失/零同名子包/EXTRA 逐包登记）+ PLAN §3 目标树修订 | ✅ |
| b 分层守卫 | 新增 `render/depcheck_test.go`（白名单 7 文件+防陈旧断言）、`gpu/depcheck_test.go`（反向 0 基线）+ `gpu/doc.go`；ui depcheck 清悬空注释+修报错格式；D2：hal `WithForceNativeReleaseOnLost`+`SetForceNativeReleaseOnLostHook`（`gpu/hal/native_release.go`），webgpu init 注册，`gpu_shared.go` 去掉全仓唯一的 rwgpu 直引；runner 正本 `tools/per-file-tests.sh`+`tools/per-file-tests.pkgs`（9 包+ui 25 包=34 包，OUT/清单参数化）；潜伏红先修（见 §3） | ✅ |
| c A2+E2 | `ui/scheduler/metrics.go` 4 标签一次切 `interval_*` + `FPSWall`/`TargetHz` 字段（fps_wall 由 PresentCount/区间导出；target_hz 经 `SeedDisplayRefreshHz` 转发 `setTargetHz`，零新公开 API）；`baseline.go` 标签改新名；p0 测试新钉 `TestMetrics_AFamilyJSONKeys`；E2：`video_vr7_perf` 补 `interval_p99_ms`（结构+映射+requiredKeys）；Go 侧旧标签残留 0 | ✅ |
| d 总账+小项 | `go run ./scripts/apidoc` **rc=0**（+PresentFramePlan 行、清 §11 两行已删符号、补 `ToPaintBlendMode` 行、扫描范围 +render/gpu+render/hdr、新增 §12 gpu/hal 汇总级节）；D3：3 条测试伪造指针改真实地址（`[1]byte` 防零尺寸同址）、2 条 unreachable 死块删除（禁用意图注释保留）；H1：`native_device_test.go` 三处 skip 文案补 OOM 说明（A3 结论）；A4：保留 gpu_nogpu.go、台账计数修正 163→190 | ✅ |

## 2 门禁终态

| 门禁 | 结果 | 证据 |
|---|---|---|
| 五目标构建 | linux×2/windows/darwin×2 全绿（已知破包 t_stress/gpupixel 除外，与 P2.4e 同口径） | `build/*.txt` |
| go vet | **56/56**（默认/nogpu，全部为 ui/platform 线外 FFI；P2.5 拥有的 3 测试指针 + 2 unreachable 清零，基线变更已登记） | 本文件 §2 + 台账 §7 |
| 逐测试文件 | 34 包 708 文件：675 PASS / 21 SKIP_TAGGED / 1 NO_TESTS / **11 FAIL 全部既有零新增**（render 6 条与 P2.4e 证据逐名一致；ui 5 条干净树 HEAD worktree 同红——ui 首次进门禁暴露潜伏红，即 J4 目的） | `per-file-tests/summary.tsv` |
| conformance | 双后端 PASS、`frame.png` 零变更、上界 0/0 不回退 | `conformance/conformance.txt` |
| -race | gpu/hal、ui/scheduler、ui/scene、render/internal/gpu 池族 4/4 exit 0（scheduler 首轮红为 build-gate 满载压计时用例，空载复跑含 -race 3 轮 PASS，非竞态非回归） | `race.txt` |
| API 快照 diff | 与白名单逐条对上：gpu/hal +2 函数、ui/scheduler FrameMetrics 标签切换 + FPSWall/TargetHz 字段，其余零变化 | `api-diff.txt` |
| 残留 grep | Go 侧旧代标签 0；render rwgpu 直引 0；白名单 7 文件与守卫测试一致 | 本文件 |
| 负向对照 | 5 条全红→绿：①render 白名单外插 webgpu import→守卫红；②gpu 侧插 render import→守卫红；③ui 侧插 gpu import→守卫红；④摘 refuseChanged union→shrink 红证复绿；⑤摘 A2 新标签→p0 钉红 | 会话转录 + `summary.tsv` |
| A2 真窗 | 3 旧代窗 × 2 后端（ui_l1_blank / kit/spin_min / ui_render_base_perfsoak）rc=0 全绿：新键齐全（interval_* + fps_wall + target_hz=60）、旧键零残留、标准关闭、桌面健康 | `a2-window-runs.txt` + `a2_*.json` |
| apidoc | `go run ./scripts/apidoc` rc=0（修复前 rc=1） | 会话转录 |

## 3 池项收账

| 池项 | 结果 |
|---|---|
| A2 + E2 | ✅ 发射端一次切 `interval_*`；fps_wall/target_hz 补齐；E2 vr7 p99 补齐 |
| A4 | ✅ 评估保留 gpu_nogpu.go（B1 引擎层正解）；消费方实测 193 文件（190 空白导入，台账 163 已过时修正）；消解路径=将来顶层包拆分时自然消解 |
| D2 | ✅ nil ImageCache 组 P2.4e 已修（7 用例带 lib PASS）；depcheck 缝 `9cca54e6` 已落地；ui/scene「无 GPU 前置」4 条本步修（前置 Skipf，断言一字不动，无 GPU 环境照常执行） |
| D3 | ✅ 3 测试指针 + 2 unreachable 清零，vet 基线 61/58→56/56 |
| H1 | ✅ skip 文案补 OOM 说明（含 A3 复现层级注记） |
| J4 | ✅ ui 25 包入逐文件清单；ui/application、ui/focus 潜伏红实核（focus 已随 P2.4e Tab 空环修复转绿；app_test 等 5 文件既有红登记） |
| （新收） | `ui/scene` refuseChanged 损伤缺口：**真产品 bug**——churn 拒绝路径丢旧条目时未并旧 bounds，缩小重录的清尾在 LoadOpLoad 帧残影；修复=拒绝时损伤取 previous∪current（对齐重录路径既有契约），负向对照红→绿。`TestShrinkRerecordDamagesClearedTail` 双环境转绿；caret 测试改写为对齐准入阈值设计（churn 拒绝契约 + 稳定 sightings 重建立尾巴） |

## 4 实施偏差（3 条，均登记）

1. **extract_perf.py / verify_perf.py 保留双代读取**：评估原计划「别名退役」，实测 verify 靠重提取冻结证据校验——退役会打爆 P1.4/P1.7 冻结证据的机检。改为发射端已切、读取端保留别名专供冻结证据复验，退役挂 P2.6 证据换代。
2. **gpu/hal 节走汇总级**：实测 hal 顶层符号 156 个，逐符号手工表必腐；§12 登记家族表 + P2 期间新 API 逐条 + `go doc ./gpu/hal` 权威清单 + 机检只扫 render 树的口径声明。
3. **E2 融入 A2 子步**（原评估已如此计划，无偏差）；caret 测试改写幅度大于预期（准入阈值设计对齐 + 稳定尾巴），已按最小语义改动执行并带负向对照。

## 5 踩坑记录（防重犯）

1. **`git checkout` 冲未提交改动（二犯）**：负向对照还原误用 `git checkout --`，把 metrics.go 未提交的 A2 改动整体洗掉（P2.4d 前科同款）；本文件落盘前已全量重建并核 diff（+29/−4）。**纪律：还原未跟踪/未提交文件一律 cp 备份对，`git checkout` 只用于已跟踪且无未提交改动的文件。**
2. **逐文件 runner 正本两坑**：OUT 相对路径在 `cd` 进包目录后失效（已归一绝对路径）；本机 grep 是 ugrep，`\s` 不识别（已改 `[[:space:]]`）；清单注释必须 `#`（`//` 会被当包名）。
3. **计时用例满载敏感**：build-gate 交叉编译满载并发下 `ui/scheduler` pacing 用例被压红（-race 与非 race 都红过）；空载复跑全绿。满载跑门禁时计时类红先空载复跑再定性。

## 6 遗留

- ui 5 条既有红（app_test SIGSEGV / x11_s6_p0 / m1_noquadratic / m5_keystroke_g1 / gpupixel blit_rrect_corner）已进红名单：J4 只要求「进清单+登记」，病因诊断与修复不阻塞 P2.5；建议随 P2.6 前诊断或独立小步处理。
- 逐文件清单后续每轮以 `tools/per-file-tests.pkgs` 为正本，阶段证据输出用第二参数落各自 evidence 目录。
