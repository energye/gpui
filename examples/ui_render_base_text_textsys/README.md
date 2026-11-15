# ui_render_base_text_textsys

文本系统值面直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 18）

- **Resolver 链**：进程级 `SetDefaultFontPath/SetSystemFontPaths` → `DefaultFontResolver` 反映（Candidates 首元素=设置值）；`Load/LoadMultiFace(UI+CJK 链)/LoadDefaultFace` 全链出真 face（name=设置路径）。
- **系统扫描**：`SystemFontsByFamily("DejaVu Sans")` 环境相关登记（本机 2 命中）。
- **枚举**：`Script.RequiresComplexShaping`（**Han=false——复杂整形列表只含印度系/泰/缅/高棉等**；Arabic=true）；`Script.String()`；`Direction.IsHorizontal/IsVertical`（LTR 横 TTB 竖）。
- **Source 内省**：`WeightClass=400/IsVariable=false/VariationAxes=0`（goregular 静态无轴）。
- **Feature 哈希**：`NewFontFeature/HashFontFeatures` 顺序无关 XOR。

## 语义记录

- `ScriptHan.RequiresComplexShaping()=false`：Han 不在复杂整形列表（内置 shaper 可处理），与"中文需 HarfBuzz"的直觉相反——实现口径如实登记。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_text_textsys -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
