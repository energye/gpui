# kit anim-image 真窗（G5-anim-image）

动图组件 `kit.BuildAnimatedImage` 的原子验证窗（P3.6 批 1d 步骤 5）。
示例只用组件公开 API 与控制动词（Play/Pause/SeekFrame/SetSpeed），组件行为一概不在示例里实现。

## 摆窗

三张卡各摆一份循环档位不同的演示数据（素材正本 `render/testdata/imgfmt/`，批 1b 网格片）：

- 左：`gif_keep.gif` — 无限循环（`RepetitionCount() < 0`）
- 中：`gif_restore_prev.gif` — restore-previous disposal、额外播 1 轮
- 右：`gif_once.gif` — 播完即停（`RepetitionCount() == 0`）

帧推进固定在渲染节点 Paint 期内（D19）；组件 Tick 返回「是否继续驱动」，
播完/暂停自动摘除（泵契约合一后组件层白拿的收益）。

## 控制面板（键盘）

- 空格：全体 播放/暂停 切换
- `s`：全体回到第 0 帧
- `x`：全体 1x/2x 倍速切换

收到真控制动词时打进 stderr 日志并改窗口标题显示状态（playing / keep 帧号 /
once 帧号 / 倍速）。

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 自动自检（门禁）：5s 秒表，stdout 出 JSON，exit 1 on fail；
# 金图在冻结帧位（全体 Pause + Seek(0)）后比对，首跑种基线：
GPUI_ACCEPT_GOLDEN=1 go run ./examples/kit/anim-image -auto-only
go run ./examples/kit/anim-image -auto-only          # 复跑比对，diff 必须 0

# 常驻人工窗：无参数。人工操作键盘控制动词，关窗（WM_DELETE）出汇总 JSON。
go run ./examples/kit/anim-image
```

## 自检断言（-auto-only）

1. 逻辑：keep 卡运行期内帧号前进过（推进真发生）；once 卡播完自停且停在末帧
   （`Tick` 返回 false → 注册表自动摘除的组件层契约）。
2. 像素：三卡画面矩形内暖色像素（素材帧红块/黄条）≥150——动图着墨存在。
3. 金图：冻结帧位后整窗掩码（标题行 + 图例 + 三卡）逐位比对，diff 必须 0。
4. 门禁：full_paint、persistent FPS ≥55、p95 ≤22ms、vsync_source 非空。

## 已知限界

- 常驻人工窗的像素/金图门禁不生效（无快照），以人工观察为准；自动化门禁走
  `-auto-only`。
- 金图基线 `testdata/showcase_anim_image_base.png` 依赖冻结帧位与固定素材；
  换素材或改冻结逻辑需重种基线并人工复核。
