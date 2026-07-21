# ui_polish_gallery

§12.3 **W3** 打磨棚：主路径 Button / Input / Checkbox / Radio / Switch / Modal 一屏对照。

## 运行

```bash
export DISPLAY=:1
export LD_LIBRARY_PATH=$PWD/lib
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
GPUI_ANIM_SECONDS=60 go run ./examples/ui_polish_gallery
```

## §12.2 手操清单（本示例）

| # | 操作 | 看什么 |
|---|------|--------|
| 1 | 静态浏览 | 圆角、1px 边、间距是否像控件 |
| 2 | 鼠标扫 Button/Checkbox | hover/press 干净（Button 每帧 SyncState） |
| 3 | Tab 走焦 | Button focus ring；Input 主色边框 |
| 4 | 点 Checkbox/Radio | 选中圆滑、居中 |
| 5 | Linux 中文输入 | **W4** 补齐 |
| 6 | 开一次 Modal | 点 **Open Modal**；遮罩/Esc/按钮 |

## 分区

1. **Button** — primary / default / dashed / text / link / disabled  
2. **Input** — placeholder、键入、焦点边框  
3. **Indicators** — Checkbox / RadioGroup / Switch  
4. **Modal** — 最小确认框  
