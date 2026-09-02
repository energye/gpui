//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/energye/gpui/ui/imeutil"
	"github.com/godbus/dbus/v5"
)

// x11Ime 是 X11 D-Bus IME 的 S1+S2+S3 实现：会话总线 + 上下文 + 会话与锚点
// S1: 单 Conn 复用 + AddMatch + 无守护降级 + GPUI_IME_DEBUG
// S2: 探针顺序 + CreateInputContext 双引擎 + SetCapabilities + 异步化 + per-window Destroy
// S3: FocusIn/Out幂等 + SetCursorLocation/Rect + SetSurroundingText 4000 + SetContentType + XTranslateCoordinates
// S5: 信号归一 + AttrList
type x11Ime struct {
	conn       *dbus.Conn
	engine     string
	engineImpl x11ImeEngine
	objectPath dbus.ObjectPath
	imeDirty   bool
	focused    bool
	mu         sync.Mutex
	host       *x11Host
	lastRect   Rect
	hasRect    bool
	purpose    ContentPurpose
	closed     bool
	composing  bool
	lastText   string
	lastCursor int
	lastAnchor int
	sigCh      chan *dbus.Signal
	sigStop    chan struct{}
	// B9: last preedit segments for debug / future DrawPreedit
	lastSegs []ImeSegment
	// B2/B4 退避：探测全失败时记录，下次 ensureReprobe 需间隔
	lastProbeFail time.Time
	// B24：上次做「归属者校验」的时间（见 sessionUsable 的节流说明）。
	lastOwnerCheck time.Time
	// 建 IC 时的总线上输入法归属快照（身份真源，见 x11ImeOwner）。
	// 「同一个守护抖了一下」与「换了输入法框架」必须靠它区分：
	// 前者 owner 不变，复用 IC（B22，防 fcitx5 回落英文）；
	// 后者 owner 变了，旧 IC 挂在已消失的服务方上，必须重建。
	icOwner x11ImeOwner
}

// S6: 全局 IME 集合用于 NameOwnerChanged 热切
var (
	x11ImesMu sync.Mutex
	x11Imes   = make(map[*x11Ime]struct{})
)

func x11RegisterIme(im *x11Ime) {
	if im == nil {
		return
	}
	x11ImesMu.Lock()
	x11Imes[im] = struct{}{}
	x11ImesMu.Unlock()
}

func x11UnregisterIme(im *x11Ime) {
	if im == nil {
		return
	}
	x11ImesMu.Lock()
	delete(x11Imes, im)
	empty := len(x11Imes) == 0
	x11ImesMu.Unlock()
	if empty {
		if c, err := sharedFcitxConn(); err == nil && c != nil {
			// 清理全部 4 条全局 Match（旧代码只清 [:2]，漏了 fcitx 与 NameOwnerChanged）
			for _, rule := range dbusMatchRules {
				_ = c.BusObject().Call(dbusServiceDBus+".RemoveMatch", 0, rule).Err
			}
		}
	}
}

func x11MarkDirtyForOwnerChange(name, oldOwner, newOwner string) {
	if name != dbusServiceIBus && name != dbusServiceFcitx5 && name != dbusServiceFcitx {
		return
	}
	x11ImesMu.Lock()
	imes := make([]*x11Ime, 0, len(x11Imes))
	for im := range x11Imes {
		imes = append(imes, im)
	}
	x11ImesMu.Unlock()

	for _, im := range imes {
		im.mu.Lock()
		if oldOwner != "" && newOwner == "" {
			// B22：守护消失时【不销毁 IC、不清空 objectPath、不发 FocusOut】。
			// 实测：Destroy 后重建 IC 会让 fcitx5 状态回落到 keyboard-us（英文），
			// 表现为「切回程序任务栏显示英文、但还能输五笔」。保持 IC 存活则状态不丢。
			// 守护真死时，旧 IC 会在 Close 或重探成功替换时销毁，不会泄漏。
			x11ImeDebug("NameOwnerChanged %s %q->%q imeDirty=true (keep IC, no FocusOut)", name, oldOwner, newOwner)
			im.imeDirty = true
			im.composing = false
			im.mu.Unlock()
		} else if oldOwner == "" && newOwner != "" {
			x11ImeDebug("NameOwnerChanged %s %q->%q imeDirty=true", name, oldOwner, newOwner)
			im.imeDirty = true
			im.mu.Unlock()
		} else if oldOwner != "" && newOwner != "" {
			// 过户：同一个名字从一个人手里交到另一个人手里（A 让位、B 接管）。
			// 这正是「用户换了输入法框架」的信号形态——旧服务方已经不在了，
			// 现有 IC 挂在它上面，必须置脏让下次输入重建。
			//
			// 修复前此分支落在 else 里什么都不做：过户被当成「无变化」忽略，
			// 程序便永远感知不到用户切了输入法（B23 直接根因）。
			x11ImeDebug("NameOwnerChanged %s %q->%q imeDirty=true (handover: framework switched)", name, oldOwner, newOwner)
			im.imeDirty = true
			im.composing = false
			im.mu.Unlock()
		} else {
			im.mu.Unlock()
		}
	}
}

// x11ConnIsCurrent 报告这条连接是否仍是某条总线的当前连接。
//
// 三个连接单例（会话总线、ibus 私有总线、fcitx 会话总线）都会起观察者循环。
// 任一单例轮换后，挂在旧连接上的循环就成了孤儿，必须自行退出，否则每次重连都
// 多留一个永阻塞的 goroutine（B4 泄漏）。
func x11ConnIsCurrent(conn *dbus.Conn) bool {
	if conn == nil {
		return false
	}
	x11FcitxMu.Lock()
	curFcitx := x11FcitxConn
	x11FcitxMu.Unlock()
	if conn == curFcitx {
		return true
	}
	x11IbusMu.Lock()
	curIbus := x11IbusConn
	x11IbusMu.Unlock()
	return conn == curIbus
}

func x11GlobalNameOwnerLoop(conn *dbus.Conn) {
	if conn == nil {
		return
	}
	ch := make(chan *dbus.Signal, 10)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)
	for sig := range ch {
		// Stale loop guard: if any singleton rotated, exit old loop (B4 leak)
		if !x11ConnIsCurrent(conn) {
			return
		}
		if sig.Name != dbusServiceDBus+".NameOwnerChanged" {
			continue
		}
		if len(sig.Body) < 3 {
			continue
		}
		name, _ := sig.Body[0].(string)
		oldOwner, _ := sig.Body[1].(string)
		newOwner, _ := sig.Body[2].(string)
		x11MarkDirtyForOwnerChange(name, oldOwner, newOwner)
	}
}

func x11BusWatchLoop(conn *dbus.Conn) {
	if conn == nil {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// Stale loop guard: if any singleton rotated, this loop is orphaned (B4)
		if !x11ConnIsCurrent(conn) {
			return
		}
		if conn.Connected() {
			continue
		}
		x11ImeDebug("bus disconnect detected, backoff reconnect")
		backoff := 200 * time.Millisecond
		for {
			time.Sleep(backoff)
			if backoff < 2*time.Second {
				backoff *= 2
				if backoff > 2*time.Second {
					backoff = 2 * time.Second
				}
			}
			// 另一条等待者可能已经重连过了，退出避免重复拨号
			if !x11ConnIsCurrent(conn) {
				return
			}
			c, err := reconnectBusFor(conn)
			if err != nil {
				x11ImeDebug("bus reconnect dial failed: %v backoff %v", err, backoff)
				continue
			}
			x11ImeDebug("bus reconnect ok")
			for _, rule := range dbusMatchRules {
				_ = c.BusObject().Call(dbusServiceDBus+".AddMatch", 0, rule).Err
			}
			go x11GlobalNameOwnerLoop(c)
			go x11BusWatchLoop(c)
			x11ImesMu.Lock()
			for im := range x11Imes {
				im.mu.Lock()
				im.conn = c // B3: propagate new conn to existing IMEs
				im.imeDirty = true
				im.mu.Unlock()
			}
			x11ImesMu.Unlock()
			return
		}
	}
}

// reconnectBusFor 为断掉的旧连接按「它属于哪条总线」重拨，并把新连接发布到对应单例。
//
// 旧实现只认会话总线一个单例，但挂这个循环的连接可能是真 ibus 的私有总线
// （dialIbusPrivateSlow 也会起一对循环）。对私有总线重连却写回会话总线单例，
// 会把两条不同用途的总线搅在一起。
func reconnectBusFor(old *dbus.Conn) (*dbus.Conn, error) {
	x11IbusMu.Lock()
	isIbus := old == x11IbusConn
	x11IbusMu.Unlock()
	if isIbus {
		c, err := dialIbusPrivate(true)
		if err != nil || c == nil {
			return nil, fmt.Errorf("ibus private redial: %w", err)
		}
		return c, nil
	}
	// 会话总线：reset + 重连，让 singleton 重新拨号
	resetFcitxSessionForTest()
	c, err := sharedFcitxConn()
	if err != nil || c == nil {
		return nil, fmt.Errorf("session redial: %w", err)
	}
	return c, nil
}

func dbusHasOwner(conn *dbus.Conn, name string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("nil conn")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	var has bool
	err := conn.BusObject().CallWithContext(ctx, dbusServiceDBus+".NameHasOwner", dbus.FlagNoAutoStart, name).Store(&has)
	if err != nil {
		return false, err
	}
	return has, nil
}

func x11ClientName() string {
	exe := os.Args[0]
	if exe == "" {
		exe = "gpui"
	}
	base := filepath.Base(exe)
	if base == "" || base == "." {
		base = "gpui"
	}
	return "gpui:" + base
}

func x11AppName() string {
	exe := os.Args[0]
	if exe == "" {
		return "gpui"
	}
	base := filepath.Base(exe)
	if base == "" || base == "." {
		return "gpui"
	}
	return base
}

// imeForX11 供 x11_linux.go 调用：Q2 单选单探，建窗不阻塞（500ms 超时）
// S1 三层：按环境变量只选一条总线，ibus 私有 vs fcitx 会话各单例，FlagNoAutoStart 防激活
func imeForX11(h *x11Host) IME {
	declared := x11ProbeOrder()
	if len(declared) == 0 {
		return nil
	}
	// 先按声明取一次：声明的那家可用就直接用，**不额外连会话总线**。
	// 只有它连不上或名字不可见时，才去会话总线看实测谁活着并校正目标（G4）。
	//
	// 为什么不能每次都先连会话总线做校正：那会为纯 ibus 用户凭空多出一条会话总线
	// 连接，而它自带的观察者会让「fcitx5 上线」变成信号可达，把真正需要兜底的
	// 「信号收不到」场景掩盖成「测试通过」。
	eng, conn, ok := tryBusFor(declared[0])
	if !ok {
		if cur := snapshotOwnerOnSessionBus(); cur.Any() {
			if ord := x11ProbeOrderForOwnerFrom(declared, cur); len(ord) > 0 && ord[0] != declared[0] {
				x11ImeDebug("imeForX11 declared %s unavailable, correct target to %s (framework switch aware)", declared[0], ord[0])
				eng, conn, ok = tryBusFor(ord[0])
			}
		}
	}
	if !ok {
		x11ImeDebug("imeForX11 no usable bus: degrade nil (S1 单选，不 fallback 另一家)")
		return nil
	}
	x11ImeDebug("imeForX11 engine=%s conn=%p", eng, conn)
	im := &x11Ime{
		conn: conn,
		host: h,
	}
	x11RegisterIme(im)
	go im.asyncProbe()
	return im
}

// tryBusFor 按引擎名取总线，并确认该引擎的名字在这条总线上确实可见。
//
// 取总线统一走引擎层的 Bus()，与运行时重探同一条路，不在此处另写分支——
// 否则引擎层补的回落（如私有地址文件为空时改走会话总线）对建窗路径不生效。
func tryBusFor(engine string) (string, *dbus.Conn, bool) {
	engImpl := x11EngineForName(engine)
	if engImpl == nil {
		x11ImeDebug("tryBusFor unknown engine %q", engine)
		return engine, nil, false
	}
	conn, err := engImpl.Bus(false)
	if err != nil || conn == nil {
		x11ImeDebug("tryBusFor %s no bus: %v", engine, err)
		return engine, nil, false
	}
	if has, _ := dbusHasOwner(conn, x11ServiceForEngine(engine)); !has {
		x11ImeDebug("tryBusFor %s no owner on %p", engine, conn)
		return engine, conn, false
	}
	return engine, conn, true
}

// 时序常量集中定义，避免同一个 500ms 在多处各写一遍、改一处漏一处。
const (
	// probeTimeout 单次探测/建上下文的超时（S2 规定 500ms，超时即放行不卡建窗）。
	probeTimeout = 500 * time.Millisecond
	// probeThrottleInterval 重探全失败后的退避间隔，避免每键全量重探（B2）。
	probeThrottleInterval = 500 * time.Millisecond
	// ownerCheckInterval 「会话还活着吗」主动校验的最小间隔（B24）。
	//
	// 校验只发生在「连接已断」这条便宜路径上（Connected() 只是 ctx.Err()，不走总线），
	// 真正发 D-Bus 往返的分支会先过这道节流，避免守护全部缺失时每次按键都查一轮。
	ownerCheckInterval = 500 * time.Millisecond
)

// sessionUsable 报告「窗口手里这套会话（连接 + IC）是否还通向一个活着的服务方」。
//
// 这是 B24（ibus→fcitx5 失效）的核心。为什么不能只盯脏标记：
// 程序跑在真 ibus 上时，手里那条是**真 ibus 的私有总线连接**。守护一退，这条
// unix:abstract 私有总线连同挂在它上面的 NameOwnerChanged 观察者**一起消失**——
// 此后**再没有任何人**能通知程序「输入法换了」：信号收不到 → imeDirty 永不置位 →
// 下一次输入时 IC 还非空、need 判为 false → 懒重探早退 → 程序抱着死连接和过期 IC
// 一动不动，只能重启进程。
//
// 反方向（fcitx5→ibus）不受影响：fcitx5 把服务开在会话总线上，那条总线由
// dbus-daemon 提供、与输入法无关，守护换人它还在，观察者也就还在，信号正常送达。
// 这就是「fcitx→ibus 正常、ibus→fcitx 失败」这条不对称的由来。
//
// 判据分三层，只在必要时才走总线：
//   - 连接还活着 → 观察者也就还活着，信号驱动有效，直接判为可用（同时保住 B22：
//     同守护抖动不重建 IC）。这一步零 D-Bus 往返，是每次按键的热路径。
//   - 连接已断 → 不是抖动，是旧守护连同它那条总线一起没了。补查会话总线：
//     fcitx5 只出现在那里，只盯手里这条死连接永远看不见它。
//   - 到处都没人 → 守护暂时全没了，按 B22 保住现有会话等它回来，不折腾。
//
// 入参由调用方在持锁状态下取的快照传入，避免本函数再来一轮加锁。
func (im *x11Ime) sessionUsable(conn *dbus.Conn, obj dbus.ObjectPath) bool {
	if im == nil {
		return false
	}
	if conn == nil || obj == "" {
		// 还没有会话可言，交给后面的重探去建。
		return false
	}
	if conn.Connected() {
		return true
	}
	im.mu.Lock()
	last := im.lastOwnerCheck
	im.mu.Unlock()
	if time.Since(last) < ownerCheckInterval {
		return true
	}
	im.mu.Lock()
	im.lastOwnerCheck = time.Now()
	im.mu.Unlock()
	cur, usable := x11EffectiveOwner(conn)
	if cur.Any() && usable != nil && usable.Connected() {
		x11ImeDebug("sessionUsable: window conn dead, but %s alive on another bus -> need reprobe",
			cur.frameworkIdentity())
		return false
	}
	return true
}

// icOwnerSame 报告「当前服务方」是否仍是「建这个 IC 时的那个服务方」。
//
// 这是热切感知的核心判据：
//   - 同一个人（owner 未变）→ 只是抖了一下，复用 IC 保住输入法状态（B22）。
//   - 换了人（某个名字的 owner 从 A 变成 B，都非空）→ 真换了框架，必须重建。
//   - 人没了（当前无任何归属）→ 见下方 connAlive 判定。
//
// connAlive 是「人没了」时的关键分水岭，B22 与「切输入法」在此分道：
//   - 连接还活着、只是名字暂时没人 → 守护在重启/抖动，**复用 IC**（B22 的本意：
//     重建会让 fcitx5 回落 keyboard-us 英文）。
//   - 连接本身已经断了 → 那不是抖动，是「旧守护连同它监听的那条总线一起没了」。
//     此时必须返回 false 让上层重探，否则会卡死在一条死连接上：
//     用户切到监听在另一条总线的新守护后，程序永远连不上，只能重启。
//     （实测：fcitx5 把 IBus 开在会话总线，真 ibus-daemon 另开 unix:abstract
//     私有总线，旧守护一退它那条私有总线即失效。）
func (im *x11Ime) icOwnerSame() bool {
	if im == nil || im.conn == nil {
		return false
	}
	cur := snapshotOwner(im.conn)
	im.mu.Lock()
	prev := im.icOwner
	im.mu.Unlock()
	if !cur.Any() {
		if im.conn.Connected() {
			// 连接还在、只是名字暂时没人：守护抖动，B22 保住 IC。
			x11ImeDebug("icOwner: no owner but conn alive, keep IC (B22)")
			return true
		}
		// 连接已断：旧守护连同它那条总线一起没了，不是抖动，须重探。
		x11ImeDebug("icOwner: no owner and conn dead -> rebuild (daemon/bus gone, not a blip)")
		return false
	}
	if !prev.Any() {
		// 建 IC 时没记到归属（异常路径），无从比对，保守复用。
		x11ImeDebug("icOwner: no recorded owner, keep IC (unknown)")
		return true
	}
	// 两种「变了」都要认：① 同一名字过户；② 换了框架（两个名字不重叠，
	// 任一名字都没有 owner 变化，只能靠整体指纹比对——否则 fcitx5↔ibus 会被漏判）。
	if x11OwnerChanged(prev, cur) || prev.frameworkIdentity() != cur.frameworkIdentity() {
		x11ImeDebug("icOwner: framework switched prev{IBus:%s Fcitx5:%s} cur{IBus:%s Fcitx5:%s} -> rebuild",
			prev.IBus, prev.Fcitx5, cur.IBus, cur.Fcitx5)
		return false
	}
	return true
}

// destroyCurrentIC 销毁当前 IC 并清空路径，供「换了服务方」时强制重建使用。
// 与 Close 不同：不置 closed，后续仍可重探出新 IC。
func (im *x11Ime) destroyCurrentIC() {
	if im == nil {
		return
	}
	im.stopSignalLoop()
	im.mu.Lock()
	obj := im.objectPath
	eng := im.engine
	engImpl := im.engineImpl
	im.objectPath = ""
	im.engine = ""
	im.engineImpl = nil
	im.composing = false
	im.icOwner = x11ImeOwner{}
	im.mu.Unlock()
	if obj == "" {
		return
	}
	if engImpl != nil {
		_ = engImpl.Destroy(im.conn, obj)
	} else if e := x11EngineForName(eng); e != nil {
		_ = e.Destroy(im.conn, obj)
	}
	x11ImeDebug("destroy stale IC %s (engine=%s) after framework switch", obj, eng)
}

// ensureReprobe 懒重探，脏标记或无对象时按 S2 顺序同步重探
func (im *x11Ime) ensureReprobe() {
	if im == nil {
		return
	}
	im.mu.Lock()
	snap := struct {
		path     dbus.ObjectPath
		dirty    bool
		lastFail time.Time
		owner    x11ImeOwner
		conn     *dbus.Conn
	}{
		path:     im.objectPath,
		dirty:    im.imeDirty,
		lastFail: im.lastProbeFail,
		owner:    im.icOwner,
		conn:     im.conn,
	}
	im.mu.Unlock()

	need := snap.dirty || snap.path == ""
	if !need {
		// B24：脏标记为空不代表会话还活着。若窗口手里那条总线已经断了（真 ibus 的
		// 私有总线随守护一起消失，连带着它的观察者也没了），此后**没有任何人**会给
		// 程序报「输入法换了」——必须自己看一眼，否则 ibus→fcitx5 永远感知不到。
		need = !im.sessionUsable(snap.conn, snap.path)
	}
	if need && !snap.lastFail.IsZero() && time.Since(snap.lastFail) < probeThrottleInterval {
		x11ImeDebug("lazy reprobe throttled lastFail=%v", snap.lastFail)
		return
	}
	if !need {
		return
	}
	// B22：已有对象且是「脏标记」触发（守护抖动）而非「无对象」时，
	// 先确认服务方是否还是建这个 IC 时的那一个。是同一个人就说明只是名字抖了一下，
	// 直接复用现有 IC 清脏即可——重建 IC 会让输入法状态回落英文（B22）。
	//
	// 判据必须是「身份不变」而非「还活着」：只看存活会把「fcitx5 让位给 ibus」的过户
	// 误判成抖动，从而复用一个挂在已消失服务方上的 IC，表现为切了输入法却接不上。
	if snap.path != "" && snap.dirty {
		if im.icOwnerSame() {
			im.mu.Lock()
			im.imeDirty = false
			im.mu.Unlock()
			x11ImeDebug("lazy reprobe skipped: same owner, reuse IC %s (keep IME state)", snap.path)
			return
		}
		// 换了服务方（或旧连接已断）：需要先重探。但**不在这里销毁旧 IC**——
		// 高可用要求：在新 IC 真正建出来之前，旧会话必须保持可用（哪怕它可能
		// 已经不灵）。提前销毁会让「重探失败」直接退化成完全不能输入。
		// 旧 IC 改由重探成功后统一销毁（见下方 oldObj 处理）。
		x11ImeDebug("lazy reprobe forced: owner/bus changed, will rebuild IC %s (keep old until new is ready)", snap.path)
	}
	x11ImeDebug("lazy reprobe triggered dirty=%v path=%q", snap.dirty, snap.path)

	// 判定本次是否属于「换了输入法框架」：旧归属有记录、现在有人、且不是同一个人。
	// 这一步决定是否强制重拨总线——ibus 换守护后总线地址会变，不重拨连不上。
	prevOwner, startConn := snap.owner, snap.conn
	cur, probeConn := x11EffectiveOwner(startConn)
	// 两种「变了」都要认：
	//   ① 同一名字过户（A 让位给 B）—— x11OwnerChanged
	//   ② 换了框架（原先占 A 名、现在占 B 名，两者不重叠）—— 指纹比对
	// 只认 ① 会漏掉 fcitx5↔ibus 这类跨框架切换（两个名字互不重叠，
	// 任一名字都没有「从 A 变 B」，于是被误判为没变）。
	switched := prevOwner.Any() && cur.Any() &&
		(x11OwnerChanged(prevOwner, cur) || prevOwner.frameworkIdentity() != cur.frameworkIdentity())
	if switched {
		x11ImeDebug("framework switch confirmed: prev{IBus:%s Fcitx5:%s} cur{IBus:%s Fcitx5:%s} -> refetch bus",
			prevOwner.IBus, prevOwner.Fcitx5, cur.IBus, cur.Fcitx5)
	}
	// 注意：**不要**在这里把 probeConn 抢先写回 im.conn。
	//
	// 「先立后破」要求：在新 IC 真正建出来之前，im.conn 必须保持为**旧连接**，否则
	// 会发生两件事——① 下面 oldConn := im.conn 拿到的是被顶替后的值，旧 IC 就被发到
	// 一条与它无关的总线上销毁（无效，泄漏）；② x11PropagateConn(oldConn, ...) 拿
	// 到的也是错的值，同进程其他窗口仍抱着真·旧连接却匹配不上，传播 0 个窗口，
	// 表现为「切完输入法只有当前窗口能用」。
	//
	// probeConn 只是「能看见新输入法的那条总线」，用于上方归属判定；
	// 真正建上下文走 eng.Bus(force)，由引擎层自己解析地址，不依赖这里预先切换。
	if probeConn != nil && probeConn.Connected() && probeConn != startConn {
		x11ImeDebug("probe target visible on recovered conn %p (im.conn %p unusable)", probeConn, startConn)
	}
	// 用户切换框架不会改写本进程的环境变量，故按「实测谁活着」校正目标，
	// 否则会一直去探一个已经没了名字的框架（G4）。声明的那家还在时严格按声明走。
	// 这里用 cur（已按 x11EffectiveOwner 修正过的合并快照）而非 im.conn 上的即时快照，
	// 保证在「旧私有总线已死、新输入法在会话总线」时也能把目标校正过去。
	order := x11ProbeOrderForOwnerFrom(x11ProbeOrder(), cur)

	for _, engName := range order {
		eng := x11EngineForName(engName)
		if eng == nil {
			continue
		}
		// 两轮取连接（高可用）：
		//   第一轮沿用现有连接——绝大多数情况（守护没换、只是掉线重连）走这条，
		//   代价最低，且不会破坏 S1 的「平时单连接复用」纪律。
		//   第二轮强制重拨——用于「换了输入法框架」：新守护可能监听在**另一条
		//   总线地址**上（实测 fcitx5 开在会话总线、真 ibus-daemon 开在
		//   unix:abstract 私有总线并改写 ~/.config/ibus/bus/），不重拨永远连不上。
		// 覆盖三种触发：① 明确换了归属者；② 当前连接上已无人应答（旧守护连同
		// 它那条总线一起没了，连归属都没法比对）；③ IC 建不出来。
		force := switched || !cur.Any()
		obj, conn, err := tryCreateOnBus(eng, force, probeTimeout)
		if err != nil && !force {
			x11ImeDebug("lazy reprobe %s failed on current bus, retry with fresh bus: %v", engName, err)
			obj, conn, err = tryCreateOnBus(eng, true, probeTimeout)
		}
		if err != nil {
			x11ImeDebug("lazy reprobe %s failed: %v", engName, err)
			continue
		}
		x11ImeDebug("lazy reprobe %s ok %s (force=%v)", engName, obj, force)
		_ = eng.SetCapabilities(conn, obj, eng.Caps())
		im.mu.Lock()
		if im.closed {
			im.mu.Unlock()
			_ = eng.Destroy(conn, obj)
			return
		}
		oldObj := im.objectPath
		oldEng := im.engine
		oldImpl := im.engineImpl
		oldConn := im.conn
		im.engine = engName
		im.engineImpl = eng
		im.objectPath = obj
		im.imeDirty = false
		im.lastProbeFail = time.Time{}
		im.icOwner = snapshotOwner(conn)
		// 换框架后会拿到一条新连接（ibus 私有总线地址变了），写回本窗口。
		// 必须在持有锁时切换，避免与按键/锚点路径读到不一致的连接。
		if conn != oldConn {
			im.conn = conn
			x11ImeDebug("conn rotated %p -> %p after framework switch", oldConn, conn)
		}
		hasRect := im.hasRect
		rect := im.lastRect
		lastText := im.lastText
		lastCursor := im.lastCursor
		lastAnchor := im.lastAnchor
		im.mu.Unlock()
		// 旧 IC 挂在旧连接上，必须用旧连接销毁，否则销毁发到新总线上无效、泄漏。
		if oldObj != "" && oldObj != obj {
			if oldImpl != nil {
				_ = oldImpl.Destroy(oldConn, oldObj)
			} else if oldEng != "" {
				if e := x11EngineForName(oldEng); e != nil && oldConn != nil {
					_ = e.Destroy(oldConn, oldObj)
				}
			}
		}
		// 多窗口高可用：把新连接传播给同进程的其他窗口，否则它们仍抱着旧连接，
		// 表现为「切完输入法只有当前窗口能用，其他窗口还是不行」。
		if (switched || !cur.Any()) && conn != oldConn {
			x11PropagateConn(oldConn, conn)
		}
		x11ImeDebug("lazy reprobe success engine=%s path=%s", engName, obj)
		im.callFocusIn()
		if hasRect {
			im.callSetCursorLocation(rect)
		}
		if lastText != "" {
			im.callSetSurroundingText(lastText, lastCursor, lastAnchor)
		}
		im.startSignalLoop()
		return
	}
	x11ImeDebug("lazy reprobe all failed")
	im.mu.Lock()
	// 高可用：重探失败时只做节流，不清空 engine/IC/连接。
	// 保住现有会话（哪怕它可能已不灵）远好过把窗口变成完全不能输入——
	// 新守护可能还在写地址文件的窗口期，下一次输入就会成功。
	im.imeDirty = false
	im.lastProbeFail = time.Now()
	im.mu.Unlock()
}

// tryCreateOnBus 取一条连接并在其上建输入上下文。
// force=true 时作废总线缓存重新解析地址并拨号（用于换了输入法框架、总线地址也变了的场景）。
// 返回可用的连接，供调用方写回窗口；失败时 conn 可能为 nil。
func tryCreateOnBus(eng x11ImeEngine, force bool, timeout time.Duration) (dbus.ObjectPath, *dbus.Conn, error) {
	conn, err := eng.Bus(force)
	if err != nil || conn == nil {
		return "", nil, fmt.Errorf("bus unavailable: %v", err)
	}
	obj, err := eng.CreateInputContext(conn, timeout)
	if err != nil {
		return "", conn, err
	}
	if obj == "" {
		return "", conn, fmt.Errorf("empty input context path")
	}
	return obj, conn, nil
}

// x11PropagateConn 把新连接传播给同进程的其他窗口（多窗口高可用）。
// 只更新仍指向旧连接的那些 IME，避免覆盖已自行重连成功的窗口。
func x11PropagateConn(old, next *dbus.Conn) {
	if next == nil {
		return
	}
	x11ImesMu.Lock()
	n := 0
	for other := range x11Imes {
		other.mu.Lock()
		if other.conn == old || other.conn == nil {
			other.conn = next
			n++
		}
		other.mu.Unlock()
	}
	x11ImesMu.Unlock()
	x11ImeDebug("conn propagated to %d window(s) %p -> %p", n, old, next)
}

func (im *x11Ime) asyncProbe() {
	order := x11ProbeOrder()
	x11ImeDebug("asyncProbe start order=%v", order)
	for _, engName := range order {
		eng := x11EngineForName(engName)
		if eng == nil {
			continue
		}
		conn, connErr := eng.Bus(false)
		if connErr != nil || conn == nil {
			x11ImeDebug("asyncProbe %s bus unavailable: %v", engName, connErr)
			continue
		}
		obj, err := eng.CreateInputContext(conn, probeTimeout)
		if err != nil {
			x11ImeDebug("CreateInputContext %s failed: %v", engName, err)
			continue
		}
		x11ImeDebug("CreateInputContext %s ok objectPath=%s", engName, obj)
		if err := eng.SetCapabilities(conn, obj, eng.Caps()); err != nil {
			x11ImeDebug("SetCapabilities %s failed: %v", engName, err)
		} else {
			x11ImeDebug("SetCapabilities %s ok caps=%d", engName, eng.Caps())
		}
		im.mu.Lock()
		if im.closed {
			im.mu.Unlock()
			_ = eng.Destroy(conn, obj)
			return
		}
		im.engine = engName
		im.engineImpl = eng
		im.objectPath = obj
		im.icOwner = snapshotOwner(conn)
		if conn != im.conn {
			im.conn = conn
		}
		x11ImeDebug("engine=%s objectPath=%s", engName, obj)
		im.mu.Unlock()
		im.startSignalLoop()
		return
	}
	x11ImeDebug("probe all failed, degrade")
	im.mu.Lock()
	im.engine = ""
	im.engineImpl = nil
	im.objectPath = ""
	im.mu.Unlock()
}

func (im *x11Ime) destroyObject(obj dbus.ObjectPath, engine string) {
	if im == nil || im.conn == nil || obj == "" {
		return
	}
	// 优先用 engineImpl（若还在），否则按名查
	var eng x11ImeEngine
	if im.engineImpl != nil && im.engine == engine {
		eng = im.engineImpl
	} else {
		eng = x11EngineForName(engine)
	}
	if eng != nil {
		_ = eng.Destroy(im.conn, obj)
		return
	}
	// fallback 旧分支（兜底，理论不进）
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if engine == "ibus" {
		o := im.conn.Object(dbusServiceIBus, obj)
		err := o.CallWithContext(ctx, dbusServiceIBus+".Service.Destroy", 0).Err
		if err != nil {
			err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".Destroy", 0).Err
		}
		if err != nil {
			err = o.CallWithContext(ctx, "Destroy", 0).Err
		}
		x11ImeDebug("Destroy ibus %s err=%v", obj, err)
	} else {
		for _, svc := range []string{dbusServiceFcitx5, dbusServiceFcitx} {
			o := im.conn.Object(svc, obj)
			_ = o.CallWithContext(ctx, dbusIfaceFcitx5IM+".DestroyIC", 0).Err
			_ = o.CallWithContext(ctx, "DestroyIC", 0).Err
		}
		x11ImeDebug("DestroyIC fcitx %s", obj)
	}
}

// Close 由 x11Host.destroy 调用，per-window Destroy
func (im *x11Ime) Close() {
	if im == nil {
		return
	}
	x11UnregisterIme(im)
	im.stopSignalLoop()
	im.mu.Lock()
	if im.closed {
		im.mu.Unlock()
		return
	}
	im.closed = true
	obj := im.objectPath
	eng := im.engine
	im.objectPath = ""
	im.engine = ""
	im.mu.Unlock()
	if obj != "" {
		im.destroyObject(obj, eng)
	}
}

// S5: 信号循环

func (im *x11Ime) startSignalLoop() {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		return
	}
	im.stopSignalLoop()
	ch := make(chan *dbus.Signal, 20)
	im.mu.Lock()
	im.sigCh = ch
	im.sigStop = make(chan struct{})
	im.mu.Unlock()
	im.conn.Signal(ch)
	go im.signalLoop()
	x11ImeDebug("signal loop started for %s engine=%s", im.ObjectPath(), im.Engine())
}

func (im *x11Ime) stopSignalLoop() {
	if im == nil {
		return
	}
	im.mu.Lock()
	ch := im.sigCh
	stop := im.sigStop
	im.sigCh = nil
	im.sigStop = nil
	im.mu.Unlock()
	// 只反注册、不 close(ch)：dbus 内部仍持有该 channel 的引用，
	// 在窗口关闭/连接轮换的竞态下 close 会触发「send on closed channel」
	// 或重复 close 的 panic。反注册后 dbus 不再写入，channel 交给 GC 回收。
	if ch != nil && im.conn != nil {
		im.conn.RemoveSignal(ch)
	}
	// stop 是本进程私有的完成信号，只由本函数关闭一次（上面已置 nil 防重入）。
	if stop != nil {
		close(stop)
	}
}

func (im *x11Ime) signalLoop() {
	if im == nil {
		return
	}
	im.mu.Lock()
	ch := im.sigCh
	stop := im.sigStop
	im.mu.Unlock()
	if ch == nil {
		return
	}
	for {
		select {
		case sig, ok := <-ch:
			if !ok {
				return
			}
			im.handleSignal(sig)
		case <-stop:
			return
		}
	}
}

func (im *x11Ime) handleSignal(sig *dbus.Signal) {
	if im == nil || sig == nil {
		return
	}
	path := im.ObjectPath()
	if sig.Path != path {
		return
	}
	x11ImeDebug("signal %s %s path=%s body=%v", sig.Sender, sig.Name, sig.Path, sig.Body)
	switch sig.Name {
	case dbusIfaceIBusCtx + ".UpdatePreeditText", "UpdatePreeditText":
		if len(sig.Body) >= 3 {
			if v, ok := sig.Body[0].(dbus.Variant); ok {
				text, segs := DecodeIBusVariant(v)
				cursor, _ := sig.Body[1].(uint32)
				visible, _ := sig.Body[2].(bool)
				x11ImeDebug("UpdatePreeditText text=%q cursor=%d visible=%v segs=%v", text, cursor, visible, segs)
				if !visible || text == "" {
					im.pushPreedit("", 0, false, segs)
				} else {
					im.pushPreedit(text, int(cursor), true, segs)
				}
			}
		}
	case dbusIfaceIBusCtx + ".CommitText", "CommitText":
		if len(sig.Body) >= 1 {
			if v, ok := sig.Body[0].(dbus.Variant); ok {
				text, _ := DecodeIBusVariant(v)
				x11ImeDebug("CommitText %q", text)
				im.pushCommit(text)
			}
		}
	case dbusIfaceIBusCtx + ".DeleteSurroundingText", "DeleteSurroundingText":
		if len(sig.Body) >= 2 {
			var offset int32
			var n uint32
			switch o := sig.Body[0].(type) {
			case int32:
				offset = o
			case int:
				offset = int32(o)
			}
			switch n2 := sig.Body[1].(type) {
			case uint32:
				n = n2
			case uint:
				n = uint32(n2)
			case int:
				n = uint32(n2)
			}
			x11ImeDebug("DeleteSurroundingText offset=%d n=%d", offset, n)
			im.pushDeleteSurrounding(int(offset), int(n))
		}
	case dbusIfaceIBusCtx + ".HidePreeditText", "HidePreeditText":
		x11ImeDebug("HidePreeditText")
		im.pushPreedit("", 0, false, nil)
	case "org.fcitx.Fcitx.InputMethod.UpdatePreedit", "UpdatePreedit", "org.fcitx.Fcitx5.InputContext.UpdatePreedit":
		if len(sig.Body) >= 2 {
			if text, ok := sig.Body[0].(string); ok {
				var cursor int32
				switch c := sig.Body[1].(type) {
				case int32:
					cursor = c
				case int:
					cursor = int32(c)
				case uint32:
					cursor = int32(c)
				}
				clean, segs := parseFcitxPreedit(text)
				x11ImeDebug("fcitx UpdatePreedit %q cursor=%d segs=%v", clean, cursor, segs)
				im.pushPreedit(clean, int(cursor), clean != "", segs)
			}
		}
	case "CommitString", "org.fcitx.Fcitx.InputMethod.CommitString", "org.fcitx.Fcitx5.InputContext.CommitString":
		if len(sig.Body) >= 1 {
			if text, ok := sig.Body[0].(string); ok {
				x11ImeDebug("fcitx CommitString %q", text)
				im.pushCommit(text)
			}
		}
	}
}

func (im *x11Ime) pushPreedit(text string, cursor int, visible bool, segs ...[]ImeSegment) {
	if im == nil || im.host == nil {
		return
	}
	var segList []ImeSegment
	if len(segs) > 0 {
		segList = segs[0]
	}
	if !visible || text == "" {
		im.mu.Lock()
		im.composing = false
		im.lastSegs = nil
		im.mu.Unlock()
		im.host.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: "", IMEStart: -1, IMEEnd: -1})
		im.host.WakeUp()
		x11ImeDebug("push Preedit end segs=%v", segList)
		return
	}
	im.mu.Lock()
	im.composing = true
	im.lastSegs = segList
	im.mu.Unlock()
	start := cursor
	if start < 0 || start > len(text) {
		start = -1
	}
	im.host.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: text, IMEStart: start, IMEEnd: start})
	im.host.WakeUp()
	x11ImeDebug("push Preedit %q cursor=%d segs=%v", text, cursor, segList)
}

func (im *x11Ime) pushCommit(text string) {
	if im == nil || im.host == nil {
		return
	}
	im.mu.Lock()
	im.composing = false
	im.mu.Unlock()
	im.host.pushIME(Event{Type: EventIME, IMEKind: 1, IMEText: text, IMEStart: -1, IMEEnd: -1})
	im.host.WakeUp()
	x11ImeDebug("push Commit %q", text)
}

func (im *x11Ime) pushDeleteSurrounding(offset, n int) {
	if im == nil || im.host == nil {
		return
	}
	// IBus DeleteSurroundingText(offset, n) is rune-based [caret+offset, caret+offset+n).
	// Wayland/composition expects before/after byte counts (-before, +after).
	// Translate via last surrounding text so multi-byte (emoji/CJK) byte lengths are correct.
	beforeBytes, afterBytes := 0, 0
	im.mu.Lock()
	text := im.lastText
	cursorByte := im.lastCursor
	im.mu.Unlock()
	if text != "" {
		// rune caret from byte cursor (cursor is at rune boundary)
		runeCaret := 0
		for i := 0; i < cursorByte; {
			_, sz := x11DecodeRune(text[i:])
			if sz == 0 {
				break
			}
			runeCaret++
			i += sz
		}
		startRune := runeCaret + offset
		endRune := startRune + n
		// Clamp
		totalRunes := 0
		for i := 0; i < len(text); {
			_, sz := x11DecodeRune(text[i:])
			if sz == 0 {
				break
			}
			totalRunes++
			i += sz
		}
		if startRune < 0 {
			startRune = 0
		}
		if endRune > totalRunes {
			endRune = totalRunes
		}
		if startRune < endRune {
			beforeEnd := endRune
			if beforeEnd > runeCaret {
				beforeEnd = runeCaret
			}
			beforeStart := startRune
			if beforeStart < 0 {
				beforeStart = 0
			}
			if beforeEnd > beforeStart {
				beforeBytes = byteLenForRunes(text, beforeStart, beforeEnd)
			}
			afterStart := startRune
			if afterStart < runeCaret {
				afterStart = runeCaret
			}
			afterEnd := endRune
			if afterEnd > afterStart {
				afterBytes = byteLenForRunes(text, afterStart, afterEnd)
			}
		}
	} else {
		// No surrounding: cannot compute bytes correctly, drop to avoid over-delete.
		x11ImeDebug("push DeleteSurrounding no surrounding, drop offset=%d n=%d", offset, n)
		return
	}
	im.host.pushIME(Event{Type: EventIME, IMEKind: 3, IMEStart: -beforeBytes, IMEEnd: afterBytes})
	im.host.WakeUp()
	x11ImeDebug("push DeleteSurrounding ibus offset=%d n=%d -> before=%d after=%d", offset, n, beforeBytes, afterBytes)
}

func x11DecodeRune(s string) (r rune, sz int) {
	if len(s) == 0 {
		return 0, 0
	}
	b := s[0]
	if b < 0x80 {
		return rune(b), 1
	}
	if b < 0xE0 {
		if len(s) < 2 {
			return rune(b), 1
		}
		return rune(b&0x1F)<<6 | rune(s[1]&0x3F), 2
	}
	if b < 0xF0 {
		if len(s) < 3 {
			return rune(b), 1
		}
		return rune(b&0x0F)<<12 | rune(s[1]&0x3F)<<6 | rune(s[2]&0x3F), 3
	}
	if len(s) < 4 {
		return rune(b), 1
	}
	return rune(b&0x07)<<18 | rune(s[1]&0x3F)<<12 | rune(s[2]&0x3F)<<6 | rune(s[3]&0x3F), 4
}

func byteLenForRunes(text string, startRune, endRune int) int {
	if startRune >= endRune || startRune < 0 {
		return 0
	}
	i, rIdx := 0, 0
	startByte, endByte := -1, -1
	for i < len(text) {
		if rIdx == startRune {
			startByte = i
		}
		if rIdx == endRune {
			endByte = i
			break
		}
		_, sz := x11DecodeRune(text[i:])
		if sz == 0 {
			break
		}
		i += sz
		rIdx++
	}
	if startByte < 0 {
		startByte = len(text)
	}
	if endByte < 0 {
		endByte = len(text)
	}
	if endByte < startByte {
		return 0
	}
	return endByte - startByte
}

// --- helpers for verification ---

func (im *x11Ime) Engine() string {
	if im == nil {
		return ""
	}
	im.mu.Lock()
	defer im.mu.Unlock()
	return im.engine
}

func (im *x11Ime) ObjectPath() dbus.ObjectPath {
	if im == nil {
		return ""
	}
	im.mu.Lock()
	defer im.mu.Unlock()
	return im.objectPath
}

// --- IME 接口（S1/S2/S3/S6）---

func (im *x11Ime) EnableIME(rect Rect) {
	if im == nil {
		return
	}
	im.ensureReprobe()
	im.mu.Lock()
	wasFocused := im.focused
	im.focused = true
	im.lastRect = rect
	im.hasRect = true
	purpose := im.purpose
	lastText := im.lastText
	lastCursor := im.lastCursor
	lastAnchor := im.lastAnchor
	hasSurrounding := lastText != "" || lastCursor != 0 || lastAnchor != 0
	im.mu.Unlock()
	if wasFocused {
		x11ImeDebug("EnableIME focused guard skip rect=%v", rect)
		return
	}
	engine, path := im.Engine(), im.ObjectPath()
	im.mu.Lock()
	dirty := im.imeDirty
	im.mu.Unlock()
	x11ImeDebug("EnableIME rect=%v engine=%s path=%s purpose=%d dirty=%v", rect, engine, path, purpose, dirty)
	im.callFocusIn()
	im.callSetCursorLocation(rect)
	im.callSetContentType(purpose)
	if hasSurrounding {
		im.callSetSurroundingText(lastText, lastCursor, lastAnchor)
	}
}

func (im *x11Ime) UpdateCursorRect(rect Rect) {
	if im == nil {
		return
	}
	im.ensureReprobe()
	im.mu.Lock()
	same := im.hasRect && im.lastRect == rect
	im.lastRect = rect
	im.hasRect = true
	composing := im.composing
	focused := im.focused
	im.mu.Unlock()
	if same {
		x11ImeDebug("UpdateCursorRect skip same rect=%v", rect)
		return
	}
	if !focused {
		x11ImeDebug("UpdateCursorRect preheat (not focused) rect=%v", rect)
		return
	}
	if !composing {
		x11ImeDebug("UpdateCursorRect preheat (not composing) rect=%v", rect)
		return
	}
	engine, composingVal := im.Engine(), composing
	im.mu.Lock()
	dirty := im.imeDirty
	im.mu.Unlock()
	x11ImeDebug("UpdateCursorRect rect=%v engine=%s composing=%v dirty=%v", rect, engine, composingVal, dirty)
	im.callSetCursorLocation(rect)
}

func (im *x11Ime) SetContentType(purpose ContentPurpose) {
	if im == nil {
		return
	}
	im.ensureReprobe()
	im.mu.Lock()
	im.purpose = purpose
	focused := im.focused
	path := im.objectPath
	dirty := im.imeDirty
	im.mu.Unlock()
	engine := im.Engine()
	x11ImeDebug("SetContentType purpose=%d engine=%s focused=%v dirty=%v", purpose, engine, focused, dirty)
	if !focused || path == "" {
		return
	}
	im.callSetContentType(purpose)
}

func (im *x11Ime) SetComposing(text string, cursor int) {
	if im == nil {
		return
	}
	im.ensureReprobe()
	// SetComposing here reports surrounding text (InputRouter.pushSurrounding).
	// Do NOT flip the IME composing flag here — composing state is driven
	// ONLY by D-Bus signals (pushPreedit/pushCommit) to keep F-D3 anchor
	// reporting honest (real report only when IME preedit is active).
	im.mu.Lock()
	im.lastText = text
	im.lastCursor = cursor
	im.lastAnchor = cursor
	rect := im.lastRect
	hasRect := im.hasRect
	purpose := im.purpose
	engine := im.engine
	dirty := im.imeDirty
	im.mu.Unlock()
	x11ImeDebug("SetComposing len=%d cur=%d engine=%s dirty=%v", len(text), cursor, engine, dirty)
	if purpose == PurposePassword {
		x11ImeDebug("SetComposing skip password purpose")
		return
	}
	im.callSetSurroundingText(text, cursor, cursor)
	// Cursor rect is driven by UpdateCursorRect (which checks im.composing);
	// do not force a cursor update here to avoid preheat-period spurious reports.
	_ = rect
	_ = hasRect
}

func (im *x11Ime) Commit(text string) {
	if im == nil {
		return
	}
	x11ImeDebug("Commit len=%d engine=%s", len(text), im.Engine())
	im.mu.Lock()
	im.composing = false
	im.mu.Unlock()
	// If app explicitly commits, forward as IME commit so editor's AddText path runs
	if text != "" && im.host != nil {
		im.host.pushIME(Event{Type: EventIME, IMEKind: 1, IMEText: text})
		im.host.WakeUp()
	}
}

func (im *x11Ime) DisableIME() {
	if im == nil {
		return
	}
	im.mu.Lock()
	wasFocused := im.focused
	im.focused = false
	im.composing = false
	im.mu.Unlock()
	if !wasFocused {
		return
	}
	x11ImeDebug("DisableIME engine=%s", im.Engine())
	im.callFocusOut()
}

// --- S3 helpers ---

func (im *x11Ime) callFocusIn() {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("FocusIn skip no object")
		return
	}
	obj := im.ObjectPath()
	var err error
	if im.engineImpl != nil {
		err = im.engineImpl.FocusIn(im.conn, obj)
	} else if eng := x11EngineForName(im.Engine()); eng != nil {
		err = eng.FocusIn(im.conn, obj)
	}
	x11ImeDebug("FocusIn %s err=%v", obj, err)
}

func (im *x11Ime) callFocusOut() {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("FocusOut skip no object")
		return
	}
	obj := im.ObjectPath()
	var err error
	if im.engineImpl != nil {
		err = im.engineImpl.FocusOut(im.conn, obj)
	} else if eng := x11EngineForName(im.Engine()); eng != nil {
		err = eng.FocusOut(im.conn, obj)
	}
	x11ImeDebug("FocusOut %s err=%v", obj, err)
	// F-D7 / P10: FocusOut must clear preedit; push IMECompose empty so editor's composingRange is cleared
	if im.host != nil {
		im.host.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: "", IMEStart: -1, IMEEnd: -1})
		im.host.WakeUp()
	}
	x11ImeDebug("EndComposing after FocusOut")
}

func (im *x11Ime) callSetCursorLocation(rect Rect) {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("SetCursorLocation skip no object rect=%v", rect)
		return
	}
	px, py, pw, ph := im.translateRect(rect)
	x11ImeDebug("SetCursorLocation ii ii rect=%v -> phys x=%d y=%d w=%d h=%d", rect, px, py, pw, ph)
	obj := im.ObjectPath()
	var err error
	if im.engineImpl != nil {
		err = im.engineImpl.SetCursorLocation(im.conn, obj, px, py, pw, ph)
	} else if eng := x11EngineForName(im.Engine()); eng != nil {
		err = eng.SetCursorLocation(im.conn, obj, px, py, pw, ph)
	}
	_ = err
}

func (im *x11Ime) callSetSurroundingText(text string, cursor, anchor int) {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("SetSurroundingText skip no object len=%d", len(text))
		return
	}
	obj := im.ObjectPath()
	if im.engineImpl != nil {
		_ = im.engineImpl.SetSurroundingText(im.conn, obj, text, cursor, anchor)
	} else if eng := x11EngineForName(im.Engine()); eng != nil {
		_ = eng.SetSurroundingText(im.conn, obj, text, cursor, anchor)
	}
}

func (im *x11Ime) callSetContentType(purpose ContentPurpose) {
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("SetContentType skip no object purpose=%d", purpose)
		return
	}
	obj := im.ObjectPath()
	if im.engineImpl != nil {
		_ = im.engineImpl.SetContentType(im.conn, obj, purpose)
	} else if eng := x11EngineForName(im.Engine()); eng != nil {
		_ = eng.SetContentType(im.conn, obj, purpose)
	}
}

// translateRect 逻辑 rect -> 物理根窗口坐标
func (im *x11Ime) translateRect(r Rect) (int, int, int, int) {
	if im == nil || im.host == nil || im.host.st == nil {
		return int(r.X), int(r.Y), int(r.W), int(r.H)
	}
	st := im.host.st
	scale := st.scale
	if scale <= 0 {
		scale = 1
	}
	w := r.W
	h := r.H
	if w <= 0 {
		w = defaultCursorW
	}
	if h <= 0 {
		h = defaultCursorH
	}
	px := int(r.X * scale)
	py := int(r.Y * scale)
	pw := int(w * scale)
	ph := int(h * scale)
	if pw < 1 {
		pw = defaultCursorW
	}
	if st.display != 0 && st.window != 0 && st.root != 0 {
		if x, y, ok := x11TranslateToRoot(st, px, py); ok {
			px, py = x, y
		}
		// B6: RandR 多显修正（查询 monitor 几何，预留 per-monitor scale）
		px, py = x11RandRAdjust(st, px, py)
	}
	return px, py, pw, ph
}

// WantsSurrounding reports that X11 D-Bus needs surrounding on every edit.
func (im *x11Ime) WantsSurrounding() bool { return true }

// IsComposing reports whether a pre-edit session is active.
func (im *x11Ime) IsComposing() bool {
	if im == nil {
		return false
	}
	im.mu.Lock()
	defer im.mu.Unlock()
	return im.composing
}

func isX11ModifierKeysym(ks uint32) bool {
	switch ks {
	case 0xffe1, 0xffe2, // Shift L/R
		0xffe3, 0xffe4, // Control L/R
		0xffe9, 0xffea, // Alt L/R
		0xffeb, 0xffec: // Super/Meta L/R
		return true
	}
	return false
}

func isX11NavKeysym(ks uint32) bool {
	switch ks {
	case 0xff50, // Home
		0xff57, // End
		0xff55, // Page_Up
		0xff56, // Page_Down
		0xff51, // Left
		0xff52, // Up
		0xff53, // Right
		0xff54, // Down
		0xff09, // Tab
		0xff0d, // Return
		0xff1b: // Escape
		return true
	}
	return false
}

// ProcessKeyEvent S4：先走 D-Bus 判 consumed，再本地 Home/End 等分流
// keycode 为 X 硬件码，state 为 X 修饰位，isPress true=Press false=Release
// xTime 为 XKeyEvent.time（ms），fcitx 侧透传，0 时回退 time.Now（兼容旧测试）
// 返回 handled==true 则拦截不再本地插入，50ms 超时按未处理放行
// ibus 的 ProcessKeyEvent 通过 state 的 IBUS_RELEASE_MASK(1<<30) 区分释放，
// 实测 state=1<<30 可正常调通，故不再丢弃释放事件。
func (im *x11Ime) ProcessKeyEvent(keycode uint32, state uint32, isPress bool, xTime ...uint32) bool {
	var xTimeVal uint32
	if len(xTime) > 0 {
		xTimeVal = xTime[0]
	}
	im.mu.Lock()
	dirty := im.imeDirty
	im.mu.Unlock()
	im.ensureReprobe()
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("ProcessKeyEvent skip no object keycode=%d state=%d press=%v dirty=%v", keycode, state, isPress, dirty)
		return false
	}
	var keysym uint32
	if im.host != nil && im.host.st != nil && im.host.st.keycodeToKeysym != nil && im.host.st.display != 0 {
		ks := xKeysymForState(im.host.st, uint(keycode), uint32(state))
		keysym = uint32(ks)
		x11ImeDebug("ProcessKeyEvent keysym=%#x keycode=%d state=%d", keysym, keycode, state)
	}
	obj := im.ObjectPath()
	var handled bool
	var err error
	if im.engineImpl != nil {
		handled, err = im.engineImpl.ProcessKeyEvent(im.conn, obj, keysym, keycode, state, xTimeVal, isPress)
	} else if eng := x11EngineForName(im.Engine()); eng != nil {
		handled, err = eng.ProcessKeyEvent(im.conn, obj, keysym, keycode, state, xTimeVal, isPress)
	} else {
		err = fmt.Errorf("no engine")
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			x11ImeDebug("ProcessKeyEvent deadline -> block to avoid double (m/没 case)")
			return true
		}
		x11ImeDebug("ProcessKeyEvent err %v -> pass-through", err)
		return false
	}
	// 修饰键（Shift/Ctrl/Alt/Meta）不拦截：需让本地感知修饰状态，否则 Shift+字母/方向 的后续组合会丢失修饰
	if handled && isX11ModifierKeysym(keysym) {
		x11ImeDebug("ProcessKeyEvent modifier %#x handled but not blocking (preserve local mods)", keysym)
		return false
	}
	// 英文非组合态：输入法已无 preedit，方向/翻页等导航键应直通本地编辑器（Shift+方向选区等快捷键才有效）
	if handled && !im.IsComposing() && isX11NavKeysym(keysym) {
		x11ImeDebug("ProcessKeyEvent nav %#x handled but not composing -> pass-through (english mode shortcut)", keysym)
		return false
	}
	return handled
}

// ProcessKeyEventAsync 挂起队列等真回话：不靠固定闹钟，回调决定塞不塞
// 与同步版共用同一套 handled/modifier/nav 规则，但不阻塞 drainX。
func (im *x11Ime) ProcessKeyEventAsync(keycode uint32, state uint32, isPress bool, xTime uint32, ev Event) {
	if im == nil || im.host == nil {
		return
	}
	im.mu.Lock()
	dirty := im.imeDirty
	im.mu.Unlock()
	im.ensureReprobe()
	if im == nil || im.conn == nil || im.ObjectPath() == "" {
		x11ImeDebug("ProcessKeyEventAsync skip no object keycode=%d state=%d press=%v dirty=%v -> pass-through push", keycode, state, isPress, dirty)
		im.host.pushPendingKey(ev)
		im.host.WakeUp()
		return
	}
	var keysym uint32
	if im.host != nil && im.host.st != nil && im.host.st.keycodeToKeysym != nil && im.host.st.display != 0 {
		ks := xKeysymForState(im.host.st, uint(keycode), uint32(state))
		keysym = uint32(ks)
		x11ImeDebug("ProcessKeyEventAsync keysym=%#x keycode=%d state=%d", keysym, keycode, state)
	}
	obj := im.ObjectPath()
	conn := im.conn
	host := im.host
	eng := im.Engine()
	// 释放事件的 IBUS_RELEASE_MASK 由引擎层按协议自行处理（G5）：
	// 统一层不再写 `if eng == "ibus"` 这类引擎专属分支，避免与引擎层漂移。
	// 引擎实现：异步路径与同步路径共用同一协议选择（含 fcitx5 的 IBus 兼容模式），
	// 避免两处签名漂移导致按键被误判为未消费。
	engImpl := im.engineImpl
	if engImpl == nil {
		engImpl = x11EngineForName(eng)
	}
	// 异步发 D-Bus，不卡事件泵；回包在 goroutine 回调决定塞不塞
	go func() {
		var handled bool
		var err error
		if engImpl != nil {
			handled, err = engImpl.ProcessKeyEvent(conn, obj, keysym, keycode, state, xTime, isPress)
			x11ImeDebug("ProcessKeyEventAsync %s (%#x,%d,%d)->%v err=%v", eng, keysym, keycode, state, handled, err)
		} else {
			err = fmt.Errorf("no engine")
		}
		// 窗口已关则丢弃 pending，避免向 dead host  push 泄漏
		im.mu.Lock()
		closed := im.closed
		im.mu.Unlock()
		if closed || host == nil {
			return
		}
		if err != nil {
			// 守护真死或极卡超时，兜底当未消费，补发本地，避免英文丢字；双写已靠“等真回话”避免
			x11ImeDebug("ProcessKeyEventAsync err %v -> pass-through push", err)
			host.pushPendingKey(ev)
			host.WakeUp()
			return
		}
		if handled && isX11ModifierKeysym(keysym) {
			x11ImeDebug("ProcessKeyEventAsync modifier %#x handled but not blocking (preserve local mods)", keysym)
			host.pushPendingKey(ev)
			host.WakeUp()
			return
		}
		if handled && !im.IsComposing() && isX11NavKeysym(keysym) {
			x11ImeDebug("ProcessKeyEventAsync nav %#x handled but not composing -> pass-through push (english mode shortcut)", keysym)
			host.pushPendingKey(ev)
			host.WakeUp()
			return
		}
		if handled {
			x11ImeDebug("ProcessKeyEventAsync handled true -> drop local (m/没 case wait true)")
			return
		}
		host.pushPendingKey(ev)
		host.WakeUp()
	}()
}

// x11TruncateSurrounding 复用 textinput.TruncateSurrounding 语义：4000 居中，UTF8 边界安全
func x11TruncateSurrounding(text string, cursor, anchor int) (string, int, int) {
	return imeutil.TruncateSurroundingWithAnchor(text, cursor, anchor)
}
