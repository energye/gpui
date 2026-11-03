# D 批过程登记

- **存量：render 整包一把跑 SIGABRT（native wgpu "invalid bind group layout for pipeline layout descriptor"）**
  复现点：`TestContextFrameDamage_MultipleFillsSeparateRects`（path_bounds_test.go）在整包顺序下 abort。
  A/B：7e146e21 与 D 批工作树**两树一致**（非 D 批引入）；子集单跑带 lib `ok`；per-file 口径
  `render/path_bounds_test.go` PASS。门禁口径本为逐文件，不因此改判；与 D8（BGL 首绘毒化）同族，
  D8 修完后回照此对账一次（若消解则销项）。
- **D8 修完回照（2026-11-03）**：整包仍在 `TestContextFrameDamage_MultipleFillsSeparateRects` 崩同一签名
  （conv.rs:519），D8（user shader 管线）未消解 → **与 D8 非同族**，改登记为独立存量问题
  （vello accelerator 管线家族），待单开专项；per-file 口径 `render/path_bounds_test.go` 继续 PASS。
- **D9 标定新洞（登记，非 D9 修复面）**：`fillMaskedAsImage` 兄弟路同样把 **premul** 栅格喂进
  按 straight 解释的合成链——实测 GPU 不透明蒙半层 `SetRGBA(1,0,0,0.5)` 画矩形得
  **(255,127,127,255)**（alpha 平方；正确 premul 合成 ≈(128,255,255,~128)），CPU 档直画正确 → GPU/CPU 分叉。归 L2-P2-7（混合丢失）/P3-7
  （路由）同族，待单开条目修；shader 链已由 D9 用 `premulToStraightRGBA` 自洽。
- **D12 登记两笔（2026-11-03）**：
  1. **存量：ui_wr_p35_rrect4 go 后端金图 0.0038%（29px 1-LSB，卡片区角点）**——D 批树与
     HEAD(17574a71) 基线差异像素集合逐点相同（同集合 29px），非 D 批引入；归 gles 角点覆盖
     量化分叉，待单开对账（native 侧 0.0000% 不受牵连）。
  2. **新能小项：旧路径栅格腿的 cubic 近似残差**——解析椭圆腿对同几何的覆盖差 ≤67/255
     （r=2 中段，实测表 D12 证据 F3），是「user-space cubic 展平后设备变形」的固有近似，
     ε→0 与边界处逐位重合；解析腿为真几何，raster 腿保留给四角异径/九宫格。
- **D 批收口 sweep 唯一差异钉（2026-11-03）**：`ui/rendering/m1_noquadratic_test.go`
  `TestBoxesForRange_FullLinear_M1` 比例门禁（阈值 15）在长跑中抖动：sweep 值 16.26，
  单跑 `-count=3` = 15.7/8.8/10.6 三态——同码非确定性，D 批零触及该路径；性能阈值型钉的
  机器负载敏感属测试口径问题（改绝对预算或多次取中位），另行处理，本批不改代码。
