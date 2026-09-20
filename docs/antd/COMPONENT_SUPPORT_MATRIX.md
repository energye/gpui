# antd 全量组件 × 架构支撑矩阵（源码实核版）

> 目标：回答“现架构能否撑起 antd 全部组件 1:1”。
> 方法：逐组件看三样——`docs/antd/<名>.md` 的 §1 外观 §2 功能 §3 API（产品真值，demo 数与 API 数见 §8 基数表）+ `ant-design/components/<名>/index.tsx` 主文件（机制：状态/动画/定位/表单/浏览器 API）+ `style/`（Token 面）。
> 架构标尺：L0 主题三级 / L1 prim / L2 behavior / L3 package kit（一组件可多文件，内分 Props/State/Render/Theme/Build 五段）/ L4 Ctx + ModalHost + 表单宿主。
> 结论三档：**撑得住**（L1+L2 直接拼）/ **有条件**（要补小件，见缺口列）/ **撑不住**（要新建大件，见缺口列）。旧 `ui/kit` 实现作废，不计入。

## 1. A 类基础显示（Stateless 拼装为主）

| 组件 | 结论 | 拼法（L1+L2） | 缺口 |
| --- | --- | --- | --- |
| Alert（25 demo/16 API：四态/可关/操作/滚动显隐） | 撑得住 | Decorated + Icon + Label + 操作行 + Interactive（关闭/展开）+ Motion（显隐） | 无结构缺口；滚动显隐走 Motion |
| Icon（11/7：双色/旋转/自定义） | 撑得住（L1 门面本身） | GlyphIcon（尺寸颜色走 Ctx）+ Motion（旋转） | F1 最先落地，供全库引用 |
| Form（50/23：注册/校验/联动） | 有条件 | Field 注册表 + 校验链 + 联动 | **缺**：校验时机 + 异步防竞态（见铁律）+ 滚到首错 + preserve 联动 |
| Typography（16 demo/25 API：省略/复制/编辑/多级标题） | 有条件 | Text/RichLabel + 按钮小件 +  Decorated | 省略多行（maxLines/ellipsis）走排版门面；复制编辑是小按钮回调，无缺口；**缺**：中日韩断行与装饰线（下划线/删除线 thickness）以排版单测为准 |
| Divider（12/12：横竖/虚线/带文） | 撑得住 | SizedBox + Center + Decorated（Border 画线，抄 Flutter `divider.dart:196` 同式） | 无 |
| Tag（17/9：预设色/可关/可点/边框） | 撑得住 | Decorated + Label + Icon（关闭）+ Interactive | 预设色走组件 Token，无缺口 |
| Badge（24/13：计数/圆点/状态/缎带/偏移） | 撑得住 | Stack + Positioned + Label/圆点 Decorated | 偏移量走 Token；滚动数字动画走 Motion 小件，无结构缺口 |
| Avatar（18/11：尺寸/形状/组/自动调字） | 有条件 | ClipOval/RRect + Picture + Label + Stack 组 | **缺**：组溢出数 + 文字自适应缩小（量字宽再缩），需排版量字门面 |
| Card（23/16：头/封面/操作/网格/嵌套） | 撑得住 | Column + Decorated + 头行 Row + 网格 Row | 无，嵌套即递归拼 |
| Empty（18/4：图/文/脚） | 撑得住 | Column + Picture + Label + 插槽 | 无 |
| Result（21/7：状态图/标题/副/操作） | 撑得住 | 同 Empty + 状态图标 | 无 |
| Statistic（15/13：数字/前后缀/倒计时） | 有条件 | Label + Row + Ticker（`statistic/Timer.tsx` 有计时） | **缺**：倒计时 ticker 驱动 + 数字滚动，以 Motion 计时小件补 |
| Descriptions（20/12：行列/边框/响应式） | 撑得住 | Table 式 Row + Label（非虚拟） | 响应式列数走 Ctx 断点（抄 `responsiveObserver`），无结构缺口 |
| Timeline（20/16：交替/自定义点/幽灵） | 撑得住 | Column/Row + 线 Decorated + 点 Icon | 交替走方向，无缺口 |
| QRCode（19/12：canvas/svg/状态/边框） | 有条件 | CustomPaint 画码块 + Decorated 边框 + 状态遮罩 | **缺**：二维码编码库（纯 Go，需引入或自写，不在 F0 内）；`@rc-component/qrcode` 的 canvas/svg 双通道以 CustomPaint 一套实现 |
| Image 展示态（25/16：占位/错图/遮罩） | 撑得住 | Picture 三态（占位→图→错）+ 遮罩 Decorated | 预览浮层见 D 类；渐显（FadeInImage）走 Motion 透明，无缺口 |

A 类共性：全部可拼；缺口只有三小件——排版量字（Avatar 自适应）、计时驱动（Statistic 倒计时）、二维码编码（QRCode 新库）。

## 2. B 类基础交互（Stateful + 状态查表）

| 组件 | 结论 | 拼法 | 缺口 |
| --- | --- | --- | --- |
| Button（27/16：六变体/三色/三尺寸/三形状/幽灵/加载/波纹） | 撑得住（标杆） | Decorated + Row（图标文）+ Interactive + Motion（波纹转圈）+ Focus + 语义 | type 糖按 `ButtonTypeMap` 转，无缺口 |
| Switch（14/8：开合/加载/自定义柄） | 撑得住 | Decorated 槽 + 柄 Transform + Interactive + Motion 位移 | 无 |
| Checkbox（19/7：勾选/半选/组） | 撑得住 | Decorated 框 + GlyphIcon 勾 + Interactive + 组 Field | 半选走 StateResolver 加一态，无缺口 |
| Radio（23/11：单选/组/按钮式） | 撑得住 | 同 Checkbox，圆框 + 组单选 | 无 |
| Rate（20/5：半星/允许清除/字符） | 有条件 | Row 星 + Interactive（悬停预览）+ 半星 Clip | **缺**：半星命中（左半右半，中间8px缝不命中，键盘0.5步进）L2 加半区命中；P1 函数字符+对象文案（图标管线+浮层）；悬停1.1倍像素级走 Motion |
| Slider（27/14：单双/纵/刻度/气泡） | 有条件 | 轨 Decorated + 柄 Transform + 拖 Pan + 焦点键盘 | **缺**：拖动吸附（step-null落刻度/dots/marks）+ 纵向双柄 + 反向reverse + 范围轨拖 + 动态节点editable/min/max + 按柄禁用数组；`document.` 监听以引擎指针替代 |
| Typography（补，见 A 类） | 有条件 | 见 A 类 | **缺**：剪贴板宿主（函数/异步文案+format mime）+ 编辑框自适应高（autoSize/maxLength） |
| Avatar（补，见 A 类） | 有条件 | 见 A 类 | **缺**：组popover（+N走D类）+ 图管srcSet/crossOrigin + 响应尺寸map |
| Segmented（24/9：块/大/竖/图标） | 撑得住 | Row 块 + 滑块 Transform + Interactive | 滑块位移动画走 Motion，无缺口 |
| FloatButton（19/10：组/回到顶/波纹） | 撑得住 | 圆 Decorated + Icon + Interactive + 组 Stack | 回到顶走滚动宿主 `scrollTo`，无缺口 |

B 类共性：全部可拼；缺口见上表（Rate 半区/函数字符、Slider 六项、Typography/Avatar 补项），无浏览器-only误判。

## 3. C 类输入受控（Field + 输入法 + 浮层下拉）

铁律见 `WIDGET_MODEL.md` §6（三条全件铁律），C 类特例：拼写中 composing 只显示不算值。

| 组件 | 结论 | 拼法 | 缺口 |
| --- | --- | --- | --- |
| Input（30/18：受控/清除/前后缀/计数/密码/搜索/多行/OTP/自适应高） | 撑得住 | Field + IME 会话（Editor+排版+会话，焦点驱动）+ 后缀 Icon | 多行自适应高（min/maxRows）走排版量高；OTP 为多框联动（焦点自动跳），以 Field 组补；无结构缺口 |
| InputNumber（26/14：步进/精度/键盘/格式化） | 撑得住 | 同 Input + 步进按钮 + 解析格式化 | 精度与步进算法为纯函数，无缺口 |
| Select（40/24：单多/标签/搜索/虚拟/分组/标签收起） | 有条件 | Field + OverlayTrigger 下拉 + 虚拟列表 + 标签行 | **缺**：多选标签收起 + 虚拟行高耦合（listHeight256/行24）+ popupMatch=false关虚拟 + 过滤排序OR + 搜索受控 + responsive损耗 + 无障碍降级（virtual=false绑真元素）+ `bind_count` 门禁 |
| AutoComplete（23/7：等 Input+联想） | 撑得住 | Input + 联想浮层（Select 简化） | 搜索框拼写铁律同 Select |
| Mentions（25/11：@人/联想/前后缀） | 撑得住 | 同 AutoComplete + 触发符解析 | 触发符解析纯函数，无缺口 |
| Cascader（33/19：多级/搜索/懒加载） | 有条件 | Field + 多列浮层 + 树数据 | **缺**：懒加载（loadData 异步子节点）+ 多列联动，以树数据异步小件补 |
| TreeSelect（25/23：等 Tree+Select） | 有条件 | Select 壳 + Tree 体 | 同 Cascader，树异步 + 勾选联动，以 E 类树补 |
| DatePicker（38/21：面板/范围/预设/确认） | 有条件 | Field + 日历浮层（年月头+星期+格） | **缺**：日期引擎（GenerateConfig可插拔+多格式format+locale/周起始/佛历+disabled矩阵+Range/多选/order/预设函数+受控面板mode/pickerValue） |
| TimePicker（28/12：滚轮/步进/范围） | 有条件 | Field + 时间滚轮浮层 | **缺**：滚轮列（时分秒列 + 步进禁用）以滚轮小件补；与 Date 共日期引擎 |
| ColorPicker（27/23：面板/渐变/预设/触发） | 有条件 | Field + 颜色浮层（饱和面板+滑条+输入） | **缺**：颜色模型（渐变多stop+cleared+双回调onChange/Complete+受控对象精度+format/disabled三开关+panelRender）+ 饱和面板拖拽 |
| Upload（34/11：队列/拖放/手动/图片墙） | 有条件 | Field + 文件队列 + 列表行 | **缺**：上传语义（LIST_IGNORE+beforeUpload三返回+受控忽略+uid补齐+maxCount替换截断+defaultRequest+预览管线previewFile/isImageUrl）+ 宿主三件（选文件/拖放/粘贴）；网络本身不在 F0 内 |

C 类共性：Field + IME + 浮层三件套全覆盖；缺口是日期引擎/颜色模型/上传语义/树异步四个大件 + Select 虚拟耦合门禁。

## 4. D 类浮层触发（OverlayTrigger + ModalHost + 队列）

D 组子结论（`index.tsx` 实核，见任务回执）：气泡三件可直接拼，其余七件要补新东西。

| 组件 | 结论 | 拼法 | 缺口 |
| --- | --- | --- | --- |
| Tooltip/Popover/Popconfirm（20/19/22） | 撑得住 | B/C 触发 + OverlayTrigger（十二向/翻转/箭头/外点关/Esc/延时） | 右键 contextMenu 对齐点（alignPoint）以触发器补；`fresh` 缓存语义钉死 |
| Dropdown（30/7：一级/右键/多级） | 有条件 | 同上 + 菜单内容 | **缺**：多级子菜单级联定位 + hover 延时链（P0 只收一级，多级延后）；划词 selection 位置注入 |
| Menu 弹出态（23/16：内嵌/横竖/溢出） | 有条件 | 内嵌先行 + 弹层走浮层宿主 | **缺**：弹层挂载（popupOffset/popupRender）+ 折叠 Tooltip 代偿 + 多级 hover 延时 |
| Modal（30/18：遮罩/脚/确认框/命令式） | 有条件 | 浮层居中遮罩 + 焦点圈地 + Motion 进出 + ModalHost | **缺**：App 级 ModalHost（栈/z1000/嵌套/confirm+contextHolder/update/destroy/destroyAll/Promise/focusable回焦自动聚焦/mask合并/memo销毁/路由/scrollLock） |
| Drawer（26/30：四向/多层/可调） | 有条件 | 同 Modal + 侧滑 | **缺**：同 ModalHost + 可调拖拽（onResize 系）+ 容器内渲染（局部宿主） |
| Message（21/11：队列/倒计时/更新） | 撑不住（合队列层立项） | 单条 Entry 可挂 | **缺**：taskQueue/GlobalHolder+三路全局+pauseOnHover冻结+maxCountvsstack+Promise恰一次（见 D 队列层） |
| Notification（25/17：六方位/进度/悬停暂停） | 撑不住（同队列层） | 同上 | **缺**：同队列层 + 四角独立池 + 进度条 + 悬停暂停 + 固定宽 |
| Tour（18/12：分步/遮罩/高亮） | 撑不住（单独立项） | 非模态仅参考 | **缺**：RCTour委托+placements自适应+scrollIntoView+语义mask+zIndex上下文+焦点管理+1001盖1000+mask=false仍绘洞 |

D 类共性缺口（最大的一组）：ModalHost 弹框栈、通用 OverlayTrigger 触发件、消息队列层、Tour 遮罩打洞洞、四缺一不可。

## 5. E 类结构导航（L1 排版 + 虚拟宿主）

| 组件 | 结论 | 拼法 | 缺口 |
| --- | --- | --- | --- |
| Layout/Grid/Flex/Space（16/18/9/18） | 撑得住 | Row/Column/Constraint/Gutter/断点 | Sider 折叠动画走 Motion；断点（`responsiveObserver`/`useBreakpoint`）走 Ctx |
| Splitter（17/13：拖拽/折叠/懒） | 有条件 | Row/Column + 柄 Interactive 拖 | **缺**：拖拽分栏（`useResizable/useResize/useSizes` 对等：尺寸分配+折叠+懒更新）+ RTL 反向 |
| Affix（9/4：固钉/占位） | 有条件 | 占位 SizedBox + 固定 Offset | **缺**：多源监听（scroll/resize/touch/pageshow/load）+ 目标容器注入 + 水平不支持 + resize跟随；与 Anchor/表头sticky共滚动宿主链路 |
| Anchor（21/5：滚动联动高亮） | 有条件 | 纵排链接 + 内容滚动 | **缺**：滚动位置回读 + 联动高亮（AnchorLink/Anchor 对） |
| Breadcrumb（20/5：溢出/下拉） | 撑得住 | Row + 分隔 + 省略 | 溢出省略走排版，分支下拉走 D 类 |
| Pagination（24/12：页码/省略/跳页） | 撑得住 | Row 页码 + Field 跳页 | 页码省略纯函数，无缺口 |
| Steps（25/22：横竖/点/进度） | 撑得住 | Row/Column + 连接线 + 状态查表 | 无 |
| Tabs（27/15：滑动线/溢出/增删） | 有条件 | 条 Row + 内容视口 + 指示 Transform | **缺**：滑动下划线动画 + 条溢出滚动（`@rc-component/tabs` 对等行为） |
| Calendar（21/7：年月头/面板切换） | 有条件 | 头 Row + 格 Grid + 面板 | **缺**：完整日历头（年月切换+星期+月年面板）+ 日期库（同 DatePicker） |
| Table（55/29：列/分页/选/固定/虚拟/展开/排序筛） | 有条件 | 标准包法（分页外/视口中/固定列钉两侧/虚拟管体行）+ 头体 | **缺**：列宽同步 + 头体同滚 + 固定列阴影 + sticky粘头滚动容器 + components行体可注入 + ellipsis-fixed布局 + 虚拟+展开+选择联动 |
| List（25/17：虚拟/分页/栅格） | 撑得住 | 虚拟宿主 + 分页条 | 无 |
| Tree（24/17：勾选/拖拽/虚拟/异步） | 有条件 | 虚拟列 + 行 Interactive | **缺**：勾选联动（父子半选）+ 拖拽（`Draggable/DragTarget` 对等，F0 延后）+ 异步加载 |
| Transfer（23/14：两栏/穿梭/搜索） | 有条件 | 两 List + 中按钮 + 选择集 | **缺**：选择联动 + 穿梭算法（纯函数）+ 搜索过滤 |
| Collapse（23/11：手风琴/高度动画） | 有条件 | Column + 头 Interactive + 体视口 | **缺**：高度动画（Motion 高度补间） |
| Carousel（18/3：自动播/拖/点） | 有条件 | 视口 + Pan + 点指示 | **缺**：P0 定时切+点指示先行；无限循环/滑动惯性延后（react-slick子集）；拖禁态 |
| QRCode（补，19/12） | 有条件 | CustomPaint 画码块 + 边框 + 遮罩 | **缺**：编码大库（版本/纠错/掩码全套，和日期库同级） |
| Spin（21/10：转圈/容器/延时） | 撑得住 | 圆环 CustomPaint + Ticker + 遮罩 | 延时（delay 到才转）同 Button，无缺口 |
| Skeleton（12/4：占位/闪光/容器） | 撑得住 | 占位 Decorated + 闪光 Motion | 闪光走独立重画，无缺口 |
| Watermark（17/17：平铺/防删） | 有条件 | CustomPaint 平铺 + 内容 Stack | **缺**：平铺算法（字行量宽+行列+像素比）+ 可见性守卫（防盖/摘/透明/高置0，摘除调onRemove+inherit传导Modal/Drawer）+ 图片水印（`ui/io`） |
| Progress（28/15：条/圈/仪表/步骤） | 撑得住 | 条 Decorated + 圈 CustomPaint + Ticker | 渐变走 Decorated 渐变，无缺口 |
| Masonry（10/4：瀑布/响应列） | 有条件 | CustomLayout（`usePositions` 对等） | **缺**：瀑布分配（按列高贪心+响应列数+`onLayoutChange`）+ 量高回读 + fresh=true持续监听脏标记 |
| BorderBeam（17/5：自有扩展，非 antd 官方） | 撑得住 | Decorated 光边 + Motion 位移动画 | 以自有 Token 为准，不占用 antd 对齐口 |

E 类共性缺口：拖拽分栏、滚动联动（Affix/Anchor/表头）、高度/下划线/走马灯动效、瀑布分配、水印平铺、树表联动，全部是“行为小件”，不碰 GPU。

## 6. F 类全局（Ctx 下发）

| 组件 | 结论 | 拼法 | 缺口 |
| --- | --- | --- | --- |
| ConfigProvider（19/9：主题/尺寸/方向/语言/波纹/虚拟/前缀） | 有条件 | Ctx 全字段（见 WIDGET §5，含挂载点/同宽溢出/空态/静态/variant） | **缺**：getPopupContainer/getTargetContainer/popupMatch/Overflow/renderEmpty/holderRender 补6件；波纹进Motion；csp跳过 |
| App（7/3：消息/弹窗/通知上下文） | 有条件 | Ctx + ModalHost + 消息队列宿主 | 依赖 D 类 Host/队列先落地；holderRender 同队列层落地 |
| Locale（语言包） | 撑得住 | Ctx Locale + 文案表 | 空态日期文案走表，无缺口 |
| Util（无 UI，纯函数） | 撑得住 | 不进 kit，按 §6 做类型对照 | 无 |
| Theme（种子切换/算法/紧凑） | 撑得住 | 换 Seed/Component 重跑 Resolve | 深色紧凑以 Token 全量为准 |

## 7. 总结论

- **撑得住**（直接拼，约 30 个）：Alert/Divider/Icon/Tag/Badge/Card/Empty/Result/Descriptions/Timeline/Button/Switch/Checkbox/Radio/Segmented/FloatButton/Input/InputNumber/AutoComplete/Mentions/气泡三件/Layout/Grid/Flex/Space/Breadcrumb/Pagination/Steps/List/Spin/Skeleton/Progress/Locale/Util/Theme/BorderBeam。
- **有条件**（补小件后拼，约 38 个）：Typography/Avatar/Statistic/QRCode/Rate/Slider/Select/Cascader/TreeSelect/DatePicker/TimePicker/ColorPicker/Upload/Dropdown/Menu弹出/Table/Calendar/Tabs/Tree/Transfer/Collapse/Carousel/Watermark/Masonry/Splitter/Affix/Anchor/Modal/Drawer/ConfigProvider/App/Form（见上表）。
- **撑不住直接拼**（G1–G7 七个大件，已补进 F0 §6.5 单独立项，不阻塞其他 60+ 个）：G1 弹窗宿主（Modal/Drawer）、G2 消息队列（Message+Notification合一项）、G3 漫游打洞（Tour）、G4 日期引擎、G5 颜色模型、G6 上传语义、G7 二维码编码库。
- 无结构性返工：所有缺口都是 L1/L2/F0 范围内的行为小件 + 日期引擎/颜色模型/上传语义/二维码编码四个大库 + 滚动/尺寸/可见性三宿主链路，没有要推翻分层和三件套的项。`border-beam` 为自有扩展，不计 antd 对齐口；`back-top`（源码有、文档无）按 FloatButton 回到顶能力覆盖，不单列。
