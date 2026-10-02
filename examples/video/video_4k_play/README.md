# video_4k_play — 4K 直传真窗（P3）

人眼可见：4K 真片循环播，大画面加右下小窗同源同帧（同一块纹理两次绘制，
第二次不重传），底部 HUD 跑帧率与门禁预览。

## 跑法

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
bash video/testdata/gen_p3_2k4k.sh   # 片子现场生成，不进仓库
RUN_SECONDS=60 go run ./examples/video/video_4k_play
```

窗口 1600×900，需要真机窗口。结束输出全族 JSON 到 stdout，
门禁不过打印 `FAIL:` 并 `exit 1`。

## 门禁（§7 的 4K 行）

- fps ≥ 55 且 p95 ≤ 22ms
- 上传纪律：上传 + 回落 == 帧数（无多余重传），同源大小窗第二次绘制不重传，视频槽零淘汰
- 首尾帧非黑（均值 > 5）
- 内存峰值 ≤ 2048MB（4K 档）
- 三证据：逻辑探针（JSON 全族）+ 像素断言（非黑 + 双窗画出）+ Golden（基线 JSON 对比）
