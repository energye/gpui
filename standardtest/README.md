# standardtest — 标准视觉测试（唯一入口）

独立包，**取代**原 `stdgate/` 与根目录 `testdata/`（标准测试相关部分）。  
渲染引擎只负责画；本包负责场景、标准图、待测图、合并对比。

## 目录

```text
standardtest/
  scenes/              画法 JSON（D01–D200 等）
  fonts/               共用字体
  canvaskit/           CanvasKit 生成标准图
  scene/               读场景 + gpui 出测试图
  compare/             像素比较
  merge/               左右合并 + 底部中文预期说明
  catalog.json         容差/索引（可选）
  diff/
    standard/          标准 PNG（CanvasKit）
    test/              待测 PNG（gpui）
    merge/             合并 PNG（左标准 | 右待测 + 底栏说明）
  cmd/standardtest/    CLI
```

## 流程

```text
scenes/*.json
    ├─ CanvasKit → diff/standard/
    └─ gpui      → diff/test/
                      └─ merge → diff/merge/*_compare.png
```

## 命令

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export DISPLAY=:1

cd standardtest/canvaskit && npm ci && cd ../..

go run ./standardtest/cmd/standardtest all
go run ./standardtest/cmd/standardtest all -id D01_ClipLayerText
go run ./standardtest/cmd/standardtest standard|test|merge
```

生成物默认不提交（见根目录 `.gitignore`）。
