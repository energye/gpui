# D9 shader 输出 alpha 语义（L2-P1-4）证据

## 实况解剖（修前）
- 文档自相矛盾：`shader_effect.go:28-29`「straight、引擎预乘」 vs `user_shader.go:26`「premultiplied output」。
- 标定矩阵（`DrawImage` 上传实测，白底）：
  - straight (255,0,0,127) → (255,128,128,255) **正确**
  - premul (127,0,0,127) → (191,128,128,255)（被再乘 alpha，**错**）
  - → 整条 image 合成链（QueueImageDraw）按 **straight** 解释 pixelData。
- shader 链（pass1 premul 回读 → mask 链按 premul 缩 → QueueImageDraw 按 straight 合成）：
  premul 半透明输入 (0.5,0,0,0.5) 落 (255,127,127,255)=alpha 平方双亮；
  straight 形式 (1,0,0,0.5) 落 (126,127,127,255)=被再缩一次。两头都不自洽——L2 报的
  「A=0.5 红→灰 (126,127,127)」即 straight 输入侧。

## 拍板（两轮，均 question 停报）
1. 初次：premul 输出（Flutter 官方契约，docs.flutter.dev「output must be premultiplied alpha」）。
2. 补充（标定翻案后如实回报）：用户改选 **straight 语义落地路径 = 引擎转**——最终形态：
   **契约保持 premul 输出（Flutter 文档一致），引擎在 pass1 回读后去预乘，喂合成链的 straight 真色**；
   A=1 时直通不变（p32/p31 金图零影响）。

## 修复
- `user_shader_fill.go`：新增 `premulToStraightRGBA`（c=c*255/a 四舍五入；a==0/255 直通），
  pass1 回读后、mask 缩放前调用。
- 文档统一：`shader_effect.go` 契约条（premul + 引擎去预乘 + 旧洞数值）、`paint.go` Paint.Shader、
  `user_shader.go` 头注释。

## 红→绿与负向对照
- 修前红：`TestD9PremulOutputContract` 首跑 (255,127,127,255)/R=255∉128±3 FAIL（打点证实转换后
  srcRegion=[255,0,0,128]，腿正确）。
- 负向对照：注释掉 `premulToStraightRGBA` → R=255 FAIL（alpha 平方态）→ 还原 → PASS。
- `TestD9CPUModeSkips`：CPU 档静默跳过、cpu_fb=0，PASS。
- 回归：TestD8*/TestShader*/TestP32Shader*/TestD6*/TestP31DrawPaint 全绿；
  p32 双后端 golden=0.0000% probes 3/3；p31 native golden=0.0000%。

## 登记（非本条修复面）
- **兄弟路同病**：`fillMaskedAsImage` 把 premul 栅格喂 straight 链——GPU `SetRGBA(1,0,0,0.5)`
  矩形实测 (255,127,127,255)（正确 ≈(128,255,255,~128)），CPU 正确 → GPU/CPU 分叉。
  已记 findings-register，归 L2-P2-7/P3-7 同族待单开。
