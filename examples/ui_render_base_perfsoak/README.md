# ui_render_base_perfsoak

**PerfSoak 轴** — §20 正向指标贯穿（60fps+ / hitch / RSS / CPU / close）。

默认 30s 烟测；长 soak：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=300 go run ./examples/ui_render_base_perfsoak
```

门禁：无 layout storm；`RUN_SECONDS≥10` 时 fps≥30；`≥60s` 时 RSS slope 不过陡。
