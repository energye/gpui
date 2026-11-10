# K4：CRLF 富文本 span 边界偏 1（Build 归一化对齐）

> 台账来源：§2.2 K4（P0-3 探针暴露 2026-11-03，登记时已定修法方向：
> 「render/paragraph Build 归一文本后再算 spanEnds」）。落地日期 2026-11-11。
> **流程注记**：本项按登记方向直接落地，未再单独停一次「停下报告」——
> 严格流程应在动刀前停；处置确认放在提交节点，瑕疵如实登记。

## 机制（源码钉死）

- **builder 侧** `render/paragraph.go`：`Build` 把 spans 原文拼接、`runeRuns`
  按原文累计（\r\n = **2 rune**）；
- **layout 侧** `render/text/layout.go:425-431`：`splitParagraphs` 先归一
  `\r\n→\n`（= **1 rune**）再排，cluster 是**归一化空间**的全局 rune 偏移
  （:203-208 注释明示「consumers that index the full string
  (Paragraph.StyleForRun) read them directly」）；
- 两空间错位 = 每个前置 CRLF 让 run→span 边界右移 1：`Build` 的 run.Style
  注解（:130-136）、`spanEnds`/`StyleForRun`（:168-178）、`styledFace.RunsAt`
  （:223-228）三处消费端全体偏移——多段 + CRLF 混用时样式跨硬换行渗色。
  LF 单段/无换行不受影响（latent，与登记一致）。

## 修法（按登记方向）

`Build` 开头对每个 span 文本做与 `splitParagraphs` 同口径的归一化
（新助手 `normalizeLineBreaks`：CRLF→LF、CR→LF），归一化后重算
`runeRuns`/`paraText`，三处消费端（styledFace.spans、run.Style 注解、
boundaries/spanEnds）统一吃归一化产物。`AddText` 保持原样存储（单点对齐在
Build）；LF/无换行文本归一化前后逐字节相同 ⇒ 零行为变化。

## 钉测（先红后绿，`render/paragraph_crlf_test.go`，goregular 字体）

| 用例 | 修前 | 修后 |
|---|---|---|
| `TestParagraph_CRLFSpanBoundary`（"AB\r\n"红+"CD"绿：cluster 2/3/4 三断言 + run.Style 注解断言） | **红**：cluster 3（'C'，第 2 行）读 span 0 渗红 | 绿 |
| `TestParagraph_LFSpanBoundaryControl`（LF 控制腿） | 绿（护栏） | 绿 |
| `TestParagraph_MultiCRLFSpanBoundary`（两个 CRLF 累积偏移 ×2） | **红**：cluster 4 渗红 | 绿 |

负向对照：Build 摘掉归一化（`t := sp.text` 变异）→ 两枚拒绝用例转红 →
Edit 还原绿（无 checkout）。

## 门禁（2026-11-11）

- `TestParagraph*` 家族全绿；`-race`（CGO_ENABLED=1）绿。
- 触及面（P37 全系 + S3c TextUnderline）：无 lib 时 Bloom GPU 系 FAIL =
  **存量环境形态**（H1 口径：单独跑 SKIP「GPU filter graph not registered」、
  合并跑走 CPU fallback；带 lib 复跑 **ok**，10.5s），与本修无关（bloom 不涉
  paragraph）；vet rc=0；apidoc rc=0（无公开 API 变化）。
