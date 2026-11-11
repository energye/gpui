# M5 P3：GPU tonemap / 曝光 pass（RGBA16Float → 8bit）

> 上游：evidence/M5/assessment-design.md §3 G4、§5 P3。本批只做 pass 本体与 CPU 对拍，
> 未接会话/真窗（P4），未改公开 API。

## 0 结论

render/internal/gpu/tonemap.go 新增自包含 tonemap pass：全屏三角 + 采样 RGBA16Float，
按「曝光(0,16] → Reinhard x/(1+x) → sRGB encode → floor(x*255+0.5)」输出 8bit，
与 render/hdr.FloatTarget.Encode 同一组数字。单测对拍（half 精确输入、两次曝光）
max_channel_diff = 0（判据 ≤1，禁 mean）。

## 1 实现

- tonemapWGSL：全屏三角；uv 做 y 翻转，保证输出第 0 行采样输入第 0 行；
  reinhard 与 srgb_encode 与 Go 侧公式逐行对应；quant8 用 floor(x*255+0.5)/255，
  与 Go 的 uint8(x*255+0.5) 同舍入口径（避免 GPU 银行家舍入造成 1 LSB 分叉）。
- tonemapPipeline：
  - 资源：shader / BGL（texture+sampler+uniform）/ pipeline layout / pipeline /
    linear-clamp sampler / 16B uniform（exposure）。
  - 目标格式走 colorFormatOr（P1 口径，零值 BGRA8）；P4 可按呈现格式换。
  - Run(src, dst, exposure)：校验曝光 (0,16]（NaN/Inf/0/负/17 拒绝），
    WriteBuffer 曝光 → 建 bind group → clear pass + draw → submit；BG 在提交后释放。
  - Destroy 逆创建序释放全部资源。
- 语义：输入按线性直色（与 hdr.FloatTarget 一致），输出直色；alpha 仅 clamp01。
  不透明内容直色=premul；半透明层策略归 P4。

## 2 单测（render/internal/gpu/tonemap_test.go）

- TestP3TonemapMatchesHDRCpuEncode：32×2 的 RGBA16Float 输入，列值
  0/0.0625/0.125/0.25/0.5/0.75/1/2/4/8/16（全 half 精确），两行 alpha=1 / 0.5；
  exposure=1 与 4 各跑一次，readback vs hdr.NewFloatTarget+Encode 逐通道比较：
  max_channel_diff=0、over1=0（两次）；方向断言 16.0 映射结果亮于 1.0。
- TestP3TonemapRejectsBadExposure：0 / -1 / 17 / NaN / +Inf 全部拒绝（且不碰视图）。
- 全 PASS（evidence/M5/p3-unit-tests.txt）。

## 3 回归

- go build ./... / go vet ./render/... / -tags nogpu build+vet 全 rc=0。
- M5 P0/P1/P2 单测 + TestConformanceDualBackend 全绿（同日志）。
- 无生产接线、无金图变化、无公开 API 变化。

## 4 下一步（P4）

- 会话/呈现接线：float 离屏面（P2 入口）+ tonemap pass（本批）+ 8bit 呈现；
  新真窗 ui_wr_m5_hdr（曝光 0.5/1/4 三档 + CPU Encode 参考），双后端金图与逐像素断言；
  OffscreenFormatFallbacks 进 JSON；GLES 腿同时补 16F 面与 tonemap 的真机验证。
