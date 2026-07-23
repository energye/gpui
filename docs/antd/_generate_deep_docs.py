#!/usr/bin/env python3
"""Generate Button-level Ant Design 6.5.x component docs for gpui kit development."""

from __future__ import annotations

import re
from pathlib import Path

SRC = Path("/tmp/antd-docs-src")
OUT = Path("/home/yanghy/app/projects/gogpu/gpui/docs/antd")

# component slug -> (url path, preserve_if_already_handcrafted)
# We regenerate ALL except keep structure consistent; button-quality for all.

CATEGORY_MAP = {
    "button": "通用（General）",
    "float-button": "通用（General）",
    "icon": "通用（General）",
    "typography": "通用（General）",
    "divider": "布局（Layout）",
    "flex": "布局（Layout）",
    "grid": "布局（Layout）",
    "layout": "布局（Layout）",
    "masonry": "布局（Layout）",
    "space": "布局（Layout）",
    "splitter": "布局（Layout）",
    "anchor": "导航（Navigation）",
    "breadcrumb": "导航（Navigation）",
    "dropdown": "导航（Navigation）",
    "menu": "导航（Navigation）",
    "pagination": "导航（Navigation）",
    "steps": "导航（Navigation）",
    "tabs": "导航（Navigation）",
    "auto-complete": "数据录入（Data Entry）",
    "cascader": "数据录入（Data Entry）",
    "checkbox": "数据录入（Data Entry）",
    "color-picker": "数据录入（Data Entry）",
    "date-picker": "数据录入（Data Entry）",
    "form": "数据录入（Data Entry）",
    "input": "数据录入（Data Entry）",
    "input-number": "数据录入（Data Entry）",
    "mentions": "数据录入（Data Entry）",
    "radio": "数据录入（Data Entry）",
    "rate": "数据录入（Data Entry）",
    "select": "数据录入（Data Entry）",
    "slider": "数据录入（Data Entry）",
    "switch": "数据录入（Data Entry）",
    "time-picker": "数据录入（Data Entry）",
    "transfer": "数据录入（Data Entry）",
    "tree-select": "数据录入（Data Entry）",
    "upload": "数据录入（Data Entry）",
    "avatar": "数据展示（Data Display）",
    "badge": "数据展示（Data Display）",
    "calendar": "数据展示（Data Display）",
    "card": "数据展示（Data Display）",
    "carousel": "数据展示（Data Display）",
    "collapse": "数据展示（Data Display）",
    "descriptions": "数据展示（Data Display）",
    "empty": "数据展示（Data Display）",
    "image": "数据展示（Data Display）",
    "list": "数据展示（Data Display）",
    "popover": "数据展示（Data Display）",
    "qr-code": "数据展示（Data Display）",
    "segmented": "数据展示（Data Display）",
    "statistic": "数据展示（Data Display）",
    "table": "数据展示（Data Display）",
    "tag": "数据展示（Data Display）",
    "timeline": "数据展示（Data Display）",
    "tooltip": "数据展示（Data Display）",
    "tour": "数据展示（Data Display）",
    "tree": "数据展示（Data Display）",
    "alert": "反馈（Feedback）",
    "drawer": "反馈（Feedback）",
    "message": "反馈（Feedback）",
    "modal": "反馈（Feedback）",
    "notification": "反馈（Feedback）",
    "popconfirm": "反馈（Feedback）",
    "progress": "反馈（Feedback）",
    "result": "反馈（Feedback）",
    "skeleton": "反馈（Feedback）",
    "spin": "反馈（Feedback）",
    "watermark": "反馈（Feedback）",
    "affix": "其他（Other）",
    "app": "其他（Other）",
    "border-beam": "其他（Other）",
    "config-provider": "其他（Other）",
    "util": "其他（Other）",
}

# Props that map to appearance subsections
APPEARANCE_PROP_HINTS = {
    "type": "类型（type）",
    "variant": "形态变体（variant）",
    "color": "颜色（color）",
    "size": "尺寸（size）",
    "shape": "形状（shape）",
    "status": "状态色（status）",
    "disabled": "禁用态（disabled）",
    "loading": "加载态（loading）",
    "danger": "危险态（danger）",
    "ghost": "幽灵/透明态（ghost）",
    "block": "块级宽度（block）",
    "bordered": "边框（bordered）",
    "orientation": "方向（orientation）",
    "vertical": "垂直方向（vertical）",
    "placement": "弹出/放置位置（placement）",
    "theme": "主题（theme）",
    "mode": "模式（mode）",
    "layout": "布局（layout）",
    "icon": "图标（icon）",
    "prefix": "前缀（prefix）",
    "suffix": "后缀（suffix）",
    "suffixIcon": "后缀图标（suffixIcon）",
    "allowClear": "清除按钮（allowClear）",
    "showSearch": "搜索框（showSearch）",
    "multiple": "多选外观（multiple）",
    "checked": "选中态（checked）",
    "indeterminate": "半选态（indeterminate）",
    "dot": "红点（dot）",
    "count": "计数徽标（count）",
    "percent": "进度百分比（percent）",
    "spinning": "旋转加载（spinning）",
    "fullscreen": "全屏（fullscreen）",
    "closable": "可关闭（closable）",
    "mask": "遮罩（mask）",
    "open": "显隐（open）",
    "title": "标题（title）",
    "description": "描述（description）",
    "content": "内容（content）",
    "avatar": "头像（avatar）",
    "cover": "封面（cover）",
    "actions": "操作区（actions）",
    "extra": "额外区域（extra）",
    "footer": "页脚（footer）",
    "header": "页头（header）",
    "tabPlacement": "页签位置（tabPlacement）",
    "buttonStyle": "按钮风格（buttonStyle）",
    "optionType": "选项类型（optionType）",
    "listType": "列表类型（listType）",
    "picker": "选择器类型（picker）",
    "showTime": "时间面板（showTime）",
    "treeLine": "树连接线（treeLine）",
    "checkable": "可勾选（checkable）",
    "selectable": "可选中（selectable）",
    "ellipsis": "省略（ellipsis）",
    "bordered": "边框模式（bordered）",
    "card": "卡片模式",
    "ribbon": "缎带",
    "badge": "徽标",
}


def frontmatter(text: str) -> dict:
    m = re.match(r"^---\n(.*?)\n---\n", text, re.S)
    if not m:
        return {}
    data = {}
    for line in m.group(1).splitlines():
        if ":" in line and not line.startswith(" "):
            k, v = line.split(":", 1)
            data[k.strip()] = v.strip().strip("\"'")
    # group title
    gm = re.search(r"group:\s*\n\s*title:\s*(.+)", m.group(1))
    if gm:
        data["group"] = gm.group(1).strip()
    return data


def strip_html(s: str) -> str:
    s = re.sub(r"<[^>]+>", "", s)
    s = s.replace("&lt;", "<").replace("&gt;", ">").replace("&amp;", "&")
    return s.strip()


def section_body(text: str, *titles: str) -> str:
    """Extract body under first matching ## title until next ##."""
    for title in titles:
        # match ## Title or ## Title {#id}
        pat = rf"##\s+{re.escape(title)}[^\n]*\n(.*?)(?=\n##\s|\Z)"
        m = re.search(pat, text, re.S | re.I)
        if m:
            return m.group(1).strip()
    return ""


def all_h2(text: str) -> list[tuple[str, str]]:
    parts = re.split(r"\n##\s+", text)
    out = []
    for p in parts[1:]:
        lines = p.split("\n", 1)
        title = strip_html(re.sub(r"\{#.*?\}", "", lines[0]).strip())
        body = lines[1] if len(lines) > 1 else ""
        # stop at end
        out.append((title, body))
    return out


def demos(text: str) -> list[tuple[str, str, bool]]:
    """Return list of (file, title, is_debug)."""
    items = []
    for m in re.finditer(
        r'<code\s+src="\./demo/([^"]+)"([^>]*)>([^<]*)</code>', text
    ):
        src, attrs, title = m.group(1), m.group(2), m.group(3).strip()
        debug = "debug" in attrs
        if title:
            items.append((src, title, debug))
    return items


def extract_api_full(text: str) -> str:
    """Extract full API section including subsections until Semantic/Token/FAQ/Design."""
    m = re.search(
        r"(##\s+API\b.*?)(?=\n##\s+(?:Semantic DOM|主题变量|Design Token|FAQ|设计指引|注意|注意事项|在 TypeScript|相关推荐)\b|\Z)",
        text,
        re.S | re.I,
    )
    if not m:
        # util-style
        m = re.search(r"(##\s+GetRef\b.*)", text, re.S)
        if m:
            return clean_api_markdown(m.group(1))
        return ""
    return clean_api_markdown(m.group(1))


def clean_api_markdown(api: str) -> str:
    # remove dumi code embeds for semantic demos
    api = re.sub(r"<!--\s*prettier-ignore\s*-->\n?", "", api)
    api = re.sub(r"<code\s+src=\"[^\"]+\"[^>]*>.*?</code>\n?", "", api)
    # expand embed of shared props
    if "<embed" in api:
        shared = Path("/tmp/antd-docs-src/tooltip-shared.md")
        if shared.exists():
            shared_body = shared.read_text(encoding="utf-8")
            api = re.sub(
                r"<embed\s+src=\"[^\"]+\"[^>]*>\n?",
                "\n#### 继承 Tooltip 共同 API\n\n"
                + shared_body
                + "\n",
                api,
            )
        else:
            api = re.sub(
                r"<embed\s+src=\"[^\"]+\"[^>]*>\n?",
                "\n> 另继承 Tooltip 共同 API（trigger / placement / open / arrow / getPopupContainer 等），详见 [Tooltip](./tooltip.md)。\n\n",
                api,
            )
    # ComponentTokenTable
    api = re.sub(
        r"<ComponentTokenTable[^/]*/>",
        "（见官方 Design Token 表）",
        api,
    )
    api = re.sub(r"<Badge>[^<]*</Badge>", "", api)
    # collapse excessive blank lines
    api = re.sub(r"\n{3,}", "\n\n", api)
    return api.strip()


def extract_faq(text: str) -> str:
    m = re.search(r"(##\s+FAQ\b.*?)(?=\n##\s+(?:设计指引|Design Guide)\b|\Z)", text, re.S | re.I)
    if not m:
        return ""
    faq = m.group(1).strip()
    faq = re.sub(r"<[^>]+>", "", faq)
    faq = re.sub(r"\n{3,}", "\n\n", faq)
    return faq


def extract_methods(text: str) -> str:
    bodies = []
    for title, body in all_h2(text):
        if "方法" in title or title.strip() in ("方法", "Methods", "Ref"):
            bodies.append(f"### {title}\n\n{body.strip()}")
    # also ### 方法 inside API
    for m in re.finditer(r"(###\s+[^\n]*方法[^\n]*\n.*?)(?=\n### |\n## |\Z)", text, re.S):
        bodies.append(m.group(1).strip())
    if not bodies:
        return ""
    return "\n\n".join(bodies)


def prop_names_from_api(api: str) -> set[str]:
    names = set()
    # table rows like | prop | or | `prop` |
    for m in re.finditer(r"^\|\s*`?([a-zA-Z_][\w.]*)`?\s*\|", api, re.M):
        name = m.group(1)
        if name.lower() in ("参数", "属性", "property", "prop", "名称", "---"):
            continue
        if re.match(r"^-+$", name):
            continue
        names.add(name)
    return names


def build_appearance(title: str, desc: str, demos_list, api: str, when: str) -> str:
    props = prop_names_from_api(api)
    lines = []
    lines.append("### 1.1 基础形态\n")
    lines.append(f"{desc}\n")
    lines.append(
        f"作为 **{title}** 控件，视觉结构以官方示例为准；实现 gpui kit 时应保证默认态与 antd 6.5 视觉语义对齐（尺寸节奏、圆角、主色、禁用灰、动效可关）。\n"
    )

    # demo-driven appearance inventory
    user_demos = [(s, t) for s, t, d in demos_list if not d]
    if user_demos:
        lines.append("### 1.2 文档示例覆盖的外观形态\n")
        lines.append("| 示例 | 外观/形态含义 |")
        lines.append("| --- | --- |")
        for src, t in user_demos:
            lines.append(f"| {t} | 见官方 demo `{src}`，需在 kit 中具备对应视觉能力 |")
        lines.append("")

    # prop-driven appearance subsections
    matched = []
    for prop, label in APPEARANCE_PROP_HINTS.items():
        if prop in props or any(prop.lower() == p.lower() for p in props):
            matched.append((prop, label))

    if matched:
        lines.append("### 1.3 由 API 驱动的外观维度\n")
        lines.append("下列属性直接改变控件外观（实现 kit 时优先建模为样式/状态枚举）：\n")
        lines.append("| 属性 | 外观维度 | 实现提示 |")
        lines.append("| --- | --- | --- |")
        for prop, label in matched:
            tip = "枚举/布尔映射到 token 与绘制状态"
            if prop in ("size",):
                tip = "影响高度/字号/内边距；与 ConfigProvider componentSize 联动"
            elif prop in ("variant", "type", "buttonStyle", "listType", "optionType"):
                tip = "切换边框/填充/线型等绘制变体"
            elif prop in ("color", "status", "theme", "danger"):
                tip = "切换语义色或主题色板"
            elif prop in ("placement", "orientation", "vertical", "tabPlacement"):
                tip = "影响布局方向或浮层锚点"
            elif prop in ("loading", "spinning", "percent"):
                tip = "叠加进度/旋转指示层"
            elif prop in ("disabled",):
                tip = "降对比 + 禁止指针事件"
            elif prop in ("open", "mask", "closable"):
                tip = "浮层/遮罩/关闭控件可见性"
            lines.append(f"| `{prop}` | {label} | {tip} |")
        lines.append("")

    lines.append("### 1.4 结构与语义化 DOM\n")
    if "classNames" in props or "styles" in props:
        lines.append(
            "- 支持 `classNames` / `styles` 按语义节点定制（antd 6.x 常见 root/content/icon 等）。\n"
            "- gpui kit 建议同样暴露语义节点样式钩子，便于主题覆盖而不侵入业务布局。\n"
        )
    else:
        lines.append(
            "- 若官方提供 Semantic DOM，kit 应对齐可定制节点；否则至少区分根容器/内容/装饰层。\n"
        )

    lines.append("### 1.5 主题与 Design Token\n")
    lines.append(
        "- 颜色、圆角、间距、字号优先走 Design Token / 主题变量。\n"
        "- 深浅色、品牌色、紧凑模式应可通过上层 Theme/Config 注入。\n"
    )
    return "\n".join(lines)


def build_function(title: str, when: str, demos_list, api: str, methods: str, faq: str) -> str:
    lines = []
    lines.append("### 2.1 使用场景（When To Use）\n")
    when_clean = strip_html(when) if when else ""
    # remove code blocks excess? keep markdown lists
    when_clean = re.sub(r"```[\s\S]*?```", "", when_clean).strip()
    if when_clean:
        lines.append(when_clean + "\n")
    else:
        lines.append(f"用于实现与 antd **{title}** 对等的业务能力，详见下方功能清单。\n")

    lines.append("### 2.2 核心能力清单\n")
    # derive capabilities from demos + keywords in api
    caps = []
    user_demos = [(s, t) for s, t, d in demos_list if not d]
    for i, (src, t) in enumerate(user_demos, 1):
        caps.append(f"{i}. **{t}** — 对应官方示例 `{src}`，kit 需支持该交互/展示路径。")
    if not caps:
        caps.append("1. 提供与官方文档一致的基础渲染与配置能力。")
    # add API-derived behavioral caps
    props = prop_names_from_api(api)
    extra_caps = []
    behavior = [
        ("onChange", "值变化回调 / 受控数据流"),
        ("onClick", "点击回调"),
        ("onSelect", "选中回调"),
        ("onOpenChange", "展开/显隐受控"),
        ("open", "受控显隐"),
        ("value", "受控值"),
        ("defaultValue", "非受控默认值"),
        ("disabled", "禁用交互"),
        ("loading", "加载中防重复提交"),
        ("showSearch", "可搜索过滤"),
        ("filterOption", "自定义过滤"),
        ("options", "数据化配置选项"),
        ("items", "数据化配置项"),
        ("treeData", "树形数据"),
        ("dataSource", "数据源"),
        ("columns", "列配置"),
        ("fileList", "文件列表受控"),
        ("customRequest", "自定义上传请求"),
        ("rules", "校验规则"),
        ("validateFields", "表单校验"),
        ("pagination", "分页"),
        ("rowSelection", "行选择"),
        ("expandable", "可展开行"),
        ("virtual", "虚拟滚动"),
        ("loadData", "异步加载"),
        ("getPopupContainer", "浮层挂载容器"),
        ("destroyOnHidden", "隐藏时销毁"),
        ("autoFocus", "自动聚焦"),
    ]
    base = len(caps)
    for prop, label in behavior:
        if prop in props:
            base += 1
            extra_caps.append(f"{base}. **{label}**（API: `{prop}`）")
    lines.extend(caps)
    if extra_caps:
        lines.append("")
        lines.append("由 API 识别的额外行为能力：\n")
        lines.extend(extra_caps)
    lines.append("")

    lines.append("### 2.3 示例与能力对照表\n")
    lines.append("| 示例 | 能力 | 调试示例 |")
    lines.append("| --- | --- | --- |")
    for src, t, dbg in demos_list:
        lines.append(f"| {t} | `{src}` | {'是' if dbg else '否'} |")
    if not demos_list:
        lines.append("| （无） | - | - |")
    lines.append("")

    if methods:
        lines.append("### 2.4 实例方法 / Ref\n")
        lines.append(methods + "\n")

    if faq:
        lines.append("### 2.5 FAQ（官方）\n")
        # keep reasonable length
        faq_lines = faq.splitlines()
        if len(faq_lines) > 120:
            faq = "\n".join(faq_lines[:120]) + "\n\n> FAQ 较长，已截取前部；完整内容见官方文档。\n"
        lines.append(faq + "\n")

    lines.append("### 2.6 与其它控件组合\n")
    lines.append(
        "- 可与 **Form** 结合做数据录入（注意 `valuePropName`、trigger）。\n"
        "- 可与 **ConfigProvider** 统一尺寸/主题/locale/空状态。\n"
        "- 浮层类注意与 **Modal/Drawer** 的 `getPopupContainer` 嵌套。\n"
        "- 反馈类（message/notification/modal）推荐走 **App** 上下文。\n"
    )
    return "\n".join(lines)


def build_kit_notes(title: str, props: set[str], demos_list) -> str:
    lines = []
    lines.append(
        f"面向 **gpui kit** 实现 antd 对齐的 **{title}** 时，建议按下列清单验收：\n"
    )
    lines.append("1. **API 面**：至少覆盖官方常用 props（上表）；冷门 props 可分阶段。")
    lines.append("2. **视觉态**：default / hover / active / focus / disabled / loading（若适用）。")
    lines.append("3. **尺寸态**：small / medium / large（若适用）与主题间距一致。")
    lines.append("4. **受控/非受控**：同时支持 value+onChange 与 defaultValue。")
    lines.append("5. **无障碍**：可聚焦、键盘操作、语义角色/标签。")
    lines.append("6. **RTL / 方向**：placement、orientation 在 RTL 下镜像。")
    lines.append("7. **浮层**：挂载容器、遮挡自动调整、滚动跟随（若适用）。")
    lines.append("8. **性能**：大列表虚拟滚动、输入防抖、避免整树无意义重绘。")
    lines.append("9. **主题**：颜色与圆角走 token，禁止写死品牌色。")
    lines.append("10. **测试**：每个非 debug 官方示例对应至少一条 kit 可视化/交互用例。")
    if "options" in props or "items" in props or "treeData" in props or "columns" in props:
        lines.append("11. **数据驱动**：优先 `options`/`items`/`columns`/`treeData`，减少 JSX 插槽分叉。")
    if any(x in props for x in ("open", "onOpenChange", "getPopupContainer")):
        lines.append("12. **弹层治理**：统一 z-index 与 getPopupContainer 策略。")
    return "\n".join(lines) + "\n"


def build_doc(slug: str, text: str) -> str:
    fm = frontmatter(text)
    title = fm.get("title") or slug
    subtitle = fm.get("subtitle") or ""
    desc = fm.get("description") or ""
    group = fm.get("group") or CATEGORY_MAP.get(slug, "")
    category = CATEGORY_MAP.get(slug, group or "组件")

    # when to use - multiple possible titles
    when = ""
    for t, body in all_h2(text):
        if "何时使用" in t or t.lower() == "when to use":
            when = body
            break
    if not when:
        when = section_body(text, "何时使用", "When To Use")

    demos_list = demos(text)
    api = extract_api_full(text)
    # if API empty, take everything after first ## API-like content
    if not api:
        # util / special
        rest = text.split("---", 2)
        body = rest[-1] if len(rest) >= 3 else text
        api = clean_api_markdown(body)

    methods = extract_methods(text)
    faq = extract_faq(text)
    props = prop_names_from_api(api)

    # design guide
    design = ""
    for t, body in all_h2(text):
        if "设计" in t:
            design = strip_html(body)[:2000]
            break

    appearance = build_appearance(title, desc, demos_list, api, when)
    function = build_function(title, when, demos_list, api, methods, faq)
    kit = build_kit_notes(title, props, demos_list)

    head_title = f"{title} {subtitle}".strip()
    url = slug

    parts = []
    parts.append(f"# {head_title}\n")
    parts.append(
        f"> 来源：[Ant Design 6.5.x {title}](https://ant.design/components/{url})  \n"
        f"> 版本依据：Ant Design **v6.5.1**  \n"
        f"> 分类：{category}  \n"
        f"> 说明：{desc}  \n"
        f"> 用途：作为 **gpui kit** 实现该控件时的功能规格（外观 / 行为 / 配置对齐 antd）。\n"
    )
    parts.append("---\n")
    parts.append("## 1. 控件外观\n")
    parts.append(appearance)
    parts.append("---\n")
    parts.append("## 2. 功能\n")
    parts.append(function)
    parts.append("---\n")
    parts.append("## 3. 配置（API）\n")
    parts.append(
        "通用属性参考：[Common props](https://ant.design/docs/react/common-props)。\n\n"
        "以下 API 摘自官方文档，实现 kit 时以该表为配置面清单。\n"
    )
    if api:
        # ensure starts with ## or ###
        if not api.lstrip().startswith("#"):
            parts.append("### API\n")
        parts.append(api + "\n")
    else:
        parts.append("_（源文档无标准 API 表，见功能说明与类型定义。）_\n")
    parts.append("\n### 导入方式\n")
    if slug == "util":
        parts.append("```ts\nimport type { GetRef, GetProps, GetProp } from 'antd';\n```\n")
    elif slug == "icon":
        parts.append(
            "```js\nimport { HomeOutlined } from '@ant-design/icons';\n// antd@6 需配套 @ant-design/icons@6\n```\n"
        )
    else:
        # convert slug to export name roughly
        export = "".join(p.capitalize() for p in slug.split("-"))
        # special cases
        special = {
            "qr-code": "QRCode",
            "input-number": "InputNumber",
            "auto-complete": "AutoComplete",
            "date-picker": "DatePicker",
            "time-picker": "TimePicker",
            "tree-select": "TreeSelect",
            "color-picker": "ColorPicker",
            "float-button": "FloatButton",
            "config-provider": "ConfigProvider",
            "border-beam": "BorderBeam",
        }
        export = special.get(slug, export)
        parts.append(f"```js\nimport {{ {export} }} from 'antd';\n```\n")

    parts.append("---\n")
    parts.append("## 4. gpui kit 实现要点\n")
    parts.append(kit)
    if design:
        parts.append("\n### 官方设计补充\n")
        parts.append(design[:1500] + "\n")

    parts.append("---\n")
    parts.append("## 5. 参考链接\n")
    parts.append(f"- 官方文档：https://ant.design/components/{url}\n")
    parts.append(f"- 中文文档：https://ant.design/components/{url}-cn\n")
    parts.append(
        f"- 源码：https://github.com/ant-design/ant-design/blob/master/components/{slug if slug != 'util' else '_util'}\n"
    )
    parts.append(f"- 本文档目标：驱动 gpui kit `{slug}` 控件开发与验收\n")

    content = "\n".join(parts)
    content = re.sub(r"\n{3,}", "\n\n", content)
    return content


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    # Keep README, regenerate component docs
    files = sorted(SRC.glob("*.md"))
    # skip generator helper and shared
    skip = {"tooltip-shared.md"}
    written = []
    errors = []
    for f in files:
        if f.name in skip:
            continue
        slug = f.stem
        try:
            text = f.read_text(encoding="utf-8")
            if len(text) < 50:
                errors.append((slug, "too small source"))
                continue
            doc = build_doc(slug, text)
            out = OUT / f"{slug}.md"
            out.write_text(doc, encoding="utf-8")
            written.append((slug, len(doc.splitlines()), out.stat().st_size))
        except Exception as e:
            errors.append((slug, str(e)))

    # stats
    written.sort(key=lambda x: x[1])
    print(f"Wrote {len(written)} docs")
    print("Smallest 10:")
    for s, lines, sz in written[:10]:
        print(f"  {lines:4d} lines  {sz:6d} B  {s}")
    print("Largest 10:")
    for s, lines, sz in written[-10:]:
        print(f"  {lines:4d} lines  {sz:6d} B  {s}")
    if errors:
        print("Errors:")
        for s, e in errors:
            print(" ", s, e)


if __name__ == "__main__":
    main()
