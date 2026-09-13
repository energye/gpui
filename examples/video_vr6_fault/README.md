# video_vr6_fault — VR6 容错真窗

跑法（关闭用 15 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/video_vr6_fault
```

## 人眼可见效果

- 左边12例坏输入矩阵：缺文件、非MP4、截断尾、花屏隔离F20、缺参数、F17分区、F17扩展、F17条带组、F20丢参考、超限等级F2、F12隔行、好片干净。每例显示过/挂+层+工具人话。
- 右边直播图：好片（B帧重排96x96）循环播，坏例跑完进程照常活，画面照常动，证明零崩溃零卡死。
- 花屏隔离例：双IDR片中间P清零，隔离2帧剩8帧，第二GOP照常播，不整片花。

## 门禁

- `fault_cases_pass==total`（12/12），否则 `FAIL:` + `exit 1`；坏输入零崩溃（panic即挂）；错误信息点名层+工具。
- 直播至少1帧，否则挂。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。
- 坏片现场用临时目录生成（好片拷贝+截断/清零），不进仓。
