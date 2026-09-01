# 渲染引擎问题修复计划（REPAIR PLAN）

> 日期：2026-08-23。
> 依据：FINAL_SUMMARY.md 终版台账（唯一结论真源；P0 五项、P1 三十五项、P2 若干）。
> 本文件是修复工作的**唯一排期真源**；每批完成情况直接在本文档勾销更新，不另开文档。

## 执行纪律（硬）

1. 每批开工前按 AGENTS.md 走对应 skill：涉及 ui/rendering / ui/embedder / ui/scene / render / gpu 的改动走 wr-engine（先影响面评估、停「停下报告」节点确认方向）；能力实现类走 wr-implement；修 R/C 真窗暴露的 bug 走 wr-debug。本计划只定顺序和验收标准，不替代 skill 流程。
2. 每条修复必须有「验证方式」列中写明的实证才算完成；JSON 门禁绿 ≠ 引擎洞已修。
3. 只改引擎层（ui/ render/ gpu/），禁止在 examples/ 绕洞；examples 只做验证。
4. 每批结束跑一次回归：`go build ./...` + 该批涉及的包测试 + 受影响的 ui_wr_* 真窗。
5. **R 能力波及规则（硬）**：每批开工前的影响面评估必须列出本次改动波及的 R/C 能力点；其中——
   - 行为不变的修复（内存安全、线程纪律等内部实现）：受影响真窗做纯回归，§2.2 指标族不回退即过；
   - 行为会变的修复（如 shaping 接线、增量呈现打通）：对应 R/C 必须走 wr-close **反攻重关**（重定标准→跑 GPU→判指标族），并回写 §2 主表 / §3 组合表 + §10 修订表；重关通过才算本批完成。
6. JSON 门禁绿 ≠ 引擎洞已修；反攻重关的绿灯以新标准下的真窗实测为准。

## 第一批：止血（预计 1~2 天）

| # | 问题 | 改动 | 验证方式 | 状态 |
|---|---|---|---|---|
| 1.1 | X01 内存洞 | adapter.go stringViewToString 显式长度分支照 cStringAt(:574-588) 改真拷贝；覆盖回调路径 callbackStringView(:593-594) | go test ./gpu/rwgpu/... 全绿 + 新增 Adapter.Info() 读 Vendor/Name 用例 | ⬜ |
| 1.2 | 构建红灯 | 移除 tmp_vlprobe、render/tmp_ref_stroke、**tmp_c4dbg**（动态验证新发现）探针目录 | go build ./... 全绿 | ⬜ |
| 1.3 | 根目录卫生 | 清理根目录 8 个二进制与 4 个 tmp_* 探针目录（删除前逐个确认非他人遗留） | git status 无散落产物 | ⬜ |
| 1.4 | 假注释/文档改口 | shaper.go:7 与 SetShaper 注释改为「默认 HbShaper」；RENDER_API_CATALOG §2 布尔行改口实际支持范围；同文档 §7.3 与 :105/:313 裁剪描述矛盾一并理顺；改 API 目录需同步跑 `go run ./scripts/apidoc` 并在 ENGINE_UI_WIDGET_RENDER §10 追加修订行 | 文档与代码一致；apidoc 校验通过 | ⬜ |
| 1.5 | vet 正式包告警 | 清理 render/internal/gpu 两处 unreachable code、ui/rendering text.go:592 自赋值 | go vet 正式包零新增告警（platform unsafe 提示属 FFI 惯用法单列） | ⬜ |

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
| 4.1 | 先落金标测试集进 testdata：自交/共线/退化 × EvenOdd/NonZero 两套规则，期望值取自 Skia 对照或纸面几何；**追加 render 根包 3 处真红测场景（dash 无间隙、描边像素反超填充、差集挖洞不生效——先由用户裁决归属）** | 金标用例先行合入并明确标注当前实现 FAIL 的用例清单 | ⬜ |
| 4.2 | path_boolean.go 换扫描线实现（参考 tiny-skia/kurbo 算法）；注意动态验证结论：并集在常规构造下已正确、差集挖洞不生效，重写以差集/相交路径为验收重点 | 金标用例全过 + 现有 path 测试不回归 + 3 处红测转绿 | ⬜ |
| 4.3 | >2048 自适应细分取代静默截断；填充规则作为参数进入签名 | 大路径用例无内容丢失 | ⬜ |
| 4.4 | 同步修正 RENDER_API_CATALOG 描述 | apidoc 通过 | ⬜ |

## 第五批：增量呈现链 C1 族（约一周）

| # | 改动 | 验证方式 | 状态 |
|---|---|---|---|
| 5.1 | damage 准确性前提：线宽外扩（context.go:1162/1175）+ Path.Reset 清 boundsValid（path.go:178-185）+ Append 合并边界（:189-196） | 单测：粗线条 damage ≥ 描边外沿；复用路径 damage 不膨胀 | ⬜ |
| 5.2 | PresentWithDamage（gpu/webgpu/surface.go:312）真实传递矩形到 wgpu partial present | wayland damage 可视化工具确认仅提交脏区 | ⬜ |
| 5.3 | 门禁实测化专项（三处公式化/缺失 + wrgate 放宽点）：C2 sumRatio（ui_wr_c2 main.go:57-66）换实测 rects 面积和；R4b realRepaintRatio 常量门禁同改；R19 补真实采样断言替换硬编码 snapped_lines:46；wrgate 收紧三处——短跑(<5s)跳过 FPS 门禁须显式标记 fps_gate=skipped、默认 55 提至与产品口径一致、interval FPS 不得单独掩护 wall FPS | 负向用例：改场景参数/制造掉帧，门禁不再自动变绿 | ⬜ |
| 5.4 | MSAA LoadOpLoad UB（render_session.go 决策本体 :4827-4831，原引 :5368-5373 为漂移行号）：中帧 pass 改正确 loadOp | 4x MSAA 真机开 damage 无闪块（钉死 ROUND3 待验尾巴 #5） | ⬜ |

## 第六批及以后：P1 存量清账

顺序原则：批 B 算错结果类 → 批 C 死链类 → 批 D 性能类。明细以 FINAL_SUMMARY.md 台账为准（唯一结论真源），每完成一条在本节追加一行记录（编号/日期/提交）。含：

- B 类重点：B1 clip 绕向、B7 bidi 补全、B8/B9 IME、B11 CPU alpha、B13 缓存键族；
- C 类重点：C3 Auto 路由接线或删码、C6/C7 双路径行为分叉；
- D 类：D1 Map 忙等、D2 渐变查表上线、D3 字体加载上限；
- N1 HiDPI 缩放接入 wl_output.scale/fractional_scale（platform 层专项，建议独立小批次）；
- 5 条待真机复测尾巴随对应批次钉死（-race 实锤、FifoRelaxed 压测、acquire 故障注入）。

## 能力债单列（非缺陷，不占修复批次，待产品节奏决策）

- 手势竞技场孤岛：NewScrollable 生产零调用、InputRouter 不进 arena、全栈无人生产 PointerCancel——组件库化前必须接线；
- U21 Golden 逐位对比规范零落地：27 个真窗全为逻辑指标单证——要么落地进 wrgate，要么真源文档降格为「逻辑门禁版 ✅」；
- 跨文档状态矛盾 5 处（基座文档把 retained 当未来时、API 目录裁剪描述前后矛盾、「26 窗」实为 27、P 计划线与 W 波次线完成口径差、帧标准按需渲染 vs 验收窗 Persistent）；
- 动画系统两条 P2（Stop 一刀切 Dismissed、AnimatedOpacity.Mutations 无上限）。

## 2026-09-01 增补：X11 D-Bus IME 六洞修复专项（按严重度）

> 来源：`docs/review/R6_text_review.md` 2026-09-01 增补节；环境 `X11 + GTK_IM_MODULE=ibus + fcitx5(pid 2146260)` 实测。

| # | 洞 | 修法 | 涉及文件 | 验收 |
|---|---|---|---|---|
| 1 致命 | 假路径 `syntheticFcitxPrefix` 导致 9 处静默跳过 | 删合成路径；`tryCreateFcitx5` 对 `CreateICv3` 成功不再拼假路径，改为存 `icid` 并走 `org.fcitx.Fcitx-0 /inputmethod` 带 `ic` 的旧接口；若 ibus 兼容层可用（本机 `org.freedesktop.IBus CreateInputContext → /org/freedesktop/IBus/InputContext_*` 实测可通）则优先 ibus，不落 synthetic 分支；新增 pid 判定规避双名同 pid 抢注 | `x11_dbus_ime_linux.go:515-532/560/590/1019/1051/1079/1124/1171/1275` | `gdbus call CreateInputContext` 返回真实路径，非 synthetic；`isSyntheticPath` 9 处不再命中；`dbus-monitor` 见 `SetSurrounding/Cursor` 真发 |
| 2 严重 | ibus 松键丢弃 | 删 `if eng=="ibus" && !isPress` 早退；松键时 `state|=1<<30(IBUS_RELEASE_MASK)` 再调 `ProcessKeyEvent(uuu)` | `x11_dbus_ime_linux.go:1283` | `state=1<<30` 的调用 `err=nil`，松键可达 |
| 3 严重 | `IBusText (sa{sv}sv)` 序列化错 | `callSetSurroundingText` 的 `dbus.MakeVariant(t)` 改为带 `IBusText` 结构的 `MakeVariant`（`godbus` 注册结构体打 `(sa{sv}sv)`），补 `attrs` 空表 | `x11_dbus_ime_linux.go:1117` | `gdbus monitor` 抓包签名为 `(sa{sv}sv)` 且输入法能取上下文 |
| 4 严重 | 未走 XKB | `purego` 绑 `XkbGetState/XkbKeycodeToKeysym/Xutf8LookupString/XRefreshKeyboardMapping`；`xKeysymForState` 取 `group`；`drainX` 加 `MappingNotify`；`decodeKey` 优先 `Xutf8LookupString` 取 `Rune` | `x11_linux.go:193/1169/1214` | 俄语布局 group 正确、死键 `´+e=é` 产出 |
| 5 中 | 焦点事件丢 | `drainX` 补 `case xFocusIn/xFocusOut` 发 `EventFocus`，上层转 `DisableIME+EndComposing` | `x11_linux.go:1028` | Alt+Tab 往返无 preedit 残留 |
| 6 轻 | 空壳与竞态 | `callFocusOut` 真调 `EndComposing`；`Commit` 真调 `CommitString`；`imeDirty` 4 处改锁内读 | `x11_dbus_ime_linux.go:1056/963/876/910/925/1272` | `go vet -race` 零报 |

> 注：洞1在本机因 `GTK_IM_MODULE=ibus` 优先走通 `org.freedesktop.IBus` 兼容路径（`InputContext_322` 实测），合成回退未命中，故“整条不通”为回退分支下的最坏情况；修后两条路径均通。

## 附：进度记录

（每批完成后在此登记：批次/完成日期/验证证据位置）
