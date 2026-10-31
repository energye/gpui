# 会话交接（P2.4e 收口 → P2.5 开工）

- 交接日期：2026-10-31；分支 `feat/p2-收敛整合`；HEAD `135529f6`（台账补记）；**36 个提交未推送**
- 工作区：全净（仅 3 张别线 showcase PNG + 未跟踪目录，勿提交）

## 一、必读文档（按序）

1. `AGENTS.md`——工作区全部纪律（git/分层/测试/资源/GPU 安全），每个新会话第一必读
2. `docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md`——收敛方案总纲：§0 标准判定口径、§2 删除/合并清单、§3 包组织目标结构、§7 任务分解与门禁
3. `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md`——**任务状态唯一真源**：§2.1 当前焦点（已切 P2.5）、§2.2 待办池、§7 状态流水、§8 修订记录
4. `docs/RENDER_API_CATALOG.md`——API 总账（每次公开 API 改动必须同步）
5. `docs/ENGINE_GPU_RESOURCE_LIFECYCLE.md`——资源谁建谁放
6. `docs/render_gpu_converge/evidence/P2.4e/`——P2.4e 全部证据：`summary.md`（收口总账）、`red-fix-campaign.md`（红修战役 18 节诊断链）、`assessment.md`
7. `docs/ENGINE_WAYLAND_NESTED_TEST.md`——Wayland 嵌套测试规程（真窗用）

## 二、需求目标

**总目标**（用户 7 条口径）：render/gpu 收敛——①重复的删一套保留标准实现；②能合并的合并；③死代码按「任意图形/UI 组件库/2.5D 引擎」三根支柱支撑需求删；④webgpu+gwgpu 双后端各平台实现；⑤缺失能力先文档再实现；⑥半成品先文档再实现；⑦包组织规范。判定「标准」三尺：对齐上游（naga/Skia/wgpu）、生产在用、双后端可验证。**能力存留不以消费者数为删除依据，删除依据=实现冗余**（AGENTS.md 硬规矩）。

**当前进度**：P0–P1 全✅（基线冻结）；P2.1–P2.4e 全✅（shader 合并→旧原型删→老渲染器删→设备/预算/延迟释放/纹理分配/裁剪/混合/呈现收敛）。

**下一任务 P2.5 包组织收口**：`go list` 对比、分层约束测试、索引与总账同步——J4（ui/ 不在逐文件门禁清单的缺口）归此。按台账纪律：先开工登记+影响面评估，停下报告拍板后再动手。

## 三、关键规矩（新会话必须遵守的硬约束摘要）

- **git**：只提交自己本次改动的文件；提交前与用户确认（`git diff --stat` 核清单）；推送需用户单独确认；禁 `git add .`/`-A`/`commit -am`
- **分层**：render 不出现后端特有类型（`hal` 统一）；ui 不直调 gpu；测试缝与创建入口除外
- **能力存留**：删除依据=实现冗余；标准能力零消费者也保留（「已验证待接线」是正常中间态）
- **测试**：数据入 `testdata/`；逐测试文件回归（禁一次全量）；GPU 包必须带 `WGPU_NATIVE_PATH` 否则假绿；环境依赖用例缺前置用 `t.Skipf` 显式跳，禁静默假绿
- **红修纪律**：每红先定性（测试过期/产品 bug/环境红）；产品 bug 真修、测试过期改绑依据、环境红显式 Skipf；**禁止为凑绿删断言**；每批修完跑相邻回归+整包对照（红名单只减不增）
- **GPU 真窗安全**：批量/压测先切隔离显示环境；单窗验收在桌面按「一轮一窗、WM_DELETE 正常关闭」；禁 `kill -9`/`pkill` 清场（隔离环境允许 `GPUI_ALLOW_GPU_HARD_KILL=1` 兜底）
- **收口口径**：生产只认组件库/2.5D 引擎真正调用；真窗示例与单测算原子验证；能力存留与删除依据见 AGENTS.md「能力存留与删除依据（硬）」

## 四、当前状态与未竟事项

- **P2.4e 已收口**（`705244f5` + 补记 `135529f6`）：四子步 e1–e4 + 红修战役 13 批；render 包整包 0 红、gpu 包整包+逐文件全绿、ui 全绿；**三个真产品 bug 修复**（整页上传永久饥饿、上下文所有权、Tab 空环吞键）
- **已立案尾巴**（`red-fix-campaign.md` 18 节带完整诊断链）：
  1. misc_draw wgpu 执行态：effect surface+高级混色 dst 采样黑——需 GPU capture 级工具（renderdoc/wgpu 调试标签），stderr 探针不可达
  2. SeatCapCB 回调 robustness：C→Go 回调派发链间歇失败（flaky 实证，二分指认被复跑推翻）——robustness 审查
  3. G3 在册（TestPresentDamageX11 环境车道）；I1 解剖完成、修复方向待 wgpu 呈现异步化评估
- **环境注意事项**：tmp/s6_present_baseline.json 已于 10-31 重采（同日基线）；隔离合成器 gpui-p24e 已关闭；桌面 :1 有本会话测试残留窗口（无碍）；worktree gpui-redab（92ca9216 A/B 用）可删
- **X11/seat 处置决议**：TestX11S4ProcessKeyEvent/TestX11StateReadback 已 Skipf 落实；TestSeatCapCBFirstSilent 归 robustness 专项（flaky 非环境）

## 五、新会话开工动作

1. 读本文档 + AGENTS.md + 台账 §2.1（焦点已切 P2.5）
2. 确认是否先推送 36 个提交（用户确认后 `git push`）
3. P2.5 开工登记：影响面评估落盘 `evidence/P2.5/assessment.md` → 停下报告拍板 → 动手
4. 红修尾巴随 P2.5/P2.6 车道处理（J4 门禁清单归 P2.5；misc_draw/SeatCapCB 各按专项）
