#!/usr/bin/env python3
"""Thicken only §6.4 (state machine) and §6.9 (test cases) to Button-level specificity.

Reads demos/API from local ant-design; uses per-control hand profiles for
ASCII state machines, rule IDs, and concrete test expectations.
Does not touch button.md.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ANTD = Path("/home/yanghy/app/projects/ant-design/components")
OUT = Path("/home/yanghy/app/projects/gogpu/gpui/docs/antd")

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
    "border-beam": "border-beam",
}


def pascal(slug: str) -> str:
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
        "popconfirm": "Popconfirm",
    }
    return special.get(slug, "".join(p.capitalize() for p in slug.split("-")))


def prefix(slug: str) -> str:
    m = {
        "float-button": "FB",
        "input-number": "INN",
        "auto-complete": "AC",
        "date-picker": "DP",
        "time-picker": "TP",
        "tree-select": "TSE",
        "color-picker": "CP",
        "config-provider": "CFG",
        "border-beam": "BB",
        "qr-code": "QR",
        "popconfirm": "PCF",
        "notification": "NTF",
        "breadcrumb": "BC",
        "pagination": "PG",
        "typography": "TYP",
        "checkbox": "CB",
        "radio": "RDO",
        "switch": "SW",
        "select": "SEL",
        "input": "INP",
        "form": "FRM",
        "table": "TBL",
        "modal": "MDL",
        "drawer": "DRW",
        "menu": "MNU",
        "tabs": "TAB",
        "upload": "UPL",
        "cascader": "CAS",
        "transfer": "TF",
        "mentions": "MEN",
        "segmented": "SEG",
        "statistic": "STA",
        "timeline": "TL",
        "tooltip": "TIP",
        "popover": "POP",
        "dropdown": "DD",
        "progress": "PRG",
        "skeleton": "SKL",
        "message": "MSG",
        "anchor": "ANC",
        "steps": "STP",
        "slider": "SLD",
        "rate": "RAT",
        "tree": "TRE",
        "list": "LST",
        "card": "CRD",
        "badge": "BDG",
        "avatar": "AV",
        "image": "IMG",
        "empty": "EMP",
        "result": "RES",
        "alert": "ALT",
        "spin": "SPN",
        "watermark": "WM",
        "affix": "AFX",
        "tour": "TOU",
        "tag": "TAG",
        "calendar": "CAL",
        "carousel": "CRS",
        "collapse": "COL",
        "descriptions": "DSC",
        "divider": "DIV",
        "flex": "FLX",
        "grid": "GRD",
        "layout": "LAY",
        "masonry": "MAS",
        "space": "SPC",
        "splitter": "SPL",
        "icon": "ICO",
        "app": "APP",
        "util": "UTL",
    }
    return m.get(slug, re.sub(r"[^A-Z]", "", pascal(slug).upper())[:3] or "CTL")


def demos(slug: str) -> list[tuple[str, str]]:
    d = ANTD / FOLDER.get(slug, slug)
    p = d / "index.zh-CN.md"
    if not p.exists():
        p = d / "index.en-US.md"
    if not p.exists():
        return []
    text = p.read_text(encoding="utf-8", errors="replace")
    out = []
    for m in re.finditer(
        r'<code\s+src="\./demo/([^"]+)"([^>]*)>([^<]*)</code>', text
    ):
        src, attrs, title = m.group(1), m.group(2), m.group(3).strip()
        if "debug" in attrs:
            continue
        out.append((src, title or src))
    return out


# ---------------------------------------------------------------------------
# Per-control §6.4 profiles: (ascii_diagram, note, rules[(id, rule, expect)])
# ---------------------------------------------------------------------------

def profile_64(slug: str, pref: str) -> tuple[str, str, list[tuple[str, str, str]]]:
    """Return control-specific state machine content."""

    # --- highly specific profiles ---
    P: dict[str, tuple[str, str, list[tuple[str, str, str]]]] = {
        "input": (
            """```text
mount ──► idle
             │
             ├── hover（未禁用）
             ├── focus ──► focused ── type/IME ──► onChange(value)
             │                ├── blur ──► idle（值保留）
             │                ├── Enter ──► onPressEnter
             │                └── maxLength 触顶 ──► 不再追加字符
             ├── allowClear 点击 ──► value="" + onChange + 焦点策略可保留
             ├── disabled ──► 不可 focus 编辑；无 onChange
             ├── readOnly ──► 可 focus 不可改
             ├── status=error|warning ──► 边框语义色（不阻断输入）
             ├── Password：visibilityToggle ──► 掩码开/关，value 不变
             ├── Search：点搜索/Enter ──► onSearch(value)
             └── TextArea autoSize ──► 高度随行数在 min/maxRows 间变化
```""",
            "IME composition 期间：实现须不把中间态当最终 value 提交（或与宿主约定一致并测）。",
            [
                (f"{pref}-S1", "受控 `value`", "显示外部值；键入只经 `onChange` 上抛，不私自写回"),
                (f"{pref}-S2", "`allowClear` 有内容时点清除", "`onChange(\"\")` 一次；空时清除图标隐藏"),
                (f"{pref}-S3", "`disabled=true`", "不可编辑；点击/键盘不产生 `onChange`"),
                (f"{pref}-S4", "`readOnly=true`", "不可编辑但可聚焦选区（若平台支持）"),
                (f"{pref}-S5", "`maxLength=N` 输入超长", "无法超过 N（或按 count 策略截断，需与文档一致）"),
                (f"{pref}-S6", "`status=error`", "错误色边框；仍可输入"),
                (f"{pref}-S7", "聚焦后 Enter", "触发 `onPressEnter`"),
                (f"{pref}-S8", "Password 显隐切换", "掩码变化，`value` 不变"),
                (f"{pref}-S9", "Search 点搜索图标", "触发 `onSearch`"),
                (f"{pref}-S10", "TextArea `autoSize={minRows,maxRows}`", "高度随内容，不小于 min、不大于 max"),
                (f"{pref}-S11", "`size` small/middle/large", "高度 24/32/40（±0.5）"),
                (f"{pref}-S12", "`variant` outlined/filled/borderless/underlined", "chrome 切换无残留错误边框"),
            ],
        ),
        "switch": (
            """```text
mount ──► unchecked | checked（受控/非受控）
             │
             ├── click / Space（聚焦）──► 若 !disabled && !loading
             │                              └── value' = !value ──► onChange(value', event)
             ├── loading=true ──► 不切换；handle 内 spinner；可保持 checked 外观
             ├── disabled=true ──► 不切换；禁用皮
             ├── 受控 checked ──► 仅外部改值才变；内部 click 只抛 onChange
             └── size=small ──► 轨几何 SM（约 28×16）
```""",
            "antd：`value` 为 `checked` 别名；loading 期间不得完成切换。",
            [
                (f"{pref}-S1", "未选中时点击", "`onChange(true)` 一次；外观变为开"),
                (f"{pref}-S2", "选中时点击", "`onChange(false)` 一次"),
                (f"{pref}-S3", "`disabled=true` 点击/Space", "不触发 `onChange`"),
                (f"{pref}-S4", "`loading=true` 点击", "不触发 `onChange`；有 spinner"),
                (f"{pref}-S5", "受控 `checked=false` 时点击", "仍显示关，直到父级改 props；回调仍抛出"),
                (f"{pref}-S6", "聚焦 + Space", "切换（同 click）"),
                (f"{pref}-S7", "`size=small`", "轨高≈16、最小宽≈28"),
                (f"{pref}-S8", "默认尺寸", "轨高≈22、最小宽≈44（公式见 §6.2）"),
                (f"{pref}-S9", "`checkedChildren`/`unCheckedChildren`", "开/关文案随状态显示"),
                (f"{pref}-S10", "主题主色", "开态轨道 = `colorPrimary`"),
            ],
        ),
        "modal": (
            """```text
closed ── SetOpen(true) / 命令式 open ──► opening ──► open
  ▲                                              │
  │                                              ├── 焦点进入对话框（陷阱）
  │                                              ├── OK ──► onOk（可返回 Promise）
  │                                              │            ├── pending ──► confirmLoading
  │                                              │            └── resolve ──► 关（或业务保持 open）
  │                                              ├── Cancel / 右上角关闭 ──► onCancel ──► closing
  │                                              ├── Esc（keyboard=true）──► onCancel
  │                                              ├── mask 点击（maskClosable）──► onCancel
  │                                              └── destroyOnHidden：关闭后卸载子树
  └── closed ◄── afterClose ◄── closing ◄─────────┘
```""",
            "命令式 `Modal.confirm`：返回销毁函数；OK/Cancel 各自回调。",
            [
                (f"{pref}-S1", "`open=true`", "遮罩+对话框可见；焦点进入"),
                (f"{pref}-S2", "点 OK", "`onOk` 一次"),
                (f"{pref}-S3", "`onOk` 返回 pending Promise", "OK 按钮 `confirmLoading`；防重复点"),
                (f"{pref}-S4", "点 Cancel / 关闭图标", "`onCancel`；关闭"),
                (f"{pref}-S5", "Esc 且 `keyboard=true`", "关闭"),
                (f"{pref}-S6", "点 mask 且 `maskClosable=true`", "关闭"),
                (f"{pref}-S7", "`maskClosable=false` 点 mask", "不关闭"),
                (f"{pref}-S8", "`destroyOnHidden=true` 关闭后再开", "子状态重置（不保留输入）"),
                (f"{pref}-S9", "`footer=null`", "无默认按钮区"),
                (f"{pref}-S10", "`centered=true`", "垂直居中（非顶距布局）"),
                (f"{pref}-S11", "默认 width", "**520**"),
                (f"{pref}-S12", "`Modal.confirm`", "命令式显示；确认/取消回调"),
            ],
        ),
        "select": (
            """```text
closed ── click/聚焦+开 ──► open（下拉 Portal）
  │                            ├── 键盘 ↑↓ 移动 active option
  │                            ├── Enter ──► 选中
  │                            │     ├── mode=单选 ──► onChange + 关闭
  │                            │     └── multiple/tags ──► 累加 tag，可保持 open
  │                            ├── 再点已选项（multiple）──► 取消选中（可配）
  │                            ├── 输入搜索（showSearch）──► 过滤 options
  │                            ├── tags 模式输入+Enter ──► 创建选项
  │                            ├── Esc / 外点 ──► 关闭
  │                            └── allowClear ──► value 空
  └── disabled ──► 不打开
```""",
            "listHeight 默认常见 256；虚拟滚动大数据为 P1 时须在 Notes 标明。",
            [
                (f"{pref}-S1", "单选选中一项", "`onChange` 一次且弹层关闭"),
                (f"{pref}-S2", "`mode=multiple` 选两项", "value 为数组长度 2；显示两个 tag"),
                (f"{pref}-S3", "`showSearch` 输入过滤", "仅匹配项可见"),
                (f"{pref}-S4", "`allowClear` 清除", "value 空；`onChange`"),
                (f"{pref}-S5", "`disabled`", "点击不打开"),
                (f"{pref}-S6", "受控 `open=false`", "无法保持展开"),
                (f"{pref}-S7", "Esc 打开时", "关闭弹层"),
                (f"{pref}-S8", "空 options", "显示 `notFoundContent`"),
                (f"{pref}-S9", "`mode=tags` 输入新项 Enter", "创建并选中"),
                (f"{pref}-S10", "`size` 三档", "高度 24/32/40"),
                (f"{pref}-S11", "键盘 Enter 选中高亮项", "同点击选中"),
                (f"{pref}-S12", "maxTagCount（适用者）", "多余折叠为 +N"),
            ],
        ),
        "table": (
            """```text
mount ──► 渲染 columns × dataSource
             ├── pagination 翻页 ──► onChange(pagination, filters, sorter)
             ├── 列头排序 ──► sorter 切换 ascend/descend/none ──► onChange
             ├── 列筛选 ──► filters ──► onChange
             ├── rowSelection 点行/全选 ──► selectedRowKeys + onChange
             ├── expandable 展开 ──► 展开行渲染
             ├── loading=true ──► 表体遮罩，列头可保留
             ├── dataSource=[] ──► Empty
             └── scroll.y / fixed ──► 表体滚动、列钉住
```""",
            "服务端排序/筛选时：受控 pagination/filters/sorter 由外部驱动。",
            [
                (f"{pref}-S1", "渲染 3 列×2 行", "可见 2 行数据单元格"),
                (f"{pref}-S2", "翻到第 2 页", "`onChange` 带 pagination.current=2"),
                (f"{pref}-S3", "点可排序列头", "排序态变化并 `onChange`"),
                (f"{pref}-S4", "行选中", "`selectedRowKeys` 含该 rowKey"),
                (f"{pref}-S5", "全选", "当前页（或跨页策略按实现）keys 全选"),
                (f"{pref}-S6", "`loading=true`", "表体 loading；不丢列头"),
                (f"{pref}-S7", "`dataSource=[]`", "显示 Empty"),
                (f"{pref}-S8", "展开行", "expanded 内容可见"),
                (f"{pref}-S9", "`scroll.y` 超高", "表体滚动、表头可固定"),
                (f"{pref}-S10", "fixed 左列 + 横滚", "左列钉住"),
                (f"{pref}-S11", "筛选确认", "`onChange` filters 有值"),
                (f"{pref}-S12", "rowKey 缺失", "开发警告或不稳定 key——测试强制提供 rowKey"),
            ],
        ),
        "form": (
            """```text
mount ──► initialValues 写入字段
             │
             ├── 字段编辑 ──► 内部 store + 子控件显示
             ├── submit ──► validateFields
             │                 ├── 失败 ──► 字段 error + onFinishFailed
             │                 └── 成功 ──► onFinish(values)
             ├── setFieldsValue ──► 更新显示
             ├── resetFields ──► 回 initial + 清 error
             ├── dependencies 变更 ──► 依赖字段再校验
             └── Form.List add/remove ──► 数组字段增删
```""",
            "与子控件桥接：Input 用 value/onChange；Switch/Checkbox 用 valuePropName=checked。",
            [
                (f"{pref}-S1", "required 字段为空提交", "`onFinishFailed`；字段展示错误"),
                (f"{pref}-S2", "字段合法后提交", "`onFinish` 收到完整 values 一次"),
                (f"{pref}-S3", "`setFieldsValue`", "对应控件显示更新"),
                (f"{pref}-S4", "`resetFields`", "回到 initialValues；错误消失"),
                (f"{pref}-S5", "`disabled` Form", "子控件不可编辑"),
                (f"{pref}-S6", "dependencies 触发", "关联字段变更后规则重跑"),
                (f"{pref}-S7", "List add", "多一项；name 路径可提交"),
                (f"{pref}-S8", "List remove", "项减少"),
                (f"{pref}-S9", "`layout=horizontal`", "label 与控件水平排布"),
                (f"{pref}-S10", "自定义 validator 失败", "展示 message"),
                (f"{pref}-S11", "validateTrigger=onBlur（若设）", "失焦才校验"),
                (f"{pref}-S12", "嵌套 name=['a','b']", "values 嵌套结构正确"),
            ],
        ),
        "menu": (
            """```text
mount ──► mode=vertical|horizontal|inline 渲染 items
             ├── 点 item ──► selectedKeys + onClick/onSelect
             ├── SubMenu ──► openKeys 展开（inline 内嵌 / 其它 popup）
             ├── inlineCollapsed ──► 窄栏；sub 变 popup
             ├── disabled item ──► 不可选
             ├── theme dark/light ──► 色板
             └── 键盘 ↑↓ Enter / Esc（popup）──► 移动/激活/关闭
```""",
            "itemHeight 默认 controlHeightLG=40。",
            [
                (f"{pref}-S1", "点菜单项", "`onClick`；`selectedKeys` 含其 key"),
                (f"{pref}-S2", "打开 SubMenu", "`openKeys` 更新；子项可见"),
                (f"{pref}-S3", "受控 selectedKeys", "外部优先"),
                (f"{pref}-S4", "disabled 项点击", "不选中"),
                (f"{pref}-S5", "`mode` 切换", "布局变为水平/直/inline"),
                (f"{pref}-S6", "`theme=dark`", "深底浅字"),
                (f"{pref}-S7", "`inlineCollapsed=true`", "宽度收窄；图标可见"),
                (f"{pref}-S8", "项高度", "≈40"),
                (f"{pref}-S9", "键盘 Enter 在项上", "激活同点击"),
                (f"{pref}-S10", "多选 multiple（若开）", "可多 selectedKeys"),
            ],
        ),
        "tabs": (
            """```text
mount ──► items；activeKey=默认第一或 defaultActiveKey
             ├── 点 tab ──► activeKey' + onChange + 面板切换 + ink 移动（line）
             ├── disabled tab ──► 不可选
             ├── editable-card：add ──► onEdit(add)
             ├── editable-card：点关闭 ──► onEdit(remove, key)
             ├── placement left/right/bottom ──► 布局变
             └── 键盘左右 ──► 在可选项间移动
```""",
            "horizontalItemGutter 固定 32；cardHeight 默认 40。",
            [
                (f"{pref}-S1", "点第二个 tab", "`onChange` 为该项 key；面板内容切换"),
                (f"{pref}-S2", "受控 activeKey", "外部固定时点击不改变显示直至 props 变"),
                (f"{pref}-S3", "disabled tab", "点击无效"),
                (f"{pref}-S4", "`type=card`", "卡片样式"),
                (f"{pref}-S5", "editable-card 删除", "`onEdit` remove"),
                (f"{pref}-S6", "`placement=left`", "页签在左"),
                (f"{pref}-S7", "line 型 ink", "指示条在 active 下"),
                (f"{pref}-S8", "键盘方向", "移动 active"),
                (f"{pref}-S9", "`destroyOnHidden`", "隐藏面板卸载"),
                (f"{pref}-S10", "size 三档", "padding/高度变化"),
            ],
        ),
        "upload": (
            """```text
idle ── 选择文件 ──► beforeUpload(file)
                      ├── 返回 false ──► 仍可入 list（antd 行为）或不上传
                      ├── 返回 Promise reject ──► 阻止
                      └── 通过 ──► customRequest / 默认上传
                                      ├── progress ──► onChange
                                      ├── done ──► file.status=done
                                      └── error ──► file.status=error
onRemove ──► 列表移除 + onChange
maxCount 触顶 ──► 不能再选（或替换策略）
disabled ──► 不可选
```""",
            "桌面 customRequest 为 P0 主路径（无浏览器 Form 默认上传）。",
            [
                (f"{pref}-S1", "选择 1 个文件", "`fileList` 增加；`onChange`"),
                (f"{pref}-S2", "`beforeUpload` 返回 false", "按 antd：列表可有文件但不自动上传"),
                (f"{pref}-S3", "`customRequest` 调成功", "status 到 done"),
                (f"{pref}-S4", "上传失败", "status=error；可展示"),
                (f"{pref}-S5", "`onRemove`", "项消失"),
                (f"{pref}-S6", "`maxCount=1` 再选", "受控替换或拒绝（与实现一致并测）"),
                (f"{pref}-S7", "`disabled`", "不能选文件"),
                (f"{pref}-S8", "`listType=picture-card`", "卡片格布局"),
                (f"{pref}-S9", "Drag 区拖入", "同等 onChange"),
                (f"{pref}-S10", "`accept` 过滤", "不接受类型不可入（或宿主过滤）"),
            ],
        ),
        "date-picker": (
            """```text
closed ── open ──► 面板（picker=date/week/month/…）
             ├── 点日 ──► onChange(dayjs, string) ──► 常关闭
             ├── Range：点起 + 点止 ──► onChange([start,end])
             ├── showTime ──► 选日后进时间或同屏
             ├── disabledDate(current) true ──► 不可点
             ├── allowClear ──► 空值
             └── Esc/外点 ──► 关闭
```""",
            "值类型桌面可用 time.Time / 整型时间戳，但语义对齐 dayjs 值。",
            [
                (f"{pref}-S1", "打开并选一天", "`onChange` 一次；输入框展示 format"),
                (f"{pref}-S2", "Range 选起止", "数组两值有序"),
                (f"{pref}-S3", "`disabledDate` 禁今天", "今天不可选"),
                (f"{pref}-S4", "`picker=month`", "月面板；选月"),
                (f"{pref}-S5", "`allowClear`", "清空"),
                (f"{pref}-S6", "`showTime`", "可选出时分秒"),
                (f"{pref}-S7", "受控 value", "外部优先"),
                (f"{pref}-S8", "格式 format", "展示字符串匹配"),
                (f"{pref}-S9", "禁用", "打不开或不响应"),
                (f"{pref}-S10", "size 高度", "24/32/40"),
            ],
        ),
        "checkbox": (
            """```text
unchecked ── click/Space ──► checked ──► onChange(e.checked=true)
checked ── click ──► unchecked
indeterminate=true ──► 半选皮；点击后通常 checked=true 并清半选
Group：各 box 独立；value 为数组
disabled ──► 不切换
```""",
            "指示器 16×16（controlInteractiveSize）。",
            [
                (f"{pref}-S1", "点未选", "checked=true；onChange"),
                (f"{pref}-S2", "点已选", "checked=false"),
                (f"{pref}-S3", "indeterminate 显示", "半选视觉"),
                (f"{pref}-S4", "indeterminate 时点击", "进入 checked 并取消半选（对齐 antd）"),
                (f"{pref}-S5", "Group 选两项", "value 长度 2"),
                (f"{pref}-S6", "disabled", "不切换"),
                (f"{pref}-S7", "Space 聚焦", "切换"),
                (f"{pref}-S8", "指示器尺寸", "16×16"),
            ],
        ),
        "radio": (
            """```text
Group value=v
  点 option A ──► value=A + onChange（互斥，仅 A）
  点已选 A ──► 保持 A（不取消，除非实现允许）
  optionType=button ──► 按钮皮
  buttonStyle=solid|outline ──► 填充/描边
  disabled option ──► 不可选
  键盘方向 ──► 组内移动选中
```""",
            "",
            [
                (f"{pref}-S1", "两点不同 option", "仅后者选中"),
                (f"{pref}-S2", "`optionType=button`", "按钮组外观"),
                (f"{pref}-S3", "`buttonStyle=solid`", "实心选中态"),
                (f"{pref}-S4", "disabled 项", "不可选"),
                (f"{pref}-S5", "受控 value", "外部优先"),
                (f"{pref}-S6", "圆点尺寸", "16"),
                (f"{pref}-S7", "Radio.Button 高度 middle", "32"),
                (f"{pref}-S8", "键盘方向", "移动选中"),
            ],
        ),
        "float-button": (
            """```text
【单钮】
mount ──► default ──hover/press──► click(onClick)
disabled ──► 吞点击
badge 叠层不抢主点击

【Group 无 trigger】子钮常显
【Group + trigger】closed ──trigger──► open 子钮 + closeIcon
受控 open；placement 四向

【BackTop】scrollY < visibilityHeight ──► 隐藏
           scrollY ≥ 400 ──► 显示 ── click ──► 滚到顶
```""",
            "默认 type=default，shape=circle，边长 40。",
            [
                (f"{pref}-S1", "点击单钮", "onClick 一次"),
                (f"{pref}-S2", "disabled", "不触发"),
                (f"{pref}-S3", "type primary/default", "色正确"),
                (f"{pref}-S4", "shape circle/square", "圆/方圆角 8"),
                (f"{pref}-S5", "边长", "40×40"),
                (f"{pref}-S6", "Group trigger=click 开", "子钮出现"),
                (f"{pref}-S7", "受控 open=false", "收起"),
                (f"{pref}-S8", "placement=left", "子钮在左"),
                (f"{pref}-S9", "BackTop scroll<400", "不可见"),
                (f"{pref}-S10", "BackTop scroll≥400 点击", "回顶"),
                (f"{pref}-S11", "badge count", "角标可见"),
                (f"{pref}-S12", "仅图标", "必须 AriaLabel"),
            ],
        ),
        "divider": (
            """```text
mount ──► orientation=horizontal|vertical 绘制
             ├── variant solid|dashed|dotted（dashed 糖）
             ├── children + titlePlacement ──► 线-文-线
             ├── plain ──► 标题正文字号
             ├── size ──► 水平 marginBlock
             └── 无点击态
```""",
            "",
            [
                (f"{pref}-S1", "默认", "水平实线"),
                (f"{pref}-S2", "vertical", "行内竖线高≈0.9em"),
                (f"{pref}-S3", "dashed/variant=dashed", "虚线"),
                (f"{pref}-S4", "variant=dotted", "点线"),
                (f"{pref}-S5", "标题 center", "中置"),
                (f"{pref}-S6", "titlePlacement=start", "偏起侧"),
                (f"{pref}-S7", "plain", "标题字号≈14"),
                (f"{pref}-S8", "size=small", "marginBlock≈8"),
                (f"{pref}-S9", "线宽", "1"),
                (f"{pref}-S10", "线色", "colorSplit"),
            ],
        ),
        "empty": (
            """```text
mount ──► 显示 image + description + children(actions)?
             ├── image=Empty.PRESENTED_IMAGE_SIMPLE ──► 简图
             ├── description=null/false ──► 可隐藏文案（按 API）
             └── 无选中/输入态
```""",
            "",
            [
                (f"{pref}-S1", "默认 Empty", "有默认插画与「暂无数据」类文案（locale）"),
                (f"{pref}-S2", "simple 图", "简图资源"),
                (f"{pref}-S3", "自定义 description", "文案替换"),
                (f"{pref}-S4", "children 按钮", "操作区可点"),
                (f"{pref}-S5", "自定义 image", "显示指定图"),
                (f"{pref}-S6", "主题切换", "字色随 Theme"),
            ],
        ),
        "progress": (
            """```text
percent=p ──► 线宽/圆弧 = p%
status=normal|active|exception|success ──► 色与图标
type=line|circle|dashboard ──► 形态
showInfo=false ──► 隐藏百分比文案
```""",
            "",
            [
                (f"{pref}-S1", "percent=50 type=line", "轨填充约一半"),
                (f"{pref}-S2", "percent=100", "成功态可自动 success（按 antd）"),
                (f"{pref}-S3", "status=exception", "错误色"),
                (f"{pref}-S4", "type=circle", "环形"),
                (f"{pref}-S5", "showInfo=false", "无百分比数字"),
                (f"{pref}-S6", "线高", "≈8"),
                (f"{pref}-S7", "环 size 默认", "≈120"),
                (f"{pref}-S8", "steps 线（适用者）", "分段显示"),
            ],
        ),
        "spin": (
            """```text
spinning=false ──► 仅 children
spinning=true ──► 指示器 + 可选 mask 盖 children
delay>0 ──► 延迟后才显示指示器
fullscreen ──► 视口级遮罩
```""",
            "",
            [
                (f"{pref}-S1", "spinning=true", "可见旋转指示"),
                (f"{pref}-S2", "spinning=false", "无指示；children 可点"),
                (f"{pref}-S3", "tip 文案", "与指示同显"),
                (f"{pref}-S4", "fullscreen", "全屏遮罩"),
                (f"{pref}-S5", "delay=500", "500ms 内不闪烁显示"),
                (f"{pref}-S6", "嵌套 children", "children 仍在树中"),
                (f"{pref}-S7", "reduced-motion", "可静止或降动画"),
            ],
        ),
        "drawer": (
            """```text
closed ── open ──► 侧滑 panel + mask
             ├── close / Esc / mask ──► onClose
             ├── placement 四边 ──► 滑出方向
             └── destroyOnHidden ──► 卸载
```""",
            "默认 width=378。",
            [
                (f"{pref}-S1", "open=true", "可见"),
                (f"{pref}-S2", "onClose 路径", "关闭"),
                (f"{pref}-S3", "placement=left", "从左出"),
                (f"{pref}-S4", "maskClosable=false", "点 mask 不关"),
                (f"{pref}-S5", "Esc", "关（keyboard）"),
                (f"{pref}-S6", "默认宽", "378"),
                (f"{pref}-S7", "footer", "底栏可见"),
                (f"{pref}-S8", "destroyOnHidden", "卸载子树"),
            ],
        ),
        "message": (
            """```text
Message.success(content) ──► 顶栏入队显示
             ├── duration 到期 ──► 离场销毁
             ├── duration=0 ──► 常驻直至 close/destroy
             ├── 同 key 再次 open ──► 更新内容
             └── maxCount ──► 超限丢最旧
```""",
            "默认 duration=3s。",
            [
                (f"{pref}-S1", "success 调用", "可见成功条"),
                (f"{pref}-S2", "duration=0.1 短时", "会自动消失"),
                (f"{pref}-S3", "duration=0", "不自动关"),
                (f"{pref}-S4", "同 key 更新", "仍一条"),
                (f"{pref}-S5", "连续多条", "堆叠"),
                (f"{pref}-S6", "destroy", "清空"),
                (f"{pref}-S7", "error/warning/info/loading", "图标类型正确"),
            ],
        ),
        "notification": (
            """```text
open ──► placement 角落显示
duration ──► 自动关（默认 4.5s）
key 更新 ──► 替换
btn 点击 ──► 业务回调
```""",
            "",
            [
                (f"{pref}-S1", "open", "可见"),
                (f"{pref}-S2", "placement=bottomLeft", "位置在左下"),
                (f"{pref}-S3", "duration 到期", "消失"),
                (f"{pref}-S4", "key 更新", "不新增一条"),
                (f"{pref}-S5", "手动 close", "onClose"),
                (f"{pref}-S6", "带 btn", "按钮可点"),
            ],
        ),
        "alert": (
            """```text
mount ──► type 语义色 + message + description?
closable ──► 点关闭 ──► onClose ──► 卸载/隐藏
banner ──► 顶栏铺满皮
```""",
            "",
            [
                (f"{pref}-S1", "type=success/info/warning/error", "四套语义色"),
                (f"{pref}-S2", "closable 点关闭", "onClose；不可见"),
                (f"{pref}-S3", "description", "双行结构"),
                (f"{pref}-S4", "showIcon", "图标可见"),
                (f"{pref}-S5", "banner", "横幅样式"),
                (f"{pref}-S6", "action 区", "可放按钮"),
            ],
        ),
        "tabs": (
            """```text
mount ──► items；activeKey=默认第一或 defaultActiveKey
             ├── 点 tab ──► activeKey' + onChange + 面板切换 + ink 移动（line）
             ├── disabled tab ──► 不可选
             ├── editable-card：add ──► onEdit(add)
             ├── editable-card：点关闭 ──► onEdit(remove, key)
             ├── placement left/right/bottom ──► 布局变
             └── 键盘左右 ──► 在可选项间移动
```""",
            "horizontalItemGutter 固定 32；cardHeight 默认 40。",
            [
                (f"{pref}-S1", "点第二个 tab", "`onChange` 为该项 key；面板内容切换"),
                (f"{pref}-S2", "受控 activeKey", "外部固定时点击不改变显示直至 props 变"),
                (f"{pref}-S3", "disabled tab", "点击无效"),
                (f"{pref}-S4", "`type=card`", "卡片样式"),
                (f"{pref}-S5", "editable-card 删除", "`onEdit` remove"),
                (f"{pref}-S6", "`placement=left`", "页签在左"),
                (f"{pref}-S7", "line 型 ink", "指示条在 active 下"),
                (f"{pref}-S8", "键盘方向", "移动 active"),
                (f"{pref}-S9", "`destroyOnHidden`", "隐藏面板卸载"),
                (f"{pref}-S10", "size 三档", "padding/高度变化"),
            ],
        ),
    }

    if slug in P:
        return P[slug]

    global _MORE
    if _MORE is None:
        _MORE = _init_more()
    if slug in _MORE:
        return _MORE[slug]

    return _fallback_profile(slug, pref)


def _fallback_profile(slug: str, pref: str) -> tuple[str, str, list[tuple[str, str, str]]]:
    export = pascal(slug)
    # Build from demos + known API patterns
    dlist = demos(slug)
    ascii_ = f"""```text
mount ──► {export} 默认态
             │
             ├── 主交互（点击/选择/开合/提交，按 API）──► 对应 onChange/onClick/onOpenChange
             ├── disabled / loading（适用者）──► 吞交互
             ├── 受控 value/open/checked/activeKey（适用者）──► 外部优先
             ├── 键盘主路径（适用者）──► 与角色一致
             └── 主题切换 ──► chrome 更新
```"""
    rules = [
        (f"{pref}-S1", f"默认创建 {export}", "稳定渲染，默认值符合 §6.10"),
        (f"{pref}-S2", "主路径交互一次", "对应回调恰好一次，状态符合 API"),
        (f"{pref}-S3", "disabled（适用者）", "无状态变更回调"),
        (f"{pref}-S4", "受控模式（适用者）", "外部值优先"),
        (f"{pref}-S5", "§6.2 关键尺寸", "与 Token 表一致（±0.5）"),
        (f"{pref}-S6", "键盘/焦点（适用者）", "可聚焦者 ring 可见"),
        (f"{pref}-S7", "非法/边界输入（适用者）", "不崩溃；有夹紧或校验反馈"),
        (f"{pref}-S8", "销毁/关闭后再开（适用者）", "destroyOnHidden 时状态重置"),
    ]
    # add demo-linked rules
    for i, (src, title) in enumerate(dlist[:4], 9):
        rules.append(
            (
                f"{pref}-S{i}",
                f"示例「{title}」主路径",
                f"行为对齐 demo `{src}` 的用户可见结果",
            )
        )
    note = f"细则以 `{FOLDER.get(slug, slug)}` 官方 API 与 demo 为准；下列规则为 P0 可测约束。"
    return ascii_, note, rules


# Complete more profiles for key remaining controls
def _init_more():
    global _MORE
    _MORE = {}
    def add(slug, ascii_, note, rules):
        _MORE[slug] = (ascii_, note, rules)

    add(
        "slider",
        """```text
value=v ∈ [min,max]
  drag handle ──► onChange(v') ── release ──► onChangeComplete(v')
  click rail ──► 跳到对应值
  range ──► 两 handle；左≤右
  marks 点击 ──► 跳到刻度
  keyboard arrows ──► ±step
  disabled ──► 不响应
```""",
        "轨厚约 4；handle 命中大于可视。",
        [
            ("SLD-S1", "拖到最大", "value=max；onChange"),
            ("SLD-S2", "min/max 夹紧", "不能越界"),
            ("SLD-S3", "step=10", "只落在 10 的倍数"),
            ("SLD-S4", "range 两柄", "数组两值"),
            ("SLD-S5", "marks 点击", "到刻度值"),
            ("SLD-S6", "disabled", "不改"),
            ("SLD-S7", "键盘右", "+step"),
            ("SLD-S8", "vertical", "垂直轨"),
            ("SLD-S9", "tooltip", "拖动时展示值（可配）"),
            ("SLD-S10", "onChangeComplete", "仅松手触发"),
        ],
    )
    add(
        "rate",
        """```text
hover 预览 ──► 临时高亮
click 第 n 星 ──► value=n + onChange
allowHalf ──► 半星区 value=n-0.5
allowClear 再点当前值 ──► 0
disabled ──► 不改
```""",
        "默认 count=5。",
        [
            ("RAT-S1", "点第 3 星", "value=3"),
            ("RAT-S2", "allowHalf 点半区", "x.5"),
            ("RAT-S3", "allowClear 再点", "0"),
            ("RAT-S4", "disabled", "不改"),
            ("RAT-S5", "count=10", "10 星"),
            ("RAT-S6", "键盘（适用）", "可调"),
            ("RAT-S7", "tooltips", "悬停文案"),
            ("RAT-S8", "受控 value", "外部优先"),
        ],
    )
    add(
        "tree",
        """```text
treeData 渲染
  expand switcher ──► expandedKeys + onExpand
  select title ──► selectedKeys + onSelect
  check（checkable）──► checkedKeys + onCheck
  loadData 展开叶 ──► 异步 children
  disabled node ──► 不可选/勾
```""",
        "行高约 24。",
        [
            ("TRE-S1", "展开节点", "子可见；onExpand"),
            ("TRE-S2", "选中", "selectedKeys"),
            ("TRE-S3", "checkable 勾选", "checkedKeys"),
            ("TRE-S4", "父子联动（默认）", "勾父影响子"),
            ("TRE-S5", "disabled 节点", "不可交互"),
            ("TRE-S6", "loadData", "异步出现子节点"),
            ("TRE-S7", "受控 expandedKeys", "外部优先"),
            ("TRE-S8", "showLine", "连接线可见"),
            ("TRE-S9", "multiple 选", "多 selected"),
            ("TRE-S10", "搜索（若接）", "过滤"),
        ],
    )
    add(
        "dropdown",
        """```text
closed ── trigger(hover/click/contextMenu) ──► open menu
  选 item ──► onClick + 常关闭
  外点/Esc ──► 关闭
  受控 open
  disabled ──► 不打开
```""",
        "",
        [
            ("DD-S1", "click 触发打开", "菜单可见"),
            ("DD-S2", "选一项", "onClick；关闭"),
            ("DD-S3", "外点", "关闭"),
            ("DD-S4", "Esc", "关闭"),
            ("DD-S5", "disabled", "不打开"),
            ("DD-S6", "受控 open=false", "关"),
            ("DD-S7", "placement", "位置正确"),
            ("DD-S8", "hover 触发", "悬停开，离开关"),
            ("DD-S9", "子菜单（适用）", "可展开"),
            ("DD-S10", "危险项", "红色样式"),
        ],
    )
    add(
        "tooltip",
        """```text
closed ── hover/focus/click ──► open tip
leave / blur / 再 click / Esc ──► close
受控 open
```""",
        "",
        [
            ("TIP-S1", "hover 打开", "title 可见"),
            ("TIP-S2", "离开关闭", "不可见"),
            ("TIP-S3", "placement=bottom", "在下方"),
            ("TIP-S4", "受控 open", "外部控制"),
            ("TIP-S5", "空 title", "不显示（antd 行为）"),
            ("TIP-S6", "arrow", "箭头指向"),
            ("TIP-S7", "color 预设", "底色变"),
            ("TIP-S8", "delay（若配）", "延迟开"),
        ],
    )
    add(
        "popover",
        """```text
同 Tooltip 开合，但内容为 title+content 卡片
click 外 ──► 关（trigger=click）
```""",
        "",
        [
            ("POP-S1", "打开", "title/content 可见"),
            ("POP-S2", "关闭", "不可见"),
            ("POP-S3", "placement", "位置"),
            ("POP-S4", "受控 open", "外部"),
            ("POP-S5", "trigger=click", "点击切换"),
            ("POP-S6", "复杂 content 按钮", "可点"),
        ],
    )
    add(
        "popconfirm",
        """```text
open ──► 确认气泡
  点 OK ──► onConfirm ──► 常关闭
  点 Cancel ──► onCancel ──► 关闭
  disabled ──► 不打开
```""",
        "",
        [
            ("PCF-S1", "确认", "onConfirm 一次"),
            ("PCF-S2", "取消", "onCancel"),
            ("PCF-S3", "disabled", "不打开"),
            ("PCF-S4", "受控 open", "外部"),
            ("PCF-S5", "showCancel=false", "无取消钮"),
            ("PCF-S6", "异步 onConfirm pending", "可保持 open 至结束（P0/P1 标明）"),
        ],
    )
    add(
        "pagination",
        """```text
current, pageSize, total
  点页码 ──► onChange(page, pageSize)
  prev/next ──► 夹紧 1..pages
  sizeChanger ──► onShowSizeChange + onChange
  quickJumper Enter ──► 跳页
  disabled ──► 无切换
```""",
        "itemSize 默认 32。",
        [
            ("PG-S1", "点第 2 页", "current=2；onChange"),
            ("PG-S2", "首页点 prev", "不变"),
            ("PG-S3", "末页点 next", "不变"),
            ("PG-S4", "改 pageSize", "回调"),
            ("PG-S5", "jumper 输入页码 Enter", "跳转"),
            ("PG-S6", "disabled", "不切换"),
            ("PG-S7", "simple", "简化 UI"),
            ("PG-S8", "项高 middle", "32"),
            ("PG-S9", "total=0", "合理空/一页"),
            ("PG-S10", "showTotal", "文案含总数"),
        ],
    )
    add(
        "steps",
        """```text
current=i
  items[0..i-1] finish；i process；>i wait（可被 status 覆盖）
  onChange 可点 ──► current'
  status=error ──► 当前错误皮
```""",
        "",
        [
            ("STP-S1", "current=1（0-based 实现需锁定）", "对应步为 process"),
            ("STP-S2", "点可点步", "onChange"),
            ("STP-S3", "status=error", "错误样式"),
            ("STP-S4", "vertical", "纵向"),
            ("STP-S5", "size=small", "更小"),
            ("STP-S6", "disabled 步", "不可点"),
            ("STP-S7", "自定义 icon", "可见"),
            ("STP-S8", "description", "可见"),
        ],
    )
    add(
        "collapse",
        """```text
activeKey 集合
  点 panel 头 ──► 切换展开
  accordion=true ──► 至多一个 key
  collapsible=disabled ──► 头不可点
```""",
        "",
        [
            ("COL-S1", "展开一项", "内容可见；onChange"),
            ("COL-S2", "accordion 开第二项", "第一项收起"),
            ("COL-S3", "collapsible=disabled", "不可点"),
            ("COL-S4", "ghost", "无边框背景弱"),
            ("COL-S5", "bordered=false", "无边框"),
            ("COL-S6", "destroyOnHidden", "收起卸载"),
            ("COL-S7", "受控 activeKey", "外部优先"),
            ("COL-S8", "键盘（适用）", "可激活头"),
        ],
    )
    add(
        "carousel",
        """```text
index=i
  next/prev / dots / autoplay ──► index' + afterChange
  infinite 边界 ──► 循环
```""",
        "",
        [
            ("CRS-S1", "next", "index+1；afterChange"),
            ("CRS-S2", "dots 点第 n", "到 n"),
            ("CRS-S3", "autoplay", "自动前进"),
            ("CRS-S4", "到末张再 next infinite", "回首"),
            ("CRS-S5", "arrows=false", "无箭头"),
            ("CRS-S6", "GoTo(i)", "跳转"),
        ],
    )
    add(
        "image",
        """```text
展示 src
  点图 preview ──► 全屏预览
  Esc/遮罩 ──► 关预览
  加载失败 ──► fallback
  PreviewGroup ──► 左右切换
```""",
        "",
        [
            ("IMG-S1", "显示 src", "可见图"),
            ("IMG-S2", "打开预览", "全屏层"),
            ("IMG-S3", "关闭预览", "层消失"),
            ("IMG-S4", "失败 fallback", "fallback 图"),
            ("IMG-S5", "Group 下一张", "切换"),
            ("IMG-S6", "preview=false", "点不预览"),
        ],
    )
    add(
        "badge",
        """```text
count/dot 叠在 children 上
count>overflowCount ──► overflowCount+
showZero=false 且 0 ──► 隐藏
```""",
        "",
        [
            ("BDG-S1", "count=5", "显示 5"),
            ("BDG-S2", "count=100 overflow=99", "99+"),
            ("BDG-S3", "dot", "红点"),
            ("BDG-S4", "showZero=false count=0", "隐藏"),
            ("BDG-S5", "showZero=true count=0", "显示 0"),
            ("BDG-S6", "status", "状态点色"),
            ("BDG-S7", "offset", "位置偏移"),
            ("BDG-S8", "Ribbon", "缎带"),
        ],
    )
    add(
        "tag",
        """```text
展示
  closable 点关 ──► onClose
  CheckableTag ──► checked 切换
```""",
        "高约 22。",
        [
            ("TAG-S1", "color=red/预设", "色皮"),
            ("TAG-S2", "closable 关闭", "onClose"),
            ("TAG-S3", "Checkable 切换", "onChange"),
            ("TAG-S4", "bordered=false", "无边"),
            ("TAG-S5", "icon", "前图标"),
            ("TAG-S6", "自定义色值", "背景/边"),
        ],
    )
    add(
        "avatar",
        """```text
优先 src 图 ── 失败 onError ──► children 字 / icon
Group max ──► +N
```""",
        "默认 32。",
        [
            ("AV-S1", "src 成功", "图"),
            ("AV-S2", "src 失败", "回退字/icon；onError"),
            ("AV-S3", "字头像", "显示字符"),
            ("AV-S4", "shape=square", "方"),
            ("AV-S5", "size=large", "更大"),
            ("AV-S6", "Group max=2 三个头", "+1 类溢出"),
            ("AV-S7", "默认 size", "32"),
        ],
    )
    add(
        "card",
        """```text
静态结构 title/extra/cover/actions
loading ──► 遮罩/骨架
hoverable ──► 悬停抬升
```""",
        "padding 24；圆角 8。",
        [
            ("CRD-S1", "title+extra", "头区"),
            ("CRD-S2", "cover", "封面"),
            ("CRD-S3", "actions", "底操作可点"),
            ("CRD-S4", "loading", "加载态"),
            ("CRD-S5", "hoverable", "悬停样式"),
            ("CRD-S6", "Meta", "avatar+title+desc"),
            ("CRD-S7", "size=small", "更紧 padding"),
            ("CRD-S8", "type=inner", "内嵌皮"),
        ],
    )
    add(
        "list",
        """```text
dataSource.map(renderItem)
pagination ──► 翻页
loading ──► 遮罩
[] ──► Empty
```""",
        "",
        [
            ("LST-S1", "2 项 dataSource", "2 行"),
            ("LST-S2", "pagination", "翻页回调"),
            ("LST-S3", "loading", "遮罩"),
            ("LST-S4", "空数组", "Empty"),
            ("LST-S5", "split=false", "无分割线"),
            ("LST-S6", "bordered", "边框"),
            ("LST-S7", "grid", "栅格项"),
            ("LST-S8", "header/footer", "可见"),
        ],
    )
    add(
        "descriptions",
        """```text
items 按 column 栅格排布
bordered 表框
```""",
        "",
        [
            ("DSC-S1", "3 项 column=3", "一行三格"),
            ("DSC-S2", "bordered", "表框"),
            ("DSC-S3", "span=2", "占两列"),
            ("DSC-S4", "size", "padding 变"),
            ("DSC-S5", "title", "标题"),
            ("DSC-S6", "layout=vertical", "标签在上"),
        ],
    )
    add(
        "timeline",
        """```text
items 时间轴渲染
mode alternate 左右
pending 末尾未完成
```""",
        "",
        [
            ("TL-S1", "3 items", "3 节点"),
            ("TL-S2", "alternate", "左右交错"),
            ("TL-S3", "pending", "末尾 pending"),
            ("TL-S4", "reverse", "倒序"),
            ("TL-S5", "color 点", "点色"),
            ("TL-S6", "自定义 dot", "自定义点"),
        ],
    )
    add(
        "statistic",
        """```text
展示 title/value
Countdown 每秒 tick ──► 0 时 onFinish
```""",
        "",
        [
            ("STA-S1", "value 显示", "数字文案"),
            ("STA-S2", "precision=2", "两位小数"),
            ("STA-S3", "prefix/suffix", "前后缀"),
            ("STA-S4", "Countdown 结束", "onFinish"),
            ("STA-S5", "loading", "加载皮"),
            ("STA-S6", "valueStyle", "色/字号覆盖"),
        ],
    )
    add(
        "segmented",
        """```text
options 单选
点 option ──► value + onChange
block ──► 均分撑满
disabled 项 ──► 不可点
```""",
        "高≈controlHeight。",
        [
            ("SEG-S1", "切换选项", "onChange"),
            ("SEG-S2", "block", "撑满宽"),
            ("SEG-S3", "disabled 项", "不可选"),
            ("SEG-S4", "size", "高度档"),
            ("SEG-S5", "受控 value", "外部优先"),
            ("SEG-S6", "仅图标 option", "可显示"),
            ("SEG-S7", "键盘（适用）", "可切换"),
        ],
    )
    add(
        "calendar",
        """```text
value ── 选日 ──► onSelect/onChange
mode month|year 切换面板
disabledDate 禁日
```""",
        "",
        [
            ("CAL-S1", "选日", "onSelect"),
            ("CAL-S2", "mode=year", "年面板"),
            ("CAL-S3", "disabledDate", "禁日不可点"),
            ("CAL-S4", "fullscreen=false", "卡片模式"),
            ("CAL-S5", "受控 value", "外部"),
            ("CAL-S6", "切月", "面板月份变"),
        ],
    )
    add(
        "result",
        """```text
status 决定图标
title/subTitle/extra 展示
```""",
        "",
        [
            ("RES-S1", "status=success", "成功图标"),
            ("RES-S2", "404", "404 皮"),
            ("RES-S3", "extra 按钮", "可点"),
            ("RES-S4", "自定义 icon", "替换"),
            ("RES-S5", "subTitle", "可见"),
        ],
    )
    add(
        "skeleton",
        """```text
loading=true ──► 骨架
loading=false ──► children
active ──► 闪烁动画
```""",
        "",
        [
            ("SKL-S1", "loading 骨架", "灰块可见"),
            ("SKL-S2", "loading false", "children"),
            ("SKL-S3", "active", "动画"),
            ("SKL-S4", "avatar+paragraph", "结构"),
            ("SKL-S5", "paragraph rows=4", "4 行"),
            ("SKL-S6", "reduced-motion", "可停动画"),
        ],
    )
    add(
        "watermark",
        """```text
content/image 平铺层盖在 children 上
gap/rotate/zIndex 变化 ──► 重绘
```""",
        "rotate 默认 -22。",
        [
            ("WM-S1", "文字 content", "可见水印"),
            ("WM-S2", "rotate=-22", "有倾角"),
            ("WM-S3", "gap", "间距变化"),
            ("WM-S4", "image", "图水印"),
            ("WM-S5", "children 可点", "不阻断（pointer-events 策略）"),
            ("WM-S6", "多行 content 数组", "多行字"),
        ],
    )
    add(
        "affix",
        """```text
scroll < 阈 ──► 静态
scroll ≥ offsetTop ──► affixed fixed + onChange(true)
回滚 ──► onChange(false) + 占位保持布局
```""",
        "",
        [
            ("AFX-S1", "过 offsetTop", "钉住；onChange true"),
            ("AFX-S2", "回滚", "解除；onChange false"),
            ("AFX-S3", "占位", "下方内容不跳变"),
            ("AFX-S4", "自定义 target 容器滚", "相对容器计算"),
            ("AFX-S5", "offsetBottom", "贴底逻辑"),
        ],
    )
    add(
        "anchor",
        """```text
click link ──► 滚动到 href 目标 + onClick
scroll ──► 计算 active link + ink + onChange
affix ──► 钉住
```""",
        "",
        [
            ("ANC-S1", "点击项", "滚到锚点"),
            ("ANC-S2", "滚动经过 section", "active 切换；onChange"),
            ("ANC-S3", "affix", "钉住"),
            ("ANC-S4", "ink", "指示条在 active"),
            ("ANC-S5", "嵌套 items", "二级可见"),
            ("ANC-S6", "targetOffset", "停位偏移"),
        ],
    )
    add(
        "breadcrumb",
        """```text
items 渲染 · separator ·
末项通常非链接
href/项点击 ──► 导航/回调
```""",
        "separatorMargin=8。",
        [
            ("BC-S1", "三项", "两 separator"),
            ("BC-S2", "自定义 separator", "可见"),
            ("BC-S3", "链接项点击", "回调/路由"),
            ("BC-S4", "末项", "非链接强调"),
            ("BC-S5", "带 menu 的项", "可下拉（P0/P1）"),
            ("BC-S6", "separator 间距", "≈8"),
        ],
    )
    add(
        "flex",
        """```text
direction/gap/justify/align/wrap 布局 children
```""",
        "gap 8/16/24。",
        [
            ("FLX-S1", "默认两子", "横向"),
            ("FLX-S2", "vertical", "纵向"),
            ("FLX-S3", "gap middle", "间距 16"),
            ("FLX-S4", "gap large", "24"),
            ("FLX-S5", "justify=space-between", "两端"),
            ("FLX-S6", "align=center", "交叉轴居中"),
            ("FLX-S7", "wrap + 窄宽", "换行"),
            ("FLX-S8", "gap=8 数字", "8px"),
        ],
    )
    add(
        "space",
        """```text
子项等距；Compact 合并边框
```""",
        "默认 size=small=8。",
        [
            ("SPC-S1", "默认三子", "横向 gap8"),
            ("SPC-S2", "size=large", "gap24"),
            ("SPC-S3", "vertical", "纵向"),
            ("SPC-S4", "wrap", "换行"),
            ("SPC-S5", "separator", "分隔可见"),
            ("SPC-S6", "Compact 双 Button", "中间无双边框缝"),
            ("SPC-S7", "align", "对齐"),
            ("SPC-S8", "size=16 数字", "16px"),
        ],
    )
    add(
        "grid",
        """```text
Row + Col span/24
gutter 间距
断点 xs…xxl 改 span
```""",
        "24 栅格。",
        [
            ("GRD-S1", "span=12+12", "各 50%"),
            ("GRD-S2", "span=8×3", "各约 33%"),
            ("GRD-S3", "offset=6", "左空 6 格"),
            ("GRD-S4", "gutter=16", "列间隙"),
            ("GRD-S5", "响应 md=12 xs=24", "断点切换"),
            ("GRD-S6", "wrap", "换行行为"),
        ],
    )
    add(
        "layout",
        """```text
Header/Sider/Content/Footer 嵌套
Sider collapsible ──► collapsed 宽变化 + onCollapse
```""",
        "Header 64；Sider 200/80。",
        [
            ("LAY-S1", "经典四区", "均可见"),
            ("LAY-S2", "Header 高", "64"),
            ("LAY-S3", "Sider 宽", "200"),
            ("LAY-S4", "折叠", "宽→80；onCollapse"),
            ("LAY-S5", "theme dark", "侧深底"),
            ("LAY-S6", "再展开", "回 200"),
            ("LAY-S7", "breakpoint（适用）", "自动折叠"),
        ],
    )
    add(
        "splitter",
        """```text
drag handle ──► 面板 size 变（夹紧 min/max）+ onResize
release ──► onResizeEnd
collapsible ──► 折叠/恢复
```""",
        "",
        [
            ("SPL-S1", "拖动", "尺寸变；onResize"),
            ("SPL-S2", "低于 min", "夹紧"),
            ("SPL-S3", "高于 max", "夹紧"),
            ("SPL-S4", "松手", "onResizeEnd"),
            ("SPL-S5", "折叠", "面板收起"),
            ("SPL-S6", "vertical", "上下分"),
            ("SPL-S7", "命中条", "≥ 可视宽"),
        ],
    )
    add(
        "masonry",
        """```text
items 按 columns 放入最短列
columns/gutter/items 变化 ──► 重排
```""",
        "",
        [
            ("MAS-S1", "columns=3", "三列有项"),
            ("MAS-S2", "gutter", "间距"),
            ("MAS-S3", "不等高项", "错落非严格网格"),
            ("MAS-S4", "增项", "重排不崩"),
            ("MAS-S5", "columns=1", "单列"),
        ],
    )
    add(
        "icon",
        """```text
resolve(name) ──► paint
spin ──► Tick 旋转（reduced-motion 停）
rotate ──► 静态角
```""",
        "",
        [
            ("ICO-S1", "已知名", "绘出"),
            ("ICO-S2", "未知名", "不 panic"),
            ("ICO-S3", "SetSize(24)", "约 24×24"),
            ("ICO-S4", "spin", "角度随时间变"),
            ("ICO-S5", "reduced-motion+spin", "不转"),
            ("ICO-S6", "rotate=180", "倒置"),
            ("ICO-S7", "SetColor", "着色"),
            ("ICO-S8", "装饰默认", "不进 Tab"),
        ],
    )
    add(
        "typography",
        """```text
Text/Title/Paragraph/Link 渲染
copyable ──► 剪贴板 + onCopy
ellipsis ──► 省略；expandable 展开
editable ──► 编辑态 Enter 提交 Esc 取消
```""",
        "Title 字号 38/30/24/20/16。",
        [
            ("TYP-S1", "Title level 1..5", "字号阶梯"),
            ("TYP-S2", "type=danger 等", "语义色"),
            ("TYP-S3", "copyable 点击", "剪贴板正确；onCopy"),
            ("TYP-S4", "ellipsis 超长", "省略号"),
            ("TYP-S5", "expandable 展开", "全文"),
            ("TYP-S6", "editable Enter", "提交新文案"),
            ("TYP-S7", "editable Esc", "取消"),
            ("TYP-S8", "disabled", "不可点复制/编辑"),
            ("TYP-S9", "strong/code/mark/delete", "样式可区分"),
            ("TYP-S10", "Link", "链接色可聚焦"),
            ("TYP-S11", "受控 expanded", "外部优先"),
            ("TYP-S12", "正文 14", "字号"),
        ],
    )
    add(
        "input-number",
        """```text
value
  up/down / 键↑↓ ──► ±step 夹紧 min/max ──► onChange
  输入 ──► 失焦校验/修正
  controls=false ──► 无按钮
```""",
        "",
        [
            ("INN-S1", "点上", "+step"),
            ("INN-S2", "点下到 min", "夹紧 min"),
            ("INN-S3", "max 再上", "不变"),
            ("INN-S4", "precision=2", "两位小数"),
            ("INN-S5", "keyboard 下", "-step"),
            ("INN-S6", "disabled", "不改"),
            ("INN-S7", "controls=false", "无加减钮"),
            ("INN-S8", "高度 middle", "32"),
            ("INN-S9", "受控 value", "外部优先"),
            ("INN-S10", "step=0.1", "按 0.1 步进"),
        ],
    )
    add(
        "auto-complete",
        """```text
输入 ──► onSearch ──► 过滤 options 开列表
选中 ──► 回填 + onSelect/onChange
allowClear 清空
```""",
        "",
        [
            ("AC-S1", "输入触发 onSearch", "回调"),
            ("AC-S2", "选建议", "回填 value"),
            ("AC-S3", "clear", "空"),
            ("AC-S4", "无匹配", "空列表/notFound"),
            ("AC-S5", "键盘选中", "Enter 选"),
            ("AC-S6", "disabled", "不交互"),
            ("AC-S7", "受控 value", "外部"),
            ("AC-S8", "高度", "32 middle"),
        ],
    )
    add(
        "cascader",
        """```text
开面板 ── 列0
选中级 ──► 下钻 / changeOnSelect 立即 onChange
loadData 异步填 children
多选 multiple ──► 多路径
```""",
        "",
        [
            ("CAS-S1", "选三级路径", "onChange 数组长度 3"),
            ("CAS-S2", "changeOnSelect", "每级触发 onChange"),
            ("CAS-S3", "loadData", "异步子列出现"),
            ("CAS-S4", "clear", "空"),
            ("CAS-S5", "search", "过滤"),
            ("CAS-S6", "disabled", "不打开"),
            ("CAS-S7", "expandTrigger=hover", "悬停下钻"),
            ("CAS-S8", "displayRender", "输入框展示定制"),
        ],
    )
    add(
        "mentions",
        """```text
输入 prefix(@) ──► 建议列表
选中 ──► 插入 token + onSelect
onSearch 过滤
```""",
        "",
        [
            ("MEN-S1", "输入 @", "开面板"),
            ("MEN-S2", "选一项", "插入；onSelect"),
            ("MEN-S3", "onSearch", "过滤"),
            ("MEN-S4", "多行 rows", "高度"),
            ("MEN-S5", "disabled", "不交互"),
            ("MEN-S6", "clear", "空"),
            ("MEN-S7", "自定义 prefix #", "# 触发"),
        ],
    )
    add(
        "time-picker",
        """```text
开面板 ── 选 H/M/S ── onChange
disabledTime 禁部分
step 控制列间隔
```""",
        "",
        [
            ("TP-S1", "选时间", "onChange"),
            ("TP-S2", "format 展示", "字符串匹配"),
            ("TP-S3", "hourStep=2", "小时列间隔 2"),
            ("TP-S4", "disabledTime", "禁项不可选"),
            ("TP-S5", "clear", "空"),
            ("TP-S6", "Range", "起止"),
            ("TP-S7", "12 小时制（适用）", "am/pm"),
            ("TP-S8", "高度", "32"),
        ],
    )
    add(
        "tree-select",
        """```text
开 ── 树 ── 选/勾 ── onChange
search 过滤
```""",
        "",
        [
            ("TSE-S1", "选节点", "value"),
            ("TSE-S2", "treeCheckable", "勾选集合"),
            ("TSE-S3", "search", "过滤"),
            ("TSE-S4", "clear", "空"),
            ("TSE-S5", "multiple", "多数组"),
            ("TSE-S6", "loadData", "异步"),
            ("TSE-S7", "disabled", "不打开"),
            ("TSE-S8", "高度", "32"),
        ],
    )
    add(
        "transfer",
        """```text
左选 ── > ── 右 targetKeys + onChange
右选 ── < ── 移回
search 过滤
全选
```""",
        "",
        [
            ("TF-S1", "右移一项", "targetKeys 含 key"),
            ("TF-S2", "左移", "移除"),
            ("TF-S3", "search 左", "过滤"),
            ("TF-S4", "全选右移", "批量"),
            ("TF-S5", "disabled", "不可移"),
            ("TF-S6", "oneWay", "无回移"),
            ("TF-S7", "受控 targetKeys", "外部优先"),
        ],
    )
    add(
        "color-picker",
        """```text
开面板 ── 调色 ── onChange
松手/完成 ── onChangeComplete
format hex/rgb/hsb
disabledAlpha 无透明
```""",
        "",
        [
            ("CP-S1", "改色", "onChange"),
            ("CP-S2", "format=hex", "字符串 #rrggbb"),
            ("CP-S3", "presets 点选", "到预设色"),
            ("CP-S4", "clear", "空"),
            ("CP-S5", "disabledAlpha", "无 alpha 滑条"),
            ("CP-S6", "disabled", "不打开"),
            ("CP-S7", "showText", "展示文本"),
            ("CP-S8", "受控 value", "外部"),
        ],
    )
    add(
        "tour",
        """```text
open ── step[current] 高亮洞 + 气泡
next/prev ── current'
close ── onClose
```""",
        "",
        [
            ("TOU-S1", "打开", "洞+气泡"),
            ("TOU-S2", "下一步", "current+1"),
            ("TOU-S3", "上一步", "current-1"),
            ("TOU-S4", "关闭", "onClose"),
            ("TOU-S5", "受控 current", "外部"),
            ("TOU-S6", "末步完成", "关闭或回调"),
        ],
    )
    add(
        "qr-code",
        """```text
value ── 生成矩阵
status=expired ── 遮罩 + onRefresh
```""",
        "默认 size=160。",
        [
            ("QR-S1", "value 非空", "绘出矩阵不崩"),
            ("QR-S2", "size=200", "边长 200"),
            ("QR-S3", "status=expired", "遮罩"),
            ("QR-S4", "onRefresh", "点击刷新回调"),
            ("QR-S5", "icon", "中心图标"),
            ("QR-S6", "errorLevel", "可生成"),
            ("QR-S7", "默认 size", "160"),
        ],
    )
    add(
        "config-provider",
        """```text
Provider 注入 theme/locale/size
子控件 rebuild 读上下文
嵌套内层覆盖外层
```""",
        "",
        [
            ("CFG-S1", "theme 改 primary", "子 Button 主色变"),
            ("CFG-S2", "componentSize=small", "子 Input 高 24"),
            ("CFG-S3", "locale", "文案变"),
            ("CFG-S4", "嵌套覆盖", "内层胜出"),
            ("CFG-S5", "direction=rtl", "布局镜像（适用）"),
            ("CFG-S6", "getPopupContainer", "浮层挂载点"),
        ],
    )
    add(
        "app",
        """```text
App 包裹 ──► message/modal/notification 上下文可用
```""",
        "",
        [
            ("APP-S1", "包裹后 Message.success", "能显示"),
            ("APP-S2", "Modal.confirm", "能显示"),
            ("APP-S3", "Notification.open", "能显示"),
            ("APP-S4", "配置 duration 默认", "透传生效"),
        ],
    )
    add(
        "util",
        """```text
（无运行时状态机；类型工具）
```""",
        "仅文档。",
        [
            ("UTL-S1", "GetProps 说明存在", "文档可查"),
            ("UTL-S2", "GetRef 说明存在", "文档可查"),
            ("UTL-S3", "与 kit 映射说明", "有对照指引"),
        ],
    )
    add(
        "border-beam",
        """```text
mount ── 光束循环（Tick）
reduced-motion ── 停
```""",
        "",
        [
            ("BB-S1", "默认", "可见流光或等价动画"),
            ("BB-S2", "reduced-motion", "静止"),
            ("BB-S3", "改 duration", "速度变"),
            ("BB-S4", "改颜色", "色变"),
            ("BB-S5", "children", "内容可见"),
        ],
    )
    return _MORE


_MORE = None


def get_profile(slug: str, pref: str):
    """Resolve per-control §6.4 profile (ASCII + rules)."""
    return profile_64(slug, pref)


def build_64(slug: str) -> str:
    pref = prefix(slug)
    export = pascal(slug)
    ascii_, note, rules = get_profile(slug, pref)
    lines = [
        "### 6.4 交互状态机（L1）\n",
        ascii_,
        "",
    ]
    if note:
        lines.append(f"\\*{note}\n")
    lines += [
        "| 规则 ID | 规则 | 期望 |",
        "| --- | --- | --- |",
    ]
    for rid, rule, exp in rules:
        lines.append(f"| {rid} | {rule} | {exp} |")
    lines.append("")
    return "\n".join(lines)


def build_69(slug: str) -> str:
    pref = prefix(slug)
    export = pascal(slug)
    _, _, rules = get_profile(slug, pref)
    dlist = demos(slug)

    lines = [
        "### 6.9 验收用例表（可测）\n",
        f"> 测试名建议：`Test{export}_PRD_<ID>` 或 gallery 场景 ID。  \n"
        f"> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 {export} 完成 1:1 主路径。\n",
        "| ID | 级别 | 步骤 | 期望 |",
        "| --- | --- | --- | --- |",
    ]
    n = 1
    # defaults
    cases: list[tuple[str, str, str, str]] = [
        (f"{pref}-{n:02d}", "L1", f"New{export} 默认创建", "不崩溃；默认值符合 §6.10 / antd"),
    ]
    n += 1
    # from rules — each rule becomes a case with concrete expect
    for rid, rule, exp in rules:
        cases.append((f"{pref}-{n:02d}", "L1", rule, exp))
        n += 1
    # demos
    for src, title in dlist[:8]:
        cases.append(
            (
                f"{pref}-{n:02d}",
                "L1",
                f"复现官方示例「{title}」（`{src}`）",
                "交互与主视觉符合文档；无控制台级错误",
            )
        )
        n += 1
    # L2/L3/L4 standard hard cases
    cases += [
        (
            f"{pref}-{n:02d}",
            "L2",
            "读取 §6.2 关键尺寸/间距",
            "与表内数字一致（±0.5px，或文档写明容差）",
        ),
        (
            f"{pref}-{n+1:02d}",
            "L2",
            "默认皮颜色",
            "无硬编码品牌色；走 Theme Token",
        ),
        (
            f"{pref}-{n+2:02d}",
            "L2",
            "disabled 外观（适用者）",
            "禁用色；无 hover 高亮",
        ),
        (
            f"{pref}-{n+3:02d}",
            "L1",
            "键盘/焦点主路径（适用者）",
            "可聚焦者 Focus ring 可见；激活键有效",
        ),
        (
            f"{pref}-{n+4:02d}",
            "L3",
            "关键态 golden 截图",
            "与仓库基线一致（AA 容差）",
        ),
        (
            f"{pref}-{n+5:02d}",
            "L4",
            "与 ant.design 并排",
            "人眼签字记录",
        ),
        (
            f"{pref}-{n+6:02d}",
            "P1",
            "§6.8 P1 任一能力（若做）",
            "单独用例；Notes 标明",
        ),
    ]
    for cid, level, step, exp in cases:
        lines.append(f"| {cid} | {level} | {step} | {exp} |")
    lines.append("")
    return "\n".join(lines)


def replace_section(text: str, header: str, next_header_prefix: str, new_body: str) -> str:
    """Replace from ### header through just before next ### section starting with prefix."""
    # header like "6.4 交互状态机（L1）"; next_header_prefix like "6.5"
    start = text.find(f"### {header}\n")
    if start < 0:
        # try without requiring exact rest
        m = re.search(rf"^### {re.escape(header.split()[0])}[^\n]*\n", text, re.M)
        if not m:
            raise RuntimeError(f"section {header} not found")
        start = m.start()
    # find next section ### 6.x
    rest = text[start + 1 :]  # skip first char so we don't rematch self
    m_next = re.search(rf"\n### {re.escape(next_header_prefix)}(?:\s|$)", rest)
    if not m_next:
        raise RuntimeError(f"next section {next_header_prefix} not found after {header}")
    end = start + 1 + m_next.start() + 1  # position of \n before ###
    # end should be at the newline before ### next — keep that newline
    # start..end exclusive of next header: content from start to m_next.start()+1 in full text
    end = start + 1 + m_next.start() + 1
    return text[:start] + new_body + text[end:]


def process(slug: str) -> str:
    if slug == "button":
        return "skip button"
    path = OUT / f"{slug}.md"
    if not path.exists():
        return f"missing {slug}"
    text = path.read_text(encoding="utf-8")
    if "### 6.4" not in text or "### 6.9" not in text:
        return f"no 6.4/6.9 {slug}"
    body64 = build_64(slug)
    body69 = build_69(slug)
    text2 = replace_section(text, "6.4 交互状态机（L1）", "6.5", body64)
    # 6.5 header might vary slightly
    if "### 6.5 " not in text2:
        # try find next after 6.4
        pass
    text3 = replace_section(text2, "6.9 验收用例表（可测）", "6.10", body69)
    path.write_text(text3, encoding="utf-8")
    # measure
    s4 = text3[text3.index("### 6.4") : text3.index("### 6.5")]
    s9 = text3[text3.index("### 6.9") : text3.index("### 6.10")]
    pref = prefix(slug)
    n_rules = s4.count("| " + pref + "-S")
    n_cases = len(re.findall(r"\| " + re.escape(pref) + r"-\d+", s9))
    return f"{slug}: 6.4={s4.count(chr(10))+1}l rules≈{n_rules} 6.9={s9.count(chr(10))+1}l cases≈{n_cases}"


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("-")]
    # ensure more profiles loaded
    _init_more()
    if args:
        slugs = args
    else:
        slugs = sorted(
            p.stem
            for p in OUT.glob("*.md")
            if not p.name.startswith("_") and p.stem not in ("README", "button")
        )
    for s in slugs:
        try:
            print(process(s))
        except Exception as e:
            print(f"ERR {s}: {e}")


if __name__ == "__main__":
    main()
