# kit/typography — G7 Typography 真窗（F1 A 类第 2 件）

官方 §2.4 非 debug 10 例一窗摆完（P0 8 例 + P1 后缀/表格同窗），三证据门禁。

## 摆盘（两列八卡）

| 卡 | 例 | 覆盖 |
|---|---|---|
| basic | 基本 | Text / Paragraph / Link 三形态行 |
| title | 标题组件 | h1..h5 阶梯 38/30/24/20/16（按字号比例步进入卡） |
| decor | 修饰 | strong / code / mark / delete / underline / keyboard |
| ellipsis | 省略号 | rows=2 二分 cut + expandable |
| middle | 省略中间 | 首尾双截（Middle 固定切，不走二分） |
| copy/edit | 可复制/可编辑 | Copy() payload+OnCopy 一次 / StartEditing→Commit→Cancel 状态机 |
| suffix | 后缀（P1） | rows=1 省略保留 suffix 可见 |
| table | 表格（P1） | 表头加粗 + 数据行排版组合 |

## 证据（三证据合证）

- 逻辑探针 11 项：copy_ok / edit_ok / edit_cancel_ok / title_ladder（Token 阶梯逐级对数）/ body_14 / 六卡墨量 ink_*（≥60px）
- 像素断言：六卡文字墨量探针（body 底色差 >24/255 计墨）
- Golden：`testdata/showcase_typography_base.png` 逐位比对（body 全区 593024px），重播种需 `GPUI_ACCEPT_GOLDEN=1` + 人工过目 + 两跑逐位绿
- 门禁：fps≥55 / p95≤22 / vsync_source / resize 回基线 1200×800

## 官网并排验收（L4）

- 2026-11-16：首建基线。与 antd 6.5.1 官网 Typography 示例逐卡目检——四形态/修饰/省略形态对齐；Title 阶梯数值走 Token（seed 对数钉死 38/30/24/20/16，genFontSizes 公式复算一致）；mark/chip chrome 常量（#ffe58f / r3）按 antd style 常量抄进 Theme 段。字体光栅差以外无肉眼可见布局差异（窗口为深色自绘壳，官网为浅色网页——色板以 Token 断言为准，官网并排只看排版结构）。
- 差异登记：窗内省略行以 render 层 MaxWidth+Overflow 呈现（ellipsis 二分/Middle 双截算法在库层单测钉死 TYP-S4/S5）；copyable/editable 操作符图标行本窗未摆（状态机以逻辑探针验证，操作符 chrome 随 Icon 组件接线补）。

## 运行

```bash
RUN_SECONDS=15 go run ./examples/kit/typography        # 常驻人工
go run ./examples/kit/typography -auto-only            # 门禁（15s scripted）
GPUI_ACCEPT_GOLDEN=1 go run ./examples/kit/typography -auto-only  # 重播种（人工过目后）
```
