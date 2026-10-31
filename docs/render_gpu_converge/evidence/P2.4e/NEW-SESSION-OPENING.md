# 新会话开工话述（把本文件路径发给新会话即可）

## 给新会话的第一句话（复制粘贴）

> 读 `docs/render_gpu_converge/evidence/P2.4e/NEW-SESSION-OPENING.md`，按其中的任务队列接着干。当前分支 feat/p2-收敛整合，HEAD 135529f6，36 个提交未推送（推送前问我）。先按 AGENTS.md 纪律读必读文档，然后开 P2.5：先做开工登记+影响面评估，停下报告拍板后再动手。

## 背景与状态（新会话必读）

### 需求文档与目标

- **总纲**：`docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md`——用户 7 条口径（重复删一套/能合则合/死代码按三根支柱删/双后端/缺失先文档/半成品先文档/包组织规范）；§0 标准判定三尺（对齐上游 naga/Skia/wgpu、生产在用、双后端可验证）；§3 包组织目标结构；§7 任务分解与门禁。
- **任务状态唯一真源**：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md`——§2.1 当前焦点（已切 **P2.5 包组织收口**：go list 对比、分层约束测试、索引与总账同步）；§2.2 待办池；§7 状态流水。
- **进度**：P0–P1 全✅（基线冻结）；P2.1–P2.4e 全✅（shader 合并→旧原型删→老渲染器删→设备/预算/延迟释放/纹理分配/裁剪/混合/呈现收敛）。

### 硬规矩摘要（全文见 AGENTS.md）

- git：只提交自己改动的文件；提交前确认（git diff --stat 核清单）；推送需用户单独确认；禁 git add . / -A / commit -am
- 分层：render 不出现后端特有类型（hal 统一）；ui 不直调 gpu；测试缝与创建入口除外
- 能力存留：删除依据=实现冗余，不以消费者数为准；标准能力零消费者也保留
- 测试：逐测试文件回归（禁一次全量）；GPU 包必须带 WGPU_NATIVE_PATH 否则假绿；环境依赖缺前置用 t.Skipf 显式跳，禁静默假绿
- 红修纪律：每红先定性（测试过期/产品 bug/环境红）；禁止为凑绿删断言；每批修完跑相邻回归+整包对照（红名单只减不增）
- GPU 真窗安全：批量/压测切隔离显示；桌面单窗按 WM_DELETE 正常关闭；隔离环境才允许 GPUI_ALLOW_GPU_HARD_KILL=1
- 收口口径：生产只认组件库/2.5D 引擎真实调用；真窗示例与单测算原子验证

### 当前状态

- 分支 feat/p2-收敛整合，HEAD `135529f6`，**36 个提交未推送**（推送前问用户），工作区全净
- P2.4e 已收口（`705244f5`）：四子步 e1–e4（G2 老抽象拆解/B2 裁剪/B3 混合枚举/B4 呈现单计划化）+ 红修战役 13 批；render 包**整包 0 红**、gpu 包整包+逐文件全绿、ui 全绿
- **三个真产品 bug 已修**：整页上传永久饥饿（compaction 页文字永不上屏→整页条目豁免预算）、上下文所有权（Wayland ownsContext 共享设备被首窗连带销毁→AdapterContext 引用计数）、Tab 空环吞键（空环穿透）
- tmp/s6_present_baseline.json 已于 10-31 同日重采（s6_9 门禁依赖同日基线）

### 已立案尾巴（不阻塞 P2.5，按档案续作）

全部诊断链在 `docs/render_gpu_converge/evidence/P2.4e/red-fix-campaign.md`（18 节）：

1. **misc_draw wgpu 执行态**：effect surface+高级混色 dst 采样黑——探针链三路完成（resolve ✓/src ✓/seed ✓/bundle ✓，执行时采样透明），需 GPU capture 级工具（renderdoc/wgpu 调试标签），stderr 探针不可达
2. **SeatCapCB 回调 robustness**：C→Go 回调派发链间歇失败（flaky 实证：sweep 挂/复跑 3-3 过；二分指认 TestXrrModeRefreshHz 被复跑推翻——纯算术测试不可能污染 C 回调链）——robustness 审查专项
3. **G3 在册**（TestPresentDamageX11 环境车道）；**I1** 解剖完成、修复方向待 wgpu 呈现异步化评估
4. **X11/seat 环境类已落实 2/3**：TestX11S4ProcessKeyEvent/TestX11StateReadback 已前置 Skipf；TestSeatCapCBFirstSilent 归 robustness 专项
5. **J4**（ui/ 不在逐文件门禁清单）归 P2.5；depcheck 测试缝豁免已落地

### 环境注意事项

- 隔离合成器 gpui-p24e 已关闭；桌面 :1 有历史测试残留窗口（无碍）
- worktree `gpui-redab`（92ca9216 A/B 用）可删：`git worktree remove ../gpui-redab`
- tmp/s6_present_baseline.json 与 tmp/s6_9_heavy_budget.json 为机器本地门禁数据（10-31 同日重采）
- 本会话在桌面 :1 开关过几十个测试窗口，P1GL 类 X11 测试对桌面状态敏感——真窗批量前先验桌面健康

### 任务队列

1. （可选）推送 36 个提交（用户确认后）
2. **P2.5 包组织收口**：开工登记+影响面评估 → 停下报告拍板 → 动手
3. misc_draw GPU capture 专项 / SeatCapCB robustness 审查（各需工具链，独立立项）
4. P2.6：P1 基线全量重放（I3/I4 等环境类在此总清算；基线按 P2.6 规程重采）
