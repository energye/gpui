# Round2 收敛报告（5组重审 → 去重 → 判死 → 补丁）

> 5组全回：A架构一致性（3P0+4P1）/ B-F0与Flutter（7漏+2半+3划分+7波次）/ C-AB类（11新增，维持原判）/ D-CD类（日期颜色上传虚拟4升级+Host队列Tour4升级+铁律3扩）/ E-EF类（表格水印瀑布走马灯QR/Affix/Config6/宿主链+队列合一Tour单列）。
> 方法：重叠去重、矛盾判死、只留增量。补丁落 `ARCHITECTURE` / `WIDGET_MODEL` / `F0_FOUNDATION_PLAN` / `COMPONENT_SUPPORT_MATRIX`，本文只记录判死理由，不重复贴补丁内容。

## 1. 判死（P0先行）

- P0-1 L0层环：成立。`ARCHITECTURE §1` L0行“+各组件theme.go”删掉，L0只留`ui/theme`规范；各组件`theme.go`归L3。理由：import检查没法对一个文件判两层。
- P0-2 Ctx流向：成立。补“import只许往下，Ctx是运行时L4→L3数据流，L3只import scope读，不算返头”。理由：App/Config不写Ctx，L3无主题可读。
- P0-3 L3碰render三说法：判死为统一禁。L3只能import prim/behavior/scope+theme，出现render/gpu即违规；render公开API只许prim调；删“除非L1没包”例外，缺能力走先补L1。理由：门禁写不出来等于没门禁。
- 日期/颜色/上传/虚拟写小了（D组）：成立。日期从小件升日期引擎（GenerateConfig+解析格式化+locale+禁用矩阵+范围多选预设+受控面板）；颜色加渐变多stop+cleared+双回调+受控对象；上传加状态机+XHR语义+受控忽略+uid+maxCount+预览管线；虚拟加行高耦合+popupMatch关虚拟+过滤排序+搜索受控+无障碍降级。理由：源码不是几个函数，是整套生成器/聚合对象/状态机/任务队列。
- ModalHost焦点销毁（D组）：成立。Host加焦点圈地回焦自动聚焦+mask合并+memo销毁+destroyAll路由+Promise。理由：关了memo不更新、Form preserve=false全在FAQ里，不做关不掉。
- 队列Tour低估（D+E组去重一致）：成立。Message/Notification合一个队列层（taskQueue/GlobalHolder+三路全局+pauseOnHover+maxCountvsstack+Promise恰一次，Notification加四角池进度悬停固定宽）；Tour单独立项（RCTour委托+placements自适应+scrollIntoView+语义mask+zIndex+焦点+1001盖1000）。理由：两组独立得出同一拆法。
- Config补6件（E组）：成立。Ctx加挂载点2（popup/target）+同宽溢出2（popupMatch/Overflow）+空态1（renderEmpty）+静态1（holderRender），波纹进Motion、csp跳过维持。理由：对`config-provider.md §3`全表，7/13能盖，6件不补F类不成立。
- F0波次7处倒挂（B组）：成立。states/resolve进F0-1；Follow搬F0-4；Provider做实F0-1；边界开关规范前移；field/IME接口提前；depcheck提前；motion窗补规格。理由：空包撑不起后波，真窗承诺的paint_count/Hit≡绘在F0-2就要用。

## 2. 去重（只留一次）

- QR升大库：E+D一致，留一次（版本纠错掩码全套，和日期库同级）。
- Affix多源监听：E组 Affix多源 + B组附带三监听宿主链，合并为滚动/尺寸/可见性三宿主链路（Affix/Anchor/表头sticky共用，Masonry/Watermark复用）。
- 表格sticky/可替换体/ellipsis-fixed：E组3件，D组铁律里Modal हिस्सा不重，留E组3件。
- 水印守卫：E组可见性守卫（含onRemove+inherit+高置0藏匿）覆盖AUDIT一句Update替代，留守卫版。
- Carousel slick拆分：E组P0定时切+点指示先行、无限循环惯性延后，留此拆法。
- 语义拆两档（B组）：有名角色先行包一层，合并排除动作退回，覆盖AUDIT过严版。
- Tween随曲线补、 gesture前置门、焦点遍历时机、文字门（B组划分修正）：全收，覆盖AUDIT旧处置。
- AB组11新增：Typography剪贴板/编辑高、Avatar popover/图管/响应、Rate函数字符/悬停缩放、Slider反向/轨拖/动态节点/按柄禁用，全收进矩阵A/B行。
- 铁律3扩（D组）：受控扩全部value/open/current/fileList；拼写扩全部带输入框；键盘按下拉/面板/弹窗三类补。覆盖WIDGET旧3条，旧3条保留为特例。

## 3. 不收（说明理由）

- Container单建：不收。维持F0现场拼（Flutter本就是Stateless组合，无自有RO）。
- Message/Notification拆两项目：不收。合一队列层，Notification只加分流进度悬停。
- Tour非模态先行乐观：不收。mask=false仍绘洞，打洞逻辑必须先立项。
- 语义退回补完再用：不收。拆两档，先行有名角色。

## 4. 补丁索引（Round3落点）

- `ARCHITECTURE.md`：P0-1/P0-2/P0-3 + P1-1 canonical表 + P1-2 casing + 映射表Build行 + Ctx6件。
- `WIDGET_MODEL.md`：P0-3统一禁 + P1-3 Build唯一 + P1-4 Resolve合并点 + 铁律3扩。
- `F0_FOUNDATION_PLAN.md`：7漏2半 + 附带三批 + Tween/门/遍历/语义/文字 + 波次7调 + motion窗。
- `COMPONENT_SUPPORT_MATRIX.md`：AB11 + EF8 + CD8 + 队列合一Tour单列。
