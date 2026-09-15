# video_b1_frag — B1 分段MP4真窗

跑法（关闭用 15 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/video_b1_frag -auto-only
go run ./examples/video_b1_frag -manual-seconds 30
```

## 人眼可见效果

- 左边四行门禁：头2片、解105帧、播2片、跳5次，每行显示通过数与一句话备注。四组全绿（114/114零坏）才算绿。
- 右边直播：分段长片（100帧13段）循环播，证明分段片在屏上真播出来，不只在门禁里解对。
- 底部HUD：分段通过数与直播帧数，门禁预览只看不判，真正判的是stdout JSON。

## 门禁

- 四组全过：`passed==total_items==114` 且 `failed_items==0`，`decode_diff_px==0`，否则 `FAIL:` + `exit 1`。
- 114 = 头2片 + 解105帧逐字节 + 播2片到尾零丢 + 跳5次落地板，与 `video/b1_ffmpeg_test.go` 四项一一对上，输出逐位一致。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 上屏至少1次，首帧超2000毫秒挂，内存峰值超1GB上限挂。
- `-auto-only`：先跑自检（不过直接 exit 1 不开窗），再开短窗取数判门禁。
- `-manual-seconds N`：常驻N秒收真事件，每事件打日志并改标题显示累计数，结束出汇总JSON；不带N则常驻到关窗。
