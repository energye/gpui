//go:build linux

package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// 第2层 引擎层 ibus：私有总线 + 生命周期
// 私有地址来自 ~/.config/ibus/bus/* 的 IBUS_ADDRESS，验 PID、跳 fcitx 污染，dialIbusPrivate 单例复用

var (
	x11IbusMu      sync.Mutex
	x11IbusConn    *dbus.Conn
	x11IbusErr     error
	x11IbusDialing bool // 单飞标记：保证并发只拨一条
	x11IbusCond    = sync.NewCond(&x11IbusMu)
)

// x11ResolveIbusAddress 读 ~/.config/ibus/bus/* → IBUS_ADDRESS
// 规则：遍历文件，解析 IBUS_ADDRESS 与 IBUS_DAEMON_PID，跳过含 fcitx 的污染条目，验 PID 存活，取最新 mtime 的有效条目
func x11ResolveIbusAddress() (string, error) {
	dir := x11IbusBusDir()
	return x11ResolveIbusAddressFromDir(dir)
}

func x11IbusBusDir() string {
	if v := os.Getenv("IBUS_BUS_DIR"); v != "" {
		return v
	}
	home := os.Getenv("HOME")
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	if home == "" {
		home = "/root"
	}
	return filepath.Join(home, ".config/ibus/bus")
}

func x11ResolveIbusAddressFromDir(dir string) (string, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil || len(entries) == 0 {
		return "", os.ErrNotExist
	}
	type cand struct {
		addr  string
		pid   string
		mtime int64
		path  string
	}
	var cands []cand
	for _, p := range entries {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			continue
		}
		addr, pid, hasAddr := x11ParseIbusBusFile(p)
		if !hasAddr || addr == "" {
			continue
		}
		// 验 PID 存活（真正的失效判据）。
		// 注意：不得按「地址/内容含 fcitx」过滤——fcitx5 运行 ibusfrontend 时会以
		// IBUS_DAEMON_PID=<自身 PID> 往此目录写合法条目，地址尾巴带
		// fcitx_random_string。按字样过滤会把它当成"污染"误杀，导致本机
		// （GTK_IM_MODULE=ibus 但实际守护是 fcitx5）候选归零、IME 降级为 nil。
		if pid != "" && !x11PidAlive(pid) {
			x11ImeDebug("ibus bus skip dead pid %s file %s addr %q", pid, p, addr)
			continue
		}
		cands = append(cands, cand{addr: addr, pid: pid, mtime: fi.ModTime().UnixNano(), path: p})
	}
	if len(cands) == 0 {
		return "", os.ErrNotExist
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].mtime > cands[j].mtime })
	best := cands[0]
	x11ImeDebug("ibus bus resolved %q pid %s file %s", best.addr, best.pid, best.path)
	return best.addr, nil
}

func x11ParseIbusBusFile(path string) (addr, pid string, hasAddr bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "IBUS_ADDRESS=") {
			v := strings.TrimPrefix(line, "IBUS_ADDRESS=")
			v = strings.Trim(v, "\"' ")
			addr = v
			hasAddr = true
		} else if strings.HasPrefix(line, "IBUS_DAEMON_PID=") {
			v := strings.TrimPrefix(line, "IBUS_DAEMON_PID=")
			v = strings.Trim(v, "\"' ")
			pid = v
		}
	}
	return addr, pid, hasAddr
}

func x11PidAlive(pidStr string) bool {
	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil || pid <= 0 {
		return false
	}
	// 先用 kill 0 验存在
	if err := syscall.Kill(pid, 0); err == nil {
		return true
	}
	// 回退 /proc 判存在（kill 在容器内可能 EPERM）
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err == nil {
		return true
	}
	return false
}

// dialIbusPrivate 私有总线拨号。
//
// 与旧实现的关键差别：旧版用 sync.Once，**成败都永久缓存**，导致用户把输入法
// 换成真 ibus-daemon 后（地址文件被新守护改写、总线地址整个变了）程序仍抱着
// 启动时那条连接不放，输入上下文建不出来，只能重启进程。
//
// 现在改为「可失效的缓存 + 单飞」：
//   - 命中缓存且连接仍健康 → 直接复用，多窗口连接数恒为 1（S1 纪律不变）；
//   - force=true（已确认换了框架）→ 作废缓存重新解析地址并拨号；
//   - 单飞保证并发调用只拨一条，不会因多窗口同时重探而连接数爆炸。
func dialIbusPrivate(force bool) (*dbus.Conn, error) {
	var old *dbus.Conn

	x11IbusMu.Lock()
	if !force && x11IbusConn != nil && x11IbusConn.Connected() {
		c := x11IbusConn
		x11IbusMu.Unlock()
		return c, nil
	}
	// 失效：清掉旧连接（force 重拨，或旧连接已断）
	if x11IbusConn != nil {
		old = x11IbusConn
		x11IbusConn = nil
	}
	x11IbusErr = nil
	// 已有重拨在进行 → 不重复拨号（单飞：保证多窗口同时触发也只建一条连接）
	if x11IbusDialing {
		pending := x11IbusCond
		x11IbusMu.Unlock()
		return waitIbusDial(pending)
	}
	x11IbusDialing = true
	x11IbusMu.Unlock()

	c, err := dialIbusPrivateSlow()

	x11IbusMu.Lock()
	x11IbusDialing = false
	// 仅当本次拨号仍是「最新一次」时才写回，避免并发的旧拨号覆盖新结果
	if err == nil && c != nil {
		x11IbusConn = c
	} else {
		x11IbusErr = err
	}
	x11IbusMu.Unlock()
	x11IbusCond.Broadcast()

	// 旧连接放到最后再关：确保上面的状态已收敛，避免窗口期拿到半死连接
	if old != nil && old != c {
		_ = old.Close()
	}
	return c, err
}

// waitIbusDial 等待并发中的那次拨号完成，共享其结果（单飞）。
func waitIbusDial(cond *sync.Cond) (*dbus.Conn, error) {
	// 用带超时的等待，避免拨号协程异常时永久挂住调用方
	done := make(chan struct{})
	var c *dbus.Conn
	var err error
	go func() {
		cond.L.Lock()
		for x11IbusDialing {
			cond.Wait()
		}
		c, err = x11IbusConn, x11IbusErr
		cond.L.Unlock()
		close(done)
	}()
	select {
	case <-done:
		return c, err
	case <-time.After(2 * time.Second):
		return nil, fmt.Errorf("ibus dial wait timeout")
	}
}

// dialIbusPrivateSlow 真正解析地址 + 拨号 + 订阅（不含缓存与并发控制）。
func dialIbusPrivateSlow() (*dbus.Conn, error) {
	addr, err := x11ResolveIbusAddress()
	if err != nil {
		x11ImeDebug("ibus private no addr: %v", err)
		return nil, err
	}
	x11ImeDebug("ibus private dial addr=%q", addr)
	c, err := dialWithTimeout(addr, 800)
	if err != nil {
		x11ImeDebug("ibus private dial failed: %v", err)
		return nil, err
	}
	x11ImeDebug("ibus private dial ok addr=%q", addr)
	// 私有/回退总线订阅 NameOwnerChanged（与会话总线一致）及 ibus 信号
	for _, rule := range dbusMatchRules {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
		err := c.BusObject().CallWithContext(ctx2, dbusServiceDBus+".AddMatch", 0, rule).Err
		cancel2()
		if err != nil {
			x11ImeDebug("ibus AddMatch %q note: %v", rule, err)
		} else {
			x11ImeDebug("ibus AddMatch %q ok", rule)
		}
	}
	go x11GlobalNameOwnerLoop(c)
	go x11BusWatchLoop(c)
	return c, nil
}

func dialWithTimeout(addr string, ms int) (*dbus.Conn, error) {
	type res struct {
		c   *dbus.Conn
		err error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := dbus.Connect(addr)
		ch <- res{c, err}
	}()
	select {
	case r := <-ch:
		return r.c, r.err
	case <-time.After(time.Duration(ms) * time.Millisecond):
		go func() {
			r := <-ch
			if r.c != nil {
				_ = r.c.Close()
			}
		}()
		return nil, fmt.Errorf("dial timeout %dms", ms)
	}
}

func resetIbusPrivateForTest() {
	x11IbusMu.Lock()
	if x11IbusConn != nil {
		_ = x11IbusConn.Close()
	}
	x11IbusConn = nil
	x11IbusErr = nil
	x11IbusDialing = false
	x11IbusMu.Unlock()
}

func (e *ibusEngine) Name() string { return "ibus" }
func (e *ibusEngine) Caps() uint32 { return ibusCaps }

// Bus ibus 走私有总线。force=true 时重新解析地址并拨号，用于「换了输入法框架」
// 后连上新守护（总线地址本身会变，必须重拨）。
//
// 地址文件解析不出来时（真机实测：fcitx5 跑 ibusfrontend 时会把
// ~/.config/ibus/bus/ 下的条目写成空值，IBUS_ADDRESS= / IBUS_DAEMON_PID=），
// 回落到**会话总线**再验一次。理由：
//   - fcitx5 的 ibusfrontend 不开私有总线，它把 IBus 服务直接开在会话总线上，
//     地址文件里写的就是会话总线地址——会话总线本来就是它真正的服务位置。
//   - 真 ibus-daemon 也会在会话总线上**占名**（名在此、物在彼），且实测
//     NameHasOwner 在会话总线上可见，可作为「这条路通不通」的判据。
//
// 只有会话总线上确实看得见 IBus 名才回落，避免无守护时无谓地拿到一条空连接。
func (e *ibusEngine) Bus(force bool) (*dbus.Conn, error) {
	conn, err := dialIbusPrivate(force)
	if err == nil && conn != nil {
		return conn, nil
	}
	if sess, serr := sharedFcitxConn(); serr == nil && sess != nil {
		if has, _ := dbusHasOwner(sess, dbusServiceIBus); has {
			x11ImeDebug("ibus private bus unavailable (%v), fallback to session bus (IBus name visible there)", err)
			return sess, nil
		}
	}
	if err == nil {
		err = fmt.Errorf("nil ibus private conn")
	}
	return nil, err
}

func (e *ibusEngine) CreateInputContext(conn *dbus.Conn, timeout time.Duration) (dbus.ObjectPath, error) {
	if conn == nil {
		return "", fmt.Errorf("nil conn")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	clientName := x11ClientName()
	x11ImeDebug("ibus CreateInputContext(s) client_name=%q", clientName)
	obj := conn.Object(dbusServiceIBus, dbusPathIBusBus)
	var path dbus.ObjectPath
	err := obj.CallWithContext(ctx, dbusIfaceIBus+".CreateInputContext", dbus.FlagNoAutoStart, clientName).Store(&path)
	if err != nil {
		x11ImeDebug("ibus single param failed, try dual: %v", err)
		ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
		defer cancel2()
		if err2 := obj.CallWithContext(ctx2, dbusIfaceIBus+".CreateInputContext", dbus.FlagNoAutoStart, clientName, clientName).Store(&path); err2 != nil {
			return "", err2
		}
	}
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	return path, nil
}

func (e *ibusEngine) SetCapabilities(conn *dbus.Conn, obj dbus.ObjectPath, caps uint32) error {
	if conn == nil {
		return fmt.Errorf("nil conn")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	o := conn.Object(dbusServiceIBus, obj)
	err := o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetCapabilities", 0, caps).Err
	if err != nil {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel2()
		if err2 := o.CallWithContext(ctx2, dbusIfaceIBusCtx+".SetCapability", 0, caps).Err; err2 == nil {
			return nil
		}
		return err
	}
	return nil
}

func (e *ibusEngine) Destroy(conn *dbus.Conn, obj dbus.ObjectPath) error {
	if conn == nil || obj == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	o := conn.Object(dbusServiceIBus, obj)
	err := o.CallWithContext(ctx, dbusServiceIBus+".Service.Destroy", 0).Err
	if err != nil {
		err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".Destroy", 0).Err
	}
	if err != nil {
		err = o.CallWithContext(ctx, "Destroy", 0).Err
	}
	x11ImeDebug("Destroy ibus %s err=%v", obj, err)
	return err
}

func (e *ibusEngine) FocusIn(conn *dbus.Conn, obj dbus.ObjectPath) error {
	if conn == nil || obj == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	o := conn.Object(dbusServiceIBus, obj)
	err := o.CallWithContext(ctx, dbusIfaceIBusCtx+".FocusIn", 0).Err
	if err != nil {
		err = o.CallWithContext(ctx, "FocusIn", 0).Err
	}
	x11ImeDebug("FocusIn %s err=%v", obj, err)
	return err
}

func (e *ibusEngine) FocusOut(conn *dbus.Conn, obj dbus.ObjectPath) error {
	if conn == nil || obj == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	o := conn.Object(dbusServiceIBus, obj)
	err := o.CallWithContext(ctx, dbusIfaceIBusCtx+".FocusOut", 0).Err
	if err != nil {
		err = o.CallWithContext(ctx, "FocusOut", 0).Err
	}
	x11ImeDebug("FocusOut %s err=%v", obj, err)
	return err
}

func (e *ibusEngine) SetCursorLocation(conn *dbus.Conn, obj dbus.ObjectPath, x, y, w, h int) error {
	if conn == nil || obj == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	o := conn.Object(dbusServiceIBus, obj)
	err := o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetCursorLocation", 0, int32(x), int32(y), int32(w), int32(h)).Err
	if err != nil {
		err = o.CallWithContext(ctx, "SetCursorLocation", 0, int32(x), int32(y), int32(w), int32(h)).Err
	}
	x11ImeDebug("ibus SetCursorLocation(%d,%d,%d,%d) err=%v", x, y, w, h, err)
	return err
}

func (e *ibusEngine) SetSurroundingText(conn *dbus.Conn, obj dbus.ObjectPath, text string, cursor, anchor int) error {
	if conn == nil || obj == "" {
		return nil
	}
	t, c, a := x11TruncateSurrounding(text, cursor, anchor)
	x11ImeDebug("SetSurroundingText len=%d->%d cursor=%d->%d anchor=%d->%d", len(text), len(t), cursor, c, anchor, a)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	ibusText := ibusFullPayload{
		Name:  ibusTextName,
		Props: map[string]dbus.Variant{},
		Text:  t,
		Attrs: dbus.MakeVariant([]interface{}{}),
	}
	v := dbus.MakeVariant(ibusText)
	o := conn.Object(dbusServiceIBus, obj)
	err := o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetSurroundingText", 0, v, uint32(c), uint32(a)).Err
	if err != nil {
		v2 := dbus.MakeVariant(ibusTextPayload{Text: t, Attrs: nil})
		err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetSurroundingText", 0, v2, uint32(c), uint32(a)).Err
	}
	if err != nil {
		err = o.CallWithContext(ctx, "SetSurroundingText", 0, v, uint32(c), uint32(a)).Err
	}
	x11ImeDebug("ibus SetSurroundingText err=%v variant=%s", err, v.Signature())
	return err
}

func (e *ibusEngine) SetContentType(conn *dbus.Conn, obj dbus.ObjectPath, purpose ContentPurpose) error {
	if conn == nil || obj == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	o := conn.Object(dbusServiceIBus, obj)
	type ibusContentType struct {
		Purpose uint32
		Hints   uint32
	}
	v := dbus.MakeVariant(ibusContentType{Purpose: uint32(purpose), Hints: 0})
	err := o.CallWithContext(ctx, dbusServiceDBus+".Properties.Set", 0, dbusIfaceIBusCtx, "ContentType", v).Err
	if err != nil {
		err = o.CallWithContext(ctx, dbusIfaceIBusCtx+".SetContentType", 0, uint32(purpose), uint32(0)).Err
	}
	if err != nil {
		err = o.CallWithContext(ctx, "SetContentType", 0, uint32(purpose), uint32(0)).Err
	}
	if err != nil {
		err = o.CallWithContext(ctx, dbusServiceDBus+".Properties.Set", 0, dbusIfaceIBusCtx, "ContentType", dbus.MakeVariant(uint32(purpose))).Err
	}
	x11ImeDebug("ibus SetContentType purpose=%d err=%v", purpose, err)
	return err
}

func (e *ibusEngine) ProcessKeyEvent(conn *dbus.Conn, obj dbus.ObjectPath, keysym, keycode, state, xTime uint32, isPress bool) (bool, error) {
	if conn == nil || obj == "" {
		return false, fmt.Errorf("nil conn")
	}
	if !isPress {
		state |= 1 << 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	o := conn.Object(dbusServiceIBus, obj)
	var handled bool
	err := o.CallWithContext(ctx, dbusIfaceIBusCtx+".ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state)).Store(&handled)
	if err != nil {
		err = o.CallWithContext(ctx, "ProcessKeyEvent", 0, uint32(keysym), uint32(keycode), uint32(state)).Store(&handled)
	}
	x11ImeDebug("ibus ProcessKeyEvent uuu (%#x,%d,%d)->%v err=%v", keysym, keycode, state, handled, err)
	return handled, err
}
