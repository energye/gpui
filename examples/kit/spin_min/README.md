# kit spin_min — single spinning icon (no chrome, metrics JSON only)

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=10 go run ./examples/kit/spin_min
```

窗口 480×320，中央一个 64px 旋转的 sync 图标，其余全静。`RUN_SECONDS`
定时跑打印 JSON（帧数、墙钟帧率、进程 CPU、旋转度数、图标非空），门禁：
帧数≥50、帧率≥50、图标区非空。示例只调 kit 公开 API，行为全在 `ui/kit`。
