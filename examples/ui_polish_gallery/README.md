# ui_polish_gallery

> **规则：** 每个控件能力必须落在左侧对应分类 Tab 的 demo section 里（见 `docs/UI_KIT_DEV_GUIDE.md` §2.7）。  
> 例：Button Ghost → General · Button；FlexShrink → Layout · Flex。禁止只写单测、gallery 无入口。

§12.3 打磨棚（W3 视觉/焦点 + **W4 IME 说明与 Modal**）：主路径 Button / Input / Checkbox / Radio / Switch / Modal。

## 运行

```bash
export DISPLAY=:1
export LD_LIBRARY_PATH=$PWD/lib
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 默认不限时（关窗 / Ctrl+C 退出）
go run ./examples/ui_polish_gallery

# CI / 限时
GPUI_ANIM_SECONDS=60 go run ./examples/ui_polish_gallery
```

| 变量 | 默认 | 含义 |
|------|------|------|
| `GPUI_ANIM_SECONDS` | **0（不限时）** | `>0` 时到达秒数后自动退出 |

## §12.2 手操清单

| # | 操作 | 看什么 |
|---|------|--------|
| 1 | 静态浏览 | 圆角、1px 边、间距是否像控件 |
| 2 | 鼠标扫 Button/Checkbox | hover/press 干净（Button 每帧 SyncState） |
| 3 | Tab 走焦 | Button focus ring；Input 主色边框 |
| 4 | 点 Checkbox/Radio | 选中圆滑、居中 |
| 5 | Linux 中文输入 | **见下方 IME 降级说明** |
| 6 | 开一次 Modal | 点 **Open Modal**；遮罩/按钮关闭 |

## #5 IME — 正式 Caps 降级（W4）

| 路径 | CapIME | 行为 |
|------|--------|------|
| **Linux 真窗** (`LinuxHost`) | **否** | 未接 XIM/XIC；**不**宣称 OS 候选窗/拼音 composition 可用 |
| 真窗 Latin / 特殊键 | CapTextInput + CapKeyboard | `XLookupString` → EventKey / EventText（字母、退格、方向键等） |
| **CI / 可测 composition** | Headless **是** | `InjectIME` + `Dispatch` → preedit → End → TextInput 上屏 |

**真窗中文步骤（当前阻塞与预期）**

1. 点击 Input 聚焦（主色边框）。
2. 切换系统 IME 输入拼音：在 **CapIME 落地前**，候选/预编辑**不会**进入 `Tree.DispatchIME`。
3. 可测闭环请跑：`go test ./ui/kit -run IME -count=1`（Headless 注入 composition 序列）。

候选位置（C3）：`EditableText.CaretLocalPos` + `core.AbsoluteOffset`；Host 实现 `IMEPositioner` 时用 `platform.SetIMEPositionIfSupported`。LinuxHost 当前未实现。

详见 `ui/platform/ime.go` 与 `docs/UI_FRAMEWORK_MAP.md` §12.1 C1–C4。

## #6 Modal

点 **Open Modal** → 面板与遮罩 → OK / Cancel（及 MaskClosable 若开启）。

## 分区

1. **Button** — primary / default / dashed / text / link / disabled  
2. **Input** — placeholder、Latin 键入、焦点边框  
3. **Indicators** — Checkbox / RadioGroup / Switch  
4. **Modal** — 最小确认框  
