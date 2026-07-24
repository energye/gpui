#!/usr/bin/env python3
"""Deepen all docs/antd/*.md §6 sections to Button-level product specs.

Strategy:
  1. Parse existing kit docs for title, demos, API props, current P0/tip.
  2. Merge with per-control / family metadata (metrics, state machine, chrome, API, a11y).
  3. Replace only ## 6. … end-of-file; leave §1–§5 and button.md untouched.

Usage:
  python3 docs/antd/_deepen_sec6.py           # all except button
  python3 docs/antd/_deepen_sec6.py input modal  # subset
"""

from __future__ import annotations

import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

OUT = Path(__file__).resolve().parent
SRC = Path("/tmp/antd-docs-src")

# ---------------------------------------------------------------------------
# Shared L1–L4 legend (short; full definition still in Button)
# ---------------------------------------------------------------------------

ALIGN_INTRO = (
    "> 与 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) **同级细度**（度量档、状态机规则 ID、"
    "chrome 表、P0/P1、可测用例、Go API、DoD）。  \n"
    "> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；"
    "**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  \n"
    "> 全库样板细节可回看 [button.md §6](./button.md#6-11-产品需求增量gpui-验收规格)。\n"
)

# ---------------------------------------------------------------------------
# Family defaults
# ---------------------------------------------------------------------------

FAMILY_METRICS: dict[str, list[tuple[str, str, str]]] = {
    "control": [
        ("控件高度 middle", "**32**", "`controlHeight`"),
        ("控件高度 small", "**24**", "`controlHeightSM`"),
        ("控件高度 large", "**40**", "`controlHeightLG`"),
        ("字号 middle", "**14**", "`fontSize`"),
        ("水平 padding", "**11** / **15**（按族）", "`controlPaddingInline` / 组件 token"),
        ("圆角", "**6**", "`borderRadius`"),
        ("边框线宽", "**1**", "`lineWidth`"),
        ("Focus ring outset", "≈ **1.5px** 可见", "实现可调，必须可见"),
    ],
    "input": [
        ("高度 middle/small/large", "**32 / 24 / 40**", "`controlHeight*`"),
        ("字号", "**14 / 12 / 16**", "`fontSize*`"),
        ("水平 padding", "**11**", "`paddingInline` / Input token"),
        ("圆角", "**6 / 4 / 8**", "`borderRadius*`"),
        ("前缀/后缀间距", "**4–8**", "`marginXXS`–`marginXS`"),
        ("清除图标可点区", "≥ **16**", "命中 ≥ 视觉"),
    ],
    "toggle": [
        ("指示器 Checkbox/Radio", "**16×16**", "与字号行高对齐"),
        ("Switch 默认", "宽 **44** 高 **22**", "small 更矮"),
        ("Rate 星", "随 size，默认约 **20**", "可半星"),
        ("行内间距", "指示器与 label **8**", "`marginXS`+"),
    ],
    "overlay": [
        ("Modal 默认宽", "**520**", "可配 `width`"),
        ("Drawer 默认宽", "**378**", "placement 决定边"),
        ("遮罩", "半透明黑", "`colorBgMask`"),
        ("圆角", "**8**（浮层）", "`borderRadiusLG`"),
        ("阴影", "浮层 elevation", "`boxShadow` / 二级"),
        ("箭头（气泡）", "约 **8** 边", "placement 指向触发器"),
    ],
    "feedback": [
        ("Message 顶距", "约 **8–16**", "堆叠间距"),
        ("Notification 宽", "约 **384**", "placement 四角"),
        ("Progress 线高", "**8**", "环默认 **120**"),
        ("Spin 默认", "**20** / large **32**", "可 fullscreen"),
    ],
    "nav": [
        ("Menu 项高", "约 **40**", "`itemHeight`"),
        ("Tabs 顶栏高", "约 **46**（line）", "card 略不同"),
        ("Pagination 项", "约 **32**", "mini 更小"),
        ("Steps 点/序号", "随 size", "title 字号 16"),
        ("inlineIndent", "**24**", "Menu"),
    ],
    "layout": [
        ("Divider 线宽", "**1**", "`lineWidth`"),
        ("Space gap", "small/middle/large → token 间距", "`margin*`"),
        ("Grid", "24 列", "`gutter` 水平/垂直"),
        ("Layout Header", "高 **64**", "Sider 宽 **200** 可折"),
        ("Splitter 拖条命中", "≥ 可拖视觉宽", "可折叠把手"),
    ],
    "data": [
        ("Table 行高 middle", "约 **55**", "small/large 档"),
        ("List 分割线", "**1**", "可分页"),
        ("Tree 行高", "约 **24**", "缩进 token"),
        ("Tag 高", "约 **22**", "可关闭图标"),
        ("Avatar 默认", "**32**", "large/small"),
        ("Card padding", "**24**", "圆角 **8**"),
    ],
    "display": [
        ("字号/字重", "Title level 阶梯；正文 **14**", "`fontSize*` / `fontWeightStrong`"),
        ("QRCode 默认边长", "**160**", "`size`"),
        ("Image 预览层", "全屏遮罩", "Portal"),
    ],
    "other": [
        ("主题/locale", "全局注入", "ConfigProvider"),
        ("Watermark 间隙/角度", "可配", "平铺"),
        ("Icon 默认", "**1em**", "随字号"),
    ],
}

FAMILY_STATE: dict[str, str] = {
    "control": """\
```text
mount ──► default ──hover──► hovered ──press──► pressed ──release/in-bounds──► action
             │                  │                    │
             │                  └── leave ───────────┘
             ├── focus ── 键盘等价 ──► action
             └── disabled ──► 吞事件，禁用皮
```
""",
    "input": """\
```text
mount ──► idle ──focus──► focused ──type──► editing ──blur/enter──► commit(onChange/onPressEnter)
             │                 │
             ├── hover（未禁用）
             ├── disabled / readOnly ──► 不提交变更
             ├── allowClear 点击 ──► value="" + onChange
             └── status=error|warning ──► 边框/反馈色（不阻塞输入，除非业务）
```
""",
    "toggle": """\
```text
mount ──► unchecked|checked
             │
             ├── click/Space ──► toggle ──► onChange(next)
             ├── disabled / loading ──► 不切换
             ├── Group：互斥（Radio）或独立（Checkbox.Group）
             └── indeterminate（Checkbox）仅视觉，点击后通常清半选
```
""",
    "overlay": """\
```text
closed ──open=true / trigger──► opening ──► open
  ▲                                      │
  │                                      ├── mask 点击（若允许）──► onCancel/onOpenChange(false)
  │                                      ├── Esc（若 keyboard）──► close
  │                                      ├── OK / Confirm ──► onOk / onConfirm → 可异步后 close
  └── closed ◄── afterClose ◄── closing ◄┘
destroyOnHidden：关闭后卸载内容；否则保活。
```
""",
    "feedback": """\
```text
trigger ──► visible（入场）──duration──► leave ──► destroy
             │
             ├── closable 点击 ──► 立即关闭
             ├── 堆叠：同 placement 纵向排列
             └── 更新 key：替换内容不新开（message/notification）
```
""",
    "nav": """\
```text
mount ──► items 渲染
             ├── 选中 selectedKeys / activeKey ──► 高亮 + onSelect/onChange
             ├── 展开 openKeys（Menu/Sub）──► 子面板
             ├── disabled item ──► 不可选
             └── 键盘：方向键移动、Enter 激活、Home/End（适用者）
```
""",
    "layout": """\
```text
mount ──► 按 orientation/gap/span 布局子节点
             ├── 响应式断点（Grid）──► 列宽重算
             ├── Splitter 拖动 ──► 面板尺寸 onResize
             └── 无「点击提交」类状态（Divider/Space/Flex 等静态为主）
```
""",
    "data": """\
```text
mount ──► dataSource/treeData 渲染
             ├── 选择/展开/排序/分页 ──► 对应 onChange
             ├── loading ──► 表体/列表遮罩或 Skeleton
             ├── 空数据 ──► Empty
             └── 虚拟滚动（P1/部分 P0）：仅绘可见行
```
""",
    "display": """\
```text
mount ──► 展示内容
             ├── 交互子集：copy / edit / ellipsis expand / preview
             └── 无值变更类 onChange（Statistic/QR 等只读为主）
```
""",
    "other": """\
```text
mount ──► 提供上下文 / 装饰 / 工具能力
             └── 行为以各控件语义为准（钉住、水印重绘、主题下发）
```
""",
}

FAMILY_A11Y: dict[str, list[tuple[str, str]]] = {
    "control": [
        ("角色", "与控件语义一致（button/textbox/listbox…）"),
        ("名称", "可交互控件有可读名称（label / aria-label）"),
        ("焦点", "Tab 可达；Focus ring 可见"),
        ("键盘", "主路径有键盘等价"),
        ("禁用", "disabled 不可激活，读屏可感知（平台支持时）"),
    ],
    "input": [
        ("角色", "textbox / searchbox（Search）/ spinbutton（InputNumber）"),
        ("标签", "与 Form.Item label 或 aria-labelledby 关联"),
        ("清除", "清除按钮有名称（如「清除」）"),
        ("状态", "`status`/`aria-invalid` 在 error 时暴露"),
        ("密码", "显隐切换按钮可访问名称"),
    ],
    "toggle": [
        ("角色", "checkbox / radio / switch"),
        ("状态", "aria-checked；indeterminate 时混合态"),
        ("键盘", "Space 切换；Radio 组方向键"),
        ("Group", "radiogroup / group 包一层"),
    ],
    "overlay": [
        ("焦点陷阱", "打开后焦点进入浮层；关闭回触发器（可配）"),
        ("Esc", "关闭（若允许）"),
        ("遮罩", "点击关闭策略明确；读屏标注 dialog/alertdialog"),
        ("标题", "Dialog 有可访问名称（title）"),
    ],
    "feedback": [
        ("实时区域", "message/notification 使用 status/alert 语义（平台等价）"),
        ("关闭", "关闭控件可聚焦或提供等价操作"),
        ("不抢焦点", "轻提示默认不抢焦点（Modal.confirm 例外）"),
    ],
    "nav": [
        ("角色", "menubar/menu/tablist/navigation"),
        ("当前页", "aria-current / selected"),
        ("键盘", "方向键、Home/End、Enter"),
    ],
    "layout": [
        ("分隔", "Divider 装饰性时 aria-hidden；有标题时作为分隔说明"),
        ("拖拽", "Splitter 把手可键盘微调（P1 可指针优先）"),
    ],
    "data": [
        ("表格", "列头与单元格关联；排序按钮可访问"),
        ("树", "aria-expanded / 层级"),
        ("选择", "行/节点选中态可读"),
    ],
    "display": [
        ("图片", "alt / 装饰图标记"),
        ("省略", "展开控件可访问"),
        ("复制", "复制按钮有名称与结果反馈"),
    ],
    "other": [
        ("主题", "不单独扮演交互角色"),
        ("图标", "装饰图标 aria-hidden；有意义图标提供名称"),
    ],
}

# ---------------------------------------------------------------------------
# Per-control metadata
# ---------------------------------------------------------------------------

@dataclass
class Ctrl:
    export: str
    family: str
    title_zh: str
    l1: str
    metrics_hint: str
    p0: list[str]
    p1: list[str]
    prefix: str  # case ID prefix e.g. INP
    structure: str
    chrome_extra: list[tuple[str, str]] = field(default_factory=list)
    rules_extra: list[tuple[str, str, str]] = field(default_factory=list)
    platform: list[tuple[str, str, str]] = field(default_factory=list)
    api_lines: list[str] = field(default_factory=list)
    defaults: list[tuple[str, str]] = field(default_factory=list)
    size_table: list[tuple[str, str, str, str, str]] = field(default_factory=list)
    # optional override family metrics key
    metrics_family: str = ""


def C(**kw: Any) -> Ctrl:
    return Ctrl(**kw)


CONTROLS: dict[str, Ctrl] = {
    # ---- General ----
    "float-button": C(
        export="FloatButton", family="control", title_zh="悬浮按钮",
        l1="点击、tooltip、badge、Group 展开、BackTop 滚动阈值",
        metrics_hint="圆形默认边长约 40；square 同高；贴边 margin 可配",
        p0=["type default|primary", "shape circle|square", "icon/content", "onClick",
            "tooltip", "badge", "FloatButton.Group open/trigger", "BackTop visibilityHeight",
            "hover/focus/disabled 态", "a11y 名（仅图标必填）"],
        p1=["href/target 桌面打开 URL", "自定义 badge 节点", "semantic classNames/styles",
            "拖拽位置", "多 Group 避让"],
        prefix="FB",
        structure="Positioned edge\n  └─ Pressable FloatBtn / Group\n       ├─ Badge?\n       ├─ Icon/Content\n       └─ Tooltip portal?",
        size_table=[
            ("default circle", "FloatDefault", "40", "—", "圆形"),
            ("square", "FloatSquare", "40", "—", "圆角方"),
            ("small（若支持）", "FloatSM", "32", "—", "紧凑"),
        ],
        api_lines=[
            "NewFloatButton() *FloatButton",
            "SetType(default|primary) SetShape(circle|square)",
            "SetIcon(...) SetContent(...) SetTooltip(...)",
            "SetBadge(...) OnClick(...)",
            "NewFloatButtonGroup() / SetTrigger / SetOpen",
            "NewBackTop() / SetVisibilityHeight",
        ],
        defaults=[("Type", "default"), ("Shape", "circle"), ("VisibilityHeight", "400")],
    ),
    "icon": C(
        export="Icon", family="display", title_zh="图标",
        l1="按名渲染矢量/字形；spin/rotate；两色/多色子集",
        metrics_hint="默认 1em 随字号；可显式 size",
        p0=["按 name 映射图标集", "size/color", "spin", "rotate", "装饰 vs 有名 义图标 a11y"],
        p1=["两色图标", "自定义 component 插槽", "全量 @ant-design/icons 对齐"],
        prefix="ICO",
        structure="IconView\n  └─ Vector/Font glyph (spin via Tick)",
        metrics_family="other",
        api_lines=["NewIcon(name) *Icon", "SetSize SetColor SetSpin SetRotate SetAriaLabel"],
        defaults=[("Size", "1em"), ("Spin", "false")],
    ),
    "typography": C(
        export="Typography", family="display", title_zh="排版",
        l1="Title/Text/Paragraph/Link；type 语义色；ellipsis/copyable/editable 主路径",
        metrics_hint="正文 14；Title h1–h5 字号阶梯；行高 token",
        p0=["Typography.Text/Title/Paragraph/Link", "type secondary|success|warning|danger",
            "disabled", "mark/code/keyboard/underline/delete/strong",
            "ellipsis 单行/多行+expand", "copyable", "Title level 1–5"],
        p1=["editable 就地编辑全能力", "ellipsis 中间省略", "copy 自定义 tip", "组件 token 细调"],
        prefix="TYP",
        structure="TextNode / Title / Paragraph\n  ├─ content\n  ├─ copy btn?\n  └─ edit / expand?",
        api_lines=[
            "NewText/NewTitle/NewParagraph/NewLink",
            "SetType SetLevel SetEllipsis SetCopyable SetDisabled",
            "OnCopy OnExpand",
        ],
        defaults=[("Level", "1 for Title"), ("Ellipsis", "false")],
    ),
    # ---- Layout ----
    "divider": C(
        export="Divider", family="layout", title_zh="分割线",
        l1="水平/垂直、虚线、标题位置、plain",
        metrics_hint="线宽 1；水平 margin 随 size；vertical 行内高度随文字",
        p0=["orientation horizontal|vertical", "dashed", "plain", "children 标题",
            "titlePlacement start|center|end", "size", "orientationMargin（非 center）"],
        p1=["semantic styles", "自定义线色 token 外覆盖"],
        prefix="DIV",
        structure="Divider\n  ├─ line (start)\n  ├─ title?\n  └─ line (end)",
        api_lines=["NewDivider()", "SetOrientation SetDashed SetPlain SetTitle SetTitlePlacement SetSize"],
        defaults=[("Orientation", "horizontal"), ("Dashed", "false"), ("TitlePlacement", "center")],
    ),
    "flex": C(
        export="Flex", family="layout", title_zh="弹性布局",
        l1="direction/wrap/justify/align/gap 布局子节点",
        metrics_hint="gap small/middle/large 映射 spacing token；可数字",
        p0=["vertical", "wrap", "justify", "align", "gap", "flex 子项"],
        p1=["与 Grid 混用响应式", "component 根标签定制"],
        prefix="FLX",
        structure="Flex (row|column)\n  └─ children…",
        api_lines=["NewFlex()", "SetVertical SetWrap SetJustify SetAlign SetGap", "Add(child)"],
        defaults=[("Vertical", "false"), ("Gap", "middle 或 0 按实现")],
    ),
    "grid": C(
        export="Grid", family="layout", title_zh="栅格",
        l1="24 栅格 Row/Col span offset push pull 响应式",
        metrics_hint="24 列；gutter 水平/垂直；断点 xs–xxl",
        p0=["Row gutter", "Col span/offset", "响应式 props", "wrap", "justify/align（Row）"],
        p1=["push/pull", "order", "Flex 混排细节"],
        prefix="GRD",
        structure="Row\n  └─ Col×N (span/offset)",
        api_lines=["NewRow() NewCol()", "SetGutter SetSpan SetOffset SetXs/Sm/…"],
        defaults=[("Span", "无默认强制",), ("Gutter", "0")],
    ),
    "layout": C(
        export="Layout", family="layout", title_zh="布局",
        l1="Header/Sider/Content/Footer 框架；Sider 折叠",
        metrics_hint="Header 64；Sider 200/collapsed 80；零级框架",
        p0=["Layout/Header/Sider/Content/Footer", "Sider collapsible/collapsed/onCollapse",
            "breakpoint 自动折叠", "theme light|dark（Sider）", "width/collapsedWidth"],
        p1=["自定义 trigger", "zeroWidthTrigger", "多 Sider"],
        prefix="LAY",
        structure="Layout\n  ├─ Sider?\n  └─ Layout\n       ├─ Header?\n       ├─ Content\n       └─ Footer?",
        api_lines=["NewLayout NewHeader NewSider NewContent NewFooter",
                   "Sider: SetCollapsible SetCollapsed SetWidth OnCollapse"],
        defaults=[("HeaderHeight", "64"), ("SiderWidth", "200"), ("CollapsedWidth", "80")],
    ),
    "masonry": C(
        export="Masonry", family="layout", title_zh="瀑布流",
        l1="多列不等高卡片流式排列",
        metrics_hint="列数 + gutter；列宽均分",
        p0=["columns", "gutter", "items/children", "响应式列数（若 API 有）"],
        p1=["虚拟滚动", "动画重排"],
        prefix="MAS",
        structure="Masonry\n  └─ Column×N\n       └─ items…",
        api_lines=["NewMasonry()", "SetColumns SetGutter SetItems"],
        defaults=[("Columns", "3")],
    ),
    "space": C(
        export="Space", family="layout", title_zh="间距",
        l1="子项间距 orientation/size/wrap/split；Compact",
        metrics_hint="size small/middle/large → 8/16/24 量级 token",
        p0=["orientation", "size", "wrap", "split", "align", "Space.Compact"],
        p1=["separator 自定义节点样式深度", "compact block"],
        prefix="SPC",
        structure="Space (row|col gap)\n  └─ item · split? · item …\nCompact: 合并边框组",
        api_lines=["NewSpace()", "SetOrientation SetSize SetWrap SetSplit", "NewSpaceCompact()"],
        defaults=[("Size", "small"), ("Orientation", "horizontal")],
    ),
    "splitter": C(
        export="Splitter", family="layout", title_zh="分隔面板",
        l1="多面板拖拽调尺寸、min/max、折叠",
        metrics_hint="拖条命中 ≥ 视觉；面板 min 约束",
        p0=["orientation", "panels size/min/max", "collapsible", "onResize/onResizeEnd", "lazy"],
        p1=["键盘微调", "多面板复杂约束算法像素级"],
        prefix="SPL",
        structure="Splitter\n  ├─ Panel\n  ├─ Handle\n  └─ Panel …",
        api_lines=["NewSplitter()", "AddPanel(min,max,size)", "SetOrientation OnResize"],
        defaults=[("Orientation", "horizontal")],
    ),
    # ---- Navigation ----
    "anchor": C(
        export="Anchor", family="nav", title_zh="锚点",
        l1="点击滚动到目标、ink 指示、affix、监听滚动高亮",
        metrics_hint="ink 指示条；offsetTop 与 Affix 类似",
        p0=["items href/title", "affix", "offsetTop/targetOffset", "onClick/onChange",
            "当前高亮 link", "direction vertical|horizontal"],
        p1=["自定义 replace 历史", "bounds 边界动画像素级"],
        prefix="ANC",
        structure="Anchor (affix?)\n  └─ Link items + ink",
        api_lines=["NewAnchor()", "SetItems SetAffix SetOffsetTop OnChange OnClick"],
        defaults=[("Affix", "true")],
    ),
    "breadcrumb": C(
        export="Breadcrumb", family="nav", title_zh="面包屑",
        l1="items 路径、separator、下拉/链接项",
        metrics_hint="分隔符默认 /；字号 14",
        p0=["items title/href/menu", "separator", "itemRender", "params"],
        p1=["下拉 menu 深度定制", "icon 项"],
        prefix="BC",
        structure="Breadcrumb\n  └─ Item · sep · Item …",
        api_lines=["NewBreadcrumb()", "SetItems SetSeparator"],
        defaults=[("Separator", "/")],
    ),
    "dropdown": C(
        export="Dropdown", family="overlay", title_zh="下拉菜单",
        l1="trigger 悬停/点击打开 menu；选中回调；placement",
        metrics_hint="触发器+弹出层；placement 与箭头",
        p0=["menu items", "trigger hover|click|contextMenu", "open/onOpenChange",
            "placement", "disabled", "arrow", "destroyOnHidden"],
        p1=["dropdownRender 全自定义", "子菜单动画像素", "getPopupContainer 复杂嵌套"],
        prefix="DD",
        structure="Trigger\n  └─ Portal Menu\n       └─ items / sub",
        api_lines=["NewDropdown(child)", "SetMenu SetTrigger SetOpen SetPlacement OnOpenChange"],
        defaults=[("Trigger", "hover"), ("Placement", "bottomLeft")],
        metrics_family="overlay",
    ),
    "menu": C(
        export="Menu", family="nav", title_zh="导航菜单",
        l1="mode、items、selected/open keys、sub 展开、禁用",
        metrics_hint="项高 ~40；inlineIndent 24；水平/垂直/inline",
        p0=["mode vertical|horizontal|inline", "items/children 结构",
            "selectedKeys/defaultSelectedKeys", "openKeys/onOpenChange",
            "onClick/onSelect", "theme light|dark", "inlineCollapsed", "disabled items"],
        p1=["forceSubMenuRender", "自定义 expandIcon 动画", "溢出 popup 水平菜单"],
        prefix="MNU",
        structure="Menu\n  ├─ Item / ItemGroup\n  ├─ SubMenu → popup|inline\n  └─ Divider",
        api_lines=["NewMenu()", "SetMode SetItems SetSelectedKeys SetOpenKeys",
                   "OnClick OnOpenChange SetTheme SetInlineCollapsed"],
        defaults=[("Mode", "vertical"), ("Theme", "light")],
    ),
    "pagination": C(
        export="Pagination", family="nav", title_zh="分页",
        l1="current/pageSize/total 翻页、快速跳转、size changer",
        metrics_hint="项高约 32；mini 更小",
        p0=["current/defaultCurrent", "pageSize/total", "onChange/onShowSizeChange",
            "showSizeChanger", "showQuickJumper", "disabled", "simple/mini", "showTotal"],
        p1=["itemRender 全自定义", "响应式隐藏 size changer 细节"],
        prefix="PG",
        structure="Pagination\n  ├─ prev\n  ├─ page items / ellipsis\n  ├─ next\n  ├─ size changer?\n  └─ quick jumper?",
        api_lines=["NewPagination()", "SetCurrent SetPageSize SetTotal OnChange",
                   "SetShowSizeChanger SetShowQuickJumper SetDisabled"],
        defaults=[("Current", "1"), ("PageSize", "10")],
    ),
    "steps": C(
        export="Steps", family="nav", title_zh="步骤条",
        l1="current/status/items；方向；点击切换（可配）",
        metrics_hint="点/序号尺寸随 size；title 字号 16",
        p0=["current", "items title/description/status/icon", "status",
            "orientation horizontal|vertical", "size", "onChange（可点击）", "type default|navigation|inline"],
        p1=["progressDot", "percent 子进度", "自定义 labelPlacement"],
        prefix="STP",
        structure="Steps\n  └─ Step×N (icon|number + title + desc) + connectors",
        api_lines=["NewSteps()", "SetCurrent SetItems SetOrientation SetSize OnChange"],
        defaults=[("Current", "0"), ("Orientation", "horizontal")],
    ),
    "tabs": C(
        export="Tabs", family="nav", title_zh="标签页",
        l1="items 切换 activeKey；type；增删；位置",
        metrics_hint="tab 高约 46(top line)；可滚动 ink",
        p0=["items key/label/children", "activeKey/onChange", "type line|card|editable-card",
            "placement top|bottom|left|right", "size", "centered", "destroyOnHidden",
            "onEdit（可编辑卡）", "disabled tab"],
        p1=["自定义 addIcon", "sticky", "popupClassName 更多", "动画开关细项"],
        prefix="TAB",
        structure="Tabs\n  ├─ TabList (ink bar)\n  └─ TabPanels",
        api_lines=["NewTabs()", "SetItems SetActiveKey OnChange SetType SetPlacement SetSize"],
        defaults=[("Type", "line"), ("Placement", "top")],
    ),
    # ---- Data Entry ----
    "auto-complete": C(
        export="AutoComplete", family="input", title_zh="自动完成",
        l1="输入过滤 options、选中回填、受控 value",
        metrics_hint="高度=controlHeight；下拉同 Select 面板",
        p0=["value/defaultValue/onChange", "options", "onSearch/onSelect",
            "allowClear", "disabled", "status", "size", "filterOption", "open/onOpenChange"],
        p1=["自定义 option 渲染", "虚拟滚动", "和 Input.TextArea 插槽深度"],
        prefix="AC",
        structure="Input-like trigger\n  └─ Portal options list",
        api_lines=["NewAutoComplete()", "SetValue SetOptions OnSearch OnSelect SetAllowClear"],
        defaults=[("Open", "false（有匹配时）")],
    ),
    "cascader": C(
        export="Cascader", family="input", title_zh="级联选择",
        l1="多列级联选路径、changeOnSelect、异步 loadData",
        metrics_hint="高度=controlHeight；多列面板等宽策略",
        p0=["options", "value/onChange", "changeOnSelect", "loadData",
            "multiple", "showSearch", "allowClear", "disabled", "displayRender",
            "expandTrigger click|hover", "placement"],
        p1=["虚拟滚动列", "自定义 tag 渲染 multiple", "字段 names 映射"],
        prefix="CAS",
        structure="Selector\n  └─ Portal multi-column menus",
        api_lines=["NewCascader()", "SetOptions SetValue OnChange SetChangeOnSelect SetLoadData"],
        defaults=[("ExpandTrigger", "click"), ("ChangeOnSelect", "false")],
        metrics_family="input",
    ),
    "checkbox": C(
        export="Checkbox", family="toggle", title_zh="多选框",
        l1="checked/indeterminate、Group、禁用",
        metrics_hint="指示器 16×16；与 label 间距 8",
        p0=["checked/defaultChecked/onChange", "indeterminate", "disabled",
            "Checkbox.Group options/value", "autoFocus"],
        p1=["semantic 深度", "与 Form valuePropName=checked"],
        prefix="CB",
        structure="Checkbox\n  ├─ indicator (check|indeterminate)\n  └─ label",
        api_lines=["NewCheckbox(label)", "SetChecked SetIndeterminate SetDisabled OnChange",
                   "NewCheckboxGroup() SetOptions SetValue"],
        defaults=[("Checked", "false")],
    ),
    "color-picker": C(
        export="ColorPicker", family="input", title_zh="颜色选择器",
        l1="色值受控、面板、预设、格式、禁用透明",
        metrics_hint="触发色块随 size；面板固定结构",
        p0=["value/defaultValue/onChange/onChangeComplete", "format hex|rgb|hsb",
            "allowClear", "disabled", "disabledAlpha", "showText", "presets",
            "trigger", "open/onOpenChange", "size"],
        p1=["自定义触发器 panelRender", "渐变模式全能力", "取色吸管桌面权限"],
        prefix="CP",
        structure="Trigger swatch\n  └─ Portal panel (palette/alpha/presets/input)",
        api_lines=["NewColorPicker()", "SetValue OnChange SetFormat SetPresets SetDisabledAlpha"],
        defaults=[("Format", "hex"), ("DisabledAlpha", "false")],
    ),
    "date-picker": C(
        export="DatePicker", family="input", title_zh="日期选择",
        l1="选日/周/月/季/年、Range、disabledDate、format、面板",
        metrics_hint="高度=controlHeight；面板日历格节奏对齐 antd",
        p0=["value/defaultValue/onChange", "picker date|week|month|quarter|year",
            "format", "disabled/disabledDate", "allowClear", "showTime",
            "RangePicker", "placeholder", "status", "size", "variant",
            "open/onOpenChange", "minDate/maxDate（若有）"],
        p1=["cellRender 全自定义", "多历法", "presets 复杂", "order 交换动画"],
        prefix="DP",
        structure="Input trigger\n  └─ Portal panel (header + grid + footer/time?)",
        api_lines=["NewDatePicker() NewRangePicker()",
                   "SetValue SetPicker SetFormat OnChange SetDisabledDate SetShowTime"],
        defaults=[("Picker", "date"), ("Format", "按 locale")],
    ),
    "form": C(
        export="Form", family="input", title_zh="表单",
        l1="Item 绑定、rules 校验、onFinish、layout、依赖字段",
        metrics_hint="label 间距；item margin 底约 24；vertical/horizontal/inline",
        p0=["Form + Form.Item name", "rules required/message/validator",
            "onFinish/onFinishFailed", "layout vertical|horizontal|inline",
            "disabled", "requiredMark", "initialValues", "validateTrigger",
            "dependencies / shouldUpdate 主路径", "Form.List 基础增删",
            "validateFields / resetFields / setFieldsValue / getFieldsValue",
            "错误展示 help / validateStatus", "labelCol/wrapperCol（horizontal）"],
        p1=["滚动到首个错误字段动画", "Form.Provider 跨表", "useWatch 全量",
            "复杂动态 rules", "feedbackIcons 深度定制"],
        prefix="FRM",
        structure="Form\n  └─ Item (label + control + help/error)\n       └─ kit control (value/onChange bridge)",
        api_lines=["NewForm()", "AddItem(name, rules, child)", "SetLayout SetDisabled SetInitialValues",
                   "OnFinish OnFinishFailed",
                   "ValidateFields() ResetFields() SetFieldsValue() GetFieldsValue()",
                   "NewFormList(name) // add/remove/move"],
        defaults=[("Layout", "horizontal（antd 默认）或产品选定"), ("RequiredMark", "true"),
                  ("ValidateTrigger", "onChange")],
        metrics_family="input",
        rules_extra=[
            ("FRM-S1", "rules required 空提交", "onFinishFailed；字段 error 文案"),
            ("FRM-S2", "校验通过提交", "onFinish(values) 一次"),
            ("FRM-S3", "setFieldsValue", "控件显示更新；可触发校验策略按实现"),
            ("FRM-S4", "resetFields", "回到 initialValues；清 error"),
            ("FRM-S5", "disabled Form", "子控件不可编辑"),
            ("FRM-S6", "dependencies", "依赖字段变更触发再校验"),
            ("FRM-S7", "Form.List add/remove", "字段数组增删；name 路径正确"),
            ("FRM-S8", "layout 切换", "label 与控件排布正确"),
        ],
        chrome_extra=[
            ("item spacing", "项底 margin ≈ 24（middle）"),
            ("error", "错误色 help；控件 status=error"),
            ("required mark", "label 前 *（可关）"),
            ("horizontal", "labelCol/wrapperCol 栅格"),
        ],
        platform=[
            ("Item 绑定 + rules + onFinish", "对等", "P0 L1"),
            ("实例方法 validate/reset/set", "对等", "P0 L1"),
            ("与 kit 录入控件 value 桥", "对等（按控件适配表）", "P0"),
            ("滚动到错误", "近似", "P1"),
            ("React 专用 hooks 名", "Go 方法/回调映射", "P0 映射"),
        ],
    ),
    "input": C(
        export="Input", family="input", title_zh="输入框",
        l1="受控输入、size/variant/status、清除、前缀后缀、TextArea/Search/Password/OTP",
        metrics_hint="高 32/24/40；paddingInline 11",
        p0=["value/defaultValue/onChange", "size", "variant outlined|filled|borderless|underlined",
            "status", "allowClear", "disabled/readOnly", "prefix/suffix",
            "maxLength/showCount", "placeholder", "onPressEnter",
            "Input.TextArea autoSize", "Input.Search onSearch", "Input.Password",
            "focus ring", "a11y 标签"],
        p1=["addonBefore/After 与 Space.Compact 深度", "OTP 全能力", "count 自定义策略",
            "semantic classNames", "输入法 composition 边界精细测试"],
        prefix="INP",
        structure="Decorated field\n  ├─ prefix?\n  ├─ Editable text\n  ├─ clear?\n  ├─ suffix?/count?\n  └─ (Search button / Password toggle)",
        size_table=[
            ("small", "InputSM", "24", "12", "paddingInline≈7–11"),
            ("middle", "InputMD", "32", "14", "paddingInline≈11"),
            ("large", "InputLG", "40", "16", "paddingInline≈11"),
        ],
        api_lines=[
            "NewInput() *Input",
            "SetValue SetPlaceholder SetSize SetVariant SetStatus",
            "SetAllowClear SetDisabled SetReadOnly SetPrefix SetSuffix",
            "SetMaxLength SetShowCount OnChange OnPressEnter",
            "NewTextArea() NewSearch() NewPassword() NewOTP()",
            "Focus() / Blur() / SetAriaLabel",
        ],
        defaults=[
            ("Size", "middle"), ("Variant", "outlined"), ("AllowClear", "false"),
            ("Disabled", "false"),
        ],
        rules_extra=[
            ("INP-S1", "受控 value", "外部值优先，键入经 onChange 上抛"),
            ("INP-S2", "allowClear", "清空并 onChange(\"\")；空时隐藏清除"),
            ("INP-S3", "disabled", "不可编辑、无变更回调"),
            ("INP-S4", "maxLength", "阻止超出（或按 count 策略）"),
            ("INP-S5", "status=error", "错误色边框；仍可输入"),
            ("INP-S6", "PressEnter", "触发 onPressEnter"),
            ("INP-S7", "Password 显隐", "切换掩码不丢 value"),
            ("INP-S8", "TextArea autoSize", "高度随内容在 min/max 行间"),
        ],
        chrome_extra=[
            ("outlined", "容器底 + 边框；hover 边框强调；focus 主色环"),
            ("filled", "浅填充无强边框；focus 提升"),
            ("borderless", "无边框"),
            ("underlined", "底边线"),
            ("error/warning", "语义色边框/提示"),
        ],
        platform=[
            ("文本编辑/选区/IME", "宿主文本输入", "P0 宿主"),
            ("variant/size/status/allowClear", "对等", "P0 L1+L2"),
            ("addon* 旧 API", "推荐 Compact", "P1"),
            ("OTP/复杂 count", "分期", "P1"),
        ],
    ),
    "input-number": C(
        export="InputNumber", family="input", title_zh="数字输入框",
        l1="数值增减、min/max/step/precision、键盘上下",
        metrics_hint="同 Input 高；步进按钮区",
        p0=["value/onChange", "min/max/step", "precision", "disabled", "controls",
            "keyboard", "stringMode（大数）", "prefix/suffix", "status", "size", "variant",
            "changeOnWheel（可 P1）"],
        p1=["formatter/parser 复杂", "wheel 步进", "semantic 深度"],
        prefix="INN",
        structure="InputNumber\n  ├─ field\n  └─ controls up/down",
        api_lines=["NewInputNumber()", "SetValue SetMin SetMax SetStep SetPrecision OnChange SetControls"],
        defaults=[("Step", "1"), ("Controls", "true")],
    ),
    "mentions": C(
        export="Mentions", family="input", title_zh="提及",
        l1="触发符弹出 options、选中插入、多行",
        metrics_hint="多行 autoSize；@ 面板同浮层",
        p0=["value/onChange", "options", "prefix @", "onSelect/onSearch",
            "rows/autoSize", "disabled", "status", "allowClear", "split"],
        p1=["多 prefix", "validateSearch 复杂", "自定义过滤"],
        prefix="MEN",
        structure="TextArea\n  └─ Portal mention list at caret",
        api_lines=["NewMentions()", "SetValue SetOptions SetPrefix OnSelect OnSearch"],
        defaults=[("Prefix", "@")],
    ),
    "radio": C(
        export="Radio", family="toggle", title_zh="单选框",
        l1="互斥选中、Group、按钮样式",
        metrics_hint="圆点 16；Button 样式同按钮高",
        p0=["value/onChange（Group）", "options", "optionType default|button",
            "buttonStyle outline|solid", "disabled", "size", "单 Radio 受控"],
        p1=["block 按钮组", "semantic 深度"],
        prefix="RDO",
        structure="Radio / Radio.Group\n  └─ indicator + label | button chrome",
        api_lines=["NewRadio() NewRadioGroup()", "SetValue SetOptions SetOptionType SetButtonStyle OnChange"],
        defaults=[("OptionType", "default"), ("ButtonStyle", "outline")],
    ),
    "rate": C(
        export="Rate", family="toggle", title_zh="评分",
        l1="星级 value、半星、只读、自定义字符",
        metrics_hint="星尺寸随 size；默认 5 星",
        p0=["value/onChange", "count", "allowHalf", "allowClear", "disabled", "character", "tooltips"],
        p1=["自定义每星节点", "键盘精细"],
        prefix="RAT",
        structure="Rate\n  └─ Star×count (full|half|empty)",
        api_lines=["NewRate()", "SetValue SetCount SetAllowHalf SetDisabled OnChange"],
        defaults=[("Count", "5"), ("AllowHalf", "false")],
    ),
    "select": C(
        export="Select", family="input", title_zh="选择器",
        l1="单/多/标签模式、搜索、options、下拉、清除",
        metrics_hint="高=controlHeight；多选 tag 间距",
        p0=["options", "value/defaultValue/onChange", "mode multiple|tags",
            "showSearch/filterOption", "allowClear", "disabled", "placeholder",
            "open/onOpenChange", "size", "variant", "status", "maxTagCount",
            "optionFilterProp", "notFoundContent", "listHeight", "loading",
            "onSelect/onDeselect", "键盘导航"],
        p1=["虚拟滚动全开", "自定义 popupRender", "labelInValue 全链路",
            "tokenSeparators", "远程搜索防抖示例外", "maxCount 多选上限交互细节"],
        prefix="SEL",
        structure="Selector (tags|single text)\n  └─ Portal list (options / groups)",
        size_table=[
            ("small", "SelSM", "24", "12", "tag 紧凑"),
            ("middle", "SelMD", "32", "14", ""),
            ("large", "SelLG", "40", "16", ""),
        ],
        api_lines=["NewSelect()", "SetOptions SetValue SetMode SetShowSearch SetAllowClear",
                   "OnChange OnSearch OnSelect OnDeselect SetOpen SetLoading",
                   "SetMaxTagCount SetVariant SetStatus SetSize"],
        defaults=[("Mode", "single"), ("ShowSearch", "false（单选默认）"), ("ListHeight", "256")],
        rules_extra=[
            ("SEL-S1", "单选选中", "关闭弹层；onChange(value)"),
            ("SEL-S2", "多选", "累加 tags；再次点选项取消（可配）"),
            ("SEL-S3", "showSearch", "过滤 options；键盘上下+Enter"),
            ("SEL-S4", "allowClear", "清空值"),
            ("SEL-S5", "disabled", "不打开、不变更"),
            ("SEL-S6", "tags 模式", "可创建不在 options 中的值"),
            ("SEL-S7", "受控 value", "外部优先"),
            ("SEL-S8", "open 受控", "onOpenChange 与外部同步"),
            ("SEL-S9", "empty options", "notFoundContent"),
            ("SEL-S10", "Esc", "关闭弹层（打开时）"),
        ],
        chrome_extra=[
            ("selector", "同 Input 高/边框/variant"),
            ("popup", "elevation 阴影；max-height listHeight"),
            ("option active", "高亮键盘/hover 项"),
            ("option selected", "主色勾选/字色"),
            ("multiple tags", "tag 可关；+N 折叠 maxTagCount"),
            ("disabled", "降对比；箭头仍可见可不点"),
        ],
        platform=[
            ("options/value/mode/search/clear", "对等", "P0"),
            ("虚拟列表", "大数据建议 P0，可 Notes 分期", "P0/P1"),
            ("getPopupContainer", "统一 Portal", "P0"),
            ("labelInValue", "分期或 P0 子集", "P1"),
        ],
    ),
    "slider": C(
        export="Slider", family="input", title_zh="滑动输入条",
        l1="拖动/点击轨道改值、range、marks、tooltip",
        metrics_hint="轨道高约 4；手柄可点区更大",
        p0=["value/defaultValue/onChange/onChangeComplete", "min/max/step",
            "range", "marks", "disabled", "vertical", "tooltip open/formatter",
            "included", "keyboard"],
        p1=["dots", "自定义 handle 节点", "range draggableTrack"],
        prefix="SLD",
        structure="Slider\n  ├─ rail / track\n  ├─ handle(s)\n  └─ marks / tooltip",
        api_lines=["NewSlider()", "SetValue SetMin SetMax SetStep SetRange SetMarks OnChange"],
        defaults=[("Min", "0"), ("Max", "100"), ("Step", "1")],
    ),
    "switch": C(
        export="Switch", family="toggle", title_zh="开关",
        l1="checked 切换、loading、禁用、size、文案槽",
        metrics_hint="默认约 44×22；small 更小；手柄圆形",
        p0=["checked/defaultChecked/onChange", "loading", "disabled", "size medium|small",
            "checkedChildren/unCheckedChildren", "autoFocus", "键盘 Space"],
        p1=["自定义 loading 图标", "semantic classNames"],
        prefix="SW",
        structure="Switch track\n  └─ handle (+ spinner if loading)\n  └─ children text?",
        size_table=[
            ("default", "SwitchMD", "22", "—", "宽≈44"),
            ("small", "SwitchSM", "16", "—", "宽≈28"),
        ],
        api_lines=["NewSwitch()", "SetChecked SetLoading SetDisabled SetSize OnChange",
                   "SetCheckedChildren SetUnCheckedChildren"],
        defaults=[("Checked", "false"), ("Size", "default")],
        rules_extra=[
            ("SW-S1", "点击切换", "onChange(!checked) 一次"),
            ("SW-S2", "disabled", "不切换"),
            ("SW-S3", "loading", "不切换；显示 spinner"),
            ("SW-S4", "受控 checked", "外部值优先"),
            ("SW-S5", "Space（聚焦）", "切换"),
        ],
        chrome_extra=[
            ("checked", "轨道主色；手柄右移"),
            ("unchecked", "轨道中性；手柄左"),
            ("loading", "手柄内 spinner"),
            ("disabled", "降对比；无 hover"),
        ],
    ),
    "time-picker": C(
        export="TimePicker", family="input", title_zh="时间选择",
        l1="时分秒列选、format、步长、禁用时刻",
        metrics_hint="同 DatePicker 触发高；面板三列",
        p0=["value/onChange", "format", "hourStep/minuteStep/secondStep",
            "disabledTime", "allowClear", "use12Hours", "RangePicker", "status", "size", "open"],
        p1=["自定义 addon 渲染", "隐藏禁用选项策略细项"],
        prefix="TP",
        structure="Input trigger\n  └─ Portal columns H/M/S + confirm?",
        api_lines=["NewTimePicker() NewTimeRangePicker()",
                   "SetValue SetFormat SetSteps OnChange SetDisabledTime"],
        defaults=[("Format", "HH:mm:ss")],
    ),
    "transfer": C(
        export="Transfer", family="data", title_zh="穿梭框",
        l1="左右列表、目标键、搜索、全选",
        metrics_hint="双栏列表；中间操作按钮",
        p0=["dataSource", "targetKeys/onChange", "selectedKeys/onSelectChange",
            "titles", "render", "showSearch", "disabled", "oneWay", "pagination（基础）"],
        p1=["树穿梭", "自定义 footer/operations 深度", "虚拟列表"],
        prefix="TF",
        structure="Transfer\n  ├─ List left\n  ├─ Operations\n  └─ List right",
        api_lines=["NewTransfer()", "SetDataSource SetTargetKeys OnChange SetShowSearch"],
        defaults=[("TargetKeys", "空")],
    ),
    "tree-select": C(
        export="TreeSelect", family="input", title_zh="树选择",
        l1="树数据选中、多选 checkable、搜索、异步",
        metrics_hint="高=controlHeight；树面板",
        p0=["treeData", "value/onChange", "treeCheckable", "showSearch",
            "allowClear", "multiple", "treeDefaultExpandAll", "loadData",
            "disabled", "status", "size", "variant"],
        p1=["treeCheckStrictly", "虚拟", "自定义 titleRender 深度"],
        prefix="TSE",
        structure="Selector\n  └─ Portal Tree",
        api_lines=["NewTreeSelect()", "SetTreeData SetValue SetTreeCheckable OnChange SetShowSearch"],
        defaults=[("TreeCheckable", "false")],
    ),
    "upload": C(
        export="Upload", family="input", title_zh="上传",
        l1="选文件、fileList 受控、beforeUpload、自定义请求、列表类型",
        metrics_hint="按钮/拖拽区/照片墙；列表项高随 type",
        p0=["fileList/defaultFileList/onChange", "beforeUpload", "customRequest",
            "accept", "multiple", "maxCount", "listType text|picture|picture-card|picture-circle",
            "disabled", "onRemove", "showUploadList", "Drag"],
        p1=["断点续传", "目录上传", "预览与 Image 联调像素", "原生进度事件细项"],
        prefix="UPL",
        structure="Upload\n  ├─ trigger (button|drag|card)\n  └─ file list items (progress/status)",
        api_lines=["NewUpload()", "SetFileList BeforeUpload CustomRequest OnChange SetListType"],
        defaults=[("ListType", "text"), ("Multiple", "false")],
        metrics_family="input",
    ),
    # ---- Data Display ----
    "avatar": C(
        export="Avatar", family="data", title_zh="头像",
        l1="图/字/图标、形状尺寸、Group 叠放",
        metrics_hint="默认 32；large/small；circle|square",
        p0=["src/icon/children", "shape", "size", "gap（字）", "onError", "Avatar.Group max"],
        p1=["响应式 size 对象", "自定义 group popup"],
        prefix="AV",
        structure="Avatar (image|text|icon)\nGroup: stack + excess",
        api_lines=["NewAvatar()", "SetSrc SetIcon SetShape SetSize", "NewAvatarGroup()"],
        defaults=[("Shape", "circle"), ("Size", "default=32")],
    ),
    "badge": C(
        export="Badge", family="data", title_zh="徽标",
        l1="count/dot/status/Ribbon 角标",
        metrics_hint="红点 6–8；数字徽标 min 宽；offset",
        p0=["count", "dot", "offset", "overflowCount", "showZero", "status/text",
            "color", "size", "Ribbon"],
        p1=["自定义 count 节点动画", "处理 0 显示策略边界"],
        prefix="BDG",
        structure="Wrapper\n  ├─ children\n  └─ sup badge | Ribbon",
        api_lines=["NewBadge(child)", "SetCount SetDot SetOffset SetStatus SetColor", "NewRibbon()"],
        defaults=[("OverflowCount", "99"), ("ShowZero", "false")],
    ),
    "calendar": C(
        export="Calendar", family="data", title_zh="日历",
        l1="面板选日、模式月/年、全屏/卡片、单元格渲染",
        metrics_hint="全屏/卡片；单元格 padding 节奏",
        p0=["value/defaultValue/onSelect/onChange", "mode month|year", "fullscreen",
            "disabledDate", "headerRender（基础）", "cellRender 主路径"],
        p1=["农历", "复杂 header 定制像素"],
        prefix="CAL",
        structure="Calendar\n  ├─ header\n  └─ month/year grid",
        api_lines=["NewCalendar()", "SetValue SetMode SetFullscreen OnSelect"],
        defaults=[("Mode", "month"), ("Fullscreen", "true")],
    ),
    "card": C(
        export="Card", family="data", title_zh="卡片",
        l1="title/extra/cover/actions/loading/grid",
        metrics_hint="圆角 8；padding 24；可 hoverable",
        p0=["title", "extra", "cover", "actions", "loading", "hoverable", "size",
            "type inner", "Card.Grid", "Card.Meta"],
        p1=["tabList 与 Tabs 深度联用像素", "自定义边框 token 外"],
        prefix="CRD",
        structure="Card\n  ├─ cover?\n  ├─ head (title/extra)\n  ├─ body / Meta\n  └─ actions?",
        api_lines=["NewCard()", "SetTitle SetExtra SetCover SetActions SetLoading SetHoverable"],
        defaults=[("Size", "default"), ("Hoverable", "false")],
    ),
    "carousel": C(
        export="Carousel", family="data", title_zh="走马灯",
        l1="自动播放、指示点、前后切换、垂直",
        metrics_hint="全宽视口；指示点底沿",
        p0=["autoplay", "dots", "afterChange/beforeChange", "arrows",
            "dotPlacement", "infinite", "speed", "vertical（若支持）"],
        p1=["淡入淡出 effect", "自适应高度动画曲线像素"],
        prefix="CRS",
        structure="Carousel viewport\n  ├─ track slides\n  ├─ arrows?\n  └─ dots",
        api_lines=["NewCarousel()", "AddSlide SetAutoplay SetDots OnAfterChange GoTo"],
        defaults=[("Autoplay", "false"), ("Dots", "true")],
    ),
    "collapse": C(
        export="Collapse", family="data", title_zh="折叠面板",
        l1="手风琴/多开、activeKey、items",
        metrics_hint="项头可点；内容区 padding；动画可关",
        p0=["items", "activeKey/defaultActiveKey/onChange", "accordion",
            "bordered", "ghost", "expandIconPosition", "collapsible", "size", "destroyOnHidden"],
        p1=["自定义 expandIcon 动画曲线", "嵌套 Collapse 细节"],
        prefix="COL",
        structure="Collapse\n  └─ Panel (header + body conditional)",
        api_lines=["NewCollapse()", "SetItems SetActiveKey SetAccordion OnChange"],
        defaults=[("Accordion", "false"), ("Bordered", "true")],
    ),
    "descriptions": C(
        export="Descriptions", family="data", title_zh="描述列表",
        l1="items 栅格、column、bordered、size",
        metrics_hint="label/content 栅格；小中间大 padding",
        p0=["items label/children/span", "column", "bordered", "size", "title", "layout", "colon"],
        p1=["响应式 column 对象", "自定义 labelStyle 深度"],
        prefix="DSC",
        structure="Descriptions\n  ├─ title?\n  └─ rows of Item(label, content)",
        api_lines=["NewDescriptions()", "SetItems SetColumn SetBordered SetSize SetTitle"],
        defaults=[("Column", "3"), ("Bordered", "false")],
    ),
    "empty": C(
        export="Empty", family="data", title_zh="空状态",
        l1="插画+描述+操作区",
        metrics_hint="默认图与 simple 图；描述字号 14",
        p0=["description", "image", "imageStyle", "children 操作区", "PRESENTED_IMAGE_SIMPLE"],
        p1=["多语言默认文案 ConfigProvider"],
        prefix="EMP",
        structure="Empty\n  ├─ image\n  ├─ description\n  └─ footer actions?",
        api_lines=["NewEmpty()", "SetDescription SetImage AddAction"],
        defaults=[("Description", "暂无数据（locale）")],
    ),
    "image": C(
        export="Image", family="data", title_zh="图片",
        l1="展示、预览组、fallback、placeholder",
        metrics_hint="图片盒+预览层全屏",
        p0=["src", "alt", "width/height", "preview open/mask", "fallback",
            "placeholder", "Image.PreviewGroup"],
        p1=["预览工具条旋转缩放像素动画", "toolbarRender 全自定义"],
        prefix="IMG",
        structure="Image box\n  └─ Preview portal (mask + img + toolbar)",
        api_lines=["NewImage()", "SetSrc SetAlt SetPreview SetFallback", "NewPreviewGroup()"],
        defaults=[("Preview", "true")],
        metrics_family="display",
    ),
    "list": C(
        export="List", family="data", title_zh="列表",
        l1="dataSource/renderItem、分页、加载更多、栅格",
        metrics_hint="项分割线；可分页",
        p0=["dataSource", "renderItem", "header/footer", "loading", "pagination",
            "size", "bordered", "split", "grid", "itemLayout", "empty 态"],
        p1=["虚拟列表（官方倾向其它方案）", "无限滚动"],
        prefix="LST",
        structure="List\n  ├─ header?\n  ├─ items\n  ├─ footer?\n  └─ pagination?",
        api_lines=["NewList()", "SetDataSource SetRenderItem SetPagination SetLoading"],
        defaults=[("Split", "true")],
    ),
    "popover": C(
        export="Popover", family="overlay", title_zh="气泡卡片",
        l1="title/content 浮层、trigger、受控 open",
        metrics_hint="气泡+箭头；placement",
        p0=["content", "title", "trigger", "open/onOpenChange", "placement", "arrow",
            "color", "destroyOnHidden"],
        p1=["mouseEnterDelay 精细", "getPopupContainer 复杂"],
        prefix="POP",
        structure="Trigger\n  └─ Portal card (title/content) + arrow",
        api_lines=["NewPopover(child)", "SetContent SetTitle SetTrigger SetOpen SetPlacement"],
        defaults=[("Trigger", "hover"), ("Placement", "top")],
    ),
    "qr-code": C(
        export="QRCode", family="display", title_zh="二维码",
        l1="value 生成、尺寸、状态、纠错、图标",
        metrics_hint="默认边长 160",
        p0=["value", "size", "status active|expired|loading|scanned",
            "errorLevel", "icon/iconSize", "color/bgColor", "type canvas|svg", "onRefresh"],
        p1=["自定义 statusRender", "导出文件 API"],
        prefix="QR",
        structure="QRCode box\n  ├─ canvas/svg\n  └─ status overlay?",
        api_lines=["NewQRCode()", "SetValue SetSize SetStatus SetErrorLevel SetIcon OnRefresh"],
        defaults=[("Size", "160"), ("Status", "active"), ("ErrorLevel", "M")],
    ),
    "segmented": C(
        export="Segmented", family="toggle", title_zh="分段控制器",
        l1="options 单选段、size、block、禁用项",
        metrics_hint="高≈controlHeight；滑块指示",
        p0=["options", "value/onChange", "size", "block", "disabled", "shape（若有）"],
        p1=["垂直", "自定义 name 渲染动画"],
        prefix="SEG",
        structure="Segmented track\n  └─ item×N + sliding thumb",
        api_lines=["NewSegmented()", "SetOptions SetValue OnChange SetSize SetBlock"],
        defaults=[("Block", "false")],
        metrics_family="control",
    ),
    "statistic": C(
        export="Statistic", family="display", title_zh="统计数值",
        l1="title/value/prefix/suffix、倒计时",
        metrics_hint="标题+数值字号阶梯",
        p0=["title", "value", "prefix/suffix", "precision", "loading",
            "Statistic.Countdown value/onFinish/format", "valueStyle"],
        p1=["分组千分位自定义", "动画数字"],
        prefix="STA",
        structure="Statistic\n  ├─ title\n  └─ value (prefix + number + suffix)",
        api_lines=["NewStatistic()", "SetTitle SetValue SetPrefix SetSuffix", "NewCountdown()"],
        defaults=[("Precision", "0 或按值")],
    ),
    "table": C(
        export="Table", family="data", title_zh="表格",
        l1="columns/dataSource、分页、行选、排序筛选主路径、loading",
        metrics_hint="行高约 55/middle；表头可固定",
        p0=["columns key/dataIndex/title/render/width/fixed/ellipsis", "dataSource",
            "rowKey", "pagination", "loading", "rowSelection（含全选）", "size", "bordered",
            "scroll x/y + fixed 列", "onChange（分页/排序/筛选）", "expandable 基础",
            "空态 Empty", "表头 sorter / filters 主路径", "showHeader"],
        p1=["虚拟滚动全量", "列拖拽宽", "树形 children 全能力", "summary 汇总行",
            "自定义 filterDropdown", "服务端受控 sort/filter 全链路", "components 自定义单元格"],
        prefix="TBL",
        structure="Table\n  ├─ header (sort/filter)\n  ├─ body rows (selection/expand)\n  └─ pagination?",
        api_lines=["NewTable()", "SetColumns SetDataSource SetRowKey SetPagination SetRowSelection",
                   "SetLoading SetScroll SetBordered SetSize OnChange",
                   "SetExpandable(expandedRowRender)"],
        defaults=[("Size", "large（antd 默认）"), ("Bordered", "false"), ("Pagination", "false 或默认")],
        rules_extra=[
            ("TBL-S1", "分页变更", "onChange 带 pagination"),
            ("TBL-S2", "排序", "列 sorter → onChange sorter"),
            ("TBL-S3", "筛选", "filters → onChange filters"),
            ("TBL-S4", "行选", "selectedRowKeys 受控；全选半选"),
            ("TBL-S5", "loading", "表体遮罩不丢列头"),
            ("TBL-S6", "空 dataSource", "显示 Empty"),
            ("TBL-S7", "scroll.y", "表体滚动；表头可固定"),
            ("TBL-S8", "fixed 列", "横向滚动时钉列"),
            ("TBL-S9", "expandable", "展开行渲染；expanded 受控可选"),
        ],
        chrome_extra=[
            ("header", "底部分割；sort 图标"),
            ("row hover", "浅底反馈"),
            ("bordered", "单元格全边框"),
            ("selected row", "浅主色底"),
            ("loading", "表体 Spin 遮罩"),
            ("empty", "居中 Empty"),
        ],
        platform=[
            ("columns/dataSource/pagination/rowSelection", "对等", "P0"),
            ("sort/filter 主路径", "对等", "P0"),
            ("fixed/scroll", "对等（桌面滚动容器）", "P0"),
            ("虚拟滚动", "分期或大数据 P0", "P1"),
            ("components 插槽", "分期", "P1"),
        ],
    ),
    "tag": C(
        export="Tag", family="data", title_zh="标签",
        l1="色/关闭/边框/CheckableTag",
        metrics_hint="高约 22；可关闭图标",
        p0=["children", "color 预设|自定义", "closable/onClose", "bordered",
            "icon", "CheckableTag checked/onChange"],
        p1=["动画关闭", "全预设色板视觉回归"],
        prefix="TAG",
        structure="Tag\n  ├─ icon?\n  ├─ text\n  └─ close?",
        api_lines=["NewTag()", "SetColor SetClosable OnClose", "NewCheckableTag()"],
        defaults=[("Bordered", "true"), ("Closable", "false")],
    ),
    "timeline": C(
        export="Timeline", family="data", title_zh="时间轴",
        l1="items 轴点、mode、pending",
        metrics_hint="轴点+线；交替布局",
        p0=["items children/label/color/dot", "mode left|right|alternate", "pending", "reverse"],
        p1=["自定义连接线样式深度"],
        prefix="TL",
        structure="Timeline\n  └─ Item (dot + line + content + label?)",
        api_lines=["NewTimeline()", "SetItems SetMode SetPending SetReverse"],
        defaults=[("Mode", "left")],
    ),
    "tooltip": C(
        export="Tooltip", family="overlay", title_zh="文字提示",
        l1="悬停/聚焦出 tip、placement、受控 open",
        metrics_hint="小气泡；placement 十二向",
        p0=["title", "trigger", "open/onOpenChange", "placement", "arrow", "color",
            "mouseEnterDelay/mouseLeaveDelay", "destroyOnHidden"],
        p1=["自动调整 overflow 算法像素", "wrap disabled 子元素策略"],
        prefix="TIP",
        structure="Trigger\n  └─ Portal tip + arrow",
        api_lines=["NewTooltip(child)", "SetTitle SetPlacement SetTrigger SetOpen OnOpenChange"],
        defaults=[("Trigger", "hover"), ("Placement", "top")],
    ),
    "tour": C(
        export="Tour", family="overlay", title_zh="漫游式引导",
        l1="steps 高亮目标、current、遮罩镂空",
        metrics_hint="高亮镂空+步骤气泡",
        p0=["steps target/title/description", "current/onChange", "open/onClose",
            "type", "placement", "indicatorsRender 基础", "masked"],
        p1=["自定义遮罩 SVG 动画", "scrollIntoView 边界"],
        prefix="TOU",
        structure="Tour layer\n  ├─ mask hole\n  └─ step popover + controls",
        api_lines=["NewTour()", "SetSteps SetCurrent SetOpen OnClose OnChange"],
        defaults=[("Open", "false"), ("Current", "0")],
    ),
    "tree": C(
        export="Tree", family="data", title_zh="树形控件",
        l1="展开/选中/勾选、treeData、异步、禁用节点",
        metrics_hint="行高约 24；缩进",
        p0=["treeData", "expandedKeys/onExpand", "selectedKeys/onSelect",
            "checkedKeys/onCheck", "checkable", "multiple", "disabled",
            "showLine/showIcon", "defaultExpandAll", "loadData", "draggable 基础可选 P1"],
        p1=["虚拟滚动", "拖拽排序全能力", "自定义 switcherIcon 动画"],
        prefix="TRE",
        structure="Tree\n  └─ Node (switcher + check? + icon + title + children)",
        api_lines=["NewTree()", "SetTreeData SetExpandedKeys SetSelectedKeys SetCheckable OnExpand OnSelect OnCheck"],
        defaults=[("Checkable", "false"), ("Multiple", "false")],
    ),
    # ---- Feedback ----
    "alert": C(
        export="Alert", family="feedback", title_zh="警告提示",
        l1="type 语义、可关闭、描述、banner、action",
        metrics_hint="图标+文案；可关闭；padding 节奏",
        p0=["type success|info|warning|error", "message", "description",
            "closable/onClose", "showIcon", "banner", "action", "icon"],
        p1=["marquee banner", "afterClose 动画回调精细"],
        prefix="ALT",
        structure="Alert\n  ├─ icon?\n  ├─ message/description\n  ├─ action?\n  └─ close?",
        api_lines=["NewAlert()", "SetType SetMessage SetDescription SetClosable SetShowIcon OnClose"],
        defaults=[("Type", "info"), ("ShowIcon", "false（非 banner）")],
    ),
    "drawer": C(
        export="Drawer", family="overlay", title_zh="抽屉",
        l1="open、placement、title、关闭、遮罩、页脚",
        metrics_hint="侧滑默认宽 378；高全屏侧",
        p0=["open/onClose", "placement left|right|top|bottom", "title", "width/height",
            "mask/maskClosable", "closable", "footer", "destroyOnHidden", "keyboard",
            "afterOpenChange", "push 多抽屉（基础）"],
        p1=["getContainer 复杂", "额外面板 extra 像素", "尺寸拖拽"],
        prefix="DRW",
        structure="Portal\n  ├─ mask\n  └─ panel (header/body/footer)",
        api_lines=["NewDrawer()", "SetOpen SetPlacement SetTitle SetWidth OnClose SetFooter SetMaskClosable"],
        defaults=[("Placement", "right"), ("Width", "378"), ("MaskClosable", "true")],
    ),
    "message": C(
        export="Message", family="feedback", title_zh="全局提示",
        l1="命令式 open、type、duration、销毁",
        metrics_hint="顶栏通知条；自动关闭；堆叠",
        p0=["message.open/success/info/warning/error/loading", "content", "duration",
            "key 更新", "onClose", "App.useApp 上下文", "maxCount"],
        p1=["自定义 style/className 深度", "RTL"],
        prefix="MSG",
        structure="Host holder (top)\n  └─ Message items stack",
        api_lines=["Message.Success/Info/Warning/Error/Loading(content, duration)",
                   "Message.Open(config) Message.Destroy()"],
        defaults=[("Duration", "3s"), ("MaxCount", "可配")],
    ),
    "modal": C(
        export="Modal", family="overlay", title_zh="对话框",
        l1="open、标题、OK/Cancel、confirmLoading、页脚、居中",
        metrics_hint="宽 520 默认；居中/顶距",
        p0=["open/onCancel", "title", "onOk", "footer", "confirmLoading",
            "closable", "mask/maskClosable", "centered", "width", "destroyOnHidden",
            "keyboard", "Modal.confirm/info/success/error/warning 命令式",
            "afterClose", "okText/cancelText", "zIndex", "焦点陷阱与关闭回焦"],
        p1=["可拖拽", "全屏", "modalRender", "focusable 细项", "静态方法与 App 上下文深度"],
        prefix="MDL",
        structure="Portal\n  ├─ mask\n  └─ dialog (header/body/footer buttons)",
        size_table=[
            ("default width", "ModalMD", "—", "—", "宽 520"),
            ("自定义 width", "—", "—", "—", "按 props"),
        ],
        api_lines=["NewModal()", "SetOpen SetTitle SetFooter OnOk OnCancel SetConfirmLoading SetCentered",
                   "SetWidth SetMaskClosable SetKeyboard SetDestroyOnHidden",
                   "Modal.Confirm/Info/Success/Error/Warning(cfg) → closer"],
        defaults=[("Width", "520"), ("MaskClosable", "true"), ("Keyboard", "true"),
                  ("Centered", "false"), ("DestroyOnHidden", "false")],
        rules_extra=[
            ("MDL-S1", "open=true", "显示遮罩+面板；焦点进入"),
            ("MDL-S2", "onOk", "触发；可返回 Promise；confirmLoading"),
            ("MDL-S3", "onCancel / Esc / mask", "按配置关闭；触发 onCancel"),
            ("MDL-S4", "confirmLoading", "OK loading，防重复 onOk"),
            ("MDL-S5", "destroyOnHidden", "关闭卸载子树"),
            ("MDL-S6", "命令式 confirm", "Promise/回调；返回销毁函数"),
            ("MDL-S7", "footer=null", "无默认按钮"),
            ("MDL-S8", "closable=false", "无右上角关闭；仍可 onCancel 路径若允许"),
            ("MDL-S9", "堆叠 Modal", "z-index 递增；关闭顺序合理"),
        ],
        chrome_extra=[
            ("mask", "colorBgMask 半透明；点击可关可选"),
            ("dialog", "容器底 + 阴影 + 圆角 LG"),
            ("header", "title + close"),
            ("footer", "默认 Cancel + OK；可自定义/null"),
            ("centered", "垂直居中 vs 顶距约 100"),
            ("confirm icons", "info/success/error/warning 语义图标"),
        ],
        platform=[
            ("open/title/footer/onOk/onCancel", "对等", "P0"),
            ("命令式 confirm 族", "对等（非 DOM 静态方法名可映射）", "P0"),
            ("焦点陷阱", "对等", "P0"),
            ("getContainer", "Portal 宿主", "P0/P1"),
            ("modalRender 拖拽", "分期", "P1"),
        ],
    ),
    "notification": C(
        export="Notification", family="feedback", title_zh="通知提醒框",
        l1="四角 placement、duration、按钮、更新 key",
        metrics_hint="宽约 384；placement 四角/边",
        p0=["open", "placement", "duration", "message/description", "btn",
            "key", "onClose", "type 图标", "App.useApp", "maxCount", "stack"],
        p1=["showProgress", "自定义图标动画", "RTL"],
        prefix="NTF",
        structure="Host holder (placement)\n  └─ Notice items",
        api_lines=["Notification.Open/Success/…(config)", "Notification.Destroy()"],
        defaults=[("Placement", "topRight"), ("Duration", "4.5s")],
    ),
    "popconfirm": C(
        export="Popconfirm", family="overlay", title_zh="气泡确认框",
        l1="确认/取消、title、触发后回调",
        metrics_hint="确认气泡；OK/Cancel",
        p0=["title/description", "onConfirm/onCancel", "open/onOpenChange",
            "okText/cancelText", "okButtonProps/cancelButtonProps", "disabled",
            "placement", "trigger", "icon", "showCancel"],
        p1=["异步 onConfirm 保持 open 至 resolve", "与 Form 联用细节"],
        prefix="PCF",
        structure="Trigger\n  └─ Portal confirm (icon+title+actions)",
        api_lines=["NewPopconfirm(child)", "SetTitle OnConfirm OnCancel SetOpen SetPlacement"],
        defaults=[("Trigger", "click"), ("ShowCancel", "true")],
    ),
    "progress": C(
        export="Progress", family="feedback", title_zh="进度条",
        l1="percent、type、status、颜色",
        metrics_hint="线高 8；环默认 120",
        p0=["percent", "type line|circle|dashboard", "status normal|exception|active|success",
            "strokeColor", "showInfo", "size", "steps（线）", "trailColor"],
        p1=["渐变 stroke", "success 区间", "自定义 format 节点"],
        prefix="PRG",
        structure="Progress\n  line: rail+track+info\n  circle: stroke ring+info",
        api_lines=["NewProgress()", "SetPercent SetType SetStatus SetStrokeColor SetShowInfo"],
        defaults=[("Type", "line"), ("ShowInfo", "true"), ("Percent", "0")],
    ),
    "result": C(
        export="Result", family="feedback", title_zh="结果",
        l1="status 插画、title、extra 操作",
        metrics_hint="大图标+标题+extra",
        p0=["status success|error|info|warning|404|403|500", "title", "subTitle",
            "extra", "icon"],
        p1=["自定义 404 插画资源包"],
        prefix="RES",
        structure="Result\n  ├─ icon/status\n  ├─ title/subTitle\n  └─ extra",
        api_lines=["NewResult()", "SetStatus SetTitle SetSubTitle SetExtra"],
        defaults=[("Status", "info")],
    ),
    "skeleton": C(
        export="Skeleton", family="feedback", title_zh="骨架屏",
        l1="active 动画、avatar/title/paragraph 组合、元素骨架",
        metrics_hint="灰块；圆角；动画可关",
        p0=["active", "loading 切换子内容", "avatar", "title", "paragraph rows",
            "Skeleton.Button/Input/Image/Avatar/Node"],
        p1=["自定义标题宽数组动画曲线"],
        prefix="SKL",
        structure="Skeleton\n  ├─ avatar?\n  └─ title + paragraph bars",
        api_lines=["NewSkeleton()", "SetActive SetLoading SetAvatar SetParagraph"],
        defaults=[("Active", "false"), ("Loading", "true")],
    ),
    "spin": C(
        export="Spin", family="feedback", title_zh="加载中",
        l1="spinning 包裹、size、tip、延迟、全屏",
        metrics_hint="默认指示 20；large 32；可 fullscreen",
        p0=["spinning", "size", "tip", "delay", "fullscreen", "indicator 自定义基础", "嵌套包裹 children"],
        p1=["自定义 indicator 动画", "percent 不确定进度"],
        prefix="SPN",
        structure="Spin\n  ├─ mask+indicator when spinning\n  └─ children",
        api_lines=["NewSpin()", "SetSpinning SetSize SetTip SetDelay SetFullscreen"],
        defaults=[("Spinning", "true"), ("Size", "default")],
    ),
    "watermark": C(
        export="Watermark", family="other", title_zh="水印",
        l1="文字/图水印平铺、间隙、旋转、嵌套内容",
        metrics_hint="平铺角度/间隙；canvas 生成",
        p0=["content 文字|数组", "image", "gap", "offset", "rotate", "zIndex",
            "font 样式", "children 内容区", "继承撑满"],
        p1=["防删 DOM 变异观察（浏览器）桌面近似", "高清 DPR 细节"],
        prefix="WM",
        structure="Watermark container\n  ├─ children\n  └─ tiled layer (canvas/texture)",
        api_lines=["NewWatermark()", "SetContent SetGap SetRotate SetImage SetFont"],
        defaults=[("Rotate", "-22"), ("Gap", "[100,100]")],
    ),
    # ---- Other ----
    "affix": C(
        export="Affix", family="other", title_zh="固钉",
        l1="滚动到阈值钉住、offset、target 容器",
        metrics_hint="钉住偏移 offsetTop/Bottom",
        p0=["offsetTop/offsetBottom", "target", "onChange", "子内容宽度保持"],
        p1=["与自定义滚动容器嵌套像素", "resize 观察精细"],
        prefix="AFX",
        structure="Affix\n  └─ child (position fixed when past threshold)",
        api_lines=["NewAffix(child)", "SetOffsetTop SetOffsetBottom SetTarget OnChange"],
        defaults=[("OffsetTop", "0")],
        metrics_family="other",
    ),
    "app": C(
        export="App", family="other", title_zh="App 包裹",
        l1="为 message/notification/modal 提供上下文",
        metrics_hint="无独立几何；透传配置",
        p0=["包裹应用根", "message/notification/modal 静态配置", "useApp 等价注入"],
        p1=["与 ConfigProvider 嵌套顺序文档外行为"],
        prefix="APP",
        structure="App provider\n  └─ children + holders",
        api_lines=["NewApp()", "SetMessageConfig SetNotificationConfig", "Children…"],
        defaults=[],
        metrics_family="other",
    ),
    "border-beam": C(
        export="BorderBeam", family="other", title_zh="边框光效",
        l1="边框流光装饰动画",
        metrics_hint="描边宽度/时长/色彩",
        p0=["color/colorFrom/colorTo", "duration", "size", "lineWidth", "包裹 children", "reduced-motion 可关"],
        p1=["多段路径", "与主题 token 自动联动"],
        prefix="BB",
        structure="Wrapper\n  ├─ children\n  └─ beam stroke animation",
        api_lines=["NewBorderBeam(child)", "SetColors SetDuration SetSize SetLineWidth"],
        defaults=[("Duration", "实现默认")],
        metrics_family="other",
    ),
    "config-provider": C(
        export="ConfigProvider", family="other", title_zh="全局化配置",
        l1="theme/locale/componentSize/direction 下发",
        metrics_hint="无独立盒；影响后代 token",
        p0=["theme algorithm/token/components", "locale", "componentSize",
            "direction ltr|rtl", "getPopupContainer", "variant 全局默认（若有）",
            "wave 禁用", "嵌套覆盖"],
        p1=["全量 component config 逐项", "动态主题切换过渡动画"],
        prefix="CFG",
        structure="ConfigProvider\n  └─ context → descendants",
        api_lines=["NewConfigProvider()", "SetTheme SetLocale SetComponentSize SetDirection"],
        defaults=[("Direction", "ltr")],
        metrics_family="other",
    ),
    "util": C(
        export="Util", family="other", title_zh="工具类型",
        l1="GetRef/GetProps/GetProp 类型工具（TS）；Go 侧文档化对应",
        metrics_hint="无运行时 UI",
        p0=["文档说明 GetProps/GetRef 用途", "kit 侧类型别名对照（若需要）"],
        p1=["完整泛型示例矩阵"],
        prefix="UTL",
        structure="（无运行时节点）",
        api_lines=["// 类型层工具，无 NewXxx UI"],
        defaults=[],
        metrics_family="other",
    ),
}


# ---------------------------------------------------------------------------
# Parse existing doc
# ---------------------------------------------------------------------------

def parse_doc(path: Path) -> dict[str, Any]:
    text = path.read_text(encoding="utf-8")
    title = text.splitlines()[0].lstrip("# ").strip() if text else path.stem
    demos: list[tuple[str, str]] = []
    # §1.2 table
    m = re.search(r"### 1\.2.*?\n\n(?:\|[^\n]+\n)+\| ---.*?\n((?:\|[^\n]+\n)+)", text)
    if m:
        for line in m.group(1).splitlines():
            cells = [c.strip() for c in line.strip("|").split("|")]
            if len(cells) >= 1 and cells[0] and not cells[0].startswith("---"):
                demos.append((cells[0], cells[1] if len(cells) > 1 else ""))
    # also §2.3 / example tables with demo file
    for m2 in re.finditer(r"\| ([^|]+) \| `([^`]+\.tsx)` \|", text):
        demos.append((m2.group(1).strip(), m2.group(2).strip()))

    # props: prefer §3 API / 配置项速查 tables, then whole doc
    props: list[tuple[str, str]] = []
    sec3 = ""
    m_sec3 = re.search(r"\n## 3\.\s+.*?\n(.*?)(?=\n## 4\.|\n## 5\.|\n## 6\.|\Z)", text, re.S)
    if m_sec3:
        sec3 = m_sec3.group(1)
    prop_src = sec3 if sec3 else text
    for m3 in re.finditer(r"^\|\s*`([a-zA-Z_][\w.]*)`\s*\|\s*([^|]+)\|", prop_src, re.M):
        name, desc = m3.group(1), m3.group(2).strip()
        if name.lower() in ("property", "prop", "参数", "配置项"):
            continue
        # skip pure type/default column mis-parses
        if name in ("string", "boolean", "number", "ReactNode", "true", "false"):
            continue
        props.append((name, desc))

    p0 = ""
    m4 = re.search(r"\*\*P0 能力摘要：\*\*\s*(.+)", text)
    if m4:
        p0 = m4.group(1).strip()
    tip = ""
    m5 = re.search(r"\*\*控件提示：\*\*\s*(.+)", text)
    if m5:
        tip = m5.group(1).strip()

    # split head / sec6
    m6 = re.search(r"\n## 6\.\s+", text)
    head = text[: m6.start()] if m6 else text.rstrip() + "\n"
    return {
        "title": title,
        "demos": demos,
        "props": props,
        "p0_summary": p0,
        "tip": tip,
        "head": head,
        "text": text,
    }


def unique_demos(demos: list[tuple[str, str]], limit: int = 24) -> list[tuple[str, str]]:
    seen = set()
    out = []
    for name, detail in demos:
        name = re.sub(r"<[^>]+>", "", name).strip()
        if not name or name in seen:
            continue
        if name.startswith("---") or name in ("示例名", "示例", "能力"):
            continue
        seen.add(name)
        out.append((name, detail))
        if len(out) >= limit:
            break
    return out


def unique_props(props: list[tuple[str, str]], limit: int = 18) -> list[tuple[str, str]]:
    seen = set()
    out: list[tuple[str, str]] = []
    skip = {"classNames", "styles", "className", "style", "rootClassName", "prefixCls"}
    # product-priority order for kit P0 scanning
    priority = [
        "value", "defaultValue", "checked", "defaultChecked", "open", "defaultOpen",
        "options", "items", "treeData", "dataSource", "columns", "fileList",
        "onChange", "onClick", "onOk", "onCancel", "onFinish", "onSelect", "onSearch",
        "disabled", "loading", "size", "variant", "status", "type", "mode", "layout",
        "allowClear", "showSearch", "placeholder", "title", "content", "children",
        "min", "max", "step", "percent", "placement", "trigger", "format",
    ]
    by_name = {}
    for name, desc in props:
        if name not in by_name:
            by_name[name] = desc
    for name in priority:
        if name in by_name and name not in seen:
            seen.add(name)
            desc = re.sub(r"\s+", " ", by_name[name]).strip()
            if len(desc) > 80:
                desc = desc[:77] + "…"
            out.append((name, desc))
    for name, desc in props:
        if name in seen or name in skip:
            continue
        seen.add(name)
        desc = re.sub(r"\s+", " ", desc).strip()
        if len(desc) > 80:
            desc = desc[:77] + "…"
        out.append((name, desc))
        if len(out) >= limit:
            break
    return out[:limit]


def pascal(slug: str) -> str:
    special = {
        "qr-code": "QRCode", "input-number": "InputNumber", "auto-complete": "AutoComplete",
        "date-picker": "DatePicker", "time-picker": "TimePicker", "tree-select": "TreeSelect",
        "color-picker": "ColorPicker", "float-button": "FloatButton",
        "config-provider": "ConfigProvider", "border-beam": "BorderBeam",
    }
    if slug in special:
        return special[slug]
    return "".join(p.capitalize() for p in slug.split("-"))


# ---------------------------------------------------------------------------
# Build §6
# ---------------------------------------------------------------------------

def build_sec6(slug: str, doc: dict[str, Any]) -> str:
    meta = CONTROLS.get(slug)
    if not meta:
        # fallback generic
        meta = C(
            export=pascal(slug),
            family="control",
            title_zh=doc["title"],
            l1="主路径交互/状态与 antd 一致",
            metrics_hint=doc.get("tip") or "走 Theme Token",
            p0=[p.strip() for p in (doc.get("p0_summary") or "主路径 API").split("/")],
            p1=["高级动画", "语义 classNames 深度", "虚拟列表/远程细节"],
            prefix=re.sub(r"[^A-Z]", "", pascal(slug).upper())[:3] or "CTL",
            structure=f"{pascal(slug)}\n  └─ content",
        )

    family = meta.family
    mf = meta.metrics_family or (
        "input" if family == "input" else
        "toggle" if family == "toggle" else
        "overlay" if family == "overlay" else
        "feedback" if family == "feedback" else
        "nav" if family == "nav" else
        "layout" if family == "layout" else
        "data" if family == "data" else
        "display" if family == "display" else
        "other" if family == "other" else
        "control"
    )
    metrics = FAMILY_METRICS.get(mf, FAMILY_METRICS["control"])
    state = FAMILY_STATE.get(family, FAMILY_STATE["control"])
    a11y = FAMILY_A11Y.get(family, FAMILY_A11Y["control"])
    demos = unique_demos(doc["demos"])
    props = unique_props(doc["props"])
    tip = meta.metrics_hint or doc.get("tip") or ""
    export = meta.export
    prefix = meta.prefix

    lines: list[str] = []
    lines.append("## 6. 1:1 产品需求增量（gpui 验收规格）\n")
    lines.append(ALIGN_INTRO)
    lines.append(f"### 6.1 对齐级别定义（{export}）\n")
    lines.append("| 级别 | 名称 | 本控件含义 | 验收方式 |")
    lines.append("| --- | --- | --- | --- |")
    lines.append(f"| **L1** | 行为 | {meta.l1} | Headless / behavior 测试 |")
    lines.append("| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 基线 | `ant_style_test` + Token 断言 |")
    lines.append("| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与仓库基线一致（AA 容差） | golden / visualtest |")
    lines.append("| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字，非 CI 绑官网 |")
    lines.append("")
    lines.append(f"**明确不做（{export}）：**\n")
    lines.append("- 与浏览器渲染 ant.design **逐像素哈希**一致。  ")
    lines.append("- 为抠图破坏 `hit == layout == paint` 边界。  ")
    lines.append("- 浏览器-only API（无桌面等价时见 §6.7）。  ")
    lines.append("")

    # 6.2 metrics
    lines.append("### 6.2 度量与 Design Token（L2 基线）\n")
    lines.append(
        "数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`）。"
        "实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。\n"
    )
    if tip:
        lines.append(f"**本控件几何提示：** {tip}\n")
    if meta.size_table:
        lines.append("#### 6.2.1 尺寸档位\n")
        lines.append("| size / 形态 | kit 枚举建议 | 高度 h | 字号 | 备注 |")
        lines.append("| --- | --- | --- | --- | --- |")
        for row in meta.size_table:
            lines.append(f"| `{row[0]}` | `{row[1]}` | **{row[2]}** | {row[3]} | {row[4]} |")
        lines.append("")
        lines.append("#### 6.2.2 通用 Token\n")
    else:
        lines.append("#### 6.2.1 几何与 Token\n")
    lines.append("| 项 | 默认值 | Token / 说明 |")
    lines.append("| --- | --- | --- |")
    for item, val, tok in metrics:
        lines.append(f"| {item} | {val} | {tok} |")
    lines.append("| 主色 / 错误 / 成功 / 警告 | 主题语义色 | `colorPrimary` / `colorError` / … |")
    lines.append("| 文本 / 边框 / 容器底 | 主题中性色 | `colorText` / `colorBorder` / `colorBgContainer` |")
    lines.append("| 禁用 | 降对比 | `colorDisabledBg` / `colorDisabledText` |")
    lines.append("")
    lines.append("实现禁止硬编码品牌色作为唯一默认皮。\n")

    # 6.3 key config
    lines.append("### 6.3 关键配置与语义\n")
    lines.append("下列为 **产品关键配置**（完整 API 见 §3）。重写 kit 时优先建模：\n")
    lines.append("| 配置 | 说明 |")
    lines.append("| --- | --- |")
    if props:
        for name, desc in props[:16]:
            lines.append(f"| `{name}` | {desc} |")
    else:
        for p in meta.p0[:12]:
            lines.append(f"| `{p}` | P0 能力 |")
    lines.append("")
    lines.append("**P0 能力清单（验收范围）：**\n")
    for p in meta.p0:
        lines.append(f"- {p}")
    lines.append("")

    # 6.4 state machine
    lines.append("### 6.4 交互状态机（L1）\n")
    lines.append(state)
    lines.append("| 规则 ID | 规则 | 期望 |")
    lines.append("| --- | --- | --- |")
    base_rules = [
        (f"{prefix}-S1", "disabled（适用者）", "不触发变更/主回调；禁用皮"),
        (f"{prefix}-S2", "受控值/open/checked（适用者）", "外部值优先，内部不擅自持久改写"),
        (f"{prefix}-S3", "主题切换", "下一帧/rebuild 后 chrome 正确"),
        (f"{prefix}-S4", "主路径交互", "见 §6.1 L1 描述与 §6.9 用例"),
    ]
    rules = meta.rules_extra if meta.rules_extra else base_rules
    # ensure at least base if extras only partial
    if meta.rules_extra:
        rules = meta.rules_extra + [r for r in base_rules if r[0] not in {x[0] for x in meta.rules_extra}]
    for rid, rule, exp in rules:
        lines.append(f"| {rid} | {rule} | {exp} |")
    lines.append("")

    # 6.5 chrome
    lines.append("### 6.5 视觉 chrome 规则（L2 摘要）\n")
    if meta.chrome_extra:
        lines.append("| 态 / 变体 | 规则 |")
        lines.append("| --- | --- |")
        for k, v in meta.chrome_extra:
            lines.append(f"| `{k}` | {v} |")
    else:
        lines.append("| 状态 | 要求 |")
        lines.append("| --- | --- |")
        lines.append("| default | 默认皮符合 Token / §6.2 |")
        lines.append("| hover / active / focus | 可交互控件具备反馈与 **可见** focus ring |")
        lines.append("| disabled | 降对比、不可点 |")
        lines.append("| loading / open / selected / checked 等 | 按本控件语义 |")
        lines.append("| 错误/警告 status（适用者） | 语义色边框或反馈，不靠硬编码红 |")
    lines.append("")
    lines.append("**动效：** 展开/入场动画须可关或尊重 reduced-motion；P0 可用瞬时切换。\n")

    # 6.6 a11y
    lines.append("### 6.6 无障碍（a11y）最低要求\n")
    lines.append("| 项 | 要求 |")
    lines.append("| --- | --- |")
    for k, v in a11y:
        lines.append(f"| {k} | {v} |")
    lines.append("")

    # 6.7 platform
    lines.append("### 6.7 平台边界（gpui vs 浏览器 antd）\n")
    lines.append("| 能力 | 策略 | 级别 |")
    lines.append("| --- | --- | --- |")
    if meta.platform:
        for cap, strat, level in meta.platform:
            lines.append(f"| {cap} | {strat} | {level} |")
    else:
        lines.append(f"| 主路径行为（§6.1 L1） | **对等** | P0 L1 |")
        lines.append("| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |")
        lines.append("| 动画/波纹/CSS 特效 | **近似**或瞬时 | P1 |")
        lines.append("| 浏览器-only API | **宿主映射**或不做 | P1 |")
        lines.append("| 逐像素官网哈希 | **不做** | — |")
        lines.append("| Semantic classNames/styles | kit 语义样式钩子 | P1 |")
        lines.append("| ConfigProvider 全局默认 | 随 ConfigProvider 能力 | P1 |")
    lines.append("")

    # 6.8 P0/P1
    lines.append("### 6.8 能力裁剪（P0 / P1）\n")
    lines.append("#### P0（本阶段必须 1:1，否则不算完成）\n")
    lines.append("| 配置 / 能力 | 说明 |")
    lines.append("| --- | --- |")
    for p in meta.p0:
        lines.append(f"| {p} | 必须 |")
    lines.append("| 度量 §6.2 | Token 断言 |")
    lines.append("| a11y §6.6 | 最低要求 |")
    lines.append("| §6.9 中 P0/L1/L2 用例 | 测试通过 |")
    lines.append("")
    lines.append("#### P1（可 later，须在 coverage Notes 写明）\n")
    lines.append("| 配置 / 能力 | 说明 |")
    lines.append("| --- | --- |")
    for p in meta.p1:
        lines.append(f"| {p} | 分期 |")
    lines.append("| 文档高级示例中的边缘能力 | 见 §1.2/§2 未列入 P0 者 |")
    lines.append("")

    # 6.9 cases
    lines.append("### 6.9 验收用例表（可测）\n")
    lines.append(
        f"> 每个用例对应测试名建议：`Test{export}_PRD_<ID>` 或 gallery 场景 ID。  \n"
        f"> **P0 相关用例全部通过** 才可宣称 {export} 完成 1:1 主路径。\n"
    )
    lines.append("| ID | 级别 | 步骤 | 期望 |")
    lines.append("| --- | --- | --- | --- |")
    n = 1
    # demo-based
    for name, detail in demos[:14]:
        lines.append(
            f"| {prefix}-{n:02d} | L1 | 复现示例「{name}」"
            + (f"（{detail}）" if detail and ".tsx" in detail else "")
            + f" | 行为与主视觉符合规格 |"
        )
        n += 1
    # synthetic essentials
    essentials = [
        ("L1", f"默认 {export} 主路径创建/展示", "不崩溃；默认值符合 §6.10"),
        ("L1", "disabled 或等价只读（适用者）", "主回调不触发"),
        ("L2", "Token 色/尺寸", "无硬编码品牌色默认皮；高度/间距符合 §6.2"),
        ("L1", "键盘/焦点（适用者）", "可聚焦者有 focus ring；主路径有键盘等价"),
        ("L1", "受控模式（适用者）", "外部值优先"),
        ("L3", "关键态截图", "与 golden 基线一致（容差内）"),
        ("L4", "与 ant.design 并排", "人眼签字记录"),
    ]
    for level, step, exp in essentials:
        lines.append(f"| {prefix}-{n:02d} | {level} | {step} | {exp} |")
        n += 1
    # extra from rules
    for rid, rule, exp in (meta.rules_extra or [])[:6]:
        lines.append(f"| {rid.replace('-S', '-R')} | L1 | {rule} | {exp} |")
    lines.append("")

    # 6.10 API
    lines.append("### 6.10 产品 API 契约（Go kit 侧）\n")
    lines.append(
        "> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。\n"
    )
    lines.append("```text")
    if meta.api_lines:
        for al in meta.api_lines:
            lines.append(al)
    else:
        lines.append(f"New{export}(...) *{export}")
        lines.append("// SetXxx 对应 §6.3 / §3；回调 OnChange/OnClick/…")
        lines.append("// Theme / Style；Node() 挂树")
    lines.append("```")
    lines.append("")
    if meta.defaults:
        lines.append("**默认值（未 Set 时）：**\n")
        lines.append("| 字段 | 默认 |")
        lines.append("| --- | --- |")
        for k, v in meta.defaults:
            lines.append(f"| {k} | {v} |")
        lines.append("")
    else:
        lines.append("默认值对齐 antd 6.5 文档 §3；未列出者实现时按官方表回落。\n")

    # 6.11 structure
    lines.append("### 6.11 结构与绘制分层（实现提示）\n")
    lines.append("```text")
    lines.append(meta.structure)
    lines.append("```")
    lines.append("")
    lines.append("- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  ")
    lines.append("- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  ")
    lines.append("- 命中区域与布局盒一致（`hit == layout == paint`）。  ")
    lines.append("")

    # 6.12 DoD
    lines.append("### 6.12 完成定义（DoD）\n")
    lines.append(f"同时满足即可宣布 **{export} 主路径 1:1 完成**：\n")
    lines.append(f"1. §6.8 **P0** 全部实现。  ")
    lines.append(f"2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  ")
    lines.append("3. L2 度量与 Token 断言通过。  ")
    lines.append("4. L3 golden 至少覆盖 1 个关键态（若控件可见）。  ")
    lines.append(f"5. gallery 展示主路径能力（对照 §1.2 / P0）。  ")
    lines.append(
        f"6. `coverage.go` Notes：P0 已对齐 `docs/antd/{slug}.md` §6；P1 显式列出。  "
    )
    lines.append("")
    lines.append("---")
    lines.append("")
    lines.append(
        f"**本章用法**：实现 `ui/kit` {export} 时以 §6 为需求与验收；"
        f"§1–§3 为 antd 能力全集；§6.8 为范围裁剪。"
        f"细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。"
    )
    lines.append("")
    return "\n".join(lines)


def process(slug: str, dry: bool = False) -> tuple[bool, str]:
    path = OUT / f"{slug}.md"
    if not path.exists():
        return False, f"missing {path}"
    if slug == "button":
        return False, "skip button (canonical sample)"
    doc = parse_doc(path)
    if "## 6." not in doc["text"] and "\n## 6." not in doc["text"]:
        # still append
        head = doc["text"].rstrip() + "\n\n"
    else:
        head = doc["head"]
        if not head.endswith("\n"):
            head += "\n"
    sec6 = build_sec6(slug, doc)
    new_text = head.rstrip() + "\n\n" + sec6
    if not dry:
        path.write_text(new_text, encoding="utf-8")
    # stats
    sec_lines = sec6.count("\n") + 1
    return True, f"{slug}: §6 ~{sec_lines} lines, demos={len(unique_demos(doc['demos']))}, props={len(unique_props(doc['props']))}"


def main() -> None:
    args = [a for a in sys.argv[1:] if not a.startswith("-")]
    dry = "--dry" in sys.argv
    if args:
        slugs = args
    else:
        slugs = sorted(
            p.stem
            for p in OUT.glob("*.md")
            if p.stem not in ("README",) and not p.name.startswith("_")
        )
    ok = 0
    for slug in slugs:
        success, msg = process(slug, dry=dry)
        print(msg)
        if success:
            ok += 1
    print(f"done: {ok}/{len(slugs)}")


if __name__ == "__main__":
    main()
