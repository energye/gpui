# go4x 黑洞定位报告（P4.1 批 4，2026-11-10）

> 性质：只读诊断 + 临时插桩（已随诊断完成，待与修复一起还原）。零修复代码。
> 修复前停「停下报告」，等用户确认修法方向。

## 0 直话

- **根因找到了，一行级**：`gpu/gwgpu/gles/command_execute.go:259`（HEAD 行号，
  `MSAAResolveCommand.ensureResolveFBO` 首用分支）裸
  `ctx.BindFramebuffer(gl.FRAMEBUFFER, fbo)` 把 **GL_READ_FRAMEBUFFER 一并绑到
  resolve FBO**，毁掉了三行之前 `st.bindRead(msaaFBO)` 刚设好的 blit 源。
  后续 `glBlitFramebuffer` 变成 resolve FBO **自拷贝**（空进空出），resolveTex
  恒零 → 一切走「离屏 nil-view 读回」的 GLES 4x 画面全黑/全丢。
- **窗口 present 面（toSurface 分支）不受影响**——它用 DRAW-only 绑定（正确孪生），
  所以 go4x 窗口跑帧大概率正常，黑的只是快照/离屏读回链。
- 无头 4x 离屏探针已确定性复现（红矩形→全白），修一行即验，不需要真窗才能红。

## 1 复现与实测（三样齐）

### 1.1 真窗复现（p39b，标准关闭单窗，3 次稳定）

- 命令：`GPUI_BACKEND=go GPUI_P39B_MSAA=4 RUN_SECONDS=6 ./p39b_bin -auto-only`
- 结果（HEAD e4b5043f，3/3 复现）：`window_not_black ok=false nonblack=0/23504`，
  锚点像素 (0,0,0)；gpu_ops=12854>0、present_count=181、msaa_setting=4 真生效、
  无 MSAA 降级/OOM/GL error 日志。
- 证据：`go4x-repro-run1.txt`（本目录）。

### 1.2 临时插桩实锤（`GPUI_GO4X_DEBUG=1`，插桩已登记待还原）

快照时刻完整命令序列（`go4x-window-debug-log.txt`）：

```
ensureResolveFBO created 11 texID 401 texTarget 0xde1      ← resolveTex 首用建 FBO
MSAAResolve.offscreen msaaFBO 10 resolveFBO 11 prevFBO 0   ← prevFBO=0=首用
MSAAResolve.after.offscreen glErr 0x0 readFBO 11 drawFBOBound 11
                                ↑↑ READ 被 ensureResolveFBO 的 FRAMEBUFFER bind 改写
CopyTexToBuf texFmt BGRA8Unorm samples 1 fbo 11 first16 [0×16]  ← 读回全零
```

窗口帧期间的 toSurface resolve（每帧 2 次）全部 `drawFBOBound=swapchainFBO`、glErr=0，
绑定正确——窗口帧面与此洞无关。

### 1.3 无头确定性复现（红测形态，4x 真生效前提=先设 MSAA 再装箱设备）

- `render/go4x_probe_test.go`（临时探针，修复后转正式红绿对或删除）：
  `SetMSAASampleCount(4)` **先于** GL 设备装箱（SetDeviceProvider 调用时即锁
  sampleCount——此前探针 4x 显示 samples=1 的假阳性即此因）→ 白底画红矩形 →
  FlushGPU → Image()：**实得全白 nonWhite=0（红矩形丢失）**；1x 同场景
  nonWhite=9600/red=9216 正常；native 4x 正常（P3.9b 金图）。

## 2 根因与标准方案（两样齐）

### 2.1 机制

GLES 3.0：`glBindFramebuffer(GL_FRAMEBUFFER, fbo)` 同时设置 READ 与 DRAW 两个
绑定点（规范 §4.4.1/9.2.1）；`glBlitFramebuffer` 从 READ 拷到 DRAW（§9.2.4），
MSAA resolve=「READ 绑多采样 FBO、DRAW 绑单采样 FBO 的 blit」。
`ensureResolveFBO` 首用分支用 GL_FRAMEBUFFER 建链+完整性检查，把 READ 从
msaaFBO 改写成 resolveFBO → blit 源=目标 → 空进空出。首用才坏（prevFBO=0），
复用路径只绑 DRAW 不碰 READ——所以只有「每个 resolveTex 的第一次离屏 resolve」
丢内容：真窗快照（首个 nil-view flush）、无头 4x 离屏渲染等。

### 2.2 标准对齐

- GL ES 3.0 规范上述条款（READ/DRAW 独立绑定点语义）。
- Skia GL 后端 resolve 纪律：blit 前显式分别绑 READ（msaa）与 DRAW（目标），
  从不依赖 GL_FRAMEBUFFER 隐式双绑（`GrGLGpu` resolve/blitFramebuffer 路径）。
- 仓内正确孪生：同文件 `MSAAResolveCommand.Execute` 的 toSurface 分支
  （`st.bindDraw` + `ctx.BindFramebuffer(gl.DRAW_FRAMEBUFFER, …)`，READ 不动）。

### 2.3 修法草案（一行级）

`ensureResolveFBO` 首用分支改用 **DRAW_FRAMEBUFFER-only** 绑定：

```go
ctx.BindFramebuffer(gl.DRAW_FRAMEBUFFER, fbo)
ctx.FramebufferTexture2D(gl.DRAW_FRAMEBUFFER, gl.COLOR_ATTACHMENT0, …)
ctx.CheckFramebufferStatus(gl.DRAW_FRAMEBUFFER)   // ES 3.0 合法
```

READ 自始至终保持 msaaFBO；复用路径本就只绑 DRAW，零变化。还原尾部的
bindRead(0)/bindDraw(0) 不动。影响面：仅 GLES 离屏 MSAA resolve（快照/滤镜
seed/层 materialize 等一切 nil-view 4x 读回）；sampleCount==1 路无 MSAA 命令
零变化；native 零变化；窗口 present 路零变化。

## 3 与 #3/#4/#5/#6 的归并边界（交接要求评估）

**不同根，不归并**：#3（PopLayer Porter-Duff GPU 层内容丢）在**默认 1x 双后端**
一致 FAIL（批 1 B 卡），go4x 只在 GLES 4x 的离屏 resolve 分支触发；#4/#5/#6
同族均为 1x 可复现的层链同步/顺序问题。go4x 修复不扩战线。

## 4 验证计划（修复后）

1. 红绿对：无头 4x 离屏读回内容测试（红=修复前全白；绿=红矩形到位），
   转正式测试入 `render/`（GO 后端 + DISPLAY 门控，缺环境 `t.Skipf`）。
2. p39b go4x `-auto-only` 真窗：window_not_black 翻 true、锚点像素过。
3. even-odd MSAA 红绿对与真窗四卡按载体块推进（依赖本修复）。
4. P3.9b 显存复核（1x/4x 门禁）。
