# video_vr9_registry — VR9 扩展注册真窗

跑法（关闭用 15 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/video_vr9_registry
```

## 人眼可见效果

- 左边四行：同片 720p 出数——探测（容器/编码问完再开）、能力（容器/编码/采样表齐）、注册播（播到结尾不黑不花）、灰桩（不改核心 8/8）。
- 右边直播：同片 720p 循环 15 秒，画面一直动；右边另列注册用例数与三坏例人话错（坏盒/坏编码/坏采样）加灰桩像素。
- 底部 HUD：fps/解码/队列/内存，外加注册用例与直播帧数。

## 门禁

- 注册全过 `registry_cases_pass==total`（探测+能力+注册名+注册播+灰桩+三坏例共 8 项），同片走注册表播到 Ended，不支持的盒子/编码/采样报人话错零崩溃，否则 `FAIL:` + `exit 1`。
- 核心流程不写名字：`TestNoHardcodedNames` 锁 `player.go` 无格式名字面量（import 路径是注册表接线，不算写死）。
- 动画窗 60 档：`fps_wall≥55` 且 `interval_p95_ms≤22`，否则挂。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。
- 只做扩展验收，不代替任何解码/播放窗。
