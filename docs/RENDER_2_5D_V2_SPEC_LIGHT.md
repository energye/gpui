# V2 规格样板：灯影（6.1 灯＋6.2 法线＋6.3 影子）

一句话：白天鼓包一眼能见，晚上手电只照人不照地，暗部和背景分得开。
状态：样板待审。审过后照此模子铺相机、精灵、动画、粒子、地图 5 节。
日期：2026-09-16。对标：Godot 4 PointLight2D / DirectionalLight2D / LightOccluder2D / CanvasModulate / CanvasTexture。

## 1. 是干嘛的

管一幅 2.5D 画面哪里亮哪里暗、哪里鼓哪里平、影子往哪倒。白天靠方向光定方向，晚上靠环境底色保底不黑死，手电靠点光加灯片只照人。鼓包靠法线图算，影子靠遮挡多边形算。只算数不画画，算完调现有画图接口，render 主路不动。

Godot 对照：
- CanvasModulate：全场染一个环境底色，对应 Scene.Night。
- PointLight2D：点光，texture 灯形对应 Cookie，texture_scale 对应 Range，energy 对应 Intensity。
- DirectionalLight2D：方向光，对应 KindDirectional＋Dir。
- LightOccluder2D：挡光多边形，对应 Occluder。
- CanvasTexture：底图＋法线＋高光，对应 Image＋NormalMap（高光 V2 先不做）。

## 2. 接口（冻住的只加不改）

位置：`game/light/`。只用 `game/core` 的数，不自立 Vec2/Color。

```text
light.go：Kind（KindPoint/KindDirectional）、Layer（LayerWorld/LayerFX/LayerUI）、LayersFor、Light{Kind,Pos,Dir,Range,Intensity,Color,Mask,Cookie}、Image{W,H,Pix,Layer}、Scene{Night,Lights}、Apply/LitCPU/LitGPU/FactorAt
normal.go：Normal{X,Y,Z}、FlatNormal、Normalize、NdotL、LightDir(l,p,height)、NormalMap{W,H,Nrm}、NormalScene{Scene,Map,Height}、ApplyNormal/LitNormalCPU/LitNormalGPU/NormalFactor
shadow.go：Occluder{Pts}、NewOccluder、Shadow{Lights,Occluders,Occlusion,Length}、Blocked/FactorFor/Lit/Apply（含 GPU 镜像）
预算：MaxLightRange=8192、MaxIntensity=8、MaxLights=256、MaxCookieDimension=512、MaxImagePixels=16M、MaxOccluderVerts=64、MaxOccluders=256、MaxShadowLength=8192
```

数学（冻住，两边同数）：
- 点光衰减：t=1-dist/range 的 Hermite smoothstep，灯下 1，边缘外 0。
- 灯片：灯包围正方形双线性，坏灯片按 1（裸灯泡）。
- 一像素：base*night＋base*sum(factor*intensity*lightRGB)，钳 [0,1]，alpha 不动。零灯也有 base*night，不黑屏。
- 法线：一像素 base*night＋base*sum(factor*cosine*intensity*lightRGB)，cosine=NdotL，朝灯＞平面＞背灯，缺法线按平面，不出 NaN。
- 影子：点光看灯到像素之间有没有墙边穿过，方向光看像素沿 -Dir 长 Length 有没有墙边穿过，端点碰不算挡。被挡乘 1-Occlusion，留 base*night 不黑死。Length=0 合法空操作。

错码（只用 core 码）：坏构造 InvalidArg，像素长对不上 BadData，超预算 OutOfMemory，坏图坏骨头占位加报错不崩。

## 3. 参数默认值（抄死，不让编）

| 参数 | 默认值 | 能调范围 | 调坏会怎样 |
|---|---|---|---|
| Night 环境底色 | (0.25,0.25,0.35) 夜蓝 | [0,1] | 全 0 会死黑，不许 |
| torch 手电 | Intensity 1.2 暖色，Range 盖人，Cookie 圆灯片，Mask 只要 World 层 | Intensity (0,8]，Range (0,8192] | Mask 带上 UI 会把按钮照花，打回 |
| 方向光 | 正午从上往下，Intensity 1.0 | 方向任意，强度 (0,8] | 强度超 8 拒收 |
| Height 灯高 | 法线窗按窗高换算 | (0,8192] | 零负越界 InvalidArg |
| 影子柔边 | 三档无/PCF5/PCF13，默认中间档 | 三选一 | PCF13 只许少量灯，多了告警 |
| Occlusion 遮挡系数 | 1.0 全挡 | [0,1] | 超界钳住记日志 |
| 真灯上限 | 带阴影不超 10 盏 | 1–10 | 超了按覆盖像素收费并告警 |

## 4. 样板图（拿什么验像不像）

4 样，1080p 备，每样底色＋法线＋粗糙三张配齐（粗糙 V2 先两档：木头哑光、铁件亮、皮肤中间）：
- 木箱 40 厘米见方，木纹清，边角磨损。
- 人物 1 米 7，脸和衣服分开，手电一照脸亮衣稍暗。
- 石头半米，鼓包最高比边上亮一半以上。
- 小场景：三样加一棵树一面墙，前后盖对。

底纹必须有。均匀灰无轮廓、无底纹、右暗部和背景一色、人眼只见一团亮晕，算红（S47 历史问题，不许重犯）。

## 5. 单点指标（每点全面，不许缩水）

S45 灯（game/light/light_test.go 6 项）：
- A：位置范围衰减灯片强度颜色分层照全局夜色全对。
- B：零灯不黑屏（最暗 0.03 以上），坏构造错码分清，500 坏图错码稳定。
- C：同一套数 CPU 与显卡受光系数一致（双重放逐位一致）。
- D：8 灯打 64x64 一次约 1.7ms（约 52.9ns/像素/灯），帧率调用次数一起记。
- E：2000 遍重放一致。
- F：离屏 light_cases.json 8 组系数＋7 组整图。

S47 法线（game/light/normal_test.go 6 项＋normal_cases.json 5 组整图＋8 组系数＋6 组点积）：
- A：受光方向对（朝灯＞平＞背灯，如 0.984＞0.880＞0）。
- B：缺法线按平面不崩，坏高 InvalidArg，坏图 BadData。
- C：双重放逐位一致＋存取往返无损。
- D：8 灯打 64x64 约 2.5ms（约 75.5ns/像素/灯）。
- E：2000 遍重放一致＋500 坏图错码稳＋穹顶左亮右暗形状保持。
- F：离屏金＋窗小金图（窗大小验，不拿 6144 像素小金顶 34 万像素大窗）。

S48 影子（game/light/shadow_test.go 6 项＋shadow_cases.json 11 组系数＋3 组整图）：
- A：影子方向对（人左半亮右半黑），墙面保持亮（端点碰不挡）。
- B：无挡不崩，Length=0 合法空操作，零 Dir 不挡不崩。
- C：同一套数 CPU 与显卡逐位一致。
- D：单手电 32 墙打 64x64 约 14.87ms（约 114ns/像素/挡）。
- E：2000 遍重放一致＋500 坏图错码稳。
- F：离屏金＋窗小金图。

## 6. 真窗（怎么做，做什么，指标是什么）

架子照抄 ui_wr_r15_soak（wrkit 搭壳＋wrgate 判门＋wrsoak 探针对比图），不另起。两步：先自动判收 JSON，再常驻人工收 JSON。跑完即关的一次性不能顶人工窗。

| 窗 | 自动判啥 | 人工看啥 | 天花板指标 |
|---|---|---|---|
| game_light--case=torch | presents 400＋、fps57＋、p95 进 18ms、hitch 每分不超 3、parity0%、金 0 差、lit 千级＋增益 0.6＋、bg 与夜色逐位同、最暗不黑屏 | 左白天中夜色右手电只照人不照背景 | 正式包＋预热 5 秒跑 3 遍取最差＋换机一遍 |
| game_light--case=normal | 同上＋穹顶行朝灯＞平＞背灯（0.97＞0.60＞0.19 量级）、lit 3 万像素＋增益 0.7＋、离屏金＋窗金双 0 差 | 中卡鼓包圈露出来、右暗部浮在亮底上不沉底、圆边软过渡、三倍放大无锯齿 | 同上＋底纹必须有，均匀灰无轮廓算红 |
| game_light--case=shadow | 同上＋moved 600＋、双金 0 差 | 右卡人左亮右黑影子方向对 | 同上＋OOM 降档记日志深查 |

对比图全进 examples/game_light/testdata/，容差写死（静图 0 容差），改基准记谁为啥哪天，先查代码不许直接更新图。

## 7. 本节总指标（少一样不算完）

- 数：三窗正式包单窗 60 帧（57＋容抖），p95 进 18ms、p99 进 20ms，每分卡不超 3 次。
- 像：双金 0 差，像素差超 1% 打回；梯形变方块、渐变变纯色、暗部糊成一色打回。
- 眼：白灯 2 倍出晕不过曝，暗场有层次，白天鼓包见、晚上只照人、暗部和背景分开。
- 量：十盏以内带阴影 60 帧，大灯按覆盖像素收费。
- 坏路：零灯不黑屏，坏灯片按裸灯泡，坏着色器只黑游戏层，静默变灰算假绿。
- 环境：写清显卡驱动分辨率垂直同步正式包与否，换一台照样过。
- 文档：能力行＋关门行＋修订三处一次写齐。

## 8. 铺排模子（后 5 节照抄本节 1–7）

相机节：Camera2D 参数＋跟随 0.5 秒贴住＋10 公里缝＋双位置查询。
精灵节：四模式＋合批 1 次提交＋排序百级 0.5ms＋翻书信号。
动画节：淡入 0.2–0.3 秒＋骨骼 JSON 最低子集＋权重和 1。
粒子节：数量行为分离＋改比例不重启＋CPU 千级 GPU 万级＋可视盒裁剪。
地图节：三段寻址＋256 块一批＋16 合并碰撞＋临界回滞不闪。
