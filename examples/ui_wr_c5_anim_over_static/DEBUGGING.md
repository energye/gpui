# C5 引擎洞专项修复 — 交接文档（新会话从此接续）

> 2026-08-25 首轮深挖未结案。本文档是完整交接：现象、已排除项、关键线索、
> 分步任务、关窗门禁。**按 AGENTS.md 先读本文件再动手；render/ 与 gpu/ 层
> 最谨慎改，修前影响面评估并停在「停下报告」征求方向确认。**

## 0. 当前状态速览

- R20 已提交关闭（`6a752a5`）：RenderColorFilter/RenderImageFilter +
  FilterResultCache 结果缓存 + 真窗 `ui_wr_r20_filter` 全绿。
- C5 组合窗骨架已完成（本目录 main.go + README.md，**未提交**）：
  性能门禁全绿（fps 59 / p95 18ms / hitch 4/min / vsync=true / retained 分带
  正常 rerecord=363 skip=92040）、Golden 基线机制就绪。
- **唯一阻塞**：引擎洞——见下。修完即可跑绿关闭 C5，回写 §3/§5/§10。

## 1. 引擎洞现象

C5 真窗中 **一切 α<1 的 RenderOpacity（scene.OpacityLayer →
dc.PushLayer(BlendNormal, α) 分支）的子树在 GPU retained 复合期不可见**；
α=1（BuildLayerTree 省层直通，无 OpacityLayer）正常显示。
同基线（6959ae9）跑 R6 真窗：OPA 呼吸卡可见且混合精确——非全局坏，
与窗口场景状态相关。C5 与 R6 的 main.go 代码高度趋同（diff 排除法做过），
唯一可观测全局差异：**frame_flushes C5=2 vs R6=1**。

## 2. 实验证据清单（全部在基线 6959ae9 worktree 上实测）

| # | 实验 | 结论 |
|---|---|---|
| 1 | LTREE dump 层树 | boundary>opacity(kids=1)>picture 结构完整存在 |
| 2 | 强制 leaf CacheKey=0（全走 vector replay） | 仍不可见 → blit/replay 两路都丢 |
| 3 | 去掉 SetRepaintBoundary | 仍不可见 → 非 RB 缓存问题 |
| 4 | 移除 wrapDense/dense 兄弟子树 | 仍不可见 → 非兄弟覆盖 |
| 5 | floatHost 加 Background / R6 OPA 板结构原样移植到右侧 | 仍不可见 |
| 6 | α 恒定 vs 呼吸 | 都不可见；α=1 直通可见 |
| 7 | mul 探针（PushLayer 后 vs blit 时） | PushLayer 后 mul=α 生效；blit 时 mul=1.00。**R6 同款探针也是 blit 时 mul=1 但可见** → mul 不是判别点 |
| 8 | 全量 BLIT 对照表（一帧采样） | **OPA 卡 child（key=33）mul=0.56、dst=(312,122)-(522,332) 正确 blit 却不可见**；FLOAT 卡 child 的 picture **从未出现在 phase2 blit 列表** |
| 9 | phase1 RECORDED 打印 | FLOAT child（key=30，bounds=(0,0)-(130,80)，ops=1）首帧录过一次，之后 hasTex=true recordedThisFrame=false |
| 10 | 全图橙色搜索 | 无任何橙像素落点（排除了「画错位置」）；注意 rotFill 红臂 (669..723,198..255) 会干扰宽松色扫描 |

## 3. 最强线索（下一步从这里切入）

1. **key=17 全屏 blit**：单帧 BLIT 表存在 `key=17 dst=(0,0)-(1200,800)`
   （全屏纹理）。若其绘制顺序晚于动画卡（walk 序或 packet 分带序），
   会盖掉一切动画输出。**未验证：key=17 属于哪个节点、实际绘制序**
   （探针打印时被 sort 破坏了原始顺序——新会话务必打原始序列）。
2. **frame_flushes 2 vs 1**：C5 每帧多一次 mid-frame flush，栈打印定位为
   phase1 纹理录制（textured.go:524 recordWith / :855 recordLocalWith 的
   FlushGPUWithView）。mid-frame flush × open F1 layer 的交互是 C8 v2
   修过的同域（gpu_render_context.go offscreenPass suspend-and-restore，
   注释里有 black-texture root cause 记录）。
3. **矛盾点待解**：key=33（OPA child）mul=0.56 + dst 正确 + hasTex=true
   + blit 执行 → 屏幕上却没有。说明丢发生在「GPU 命令编码之后」：
   要么纹理内容空（recordLocalWith 录制时机×layerStack 状态）、
   要么命令被后续全屏 blit/key=17 覆盖、要么 present 编码吞 op。

## 4. 复现步骤

```bash
cd /home/yanghy/app/projects/gogpu/gpui
# 若主仓 go build ./examples/... 因 ui/platform wayland_keyboard_linux.go
# 编译断（IME 线半成品），用干净基线 worktree：
mkdir -p /tmp/wt5 && ln -sfn $HOME/app/projects/gogpu/purego /tmp/wt5/purego
git worktree add /tmp/wt5/gpui 6959ae9
cp -r examples/ui_wr_c5_anim_over_static examples/ui_wr_r6_layer_anim /tmp/wt5/gpui/examples/
cd /tmp/wt5/gpui
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib \
       WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
RUN_SECONDS=6 go run ./examples/ui_wr_c5_anim_over_static
# 终帧快照 /tmp/c5_anim_over_static/c5_final.png：
# FLOAT 卡区域 (x≈1008..1138, y≈136..216) 应有橙混合却无；
# 对照 RUN_SECONDS=6 go run ./examples/ui_wr_r6_layer_anim 的 op_card 断言 ok=true。
```

注意：EnsureCacheID 序列随树遍历变化，**不要假设 key 编号跨版本稳定**；
用 bounds+dst 对号（FLOAT child = bounds(0,0)-(130,80) ops=1）。

## 5. 分步任务（新会话执行）

1. 读 AGENTS.md → 本文件 → `examples/ui_wr_c5_anim_over_static/main.go`。
2. 重建复现环境（第 4 节）；若主仓能直接编译则优先在主仓跑（少一层）。
3. 打印**单帧原始绘制序列**（不排序！）：compositeWalk PictureLayer 公共段
   `draw key/dst/mul` + phase1 RECORDED，看 key=17（全屏 blit）相对动画卡
   的顺序，以及 FLOAT child 是否真的缺席。
4. 按 3 的结果分支：
   a. 若 key=17 或其他全屏/大块内容后画覆盖 → 查该 picture 的归属节点与
      z-order 来源（packet 构建顺序），定点修排序；
   b. 若 FLOAT child 缺席 phase2 → 查 FramePacket 构建（packet.go）是否
      把该子树分到了未渲染 band（Shell/Overlay 分流条件）；
   c. 若两者都在且顺序正确 → 用 ui/scene/internal/gpupixel 写最小 GPU
      契约测试（BoundaryLayer>OpacityLayer>Picture + mid-frame
      FlushGPUWithView），单步修 render/internal/gpu 会话层。
5. 修复后回归：`go test ./ui/...`（18 包零回归）+ R6/R20/C4/C8 真窗抽查
   （它们的 opacity/filter 输出不能回退）。
6. 回 C5：两连跑 `RUN_SECONDS=30`（首跑产 Golden 基线、次跑逐位对比），
   门禁全绿 → 回写 docs §3 C5 行 ⬜→✅、§5 W5 行、§10 修订行。
7. metrics-audit 串审 JSON 字段诚实性后再标 ✅。

## 6. C5 关窗门禁（已写进 README，不许放）

fps_wall≥55、interval_p95_ms≤22、hitch≤6/min、present_policy=retained、
boundary_skip≥1+boundary_rerecord≥1、paint_count drift≤40、CPU 非双 0、
scripted 像素断言 6/6、Golden 掩码（绕开 FLOAT 卡矩形的四块并集）
跨跑逐位一致零容差、RSS 如实上报。

## 7. 提交纪律提醒

- 工作区混有 IME 线活跃改动（ui/input、ui/textinput、ui/platform、
  ui/rendering/text.go、docs/ENGINE_INPUT_IME_PLAN.md、
  examples/ui_textinput_ime 等）——**禁止一并提交**。
- C5 相关待提交文件：examples/ui_wr_c5_anim_over_static/（main.go、
  README.md、DEBUGGING.md 可选入库）+ 引擎修复涉及的 ui/scene、
  ui/embedder、render 文件 + docs 回写。
- 提交前跑 `go run ./scripts/apidoc`（非零=补 RENDER_API_CATALOG.md 再合入）。
