# 渲染引擎问题修复计划（REPAIR PLAN）

> 日期：2026-08-23。
> 依据：RECHECK_SUMMARY.md 终版台账（P0 五项、P1 三十二项、P2 若干）+ 各 RECHECK 报告的行号证据。
> 本文件是修复工作的**唯一排期真源**；每批完成情况直接在本文档勾销更新，不另开文档。

## 执行纪律（硬）

1. 每批开工前按 AGENTS.md 走对应 skill：涉及 ui/rendering / ui/embedder / ui/scene / render / gpu 的改动走 wr-engine（先影响面评估、停「停下报告」节点确认方向）；能力实现类走 wr-implement；修 R/C 真窗暴露的 bug 走 wr-debug。本计划只定顺序和验收标准，不替代 skill 流程。
2. 每条修复必须有「验证方式」列中写明的实证才算完成；JSON 门禁绿 ≠ 引擎洞已修。
3. 只改引擎层（ui/ render/ gpu/），禁止在 examples/ 绕洞；examples 只做验证。
4. 每批结束跑一次回归：`go build ./...` + 该批涉及的包测试 + 受影响的 ui_wr_* 真窗。

## 第一批：止血（预计 1~2 天）

| # | 问题 | 改动 | 验证方式 | 状态 |
|---|---|---|---|---|
| 1.1 | X01 内存洞 | adapter.go stringViewToString 显式长度分支照 cStringAt(:574-588) 改真拷贝；覆盖回调路径 callbackStringView(:593-594) | go test ./gpu/rwgpu/... 全绿 + 新增 Adapter.Info() 读 Vendor/Name 用例 | ⬜ |
| 1.2 | 构建红灯 | 移除 tmp_vlprobe、render/tmp_ref_stroke 探针目录 | go build ./... 全绿 | ⬜ |
| 1.3 | 根目录卫生 | 清理根目录 8 个二进制与 4 个 tmp_* 探针目录（删除前逐个确认非他人遗留） | git status 无散落产物 | ⬜ |
| 1.4 | 假注释/文档改口 | shaper.go:7 与 SetShaper 注释改为「默认 HbShaper」；RENDER_API_CATALOG §2 布尔行改口实际支持范围；改 API 目录需同步跑 `go run ./scripts/apidoc` 并在 ENGINE_UI_WIDGET_RENDER §10 追加修订行 | 文档与代码一致；apidoc 校验通过 | ⬜ |

## 第二批：线程纪律 X02（预计约一周）

| # | 改动 | 验证方式 | 状态 |
|---|---|---|---|
| 2.1 | 定所有权规则：UI 线程独占 RO 树脏标志与 BoundaryCache 写权，光栅线程只消费不可变快照（对齐 Flutter raster 线程模型） | 设计说明写入本文件附录后再动手 | ⬜ |
| 2.2 | ConsumeNeedsPaint（pipeline_app.go:1199）移出光栅闭包：脏标志收集提前到 UI 线程提交前并入 FramePacket | 光栅侧不再出现对 UI 脏标志的写访问（代码审查 + grep 佐证） | ⬜ |
| 2.3 | BoundaryCache 改为整体不可变结构换交（优先），避免细粒度锁 | -race 跑 ui_wr_r7_virtlist 滚动 + resize 场景零报告 | ⬜ |
| 2.4 | 回归：ui/rendering + ui/embedder 全部包测试 + 受影响真窗 | 全绿 | ⬜ |

前置：本批开工前先由 wr-engine 出影响面评估并停下等用户确认方向。

## 第三批：文字 shaping 接线 X05（预计数天）

| # | 改动 | 验证方式 | 状态 |
|---|---|---|---|
| 3.1 | MSDF/GlyphMask 两条位图管线从 LayoutGlyphs 切到 text.Shape（HbShaper 已有完整实现） | 带连字英文（fi/fl）+ 阿拉伯文样张：CPU 路径与 GPU 路径输出一致 | ⬜ |
| 3.2 | 接通 ShapingCache 缓存防每帧重排（架子已在 cache/shaping.go） | shape 缓存命中率指标接入现有 JSON 门禁体系 | ⬜ |
| 3.3 | 回归 render/text 全部包（按 AGENTS.md 分文件分组跑） | 不回退 | ⬜ |

## 第四批：布尔运算重写 X04（约一周）

| # | 改动 | 验证方式 | 状态 |
|---|---|---|---|
| 4.1 | 先落金标测试集进 testdata：自交/共线/退化 × EvenOdd/NonZero 两套规则，期望值取自 Skia 对照或纸面几何 | 金标用例先行合入并明确标注当前实现 FAIL 的用例清单 | ⬜ |
| 4.2 | path_boolean.go 换扫描线实现（参考 tiny-skia/kurbo 算法） | 金标用例全过 + 现有 path 测试不回归 | ⬜ |
| 4.3 | >2048 自适应细分取代静默截断；填充规则作为参数进入签名 | 大路径用例无内容丢失 | ⬜ |
| 4.4 | 同步修正 RENDER_API_CATALOG 描述 | apidoc 通过 | ⬜ |

## 第五批：增量呈现链 C1 族（约一周）

| # | 改动 | 验证方式 | 状态 |
|---|---|---|---|
| 5.1 | damage 准确性前提：线宽外扩（context.go:1162/1175）+ Path.Reset 清 boundsValid（path.go:178-185）+ Append 合并边界（:189-196） | 单测：粗线条 damage ≥ 描边外沿；复用路径 damage 不膨胀 | ⬜ |
| 5.2 | PresentWithDamage（gpu/webgpu/surface.go:312）真实传递矩形到 wgpu partial present | wayland damage 可视化工具确认仅提交脏区 | ⬜ |
| 5.3 | C2 门禁去公式化：main.go:63-67 硬编码几何常数换成 frame 决策处导出的实测 rects 面积和 | 改场景参数门禁不再自动变绿（负向用例） | ⬜ |
| 5.4 | MSAA LoadOpLoad UB（render_session.go:5368-5373）：中帧 pass 改正确 loadOp | 4x MSAA 真机开 damage 无闪块（钉死 ROUND3 待验尾巴 #5） | ⬜ |

## 第六批及以后：P1 存量清账

顺序原则：批 B 算错结果类 → 批 C 死链类 → 批 D 性能类。明细以 RECHECK_SUMMARY.md 台账为准，每完成一条在本节追加一行记录（编号/日期/提交）。含：

- B 类重点：B1 clip 绕向、B7 bidi 补全、B8/B9 IME、B11 CPU alpha、B13 缓存键族；
- C 类重点：C3 Auto 路由接线或删码、C6/C7 双路径行为分叉；
- D 类：D1 Map 忙等、D2 渐变查表、D3 字体加载上限;
- N1 HiDPI 缩放接入 wl_output.scale/fractional_scale（platform 层专项，建议独立小批次）；
- 5 条待真机复测尾巴随对应批次钉死（-race 实锤、FifoRelaxed 压测、acquire 故障注入）。

## 附：进度记录

（每批完成后在此登记：批次/完成日期/验证证据位置）
