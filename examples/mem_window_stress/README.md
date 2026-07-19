# mem_window_stress 运行命令

`mem_window_stress` 是 offscreen 渲染资源 churn 示例，不创建 X11 窗口，也不使用 swapchain present；它主要用于反复创建 `render.Context`、offscreen texture 和 GPU 资源，观察释放路径和内存稳定性。

## 基础运行

在仓库根目录运行：

```bash
go run ./examples/mem_window_stress
```

编译后二进制运行：

```bash
go build -o /tmp/mem_window_stress ./examples/mem_window_stress
/tmp/mem_window_stress
```

## 调整迭代次数

默认 120 次：

```bash
GPUI_MEM_ITERS=500 go run ./examples/mem_window_stress
```

带超时保护：

```bash
GPUI_MEM_ITERS=1000 timeout -s INT 180s go run ./examples/mem_window_stress
```

## 说明

这个示例没有最小化/切后台问题，因为它没有窗口 surface。device-lost/OOM 后的 native release 防护在底层 `rwgpu` 中统一处理，因此该示例的 offscreen buffer/texture/pipeline 等释放路径也会受保护。
