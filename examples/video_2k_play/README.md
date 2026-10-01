# video_2k_play — 2K 直传真窗（P3）

人眼可见：2K 真片循环播，左栏是片名与门禁行，右边是 960×540 直播画面
（解码 2560×1440 不动，显示由显卡缩到窗内），底部 HUD 跑帧率与门禁预览。

## 跑法

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
bash video/testdata/gen_p3_2k4k.sh   # 片子现场生成，不进仓库
RUN_SECONDS=30 go run ./examples/video_2k_play
```

窗口 1600×900，需要真机窗口。结束输出全族 JSON 到 stdout，
门禁不过打印 `FAIL:` 并 `exit 1`。

## 门禁（§7 的 2K 行）

- fps ≥ 55 且 p95 ≤ 22ms
- 上传纪律：上传 + 回落 == 帧数（无多余重传），视频槽零淘汰
- 首尾帧非黑（均值 > 5）
- 内存峰值 ≤ 1024MB（1440p 档）
- 三证据：逻辑探针（JSON 全族）+ 像素断言（非黑 + 直播画出）+ Golden（基线 JSON 对比）
