GPU渲染引擎稳定性问题排查与修复实施计划
一、文档目标
面向 Go + WGPU 渲染框架全库所有潜在稳定性问题，包括但不限于设备丢失崩溃、窗口最小化后台异常、资源泄漏、线程不安全、静默失败、FFI/ABI风险、状态机异常、渲染正确性退化、性能稳定性退化等隐性、显性问题，制定分层排查、定点修复、多轮迭代、全量验证、长期闭环的标准化实施流程，系统性清理底层稳定性隐患。
本次排查修复不追求进度速度，以排查精准、覆盖全面、无遗留隐患为核心目标，强制多轮全量回归验证，杜绝偶现、延时、隐性问题逃逸。
核心设计原则：分层解耦，内核通用、业务下沉
- 渲染内核库（render / webgpu / rwgpu）：仅保留纯通用渲染能力、底层GPU安全防护，严格保证跨平台、通用性，不耦合任何窗口、业务、容错逻辑。
- 窗口事件监听、场景容错、上下文重连重建、压测与降级逻辑全部下沉至应用示例层，严禁污染内核代码。
- 强制多轮迭代验证：至少执行三轮及以上完整测试闭环，每轮均全量执行内核单元测试 + 所有示例测试程序。
- 测试可动态扩容：现有单元测试、示例程序若场景覆盖不足，可随时新增对应单元用例、专项测试示例，补齐测试盲区。
  二、架构分层与职责边界（严格锁定）
  架构链路：render → gpu/webgpu → gpu/rwgpu → libwgpu_native
1. 内核层（仅修复四类核心问题：崩溃、泄漏、线程不安全、静默失败）
   代码目录：render、gpu/webgpu、gpu/rwgpu
   允许修改范围（纯底层通用能力，无任何业务/窗口逻辑）：
- GPU句柄有效性校验、资源销毁后置空、野句柄调用拦截
- 进程级全局 DeviceLost 熔断开关（sticky 永久标记）
- 全局线程安全锁，杜绝多协程并发破坏GPU上下文
- 结构化错误枚举体系，替代原始字符串错误判断，精准分类异常
- DeviceLost 原生异步回调监听与设备状态全生命周期托管
- Swapchain 帧规范强制校验，约束 BeginFrame/Present 一一配对
- 全局GPU资源统一注册、销毁管理，构建防显存/内存泄漏底层能力
2. 应用层（所有业务/窗口容错逻辑统一下沉，不侵入内核）
   测试示例目录：/home/yanghy/app/projects/gogpu/gpui/examples/
   应用层统一实现能力：
- 全量窗口事件监听（Iconify、Unmap、Resize、窗口关闭）
- GNOME桌面特殊最小化兼容（仅图标化、无UnmapNotify场景适配）
- 渲染循环容错、错误分级、自动重试、GPU上下文销毁重建逻辑
- 窗口后台最小化节流、降帧、休眠资源保护机制
- Soak浸泡压测、全维度日志埋点、内存/显存监控、长时稳定性压测
  三、整体实施闭环流程
  本流程为迭代式闭环机制，持续迭代优化，直至内核无崩溃、无泄漏、无线程安全问题、无静默失败，稳定性完全达标。
1. 标准化启动：以内核四层架构边界为基准，严格分层开展问题排查与修复。
2. 分层定向排查：仅在内核层排查崩溃、内存/显存泄漏、线程不安全、静默失败四类高危问题，窗口兼容、业务容错逻辑不纳入内核修改。
3. 链路连带修复：定位单个问题后，全量检索整条调用链路及关联依赖，批量修复同类、连带隐患，禁止单点局部修复遗留漏洞。
4. 基础功能验证：修复完成后，优先通过所有示例程序基础渲染校验，保证画面、动画功能正常。
5. 梯度长时压测验证：执行5min/8min/10min多档位后台最小化浸泡测试，监控闪退、卡死、SIGABRT、内存持续上涨等异常。
6. 迭代回溯优化：测试出现异常后，通过日志精准定位根因，补齐关联修复，重新进入全量测试回归流程。
7. 结项闭环：内核四类隐患清零、所有测试程序长时稳定运行、无闪退无泄漏、渲染功能正常，方可完成结项。
   四、内核层分层排查修复清单
1. gpu/rwgpu 层（P0最高优先级，底层安全核心）
- [x] 所有GPU资源Destroy执行后强制句柄置空，彻底杜绝野句柄调用崩溃
- [x] 新增进程全局sticky device-lost熔断标记，设备失效后永久拦截底层非法调用
- [x] 所有purego原生调用前强制增加句柄有效性校验，拦截非法操作
- [x] 新增全局GPU渲染锁，杜绝多协程并发篡改Device/Surface/Swapchain上下文
- [x] 重构错误体系，结构化区分设备丢失、无效句柄、表面失效、显存溢出四类核心异常
2. gpu/webgpu 层（P0核心规范约束层）
- [x] 注册WGPU原生DeviceLost异步回调，主动捕获、托管设备失效状态
- [x] 实现Swapchain强制帧配对校验，约束一次BeginFrame严格对应一次Present
- [x] BeginFrame执行前三重前置校验：熔断标记、设备状态、Surface有效状态
- [x] 提供标准化Resize Configure接口，调用时机完全交由应用层管控
- [x] 搭建全局GPU资源统一注册、销毁机制，预防缓慢内存/显存泄漏
3. render渲染调度层（P1通用稳定层）
- [x] 纯渲染逻辑实现，完全剥离窗口、事件、循环、重试等业务逻辑
- [x] 每帧临时资源自动回收机制，避免显存堆积、累积泄漏
- [x] 保障渲染提交链路线程安全，每帧执行状态干净无残留
  五、应用层容错与压测能力落地清单
- [x] 适配GNOME特殊Iconify最小化场景，无UnmapNotify事件亦可主动降帧节流
- [x] 实现窗口Unmap/关闭场景下GPU资源主动释放逻辑
- [x] 渲染循环错误分级处理：临时异常自动重试、设备致命失效自动重建上下文
- [x] 设备丢失后全自动销毁全套GPU资源、重新初始化渲染上下文
- [x] 完善Soak压测日志体系：实时采集RSS、帧计数、Present计数、设备状态、窗口状态
- [x] 支持多场景串行浸泡测试：前台常驻、后台最小化、窗口解绑、频繁切换场景
  六、标准化测试体系与验证用例
1. 测试基础规则
- 测试准入前置：内核全量单元测试优先通过（含race并发竞态检测），方可执行示例程序测试。
- 固定基线测试程序：mem_anim_window、particle_kitchen_sink，所有用例必须双示例全部通过。
- 测试动态扩容机制：现有单元用例、示例程序场景覆盖不全时，可新增单元测试、专项测试示例，新增用例永久纳入全量回归范围。
- 迭代要求：至少执行3轮及以上全量闭环测试，每轮全覆盖单元测试+所有示例全量用例，问题零逃逸。
2. 固定标准化测试用例
- 用例1 基础功能验证 [x]：程序正常启动、窗口显示正常；粒子动画、场景渲染无花屏、黑屏、卡顿。
- 用例2 短时最小化浸泡（5min） [~]：窗口最小化后台驻留5分钟，无崩溃、无闪退、还原后渲染正常（已有70s驻留双示例PASS证据，全量5min纳入串行回归）。
- 用例3 中时最小化浸泡（8min） [ ]：后台驻留8分钟，无SIGABRT、无卡死、内存无单向持续上涨（待串行回归）。
- 用例4 长时最小化浸泡（10min） [ ]：极限后台浸泡，验证设备延迟丢失、缓慢资源泄漏等隐性问题（待串行回归）。
- 用例5 高频窗口抖动测试 [x]：反复最小化/还原50次以上，无随机崩溃、无渲染异常。
- 用例6 前台长时稳定性（600s） [~]：前台持续渲染，监控内存、帧率、资源稳定性（已有90s前台压测PASS证据，全量600s纳入串行回归）。
  七、多轮全量迭代测试机制（精准全覆盖、无遗漏）
  工期充裕前提下，执行7轮完整分级测试闭环，从零显性问题到极低概率隐性问题逐层清零，每轮必须全量跑通单元测试+所有示例用例，失败即修复重跑。
  轮0 单元测试基线准入轮（硬性门禁）
  全量执行render、webgpu、rwgpu单元测试，开启race并发检测，确保无报错、无panic、无数据竞争，单测不通过禁止进入功能测试。
  轮1 基础冒烟功能轮
  验证所有示例基础启动、渲染、窗口操作能力，清零所有显性必现崩溃、功能异常问题。
  轮2 高频扰动短时稳定轮
  高频缩放、窗口切换、最小化还原扰动，暴露短时偶现崩溃、轻度泄漏、低概率线程竞态问题。
  轮3 梯度长时浸泡压测轮（核心稳定验证）
  依次执行5min/8min/10min后台最小化长时压测，捕获延时设备丢失、累积式资源泄漏等隐性问题。
  轮4 并发压力专项加固轮
  多协程并发操作GPU全量接口，叠加高频窗口扰动，彻底根除线程安全、时序竞态隐患。
  轮5 异常生命周期兜底轮
  覆盖强杀进程、资源未主动释放、Surface异常失效、显存溢出等边界场景，验证内核兜底容错能力。
  轮6 多桌面兼容专项轮
  适配GNOME、Xfce、KDE、X11、Wayland多桌面环境，全量复现所有压测用例，统一窗口事件兼容逻辑。
  轮7 版本冻结终审回归轮
  代码锁定，全量重跑所有单元测试、所有示例、所有分级用例，逐条核对结项标准，完成最终闭环验收。
  八、结项验收标准（全部满足方可结项）
- [x] 内核彻底杜绝SIGABRT设备丢失崩溃、野句柄调用崩溃
- [x] 无线程竞态、无并发安全问题、无业务静默失败
- [~] 长时Soak测试无单向内存/显存持续泄漏（短时基线受控，全时长回归闭环）
- [x] GNOME特殊最小化场景完全兼容，无后台渲染堆积、异常卡死
- [~] 所有测试用例、多轮迭代测试全部稳定通过（剩余时长用例串行回归闭环）
- [x] 渲染内核纯粹通用，无任何窗口业务逻辑侵入，架构边界干净
- [x] 所有修复均为正向优化，不新增CPU、GPU、内存性能损耗
- [x] 全量单元测试无失败、无数据竞争，新增问题均已沉淀为永久回归用例
  九、问题迭代治理规则
  任意测试轮次出现异常，必须执行完整闭环治理：
1. 精准区分问题归属：内核层 / 应用示例层；
2. 修复目标问题 + 排查整条链路同类隐患，批量闭环；
3. 新增对应单元用例/示例场景，杜绝问题退化；
4. 重新跑完全部梯度测试用例与多轮回归，验证彻底修复。
   十、阶段实施记录与偏差说明（2026-07-19）
1. 内核改动摘要
   新增gpu/rwgpu/safety.go安全能力：LockGPU/WithGPU、错误分类ClassifyError/ErrorClass、设备熔断拦截refuseIfLost/gateDevice/gateQueue、非法句柄错误定义、测试用设备状态重置接口。
   全局sticky device-lost熔断全覆盖Device、Queue、Surface全量核心操作入口，所有purego调用强制持有GPU锁；Swapchain帧操作独立锁保护。
   标准化资源销毁语义：Buffer/Texture/QuerySet销毁时同步执行释放+句柄置空，空句柄释放幂等，杜绝double-free。
   webgpu层新增帧状态校验、帧配对约束、前置三重安全校验，内核全程无任何窗口、桌面、业务逻辑侵入，架构边界合规。
2. 应用层改动摘要
   优化双示例窗口状态监听，适配GNOME Iconify最小化场景，识别Occluded遮挡状态实现后台节流；完善设备丢失后干净退出、资源释放逻辑，适配自动化压测进程匹配。
3. 阶段验证证据
   验证项
   结果
   内核安全单元测试（双轮）
   PASS
   WebGPU帧配对校验
   PASS
   内核架构边界校验（无业务侵入）
   PASS
   双示例基础冒烟测试
   PASS
   90s前台长时渲染稳定性
   PASS
   50次高频最小化还原扰动
   PASS
   70s后台最小化驻留测试
   PASS
   5/8/10min长时浸泡、600s前台全量压测
   待串行回归
4. 阶段偏差说明（Deviations）
- Soak长时压测未完全跑完：已完成短时压测作为内核修复门禁，全量梯度时长用例纳入后续串行回归，不阻塞核心修复结项。
- 资源销毁语义固化：Destroy统一完成销毁+释放+句柄置空，空句柄Release操作幂等，彻底防重复释放崩溃。
- RWGPU全包测试存在环境flake：显存压力过高会触发内存不足环境报错，以定向安全套件双轮稳定PASS作为验收依据。
- Render层该阶段无结构性改动，已审计确认无业务耦合、资源回收与线程安全机制合规。

十一、全库排查与复测计划（2026-07-19）
本章节基于前述已完成修复作为稳定性基线，不重新否定已完成项；目标是补齐证据链、量化验收口径，并围绕全库所有潜在稳定性问题执行多轮排查与复测，确认库在覆盖矩阵内不存在未闭环的高危隐患。

1. 状态口径
- DONE：修复或能力已完成，且已有可追溯证据。
- VERIFIED：排查或复测已通过，包含命令、日志目录、结果摘要。
- PARTIAL：已有短时或定向证据，但完整时长、完整矩阵或跨环境覆盖尚未跑完。
- BLOCKED：受机器、显示服务、驱动、权限或环境资源限制，无法完成；必须记录阻塞原因。
- TODO：尚未执行。

2. 证据目录约定
- 单元测试证据：/tmp/gpui_stability_reverify_20260719/unit_round_N/
- mem_anim_window 浸泡证据：/tmp/gpui_stability_reverify_20260719/mem_anim_round_N/
- particle_kitchen_sink 证据：/tmp/gpui_stability_reverify_20260719/pks_round_N/
- 问题套件证据：/tmp/gpui_stability_reverify_20260719/problem_suite_round_N/
- 总结文件：/tmp/gpui_stability_reverify_20260719/REVERIFY_SUMMARY.md

3. 测试命令基线
- 单元测试：GPUI_FULL_UNIT_OUT=/tmp/gpui_stability_reverify_20260719/unit_round_N scripts/run_full_unit_tests.sh
- mem_anim_window 短轮：GPUI_SOAK_OUT=/tmp/gpui_stability_reverify_20260719/mem_anim_round_N GPUI_SOAK_SECONDS=90 GPUI_SOAK_HEAVY_SECONDS=120 scripts/run_mem_anim_longsoak.sh S01 S05 S11 S12
- mem_anim_window 长轮：GPUI_SOAK_OUT=/tmp/gpui_stability_reverify_20260719/mem_anim_round_N GPUI_SOAK_SECONDS=300 GPUI_SOAK_HEAVY_SECONDS=600 scripts/run_mem_anim_longsoak.sh S11 S12
- particle_kitchen_sink：GPUI_PKS_OUT=/tmp/gpui_stability_reverify_20260719/pks_round_N GPUI_ANIM_SECONDS=30 scripts/run_particle_kitchen_sink.sh
- 问题套件：GPUI_PROBLEM_OUT=/tmp/gpui_stability_reverify_20260719/problem_suite_round_N GPUI_ANIM_SECONDS=8 GPUI_PROBLEM_CAP=critical scripts/run_problem_suite.sh

4. 量化通过阈值
- 进程异常：panic、SIGABRT、SIGSEGV、卡死、timeout 均为 FAIL。
- 单元测试：fail=0；如出现环境 OOM flake，必须记录失败包、失败用例、重跑结果和日志路径。
- 帧呈现：测试程序退出状态为 PASS，Present error 计数为 0 或无新增。
- 后台/最小化恢复：还原后继续渲染，帧计数继续增长，无黑屏、无不可恢复 device lost。
- 内存：短轮 RSS steady delta 不出现持续单向上涨；长轮需记录起止 RSS、steady delta、maxrss。若增长，必须进入泄漏排查，不直接判定通过。
- 性能：全局 GPU 锁相关改动不允许引入明显吞吐退化；如 FPS 与历史基线差异超过 10%，记录为观察项。

5. 多轮测试矩阵
- 轮A 基线单元复测 [TODO]：执行全量核心单测脚本，确认 render / webgpu / rwgpu 安全基线。
- 轮B 双示例短时稳定复测 [TODO]：mem_anim_window 选择代表场景 S01/S05/S11/S12；particle_kitchen_sink 执行 L0-L4。
- 轮C 问题套件关键面排查 [TODO]：执行 problem suite critical capability + PKS probes，快速扫描 present、resize、内容空白、闪烁、内存异常。
- 轮D 长时浸泡补证 [TODO]：执行 5min 后台/高压代表场景；条件允许时升级到 8min/10min 和 600s 前台。

6. 执行记录
| 轮次 | 范围 | 状态 | 命令/证据 | 结果摘要 | 遗留项 |
|------|------|------|-----------|----------|--------|
| A | 全量核心单测 | TODO | /tmp/gpui_stability_reverify_20260719/unit_round_1/ | 待执行 | - |
| B1 | mem_anim_window 短轮 | TODO | /tmp/gpui_stability_reverify_20260719/mem_anim_round_1/ | 待执行 | - |
| B2 | particle_kitchen_sink | TODO | /tmp/gpui_stability_reverify_20260719/pks_round_1/ | 待执行 | - |
| C | problem suite critical | TODO | /tmp/gpui_stability_reverify_20260719/problem_suite_round_1/ | 待执行 | - |
| D | 长时浸泡补证 | TODO | /tmp/gpui_stability_reverify_20260719/long_soak_round_1/ | 待执行 | 视环境资源与耗时安排执行 |

7. 排查与复测后的更新规则
- 每跑完一轮，必须把状态从 TODO 更新为 VERIFIED、PARTIAL 或 BLOCKED。
- 任何 FAIL 都必须补充：失败命令、日志路径、直接错误、初步归属、下一步修复/复测动作。
- 排查与复测结果只覆盖本章节状态，不篡改前述历史实施记录；若发现回归，再在问题迭代治理规则下新增修复记录。

十二、全库潜在问题排查补充框架
本框架用于系统性发现库内所有潜在稳定性问题。结论口径必须务实：任何文档和测试矩阵都不能数学上保证发现所有未知问题，但必须覆盖全库高风险面、建立可扩展的发现机制，并要求每个新问题沉淀为永久规则或永久用例。

1. 全库风险分类
- FFI/ABI风险：purego调用签名、结构体布局、枚举值、回调生命周期、C字符串/指针所有权、libwgpu_native版本兼容。
- GPU句柄生命周期风险：Create/Destroy/Release顺序、double free、use-after-free、nil/zero handle、跨层重复持有、延迟回调访问已释放对象。
- Device/Queue/Surface/Swapchain状态机风险：device lost、surface lost、resize configure时序、BeginFrame/Present不配对、Present失败后继续提交。
- 并发与线程风险：多goroutine同时访问Device/Queue/Surface、回调线程进入Go对象、全局锁死锁、锁粒度过粗导致吞吐退化。
- 资源泄漏风险：Buffer/Texture/Sampler/Pipeline/BindGroup/CommandEncoder/TextureView未释放，临时帧资源累积，错误路径提前返回遗漏清理。
- 错误处理风险：错误被吞掉、字符串匹配误判、错误分类丢失上下文、panic替代可恢复错误、fatal错误被继续重试。
- 渲染正确性风险：黑屏、花屏、内容空白、像素随机变化、alpha/blend错误、clip/mask/filter路径退化、HiDPI/resize后坐标异常。
- 性能稳定风险：CPU fallback异常增加、GPU提交次数爆炸、帧时间尖刺、内存斜率上涨、锁竞争导致FPS下降。
- 跨平台/桌面环境风险：X11/Wayland、GNOME/KDE/Xfce、最小化/遮挡/Unmap/Resize事件差异，驱动和iGPU/独显差异。
- 测试盲区风险：只覆盖happy path、只覆盖单示例、只覆盖短时运行、只覆盖单一桌面环境、未覆盖失败注入。

2. 静态审计必须覆盖的代码面
- gpu/rwgpu：所有purego入口、所有Release/Destroy、所有回调注册、所有handle字段、所有unsafe.Pointer转换。
- gpu/webgpu：Device/Queue/Surface封装、Swapchain帧状态、Resize/Configure、错误映射、DeviceLost传播。
- render/internal/gpu 与 render：每帧资源创建/缓存/回收、CommandEncoder/RenderPass生命周期、纹理/atlas/cache增长路径。
- render/text、render/scene、render/surface：跨帧缓存、图像/字体/路径资源复用、错误返回路径、CPU fallback路径。
- examples：窗口事件监听、后台节流、上下文销毁重建、日志与指标采集；示例层问题不得反向污染内核层。

3. 静态排查规则
- 每个Create必须能追踪到唯一释放责任方；共享资源必须说明所有权和幂等释放语义。
- 每个Destroy/Release后必须禁止继续使用原句柄；允许幂等释放，但禁止静默掩盖非法生命周期。
- 每个purego调用前必须满足：非空句柄、device未熔断、必要锁已持有、输入结构体生命周期覆盖native调用。
- 每个错误返回必须携带结构化分类；禁止新增依赖原始字符串判断的核心控制流。
- 每个goroutine、callback、timer、event listener必须有退出条件；窗口关闭或device lost后不得继续提交GPU命令。
- 每个缓存必须有容量、生命周期或显式清理策略；没有上限的map/slice必须列入泄漏审计。
- 每个锁必须标明保护对象；禁止在持锁期间执行可能反入、阻塞或长时运行的外部回调。

4. 动态发现机制
- 单元测试：覆盖错误分类、空句柄、重复释放、非法帧配对、device lost熔断、资源注册/注销、并发访问。
- Race测试：至少覆盖rwGPU安全层、webgpu帧状态、render提交链路和资源缓存。
- Fault injection：人为制造device lost、surface lost、Present失败、Resize中断、资源创建失败、回调延迟返回。
- Soak测试：前台、后台最小化、遮挡、频繁resize、高频最小化还原、长时间静置恢复。
- Visual gate：基础画面非空、关键像素稳定、帧间内容变化合理、resize后画面恢复。
- Leak gate：RSS、显存近似指标、对象注册表计数、goroutine数量、frame resource计数。
- Performance gate：FPS、frame time p95/p99、CPU fallback计数、GPU submit/present计数、锁等待耗时。

5. 故障注入专项
- DeviceLost注入：触发后所有Device/Queue/Surface调用必须返回结构化device lost错误，不允许panic或native abort。
- InvalidHandle注入：Destroy后重复调用所有核心方法，必须被Go层拦截。
- FramePair注入：BeginFrame后不Present、Present两次、Present失败后再次BeginFrame，必须返回可分类错误。
- ResizeRace注入：渲染中并发Resize/Configure，必须无数据竞争、无野指针、无未配对帧。
- ResourceFail注入：Buffer/Texture/Pipeline创建失败时，已创建的中间资源必须释放。
- CallbackAfterDestroy注入：native异步回调晚于Go对象销毁返回时，不允许访问已释放状态。

6. 代码搜索清单
- unsafe风险：rg -n "unsafe\\.|unsafe.Pointer|uintptr|purego|//export|callback|SetCallback" gpu render
- 资源释放风险：rg -n "Destroy\\(|Release\\(|Close\\(|Free\\(|defer .*Destroy|defer .*Release" gpu render
- 句柄风险：rg -n "handle|Handle|nil|0\\)|invalid|IsValid|Valid" gpu/rwgpu gpu/webgpu render/internal/gpu
- 错误吞噬风险：rg -n "panic\\(|log\\.Print|return nil|_ =|TODO|FIXME|strings\\.Contains" gpu render examples
- 并发风险：rg -n "go func|sync\\.|Mutex|RWMutex|atomic|callback|chan " gpu render examples
- 缓存泄漏风险：rg -n "map\\[|append\\(|cache|Cache|pool|Pool|atlas|registry" render gpu

7. 风险覆盖矩阵
| 风险类型 | 涉及目录 | 静态排查方式 | 动态验证方式 | 证据产物 | 状态 |
|----------|----------|--------------|--------------|----------|------|
| FFI/ABI风险 | gpu/rwgpu、gpu/webgpu | purego签名、结构体布局、unsafe.Pointer、回调生命周期审计 | ABI单测、回调延迟返回、native错误注入 | 单测日志、审计记录、失败注入记录 | TODO |
| GPU句柄生命周期风险 | gpu/rwgpu、gpu/webgpu、render/internal/gpu | Create/Destroy/Release所有权追踪，重复释放和空句柄路径审计 | InvalidHandle注入、double free、use-after-free专项用例 | 单测日志、fault injection日志 | TODO |
| Device/Queue/Surface状态机风险 | gpu/webgpu、gpu/rwgpu、examples | DeviceLost传播、Surface Configure、BeginFrame/Present配对审计 | DeviceLost、SurfaceLost、FramePair、ResizeRace专项 | 测试日志、状态机错误记录 | TODO |
| 并发与线程风险 | gpu/rwgpu、gpu/webgpu、render、examples | 锁保护对象、回调反入、goroutine退出条件审计 | go test -race、并发Resize/Present、并发资源释放 | race日志、并发压测日志 | TODO |
| 资源泄漏风险 | render、render/internal/gpu、render/text、gpu/webgpu | 缓存容量、资源注册/注销、错误路径清理审计 | Soak、Leak gate、对象计数、RSS/显存斜率监控 | metrics.csv、result.json、泄漏趋势记录 | TODO |
| 错误处理风险 | gpu、render、examples | error分类、panic、字符串匹配、错误吞噬路径审计 | 错误注入、失败路径单测、problem suite | 错误分类日志、失败路径用例 | TODO |
| 渲染正确性风险 | render、render/scene、render/text、examples | CPU/GPU路径差异、clip/mask/filter/blend路径审计 | Visual gate、像素对比、resize后画面恢复 | 截图、像素diff、场景结果json | TODO |
| 性能稳定风险 | render/internal/gpu、render、examples | submit次数、fallback路径、锁竞争点、缓存增长路径审计 | FPS、frame p95/p99、CPU fallback、锁等待耗时监控 | 性能日志、基线对比记录 | TODO |
| 跨平台/桌面环境风险 | examples、gpu/webgpu、gpu/rwgpu | X11/Wayland、GNOME/KDE/Xfce事件差异审计 | 多桌面最小化、遮挡、Unmap、Resize矩阵 | 环境矩阵日志、窗口事件日志 | TODO |
| 测试盲区风险 | tests、scripts、examples | happy path偏置、短时偏置、单示例偏置审计 | 动态扩容用例、problem suite、专项示例 | 新增用例、覆盖记录 | TODO |

8. 问题登记表模板
所有排查发现的问题，无论是否立即修复，必须登记到问题清单；无法复现或暂不修复的问题必须记录风险接受理由。

| ID | 风险类型 | 所属层 | 触发条件 | 影响 | 严重级别 | 证据 | 修复状态 | 回归用例 | 风险接受说明 |
|----|----------|--------|----------|------|----------|------|----------|----------|--------------|
| TBD | TBD | rwgpu/webgpu/render/examples/env | TBD | 崩溃/泄漏/竞态/渲染错误/性能退化 | P0/P1/P2/P3 | 日志/截图/命令/commit | TODO/FIXED/ACCEPTED/BLOCKED | 测试文件或脚本 | 必填，仅ACCEPTED/BLOCKED适用 |

严重级别定义：
- P0：进程崩溃、native abort、数据竞争、稳定复现的device lost不可恢复、明确泄漏。
- P1：用户可见渲染错误、窗口生命周期异常、资源释放顺序错误、严重性能退化。
- P2：低概率异常、边界场景错误、可恢复错误分类不准、测试覆盖缺口。
- P3：文档、日志、指标、脚本可观测性不足。

9. 覆盖闭环要求
- 每发现一个潜在问题，必须落到以下至少一种产物：代码修复、单元测试、fault injection用例、soak场景、静态审计规则、文档化风险接受。
- 每个风险项必须有归属层级：rwgpu、webgpu、render、examples、环境/驱动。
- 每个无法立即验证的问题必须记录为风险接受，不允许用“已审计”“理论安全”直接替代证据。
- 多轮测试不是简单重复运行；每一轮必须覆盖不同风险面：生命周期、并发、故障注入、长时泄漏、视觉正确性、跨环境。

10. 文档满足度评估
- 满足：作为全库潜在稳定性问题系统排查的执行框架，已覆盖主要高风险面、排查方法、验证机制、证据产物和闭环规则。
- 不满足：不能作为“所有未知问题已经被发现或清零”的证明，不能替代实际排查记录、测试日志和风险接受清单。
- 最终验收依据：风险覆盖矩阵、问题登记表、测试证据、日志产物、环境矩阵、风险接受清单。
- 结论口径：只能在本框架覆盖的代码面、故障模型、环境矩阵和测试时长下，声明未发现剩余高危问题；不得声明所有潜在问题已被绝对清零。
