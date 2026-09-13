# ui_polish_gallery 总预览窗

左标签（组件列表，临时 plain 列表，W2 Tabs 落定后回替）、右展示（该组件页）。

## 可见效果

- 1200×800 窗：左侧导航列出已注册控件页，右侧内容卡 + section 色带。
- W0 只有 `overview` 页：地基/主题/浮层三段占位；控件页随 W1+ 逐个注册。
- 点击左侧条目切换页；`-tab=<名>` 只开单页（独立测/独立截图用）。

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
CGO_ENABLED=0 go run ./examples/ui_polish_gallery -list
RUN_SECONDS=15 go run ./examples/ui_polish_gallery
RUN_SECONDS=5 go run ./examples/ui_polish_gallery -tab=overview
```

正式验收以单标签独立断言为准，总窗只作平时预览（组合窗不代替单能力）。
