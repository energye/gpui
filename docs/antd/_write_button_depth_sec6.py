#!/usr/bin/env python3
"""Rewrite docs/antd/<slug>.md §6 to Button-level depth from local ant-design source.

One control per invocation of build_sec6(slug); main loops all thin docs.
Does NOT touch button.md. Prefer keeping float-button if already deep unless --force.

Source: /home/yanghy/app/projects/ant-design/components/<slug>/
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ANTD = Path("/home/yanghy/app/projects/ant-design/components")
OUT = Path("/home/yanghy/app/projects/gogpu/gpui/docs/antd")

# seed token defaults (antd default algorithm, fontSize=14, controlHeight=32)
SEED = {
    "controlHeight": 32,
    "controlHeightSM": 24,
    "controlHeightLG": 40,
    "fontSize": 14,
    "fontSizeSM": 12,
    "fontSizeLG": 16,
    "fontSizeIcon": 12,
    "lineWidth": 1,
    "borderRadius": 6,
    "borderRadiusSM": 4,
    "borderRadiusLG": 8,
    "padding": 16,
    "paddingXS": 8,
    "paddingSM": 12,
    "paddingLG": 24,
    "paddingXXS": 4,
    "margin": 16,
    "marginXS": 8,
    "marginXXS": 4,
    "marginLG": 24,
    "marginXXL": 48,
    "controlInteractiveSize": 16,  # controlHeight/2
    "colorPrimary": "colorPrimary",
    "colorError": "colorError",
    "colorText": "colorText",
    "colorBorder": "colorBorder",
    "colorBgContainer": "colorBgContainer",
    "colorSplit": "colorSplit",
    "colorBgMask": "colorBgMask",
    "boxShadowSecondary": "boxShadowSecondary",
}

# slug -> antd folder (when different)
FOLDER = {
    "qr-code": "qrcode",
    "float-button": "float-button",
    "input-number": "input-number",
    "auto-complete": "auto-complete",
    "date-picker": "date-picker",
    "time-picker": "time-picker",
    "tree-select": "tree-select",
    "color-picker": "color-picker",
    "config-provider": "config-provider",
    "border-beam": "border-beam",  # may not exist
}

# Export / prefix for test IDs
EXPORT = {
    "float-button": ("FloatButton", "FB"),
    "input-number": ("InputNumber", "INN"),
    "auto-complete": ("AutoComplete", "AC"),
    "date-picker": ("DatePicker", "DP"),
    "time-picker": ("TimePicker", "TP"),
    "tree-select": ("TreeSelect", "TSE"),
    "color-picker": ("ColorPicker", "CP"),
    "config-provider": ("ConfigProvider", "CFG"),
    "border-beam": ("BorderBeam", "BB"),
    "qr-code": ("QRCode", "QR"),
    "popconfirm": ("Popconfirm", "PCF"),
    "notification": ("Notification", "NTF"),
    "breadcrumb": ("Breadcrumb", "BC"),
    "pagination": ("Pagination", "PG"),
    "typography": ("Typography", "TYP"),
    "checkbox": ("Checkbox", "CB"),
    "radio": ("Radio", "RDO"),
    "switch": ("Switch", "SW"),
    "select": ("Select", "SEL"),
    "input": ("Input", "INP"),
    "form": ("Form", "FRM"),
    "table": ("Table", "TBL"),
    "modal": ("Modal", "MDL"),
    "drawer": ("Drawer", "DRW"),
    "menu": ("Menu", "MNU"),
    "tabs": ("Tabs", "TAB"),
    "upload": ("Upload", "UPL"),
    "cascader": ("Cascader", "CAS"),
    "transfer": ("Transfer", "TF"),
    "mentions": ("Mentions", "MEN"),
    "segmented": ("Segmented", "SEG"),
    "statistic": ("Statistic", "STA"),
    "timeline": ("Timeline", "TL"),
    "tooltip": ("Tooltip", "TIP"),
    "popover": ("Popover", "POP"),
    "dropdown": ("Dropdown", "DD"),
    "progress": ("Progress", "PRG"),
    "skeleton": ("Skeleton", "SKL"),
    "message": ("Message", "MSG"),
    "anchor": ("Anchor", "ANC"),
    "steps": ("Steps", "STP"),
    "slider": ("Slider", "SLD"),
    "rate": ("Rate", "RAT"),
    "tree": ("Tree", "TRE"),
    "list": ("List", "LST"),
    "card": ("Card", "CRD"),
    "badge": ("Badge", "BDG"),
    "avatar": ("Avatar", "AV"),
    "image": ("Image", "IMG"),
    "empty": ("Empty", "EMP"),
    "result": ("Result", "RES"),
    "alert": ("Alert", "ALT"),
    "spin": ("Spin", "SPN"),
    "watermark": ("Watermark", "WM"),
    "affix": ("Affix", "AFX"),
    "tour": ("Tour", "TOU"),
    "tag": ("Tag", "TAG"),
    "calendar": ("Calendar", "CAL"),
    "carousel": ("Carousel", "CRS"),
    "collapse": ("Collapse", "COL"),
    "descriptions": ("Descriptions", "DSC"),
    "divider": ("Divider", "DIV"),
    "flex": ("Flex", "FLX"),
    "grid": ("Grid", "GRD"),
    "layout": ("Layout", "LAY"),
    "masonry": ("Masonry", "MAS"),
    "space": ("Space", "SPC"),
    "splitter": ("Splitter", "SPL"),
    "icon": ("Icon", "ICO"),
    "app": ("App", "APP"),
    "util": ("Util", "UTL"),
}

# Per-control deep profiles: l1, metrics rows, state ascii, rules, chrome, a11y, platform, p0, p1, structure, api, defaults, cases extra
# Metrics rows: list of (item, value, token)
PROFILES: dict[str, dict] = {}


def pascal(slug: str) -> str:
    if slug in EXPORT:
        return EXPORT[slug][0]
    return "".join(p.capitalize() for p in slug.split("-"))


def prefix_of(slug: str) -> str:
    if slug in EXPORT:
        return EXPORT[slug][1]
    return re.sub(r"[^A-Z]", "", pascal(slug).upper())[:3] or "CTL"


def antd_dir(slug: str) -> Path:
    return ANTD / FOLDER.get(slug, slug)


def read_zh(slug: str) -> str:
    d = antd_dir(slug)
    for name in ("index.zh-CN.md", "index.en-US.md"):
        p = d / name
        if p.exists():
            return p.read_text(encoding="utf-8", errors="replace")
    return ""


def demos(text: str) -> list[tuple[str, str, bool]]:
    out = []
    for m in re.finditer(
        r'<code\s+src="\./demo/([^"]+)"([^>]*)>([^<]*)</code>', text
    ):
        src, attrs, title = m.group(1), m.group(2), m.group(3).strip()
        out.append((src, title or src, "debug" in attrs))
    return out


def api_props(text: str) -> list[tuple[str, str, str, str]]:
    """Return list of (name, desc, type, default) from markdown tables."""
    props = []
    for m in re.finditer(
        r"^\|\s*`?([a-zA-Z_][\w.]*)`?\s*\|\s*([^|]+)\|\s*([^|]*)\|\s*([^|]*)\|",
        text,
        re.M,
    ):
        name = m.group(1).strip()
        if name.lower() in ("参数", "property", "prop", "名称", "---") or re.match(
            r"^-+$", name
        ):
            continue
        if not re.match(r"^[a-zA-Z_]", name):
            continue
        desc = re.sub(r"\s+", " ", m.group(2)).strip()
        typ = re.sub(r"\s+", " ", m.group(3)).strip()
        default = re.sub(r"\s+", " ", m.group(4)).strip()
        props.append((name, desc, typ, default))
    # dedupe keep first
    seen = set()
    uniq = []
    for p in props:
        if p[0] in seen:
            continue
        seen.add(p[0])
        uniq.append(p)
    return uniq


def extract_token_hints(slug: str) -> list[tuple[str, str, str]]:
    """Best-effort parse prepareComponentToken / numeric merges from style ts."""
    d = antd_dir(slug)
    style_files = list((d / "style").glob("**/*.ts")) if (d / "style").exists() else []
    rows: list[tuple[str, str, str]] = []
    text = ""
    for f in style_files:
        try:
            text += f.read_text(encoding="utf-8", errors="replace") + "\n"
        except OSError:
            pass
    # controlHeight bindings
    if "controlHeightLG" in text and "floatButtonSize" in text:
        rows.append(("边长 floatButtonSize", "**40**", "controlHeightLG"))
    if re.search(r"itemHeight:\s*controlHeightLG", text):
        rows.append(("菜单项高 itemHeight", "**40**", "controlHeightLG"))
    if re.search(r"itemSize:\s*token\.controlHeight\b", text) or re.search(
        r"itemSize:\s*controlHeight\b", text
    ):
        rows.append(("分页项高 itemSize", "**32**", "controlHeight"))
    if "headerHeight: controlHeight * 2" in text or "headerHeight: controlHeight*2" in text:
        rows.append(("Layout Header 高", "**64**", "controlHeight×2"))
    if "gridColumns" in text:
        rows.append(("栅格列数", "**24**", "gridColumns"))
    if "checkboxSize: token.controlInteractiveSize" in text:
        rows.append(("Checkbox 指示器", "**16**", "controlInteractiveSize"))
    if "trackHeight" in text and "Switch" in str(d) or slug == "switch":
        # fontSize * lineHeight ≈ 22, width 44
        rows.append(("Switch 轨高 trackHeight", "**≈22**", "fontSize×lineHeight"))
        rows.append(("Switch 轨最小宽", "**≈44**", "handle×2+padding×4"))
        rows.append(("Switch small 轨高", "**16**", "controlHeight/2"))
    if "paddingInline: paddingSM - lineWidth" in text or "paddingSM - lineWidth" in text:
        rows.append(("Input paddingInline", "**≈11**", "paddingSM−lineWidth"))
    if "flexGapSM: paddingXS" in text:
        rows.append(("Flex gap small", "**8**", "paddingXS"))
        rows.append(("Flex gap medium", "**16**", "padding"))
        rows.append(("Flex gap large", "**24**", "paddingLG"))
    if "spaceGapSmallSize: token.paddingXS" in text:
        rows.append(("Space size small", "**8**", "paddingXS"))
        rows.append(("Space size middle", "**16**", "padding"))
        rows.append(("Space size large", "**24**", "paddingLG"))
    if "horizontalItemGutter: 32" in text:
        rows.append(("Tabs 水平 gutter", "**32**", "horizontalItemGutter 固定"))
    if "cardHeight" in text and slug == "tabs":
        rows.append(("Tabs cardHeight", "**40**", "controlHeightLG"))
    if "marginBlock: token.marginXS" in text and slug == "divider":
        rows.append(("Divider size=small marginBlock", "**8**", "marginXS"))
    if "visibilityHeight = 400" in text or "visibilityHeight = 400" in text:
        rows.append(("BackTop visibilityHeight", "**400**", "API 默认"))
    # always add control height ladder for form-like
    return rows


def frontmatter_title(text: str, slug: str) -> tuple[str, str]:
    title = pascal(slug)
    subtitle = ""
    m = re.search(r"^title:\s*(.+)$", text, re.M)
    if m:
        title = m.group(1).strip()
    m = re.search(r"^subtitle:\s*(.+)$", text, re.M)
    if m:
        subtitle = m.group(1).strip()
    m = re.search(r"^description:\s*(.+)$", text, re.M)
    desc = m.group(1).strip() if m else ""
    return f"{title} {subtitle}".strip(), desc


def category_of(slug: str) -> str:
    # rough from docs index
    cats = {
        "button": "通用",
        "float-button": "通用",
        "icon": "通用",
        "typography": "通用",
        "divider": "布局",
        "flex": "布局",
        "grid": "布局",
        "layout": "布局",
        "masonry": "布局",
        "space": "布局",
        "splitter": "布局",
        "anchor": "导航",
        "breadcrumb": "导航",
        "dropdown": "导航",
        "menu": "导航",
        "pagination": "导航",
        "steps": "导航",
        "tabs": "导航",
    }
    data_entry = {
        "auto-complete",
        "cascader",
        "checkbox",
        "color-picker",
        "date-picker",
        "form",
        "input",
        "input-number",
        "mentions",
        "radio",
        "rate",
        "select",
        "slider",
        "switch",
        "time-picker",
        "transfer",
        "tree-select",
        "upload",
    }
    data_display = {
        "avatar",
        "badge",
        "calendar",
        "card",
        "carousel",
        "collapse",
        "descriptions",
        "empty",
        "image",
        "list",
        "popover",
        "qr-code",
        "segmented",
        "statistic",
        "table",
        "tag",
        "timeline",
        "tooltip",
        "tour",
        "tree",
    }
    feedback = {
        "alert",
        "drawer",
        "message",
        "modal",
        "notification",
        "popconfirm",
        "progress",
        "result",
        "skeleton",
        "spin",
        "watermark",
    }
    if slug in cats:
        return cats[slug]
    if slug in data_entry:
        return "数据录入"
    if slug in data_display:
        return "数据展示"
    if slug in feedback:
        return "反馈"
    return "其他"


def kind(slug: str) -> str:
    """Interaction kind for state machine templates."""
    if slug in (
        "button",
        "float-button",
        "switch",
        "checkbox",
        "radio",
        "rate",
        "segmented",
        "tag",
    ):
        return "toggle_or_click"
    if slug in (
        "input",
        "input-number",
        "mentions",
        "auto-complete",
        "select",
        "cascader",
        "tree-select",
        "date-picker",
        "time-picker",
        "color-picker",
        "slider",
        "upload",
        "form",
        "transfer",
    ):
        return "input_or_picker"
    if slug in (
        "modal",
        "drawer",
        "popover",
        "tooltip",
        "popconfirm",
        "dropdown",
        "tour",
        "image",
    ):
        return "overlay"
    if slug in ("message", "notification", "alert", "result", "progress", "spin", "skeleton"):
        return "feedback"
    if slug in ("menu", "tabs", "steps", "pagination", "anchor", "breadcrumb"):
        return "nav"
    if slug in (
        "divider",
        "flex",
        "grid",
        "layout",
        "space",
        "splitter",
        "masonry",
    ):
        return "layout"
    if slug in ("table", "list", "tree", "descriptions", "card", "collapse", "calendar", "timeline", "carousel"):
        return "data"
    return "display"


def state_machine(slug: str, pref: str) -> tuple[str, list[tuple[str, str, str]]]:
    k = kind(slug)
    if k == "toggle_or_click":
        ascii_ = f"""```text
                     ┌──────────────┐
                     │   disabled   │◄──── SetDisabled(true) / loading 吞交互*
                     └──────▲───────┘
                            │
  mount ──► default ──hover──► hovered ──press──► pressed ──release/in-bounds──► action
               │                  │                    │
               │                  └── leave ───────────┘
               ├── focus ── Space/Enter ──► action
               └── 受控 value/checked/open（适用者）──► 外部值优先
```
"""
        rules = [
            (f"{pref}-S1", "disabled=true", "不触发主回调；禁用皮"),
            (f"{pref}-S2", "press 界内 release / 键盘激活", "主回调 **一次**"),
            (f"{pref}-S3", "press 后拖出界外 release", "**不**触发主回调"),
            (f"{pref}-S4", "受控值/checked", "外部优先，经 onChange 上抛"),
            (f"{pref}-S5", "主题/type/size 切换", "下一帧 chrome 正确"),
            (f"{pref}-S6", "loading（适用者）", "吞重复激活；有指示"),
            (f"{pref}-S7", "聚焦 + Space/Enter", "与可访问控件一致"),
            (f"{pref}-S8", "仅图标无文案（适用者）", "必须 AriaLabel"),
        ]
        return ascii_, rules
    if k == "input_or_picker":
        ascii_ = f"""```text
mount ──► idle ──focus──► focused ──input/pick──► editing ──blur/confirm──► commit(onChange)
             │                 │
             ├── open 弹层（适用者）──► selecting ──Esc/外点──► closed
             ├── allowClear ──► value 空 + onChange
             ├── disabled / readOnly ──► 无变更
             └── status=error|warning ──► 语义边框（默认不阻断输入）
```
"""
        rules = [
            (f"{pref}-S1", "受控 value", "外部优先；键入/选择经 onChange"),
            (f"{pref}-S2", "disabled", "不可改、无变更回调"),
            (f"{pref}-S3", "allowClear（适用者）", "清空并 onChange"),
            (f"{pref}-S4", "弹层 open（适用者）", "受控 open/onOpenChange"),
            (f"{pref}-S5", "Esc 关闭弹层（适用者）", "关闭且不误提交"),
            (f"{pref}-S6", "键盘主路径", "Enter/方向键按控件语义"),
            (f"{pref}-S7", "status=error", "错误皮仍可输入/打开"),
            (f"{pref}-S8", "size 切换", "高度 24/32/40 档正确"),
        ]
        return ascii_, rules
    if k == "overlay":
        ascii_ = f"""```text
closed ──open=true / trigger──► opening ──► open
  ▲                                      │
  │                                      ├── mask/外点（若允许）──► onOpenChange(false)
  │                                      ├── Esc（若 keyboard）──► close
  │                                      └── 确认/OK（适用者）──► onOk/onConfirm → close
  └── closed ◄── afterClose ◄── closing ◄┘
destroyOnHidden：关闭后可卸载内容。
```
"""
        rules = [
            (f"{pref}-S1", "open=true", "显示浮层；焦点策略见 a11y"),
            (f"{pref}-S2", "onCancel/Esc/mask", "按配置关闭"),
            (f"{pref}-S3", "受控 open", "外部 false 时不得保持展开"),
            (f"{pref}-S4", "placement", "相对触发器位置正确"),
            (f"{pref}-S5", "destroyOnHidden", "关闭卸载（若开启）"),
            (f"{pref}-S6", "disabled 触发器", "不打开"),
            (f"{pref}-S7", "确认路径（适用者）", "onOk/onConfirm 一次"),
            (f"{pref}-S8", "堆叠 z-index", "多层有序"),
        ]
        return ascii_, rules
    if k == "feedback":
        ascii_ = f"""```text
trigger ──► visible（入场）──duration──► leave ──► destroy
             ├── closable ──► 立即关闭
             ├── 同 key 更新 ──► 替换内容不新开（message/notification）
             └── 堆叠：同 placement 排列
```
"""
        rules = [
            (f"{pref}-S1", "打开/触发", "可见且类型图标正确"),
            (f"{pref}-S2", "duration", "到期自动关闭（0=不自动关）"),
            (f"{pref}-S3", "手动关闭", "onClose"),
            (f"{pref}-S4", "key 更新（适用者）", "不重复堆一条"),
            (f"{pref}-S5", "percent/spinning（适用者）", "指示正确"),
            (f"{pref}-S6", "reduced-motion", "动画可关"),
        ]
        return ascii_, rules
    if k == "nav":
        ascii_ = f"""```text
mount ──► items 渲染
             ├── 选中 selectedKeys / activeKey / current ──► 高亮 + onChange/onSelect
             ├── 展开 openKeys（适用者）──► 子面板
             ├── disabled item ──► 不可选
             └── 键盘：方向键 / Home/End / Enter（适用者）
```
"""
        rules = [
            (f"{pref}-S1", "选中主路径", "高亮 + 回调"),
            (f"{pref}-S2", "受控 keys", "外部优先"),
            (f"{pref}-S3", "disabled 项", "不可选"),
            (f"{pref}-S4", "展开/分页/步骤（适用者）", "状态与回调正确"),
            (f"{pref}-S5", "键盘导航", "主路径可达"),
            (f"{pref}-S6", "主题/mode 切换", "chrome 正确"),
        ]
        return ascii_, rules
    if k == "layout":
        ascii_ = f"""```text
mount ──► 按 orientation/gap/span/panels 布局子节点
             ├── 尺寸变化 / 断点 ──► 重布局
             ├── Splitter 拖动 ──► onResize（适用者）
             └── 无「提交」类状态（Divider/Space/Flex 等静态为主）
```
"""
        rules = [
            (f"{pref}-S1", "主布局方向/参数", "子项位置正确"),
            (f"{pref}-S2", "gap/gutter/size", "间距符合 §6.2"),
            (f"{pref}-S3", "响应式/折叠（适用者）", "断点或 collapsed 生效"),
            (f"{pref}-S4", "拖拽夹紧 min/max（适用者）", "不越界"),
            (f"{pref}-S5", "主题切换", "线色/底色更新"),
        ]
        return ascii_, rules
    if k == "data":
        ascii_ = f"""```text
mount ──► dataSource/treeData/items 渲染
             ├── 选择/展开/排序/分页 ──► 对应 onChange
             ├── loading ──► 遮罩或 Skeleton
             └── 空数据 ──► Empty
```
"""
        rules = [
            (f"{pref}-S1", "数据渲染", "行/节点数量正确"),
            (f"{pref}-S2", "交互变更", "回调参数完整"),
            (f"{pref}-S3", "loading", "有反馈且可恢复"),
            (f"{pref}-S4", "空态", "Empty 或空列表"),
            (f"{pref}-S5", "受控 keys/value", "外部优先"),
            (f"{pref}-S6", "禁用行/节点", "不可交互"),
        ]
        return ascii_, rules
    # display
    ascii_ = f"""```text
mount ──► 展示内容
             ├── 交互子集：copy / expand / preview / close（适用者）
             └── 主题切换 ──► 色/字号更新
```
"""
    rules = [
        (f"{pref}-S1", "默认渲染", "不崩溃；内容可见"),
        (f"{pref}-S2", "type/status/size（适用者）", "语义正确"),
        (f"{pref}-S3", "交互子集", "回调一次"),
        (f"{pref}-S4", "a11y 名（有意义时）", "可读"),
    ]
    return ascii_, rules


def default_metrics(slug: str, hints: list[tuple[str, str, str]]) -> list[tuple[str, str, str]]:
    rows = list(hints)
    k = kind(slug)
    # baseline always
    base = [
        ("字号 middle", f"**{SEED['fontSize']}**", "`fontSize`"),
        ("圆角", f"**{SEED['borderRadius']}**", "`borderRadius`"),
        ("边框线宽", f"**{SEED['lineWidth']}**", "`lineWidth`"),
        ("Focus ring outset", "≈ **1.5px** 可见", "可调，必须可见"),
    ]
    if k in ("input_or_picker", "toggle_or_click", "nav") or slug in (
        "select",
        "input",
        "pagination",
        "tabs",
        "menu",
    ):
        base = [
            ("控件高度 middle", f"**{SEED['controlHeight']}**", "`controlHeight`"),
            ("控件高度 small", f"**{SEED['controlHeightSM']}**", "`controlHeightSM`"),
            ("控件高度 large", f"**{SEED['controlHeightLG']}**", "`controlHeightLG`"),
        ] + base
    # slug specific hard numbers
    extra = {
        "modal": [("默认 width", "**520**", "API 默认")],
        "drawer": [("默认 width", "**378**", "API 默认")],
        "qr-code": [("默认边长", "**160**", "API size 默认")],
        "avatar": [("默认 size", "**32**", "默认尺寸")],
        "float-button": [
            ("边长", "**40**", "controlHeightLG"),
            ("贴边 right/bottom", "**24 / 48**", "marginLG / marginXXL"),
        ],
        "layout": [
            ("Header 高", "**64**", "controlHeight×2"),
            ("Sider 宽", "**200**", "组件默认"),
            ("collapsedWidth", "**80**", "常见默认"),
        ],
        "progress": [("线高", "**8**", "line stroke"), ("环默认", "**120**", "circle size")],
        "table": [("行高 middle 约", "**55**", "实现按 size/token")],
        "badge": [("overflowCount", "**99**", "API 默认")],
        "rate": [("默认 count", "**5**", "API")],
        "watermark": [("rotate 默认", "**-22**", "API")],
        "notification": [("duration 默认", "**4.5s**", "API"), ("宽约", "**384**", "实现/token")],
        "message": [("duration 默认", "**3s**", "API")],
        "grid": [("列数", "**24**", "gridColumns")],
        "typography": [
            ("Title h1..h5", "**38/30/24/20/16**", "fontSizeHeading*"),
            ("正文", "**14**", "fontSize"),
        ],
        "icon": [("默认", "**1em / 14–16px**", "随字号或显式 size")],
        "divider": [
            ("水平 margin 默认", "**24**", "marginLG"),
            ("size=small margin", "**8**", "marginXS"),
        ],
        "space": [
            ("size small 默认", "**8**", "paddingXS"),
            ("middle/large", "**16 / 24**", "padding / paddingLG"),
        ],
        "flex": [
            ("gap s/m/l", "**8 / 16 / 24**", "paddingXS/padding/paddingLG"),
        ],
        "switch": [
            ("默认轨", "**≈44×22**", "prepareComponentToken 公式"),
            ("small 轨", "**≈28×16**", "SM 公式"),
        ],
        "checkbox": [("指示器", "**16×16**", "controlInteractiveSize")],
        "radio": [("圆点", "**16**", "interactive"), ("Button 高", "**32/24/40**", "controlHeight*")],
        "input": [
            ("高 s/m/l", "**24/32/40**", "controlHeight*"),
            ("paddingInline middle", "**≈11**", "paddingSM−lineWidth"),
        ],
        "menu": [("itemHeight", "**40**", "controlHeightLG")],
        "pagination": [("itemSize", "**32/24/40**", "controlHeight*")],
        "tabs": [("cardHeight", "**40**", "controlHeightLG"), ("itemGutter", "**32**", "固定")],
        "select": [("选择器高", "**32/24/40**", "controlHeight*"), ("listHeight", "**256**", "常见默认")],
    }
    # merge unique by first col
    seen = {r[0] for r in rows}
    for r in extra.get(slug, []) + base:
        if r[0] not in seen:
            rows.append(r)
            seen.add(r[0])
    return rows


def build_sec6(slug: str) -> str:
    if slug == "button":
        raise SystemExit("refusing to overwrite button.md sample")

    zh = read_zh(slug)
    title, desc = frontmatter_title(zh, slug) if zh else (pascal(slug), "")
    export = pascal(slug)
    pref = prefix_of(slug)
    demo_list = demos(zh) if zh else []
    props = api_props(zh) if zh else []
    hints = extract_token_hints(slug)
    metrics = default_metrics(slug, hints)
    ascii_sm, rules = state_machine(slug, pref)
    cat = category_of(slug)
    k = kind(slug)
    folder = FOLDER.get(slug, slug)
    src_path = f"/home/yanghy/app/projects/ant-design/components/{folder}/"

    # L1 meaning
    l1_map = {
        "toggle_or_click": "点击/切换、禁用、键盘激活、受控值正确",
        "input_or_picker": "受控输入/选择、弹层、清除、校验 status、尺寸档",
        "overlay": "开合、遮罩/Esc、placement、确认/取消主路径",
        "feedback": "展示/自动关闭/堆叠/类型语义",
        "nav": "选中/展开/分页或步骤切换与键盘",
        "layout": "布局参数驱动子项几何正确",
        "data": "数据渲染与选择/展开/分页/加载主路径",
        "display": "展示形态与可选交互（复制/预览/关闭）",
    }
    l1 = l1_map.get(k, "主路径行为与 antd 对齐")

    # props table top 16
    prop_rows = props[:16]
    if not prop_rows:
        prop_rows = [
            ("（见 §3）", "以文档 API 表为准", "", ""),
        ]

    # P0 from props names + kind
    p0 = []
    important = [
        "value",
        "defaultValue",
        "checked",
        "onChange",
        "onClick",
        "disabled",
        "loading",
        "size",
        "type",
        "variant",
        "status",
        "open",
        "onOpenChange",
        "options",
        "items",
        "treeData",
        "dataSource",
        "columns",
        "fileList",
        "children",
        "title",
        "content",
        "placement",
        "trigger",
        "allowClear",
        "showSearch",
        "mode",
        "orientation",
        "shape",
        "icon",
        "percent",
        "spinning",
    ]
    names = {p[0] for p in props}
    for n in important:
        if n in names:
            p0.append(n)
    if not p0:
        p0 = [p[0] for p in props[:8]] or ["主路径 API（§3）", "Theme Token", "基础 a11y"]

    p1 = [
        "semantic classNames/styles 深度",
        "动画像素级 / 复杂虚拟列表",
        "浏览器-only API 或桌面无等价项",
        "debug 示例与官网逐像素哈希",
    ]

    # demos for cases
    user_demos = [(s, t) for s, t, d in demo_list if not d][:12]

    # chrome table by kind
    if k == "input_or_picker":
        chrome = """| 态 / 变体 | 规则 |
| --- | --- |
| default | 容器底 + 边框（outlined）或族默认皮；Token 色 |
| hover | 边框/底强调 |
| focus | **可见** focus ring；主色边 |
| disabled | 降对比；不可编辑 |
| status=error/warning | 语义色边框/反馈 |
| 弹层 open | elevation 阴影；与触发器对齐 placement |
"""
    elif k == "overlay":
        chrome = """| 态 | 规则 |
| --- | --- |
| mask | `colorBgMask` 半透明（适用者） |
| panel/popup | 容器底 + 阴影 + 圆角 LG |
| open/close | 动画可关 / reduced-motion |
| disabled 触发 | 触发器禁用皮，不打开 |
"""
    elif k == "toggle_or_click":
        chrome = """| 态 | 规则 |
| --- | --- |
| default | Token 默认皮 |
| hover / active | 可交互反馈 |
| focus | 可见 focus ring |
| checked/selected/active（适用者） | 主色强调 |
| disabled | 降对比；无 hover |
| loading | 指示器；防重复 |
"""
    else:
        chrome = """| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token |
| hover/active/focus | 可交互者具备反馈与 focus ring |
| disabled / loading / empty | 按本控件语义 |
| 主题切换 | 色与间距随 Theme 更新 |
"""

    a11y_by_k = {
        "toggle_or_click": [
            ("角色", "button / checkbox / switch / radio 等与语义一致"),
            ("名称", "可交互必有名；仅图标必须 AriaLabel"),
            ("焦点", "Tab 可达；ring 可见"),
            ("键盘", "Space/Enter 或方向键按角色"),
            ("禁用", "不可激活；读屏可感知（平台支持时）"),
        ],
        "input_or_picker": [
            ("角色", "textbox / combobox / spinbutton / listbox 等"),
            ("标签", "与 Form.Item label 或 aria-labelledby 关联"),
            ("清除/下拉", "控件有可访问名称"),
            ("错误", "status=error 时暴露 invalid"),
            ("键盘", "主路径可选/提交/关闭"),
        ],
        "overlay": [
            ("角色", "dialog / menu / tooltip 等"),
            ("焦点", "打开进入浮层；关闭回触发器（可配）"),
            ("Esc", "关闭（若允许）"),
            ("标题", "Dialog 必须有可访问名"),
            ("遮罩", "点击策略明确"),
        ],
        "feedback": [
            ("实时区域", "message/notification 用 status 语义等价"),
            ("关闭", "可关控件可操作"),
            ("不抢焦点", "轻提示默认不抢（Modal 例外）"),
        ],
        "nav": [
            ("角色", "navigation / menu / tablist 等"),
            ("当前", "aria-current / selected"),
            ("键盘", "方向键与激活"),
        ],
        "layout": [
            ("装饰分隔", "纯装饰可 aria-hidden"),
            ("拖拽把手", "可命名；键盘微调 P0/P1 按控件"),
        ],
        "data": [
            ("表格/树/列表", "结构角色与展开/选中态可读"),
            ("排序/筛选", "控件有名"),
        ],
        "display": [
            ("装饰图", "alt 或 aria-hidden"),
            ("有意义操作", "复制/关闭/展开有名"),
        ],
    }
    a11y = a11y_by_k.get(k, a11y_by_k["display"])

    # cases
    cases = []
    n = 1
    # default create
    cases.append(
        (
            f"{pref}-{n:02d}",
            "L1",
            f"创建默认 {export}",
            "不崩溃；默认值符合 §6.10",
        )
    )
    n += 1
    for src, title in user_demos[:10]:
        cases.append(
            (
                f"{pref}-{n:02d}",
                "L1",
                f"复现官方示例「{title}」（`{src}`）",
                "行为与主视觉符合 §6；P0 路径必须通过",
            )
        )
        n += 1
    # standard
    std = [
        ("L1", "disabled 或等价只读（适用者）", "主回调不触发"),
        ("L1", "受控模式（适用者）", "外部值优先"),
        ("L2", "§6.2 关键尺寸/间距 Token", "与表内数字一致（±0.5px 或文档容差）"),
        ("L2", "无硬编码品牌色默认皮", "色来自 Theme/Token"),
        ("L1", "键盘/焦点（适用者）", "可聚焦者 ring 可见；主路径有键盘等价"),
        ("L3", "关键态 golden 截图", "与仓库基线一致（AA 容差）"),
        ("L4", "与 ant.design 并排", "人眼签字记录"),
    ]
    for level, step, exp in std:
        cases.append((f"{pref}-{n:02d}", level, step, exp))
        n += 1
    # from rules
    for rid, rule, exp in rules[:6]:
        cases.append((rid.replace("-S", "-R"), "L1", rule, exp))

    # API lines
    api = f"""New{export}(...) *{export}

// 配置：对 §6.3 / §3 中 P0 字段提供 SetXxx
// 回调：OnChange / OnClick / OnOpenChange / OnConfirm … 按 API
// 状态：SetDisabled / SetLoading（适用者）
// 主题：SetTheme(*Theme)；Style 可选覆盖
// a11y：SetAriaLabel / 焦点与键盘
// 挂树：Node() core.Node
"""
    defaults = [
        ("Disabled", "false"),
        ("Size（适用者）", "middle / 控件默认"),
        ("受控值", "未 Set 时用 default* 或零值"),
    ]

    # structure
    structures = {
        "toggle_or_click": "Pressable\n  └─ Decorated chrome\n       └─ content (icon/label/indicator)",
        "input_or_picker": "Field / Selector\n  ├─ prefix?\n  ├─ editable / display value\n  ├─ clear? / suffix?\n  └─ Portal popup? (list/panel)",
        "overlay": "Trigger?\n  └─ Portal\n       ├─ mask?\n       └─ panel / popup (+ arrow?)",
        "feedback": "Host holder or inline\n  └─ item (icon + content + close?)",
        "nav": "Nav root\n  └─ items / panels / connectors",
        "layout": "Layout root\n  └─ children with gap/span/handles",
        "data": "Data view\n  ├─ header?\n  ├─ body rows/nodes\n  └─ pagination/footer?",
        "display": "Display root\n  └─ content (+ actions?)",
    }
    structure = structures.get(k, structures["display"])

    lines: list[str] = []
    lines.append("## 6. 1:1 产品需求增量（gpui 验收规格）\n")
    lines.append(
        f"> 本章把 antd **{export}** 补成 **可开发、可测试、可裁剪** 的产品规格。  \n"
        f"> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；"
        f"**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  \n"
        f"> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  \n"
        f"> 源码：`{src_path}`（`index.zh-CN.md` + `style/` + 组件实现）。\n"
    )

    # 6.1
    lines.append(f"### 6.1 对齐级别定义（{export}）\n")
    lines.append("| 级别 | 名称 | 本控件含义 | 验收方式 |")
    lines.append("| --- | --- | --- | --- |")
    lines.append(f"| **L1** | 行为 | {l1} | Headless / behavior 测试 |")
    lines.append(
        "| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |"
    )
    lines.append(
        "| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |"
    )
    lines.append(
        "| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |"
    )
    lines.append("")
    lines.append(f"**明确不做（{export}）：**\n")
    lines.append("- 与浏览器渲染 ant.design **逐像素哈希**一致。  ")
    lines.append("- 为抠图破坏 `hit == layout == paint` 边界。  ")
    lines.append("- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  ")
    lines.append("- 官方 **debug** 示例不计入 P0 验收。  ")
    if desc:
        lines.append(f"\n> 控件说明：{desc}\n")

    # 6.2
    lines.append("### 6.2 度量与 Design Token（L2 基线）\n")
    lines.append(
        "数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，"
        f"常用种子：`controlHeight={SEED['controlHeight']}`、`fontSize={SEED['fontSize']}`）。"
        "实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。\n"
    )
    lines.append("#### 6.2.1 几何与组件 Token\n")
    lines.append("| 项 | 默认值 | Token / 来源 |")
    lines.append("| --- | --- | --- |")
    for item, val, tok in metrics:
        lines.append(f"| {item} | {val} | {tok} |")
    lines.append("")
    lines.append("#### 6.2.2 颜色 Token（语义）\n")
    lines.append("| 用途 | Token 建议 | 备注 |")
    lines.append("| --- | --- | --- |")
    lines.append("| 主色 / hover / active | `colorPrimary` + 变体 | 强调、选中、开态 |")
    lines.append("| 错误 / 成功 / 警告 | `colorError` / `Success` / `Warning` | status 与反馈 |")
    lines.append("| 文本 / 次级文本 | `colorText` / `colorTextSecondary` | |")
    lines.append("| 边框 / 分割 / 容器底 | `colorBorder` / `colorSplit` / `colorBgContainer` | |")
    lines.append("| 禁用 | `colorDisabledBg` / `colorDisabledText` | 无 hover 高亮 |")
    lines.append("| 浮层阴影 / 遮罩 | `boxShadowSecondary` / `colorBgMask` | 适用者 |")
    lines.append("")
    lines.append("禁止硬编码品牌色作为唯一默认皮。\n")

    # 6.3
    lines.append("### 6.3 关键配置与语义\n")
    lines.append(
        "下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。"
        f"分类：**{cat}**。\n"
    )
    lines.append("| 配置 | 说明 | 类型（摘录） | 默认 |")
    lines.append("| --- | --- | --- | --- |")
    for name, desc, typ, default in prop_rows:
        d = desc if len(desc) <= 60 else desc[:57] + "…"
        t = typ if len(typ) <= 40 else typ[:37] + "…"
        lines.append(f"| `{name}` | {d} | {t} | {default or '—'} |")
    lines.append("")
    lines.append("**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。\n")

    # 6.4
    lines.append("### 6.4 交互状态机（L1）\n")
    lines.append(ascii_sm)
    lines.append("| 规则 ID | 规则 | 期望 |")
    lines.append("| --- | --- | --- |")
    for rid, rule, exp in rules:
        lines.append(f"| {rid} | {rule} | {exp} |")
    lines.append("")

    # 6.5
    lines.append("### 6.5 视觉 chrome 规则（L2 摘要）\n")
    lines.append(chrome)
    lines.append(
        "\n**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。\n"
    )

    # 6.6
    lines.append("### 6.6 无障碍（a11y）最低要求\n")
    lines.append("| 项 | 要求 |")
    lines.append("| --- | --- |")
    for a, b in a11y:
        lines.append(f"| {a} | {b} |")
    lines.append("")

    # 6.7
    lines.append("### 6.7 平台边界（gpui vs 浏览器 antd）\n")
    lines.append("| 能力 | 策略 | 级别 |")
    lines.append("| --- | --- | --- |")
    lines.append(f"| 主路径行为（§6.1 L1） | **对等** | P0 L1 |")
    lines.append("| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |")
    lines.append("| 动画/波纹/CSS 特效 | **近似**或瞬时 | P1 |")
    lines.append("| IME/剪贴板/滚动宿主（适用者） | **宿主** | P0 宿主 |")
    lines.append("| 浏览器-only API | **映射**或 P1 不做 | P1 |")
    lines.append("| Semantic classNames/styles | kit 语义钩子 | P1 |")
    lines.append("| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |")
    lines.append("| 逐像素官网哈希 | **不做** | — |")
    lines.append("")

    # 6.8
    lines.append("### 6.8 能力裁剪（P0 / P1）\n")
    lines.append("#### P0（本阶段必须 1:1，否则不算完成）\n")
    lines.append("| 配置 / 能力 | 说明 |")
    lines.append("| --- | --- |")
    for p in p0:
        lines.append(f"| `{p}` | 必须 |")
    # demos p0
    if user_demos:
        demo_names = "、".join(t for _, t in user_demos[:8])
        lines.append(f"| 官方主路径示例 | {demo_names} |")
    lines.append("| 度量 §6.2 | Token 断言 |")
    lines.append("| a11y §6.6 | 最低要求 |")
    lines.append(f"| §6.9 中 L1/L2 用例 | 测试通过 |")
    lines.append("")
    lines.append("#### P1（可 later，须在 coverage Notes 写明）\n")
    lines.append("| 配置 / 能力 | 说明 |")
    lines.append("| --- | --- |")
    for p in p1:
        lines.append(f"| {p} | 分期 |")
    # remaining demos
    if len(user_demos) > 8:
        lines.append(
            f"| 其余示例 | {', '.join(t for _, t in user_demos[8:])} |"
        )
    lines.append("")

    # 6.9
    lines.append("### 6.9 验收用例表（可测）\n")
    lines.append(
        f"> 测试名建议：`Test{export}_PRD_<ID>` 或 gallery 场景 ID。  \n"
        f"> **P0 相关 L1/L2 用例全部通过** 才可宣称 {export} 完成 1:1 主路径。\n"
    )
    lines.append("| ID | 级别 | 步骤 | 期望 |")
    lines.append("| --- | --- | --- | --- |")
    for cid, level, step, exp in cases:
        lines.append(f"| {cid} | {level} | {step} | {exp} |")
    lines.append("")

    # 6.10
    lines.append("### 6.10 产品 API 契约（Go kit 侧）\n")
    lines.append(
        "> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。\n"
    )
    lines.append("```text")
    lines.append(api.rstrip())
    lines.append("```")
    lines.append("")
    lines.append("**默认值（未 Set 时）：**\n")
    lines.append("| 字段 | 默认 |")
    lines.append("| --- | --- |")
    for k_, v_ in defaults:
        lines.append(f"| {k_} | {v_} |")
    lines.append("| 其余 | 对齐 antd 6.5 §3 表 |")
    lines.append("")

    # 6.11
    lines.append("### 6.11 结构与绘制分层（实现提示）\n")
    lines.append("```text")
    lines.append(structure)
    lines.append("```")
    lines.append("")
    lines.append("- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  ")
    lines.append("- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  ")
    lines.append("- 命中区域与布局盒一致（`hit == layout == paint`）。  ")
    lines.append("- 动画跟随 Host Tick；尊重 reduced-motion。  ")
    lines.append("")

    # 6.12
    lines.append("### 6.12 完成定义（DoD）\n")
    lines.append(f"同时满足即可宣布 **{export} 主路径 1:1 完成**：\n")
    lines.append("1. §6.8 **P0** 全部实现。  ")
    lines.append("2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  ")
    lines.append("3. L2 度量与 Token 断言通过（§6.2 关键数字）。  ")
    lines.append("4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  ")
    lines.append("5. gallery 展示主路径（对照官方非 debug 示例与 P0）。  ")
    lines.append(
        f"6. `coverage.go` Notes：P0 已对齐 `docs/antd/{slug}.md` §6；P1 显式列出。  "
    )
    lines.append("")
    lines.append("---")
    lines.append("")
    lines.append(
        f"**本章用法**：实现 `ui/kit` {export} 时以 **§6 为需求与验收**；"
        f"§1–§3 为 antd 能力全集；§6.8 为范围裁剪。"
        f"细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。"
    )
    lines.append("")
    return "\n".join(lines)


def replace_sec6(slug: str, force: bool = False) -> str:
    path = OUT / f"{slug}.md"
    if not path.exists():
        return f"skip missing {slug}"
    if slug == "button":
        return "skip button"
    t = path.read_text(encoding="utf-8")
    if not force and slug in ("float-button",):
        # still rewrite if thin? float-button is 425 lines - keep
        sec = t[t.index("## 6.") :] if "## 6." in t else ""
        if sec.count("\n") >= 250 and "见上文同级说明" not in sec:
            return f"keep deep {slug} ({sec.count(chr(10))+1} lines)"

    sec6 = build_sec6(slug)
    if "\n## 6. " in t:
        head = t[: t.index("\n## 6. ") + 1]
    elif t.startswith("## 6. "):
        head = ""
    else:
        head = t.rstrip() + "\n\n---\n\n"

    intro = (
        "**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ "
        "[§6](#6-11-产品需求增量gpui-验收规格)**。"
        "手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。"
    )
    head = re.sub(r"\*\*1:1 产品验收[^*]*\*\*[^\n]*", intro, head, count=1)
    path.write_text(head + sec6, encoding="utf-8")
    n = sec6.count("\n") + 1
    return f"{slug}: §6={n}"


def main() -> None:
    force = "--force" in sys.argv
    args = [a for a in sys.argv[1:] if not a.startswith("-")]
    if args:
        slugs = args
    else:
        slugs = sorted(
            p.stem
            for p in OUT.glob("*.md")
            if not p.name.startswith("_") and p.stem not in ("README",)
        )
    for slug in slugs:
        try:
            print(replace_sec6(slug, force=force))
        except Exception as e:
            print(f"ERR {slug}: {e}")
    # verify
    bad = []
    for p in OUT.glob("*.md"):
        if p.name.startswith("_") or p.name == "README.md":
            continue
        t = p.read_text(encoding="utf-8")
        if "## 6." not in t:
            bad.append((p.stem, "no sec6"))
            continue
        sec = t[t.index("## 6.") :]
        for i in range(1, 13):
            if f"### 6.{i}" not in sec:
                bad.append((p.stem, f"miss 6.{i}"))
        if "见上文同级说明" in sec:
            bad.append((p.stem, "filler"))
        if sec.count("\n") + 1 < 150 and p.stem != "util":
            # util can be shorter but we still aim high
            if p.stem not in ("util",):
                bad.append((p.stem, f"short {sec.count(chr(10))+1}"))
    print("verify bad:", len(bad))
    for b in bad[:30]:
        print(" ", b)


if __name__ == "__main__":
    main()
