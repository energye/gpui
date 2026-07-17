# standardtest — 标准视觉测试（唯一入口）

独立包，**取代**原 `stdgate/` 与根目录 `testdata/`（标准测试相关部分）。  
渲染引擎只负责画；本包负责场景、标准图、待测图、合并对比、像素比较。

## 目录

```text
standardtest/
  scenes/              画法 JSON（D01–D200 等）
  fonts/               共用字体
  canvaskit/           CanvasKit 生成标准图
  scene/               读场景 + gpui 出测试图
  compare/             像素比较库
  merge/               左右合并 + 底部中文预期说明
  catalog.json         容差/索引
  diff/
    standard/          标准 PNG（CanvasKit）
    test/              待测 PNG（gpui）
    merge/             合并 PNG（左标准 | 右待测 + 底栏说明）
    pixel/             像素差异红图（仅失败项）
    report.json        批量比较报告
    report.txt         失败摘要
  cmd/standardtest/    CLI
```

## 流程

```text
scenes/*.json
    ├─ CanvasKit → diff/standard/
    └─ gpui      → diff/test/
                      ├─ merge   → diff/merge/*_compare.png
                      └─ compare → diff/report.json + diff/pixel/*_diff.png
```

## 命令

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export DISPLAY=:1

cd standardtest/canvaskit && npm ci && cd ../..

# 全流程：标准图 → 测试图 → 合并 → 像素比较
go run ./standardtest/cmd/standardtest all
go run ./standardtest/cmd/standardtest all -id D01_ClipLayerText

# 分步
go run ./standardtest/cmd/standardtest standard
go run ./standardtest/cmd/standardtest test
go run ./standardtest/cmd/standardtest merge
go run ./standardtest/cmd/standardtest compare
```

`compare` 使用 `catalog.json` 中的 per-case 容差；缺省走 `compare.DefaultPolicy()`。  
任一项 fail/miss/error 时 CLI 退出码为 1，报告仍会完整写出。

生成物默认不提交（见根目录 `.gitignore`）。

## Oracle 说明（CanvasKit 标准图）

标准图由 `canvaskit/render_scene.mjs` 按 `scenes/*.json` 绘制，**不是** gpui 截图。

已修关键路径问题：
- `reset_clip`：用 save/restore 栈真正清 clip（旧实现 `clipRect(全屏)` 无效，多窗格会被裁错）
- `push/pop` + `layer_*` 与 clip 交错时先卸 clip 再 restore
- `set_mask_alpha`：mask 在画布空间套用（不受 CTM 错误变换）

仍可能与 gpui 有小像素差（文字 AA、虚线、blend 精度等）。看对比请用最新：

```bash
go run ./standardtest/cmd/standardtest standard
go run ./standardtest/cmd/standardtest merge
go run ./standardtest/cmd/standardtest compare
```

- 合并图：`diff/merge/*_compare.png`（左标准 | 右待测）
- 报告：`diff/report.json` / `diff/report.txt`
- 失败红图：`diff/pixel/*_diff.png`
