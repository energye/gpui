# 组件实现架构图（Flutter 对齐版）

> 与 `ARCHITECTURE.md` + `WIDGET_MODEL.md` 同源。下文两张图：层图（谁包谁）+ 流程图（一次点击从哪进从哪出）。

## 层图（只许往下调）

```mermaid
flowchart TB
    L0["L0 主题地基<br/>ui/theme 种子别名<br/>Seed Alias Component"]
    L1["L1 基础件 prim<br/>Box Flex Stack Text Icon Image<br/>Decorated Clip Opacity<br/>只包 ui/rendering"]
    L2["L2 行为件 behavior<br/>Interactive Field OverlayTrigger Motion<br/>只包 gestures focus overlay animation scheduler textinput"]
    L3["L3 产品组件 package kit<br/>button_*.go alert_*.go… 一组件可多文件<br/>只拼 L1+L2 禁直调 render gpu"]
    L4["L4 组合全局 package kit<br/>app_*.go config_provider_*.go…<br/>往 Ctx 放东西"]
    ENG["引擎现货 直接复用<br/>render scene scheduler<br/>R0-R22 C0-C11 IME R1-R5 真窗已证"]
    L0 --> L1 --> L2 --> L3 --> L4
    ENG -.复用.-> L1
    ENG -.复用.-> L2
```

## 流程图（一次点击）

```mermaid
flowchart LR
    P["Props 不可变<br/>这次长啥样"] --> B["Build<Comp> ctx+Props<br/>Build 段拼树"]
    B --> S["Instance 状态机<br/>hover pressed focus<br/>disabled loading"]
    S --> L["Layout 约束往下<br/>尺寸往上 L1排版"]
    L --> PT["Paint 只调L1<br/>色走Resolve"]
    PT --> C["Composite Layer合成<br/>动画标独立重画"]
    C --> PR["Present 1200x800<br/>指标A-J+三证据"]
    HIT["点哪 HitTest<br/>Hit等绘"] --> S
    S -->|SetState 标脏| L
    TH["主题三级<br/>Props大于组件主题大于种子"] -.Resolve.-> PT
    CTX["Ctx 显式往下<br/>主题尺寸禁用方向动效语言"] -.Use.-> B
```

## 文字版（聊天里看）

```text
L0 主题地基（种子别名组件三级，悬停按压派生对数）
  | 只许往下用
L1 基础件 prim（Box/横排竖排/叠放/文字图标图片/底边圆角/裁剪透明变换）
  | 包住 ui/rendering，不写 GPU
L2 行为件 behavior（五态机/表单值校验/浮层定位外点关焦点锁/动效节拍）
  | 包住 gestures/focus/overlay/animation/scheduler/textinput
L3 产品组件（package kit，一组件可多文件，文件内分 Props/State/Render/Theme/Build 五段，只拼 L1+L2）
  | 禁直调 render/gpu，禁自写定位手势焦点
L4 组合全局（表单表格弹窗联调，App/配置往 Ctx 放东西）

一次渲染：Props + Ctx -> 拼树 -> 状态机 -> 布局 -> 绘制 -> 合成 -> 呈现
一次点击：命中（点哪高亮哪）-> 状态机 -> 标脏 -> 重排重画 -> 呈现
主题取值：这次写的 > 组件主题 > 全局种子，按状态解析，不写散装判断
```
