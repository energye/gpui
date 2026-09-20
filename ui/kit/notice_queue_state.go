package kit

import (
	"sync"

	"github.com/energye/gpui/ui/kit/internal/scope"
)

// NoticeQueueResult is the promise outcome: closed exactly once.
type NoticeQueueResult struct {
	Closed bool
	Key    string
}

// NoticeQueueMessageEntry is one lightweight top-center tip.
type NoticeQueueMessageEntry struct {
	ID           uint64
	Key          string
	Content      string
	Type         NoticeQueueMessageType
	Duration     float64
	Remaining    float64
	PauseOnHover bool
	Hovered      bool
	IconName     string
	OnClick      func()
	OnClose      func()
}

// NoticeQueueNotificationEntry is one corner card.
type NoticeQueueNotificationEntry struct {
	ID           uint64
	Key          string
	Title        string
	Description  string
	Type         NoticeQueueNotificationType
	Placement    NoticeQueuePlacement
	Duration     float64
	Remaining    float64
	Closable     bool
	ShowProgress bool
	PauseOnHover bool
	Hovered      bool
	IconName     string
	Actions      []NoticeQueueAction
	Role         string
	OnClick      func()
	OnClose      func()
}

// NoticeQueueMessageHandle is the imperative message handle.
type NoticeQueueMessageHandle struct {
	host     *NoticeQueueInstance
	id       uint64
	done     chan NoticeQueueResult
	once     sync.Once
	mu       sync.Mutex
	thens    []func()
	resolved bool
	result   NoticeQueueResult
}

// ID returns the entry id.
func (h *NoticeQueueMessageHandle) ID() uint64 {
	if h == nil {
		return 0
	}
	return h.id
}

// Done returns the promise channel closed exactly once on close.
func (h *NoticeQueueMessageHandle) Done() <-chan NoticeQueueResult {
	return h.done
}

// Then registers afterClose invoked exactly once on close.
func (h *NoticeQueueMessageHandle) Then(fn func()) {
	if h == nil || fn == nil {
		return
	}
	h.mu.Lock()
	if h.resolved {
		res := h.result
		_ = res
		h.mu.Unlock()
		fn()
		return
	}
	h.thens = append(h.thens, fn)
	h.mu.Unlock()
}

// Destroy closes one message by handle.
func (h *NoticeQueueMessageHandle) Destroy() {
	if h == nil || h.host == nil {
		return
	}
	h.host.closeMessage(h.id)
}

// NoticeQueueNotificationHandle is the imperative notification handle.
type NoticeQueueNotificationHandle struct {
	host     *NoticeQueueInstance
	id       uint64
	done     chan NoticeQueueResult
	once     sync.Once
	mu       sync.Mutex
	thens    []func()
	resolved bool
	result   NoticeQueueResult
}

// ID returns the entry id.
func (h *NoticeQueueNotificationHandle) ID() uint64 {
	if h == nil {
		return 0
	}
	return h.id
}

// Done returns the promise channel closed exactly once on close.
func (h *NoticeQueueNotificationHandle) Done() <-chan NoticeQueueResult {
	return h.done
}

// Then registers afterClose invoked exactly once on close.
func (h *NoticeQueueNotificationHandle) Then(fn func()) {
	if h == nil || fn == nil {
		return
	}
	h.mu.Lock()
	if h.resolved {
		h.mu.Unlock()
		fn()
		return
	}
	h.thens = append(h.thens, fn)
	h.mu.Unlock()
}

// Destroy closes one notification by handle.
func (h *NoticeQueueNotificationHandle) Destroy() {
	if h == nil || h.host == nil {
		return
	}
	h.host.closeNotification(h.id)
}

// NoticeQueueInstance owns the message single queue plus the
// notification six-corner pools for one host mount.
type NoticeQueueInstance struct {
	props         NoticeQueueProps
	ctx           scope.Ctx
	mu            sync.Mutex
	seq           uint64
	messages      []*NoticeQueueMessageEntry
	notifications []*NoticeQueueNotificationEntry
	msgHandles    map[uint64]*NoticeQueueMessageHandle
	ntfHandles    map[uint64]*NoticeQueueNotificationHandle
	mounted       bool
}

func newNoticeQueueInstance(ctx scope.Ctx, props NoticeQueueProps) *NoticeQueueInstance {
	props = normalizeNoticeQueueProps(props)
	return &NoticeQueueInstance{
		props:      props,
		ctx:        ctx.Normalize(),
		msgHandles: make(map[uint64]*NoticeQueueMessageHandle),
		ntfHandles: make(map[uint64]*NoticeQueueNotificationHandle),
	}
}

func normalizeNoticeQueueProps(p NoticeQueueProps) NoticeQueueProps {
	if p.ZIndexBase == 0 {
		p.ZIndexBase = 1000
	}
	if !p.MessageTopSet || p.MessageTop <= 0 {
		if !p.MessageTopSet {
			p.MessageTop = 8
		}
	}
	if !p.MessageDurationSet {
		p.MessageDuration = 3
	}
	if !p.NotificationDurSet {
		p.NotificationDuration = 4.5
	}
	if !p.NotificationPlaceSet || p.NotificationPlacement == "" {
		p.NotificationPlacement = NoticeQueuePlacementTopRight
	}
	if !p.NotificationTopSet {
		p.NotificationTop = 24
	}
	if !p.NotificationBotSet {
		p.NotificationBottom = 24
	}
	if !p.StackThresholdSet || p.StackThreshold <= 0 {
		p.StackThreshold = 3
	}
	if !p.PauseOnHoverSet {
		p.PauseOnHover = true
	}
	return p
}

// Mount marks the host live.
func (in *NoticeQueueInstance) Mount(ctx scope.Ctx) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.mounted = true
}

// Update swaps props/ctx snapshots.
func (in *NoticeQueueInstance) Update(ctx scope.Ctx, next NoticeQueueProps) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.props = normalizeNoticeQueueProps(next)
}

// Unmount destroys all entries top-down.
func (in *NoticeQueueInstance) Unmount() {
	if in == nil {
		return
	}
	in.DestroyAll()
	in.mu.Lock()
	in.mounted = false
	in.mu.Unlock()
}

// SetState runs f under the host lock (sole state mutation gate).
func (in *NoticeQueueInstance) SetState(f func(*NoticeQueueInstance)) {
	if in == nil || f == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	f(in)
}

func resolveMessageDuration(props NoticeQueueProps, cfg NoticeQueueMessageConfig) float64 {
	if cfg.DurationSet {
		return cfg.Duration
	}
	return props.MessageDuration
}

func resolveNotificationDuration(props NoticeQueueProps, cfg NoticeQueueNotificationConfig) float64 {
	if cfg.DurationSet {
		return cfg.Duration
	}
	return props.NotificationDuration
}

func resolvePauseOnHover(props NoticeQueueProps, set bool, v bool) bool {
	if set {
		return v
	}
	return props.PauseOnHover
}

func resolveNotificationPlacement(props NoticeQueueProps, cfg NoticeQueueNotificationConfig) NoticeQueuePlacement {
	if cfg.PlacementSet && cfg.Placement != "" {
		return cfg.Placement
	}
	if props.NotificationPlacement != "" {
		return props.NotificationPlacement
	}
	return NoticeQueuePlacementTopRight
}

func (in *NoticeQueueInstance) findMessageLocked(id uint64) int {
	for i, e := range in.messages {
		if e.ID == id {
			return i
		}
	}
	return -1
}

func (in *NoticeQueueInstance) findMessageByKeyLocked(key string) int {
	if key == "" {
		return -1
	}
	for i, e := range in.messages {
		if e.Key == key {
			return i
		}
	}
	return -1
}

func (in *NoticeQueueInstance) findNotificationLocked(id uint64) int {
	for i, e := range in.notifications {
		if e.ID == id {
			return i
		}
	}
	return -1
}

func (in *NoticeQueueInstance) findNotificationByKeyLocked(key string) int {
	if key == "" {
		return -1
	}
	for i, e := range in.notifications {
		if e.Key == key {
			return i
		}
	}
	return -1
}

func (in *NoticeQueueInstance) poolCountLocked(pl NoticeQueuePlacement) int {
	n := 0
	for _, e := range in.notifications {
		if e.Placement == pl {
			n++
		}
	}
	return n
}

func (in *NoticeQueueInstance) dropOldestMessageLocked() {
	if len(in.messages) == 0 {
		return
	}
	e := in.messages[0]
	in.messages = in.messages[1:]
	in.resolveMessageHandleLocked(e, true)
}

func (in *NoticeQueueInstance) dropOldestInPoolLocked(pl NoticeQueuePlacement) {
	for i, e := range in.notifications {
		if e.Placement == pl {
			in.notifications = append(in.notifications[:i], in.notifications[i+1:]...)
			in.resolveNotificationHandleLocked(e, true)
			return
		}
	}
}

func (in *NoticeQueueInstance) resolveMessageHandleLocked(e *NoticeQueueMessageEntry, closed bool) {
	h, ok := in.msgHandles[e.ID]
	if !ok {
		if e.OnClose != nil {
			oc := e.OnClose
			// OnClose invoked by caller outside lock.
			_ = oc
		}
		return
	}
	delete(in.msgHandles, e.ID)
	res := NoticeQueueResult{Closed: closed, Key: e.Key}
	h.once.Do(func() {
		h.done <- res
		close(h.done)
	})
	h.mu.Lock()
	h.resolved = true
	h.result = res
	thens := append([]func(){}, h.thens...)
	h.thens = nil
	onClose := e.OnClose
	h.mu.Unlock()
	for _, fn := range thens {
		fn()
	}
	if onClose != nil {
		onClose()
	}
}

func (in *NoticeQueueInstance) resolveNotificationHandleLocked(e *NoticeQueueNotificationEntry, closed bool) {
	h, ok := in.ntfHandles[e.ID]
	if !ok {
		return
	}
	delete(in.ntfHandles, e.ID)
	res := NoticeQueueResult{Closed: closed, Key: e.Key}
	h.once.Do(func() {
		h.done <- res
		close(h.done)
	})
	h.mu.Lock()
	h.resolved = true
	h.result = res
	thens := append([]func(){}, h.thens...)
	h.thens = nil
	onClose := e.OnClose
	h.mu.Unlock()
	for _, fn := range thens {
		fn()
	}
	if onClose != nil {
		onClose()
	}
}

// OpenMessage appends a tip or updates the same key in place.
func (in *NoticeQueueInstance) OpenMessage(cfg NoticeQueueMessageConfig) *NoticeQueueMessageHandle {
	if in == nil {
		return nil
	}
	if cfg.Type == "" {
		cfg.Type = NoticeQueueMessageInfo
	}
	in.mu.Lock()
	if idx := in.findMessageByKeyLocked(cfg.Key); idx >= 0 && cfg.Key != "" {
		e := in.messages[idx]
		e.Content = cfg.Content
		e.Type = cfg.Type
		e.Duration = resolveMessageDuration(in.props, cfg)
		e.Remaining = e.Duration
		e.PauseOnHover = resolvePauseOnHover(in.props, cfg.PauseOnHoverSet, cfg.PauseOnHover)
		e.IconName = cfg.IconName
		e.OnClick = cfg.OnClick
		e.OnClose = cfg.OnClose
		h := in.msgHandles[e.ID]
		in.mu.Unlock()
		return h
	}
	in.seq++
	id := in.seq
	dur := resolveMessageDuration(in.props, cfg)
	key := cfg.Key
	if key == "" {
		// Auto key keeps handles addressable without colliding updates.
		key = string(rune('m')) + itoa(id)
	}
	e := &NoticeQueueMessageEntry{
		ID:           id,
		Key:          key,
		Content:      cfg.Content,
		Type:         cfg.Type,
		Duration:     dur,
		Remaining:    dur,
		PauseOnHover: resolvePauseOnHover(in.props, cfg.PauseOnHoverSet, cfg.PauseOnHover),
		IconName:     cfg.IconName,
		OnClick:      cfg.OnClick,
		OnClose:      cfg.OnClose,
	}
	if in.props.MaxCount > 0 {
		for len(in.messages) >= in.props.MaxCount {
			in.dropOldestMessageLocked()
		}
	}
	in.messages = append(in.messages, e)
	h := &NoticeQueueMessageHandle{host: in, id: id, done: make(chan NoticeQueueResult, 1)}
	in.msgHandles[id] = h
	in.mu.Unlock()
	return h
}

// Message convenience openers (content + optional duration).
func (in *NoticeQueueInstance) MessageInfo(content string, duration ...float64) *NoticeQueueMessageHandle {
	return in.OpenMessage(messageArgs(content, NoticeQueueMessageInfo, duration))
}

func (in *NoticeQueueInstance) MessageSuccess(content string, duration ...float64) *NoticeQueueMessageHandle {
	return in.OpenMessage(messageArgs(content, NoticeQueueMessageSuccess, duration))
}

func (in *NoticeQueueInstance) MessageError(content string, duration ...float64) *NoticeQueueMessageHandle {
	return in.OpenMessage(messageArgs(content, NoticeQueueMessageError, duration))
}

func (in *NoticeQueueInstance) MessageWarning(content string, duration ...float64) *NoticeQueueMessageHandle {
	return in.OpenMessage(messageArgs(content, NoticeQueueMessageWarning, duration))
}

func (in *NoticeQueueInstance) MessageLoading(content string, duration ...float64) *NoticeQueueMessageHandle {
	return in.OpenMessage(messageArgs(content, NoticeQueueMessageLoading, duration))
}

func messageArgs(content string, t NoticeQueueMessageType, duration []float64) NoticeQueueMessageConfig {
	cfg := DefaultNoticeQueueMessageConfig()
	cfg.Content = content
	cfg.Type = t
	if len(duration) > 0 {
		cfg.Duration = duration[0]
		cfg.DurationSet = true
	}
	return cfg
}

// OpenNotification appends a card into its placement pool or updates same key.
func (in *NoticeQueueInstance) OpenNotification(cfg NoticeQueueNotificationConfig) *NoticeQueueNotificationHandle {
	if in == nil {
		return nil
	}
	if cfg.Type == "" {
		cfg.Type = NoticeQueueNotificationOpen
	}
	pl := resolveNotificationPlacement(in.props, cfg)
	if cfg.Role == "" {
		if cfg.Type == NoticeQueueNotificationOpen && cfg.Title == "" {
			cfg.Role = "alert"
		} else if cfg.Role == "" {
			cfg.Role = "alert"
		}
	}
	closable := true
	if cfg.ClosableSet {
		closable = cfg.Closable
	}
	in.mu.Lock()
	if idx := in.findNotificationByKeyLocked(cfg.Key); idx >= 0 && cfg.Key != "" {
		e := in.notifications[idx]
		e.Title = cfg.Title
		e.Description = cfg.Description
		e.Type = cfg.Type
		e.Placement = pl
		e.Duration = resolveNotificationDuration(in.props, cfg)
		e.Remaining = e.Duration
		e.Closable = closable
		e.ShowProgress = cfg.ShowProgress
		e.PauseOnHover = resolvePauseOnHover(in.props, cfg.PauseOnHoverSet, cfg.PauseOnHover)
		e.IconName = cfg.IconName
		e.Actions = cfg.Actions
		if cfg.Role != "" {
			e.Role = cfg.Role
		}
		e.OnClick = cfg.OnClick
		e.OnClose = cfg.OnClose
		h := in.ntfHandles[e.ID]
		in.mu.Unlock()
		return h
	}
	in.seq++
	id := in.seq
	dur := resolveNotificationDuration(in.props, cfg)
	key := cfg.Key
	if key == "" {
		key = string(rune('n')) + itoa(id)
	}
	e := &NoticeQueueNotificationEntry{
		ID:           id,
		Key:          key,
		Title:        cfg.Title,
		Description:  cfg.Description,
		Type:         cfg.Type,
		Placement:    pl,
		Duration:     dur,
		Remaining:    dur,
		Closable:     closable,
		ShowProgress: cfg.ShowProgress,
		PauseOnHover: resolvePauseOnHover(in.props, cfg.PauseOnHoverSet, cfg.PauseOnHover),
		IconName:     cfg.IconName,
		Actions:      cfg.Actions,
		Role:         cfg.Role,
		OnClick:      cfg.OnClick,
		OnClose:      cfg.OnClose,
	}
	if e.Role == "" {
		e.Role = "alert"
	}
	if in.props.MaxCount > 0 {
		for in.poolCountLocked(pl) >= in.props.MaxCount {
			in.dropOldestInPoolLocked(pl)
		}
	}
	in.notifications = append(in.notifications, e)
	h := &NoticeQueueNotificationHandle{host: in, id: id, done: make(chan NoticeQueueResult, 1)}
	in.ntfHandles[id] = h
	in.mu.Unlock()
	return h
}

// Notification convenience openers.
func (in *NoticeQueueInstance) NotifyOpen(title, desc string) *NoticeQueueNotificationHandle {
	cfg := DefaultNoticeQueueNotificationConfig()
	cfg.Title, cfg.Description = title, desc
	return in.OpenNotification(cfg)
}

func (in *NoticeQueueInstance) NotifyInfo(title, desc string) *NoticeQueueNotificationHandle {
	cfg := DefaultNoticeQueueNotificationConfig()
	cfg.Type = NoticeQueueNotificationInfo
	cfg.Title, cfg.Description = title, desc
	return in.OpenNotification(cfg)
}

func (in *NoticeQueueInstance) NotifySuccess(title, desc string) *NoticeQueueNotificationHandle {
	cfg := DefaultNoticeQueueNotificationConfig()
	cfg.Type = NoticeQueueNotificationSuccess
	cfg.Title, cfg.Description = title, desc
	return in.OpenNotification(cfg)
}

func (in *NoticeQueueInstance) NotifyWarning(title, desc string) *NoticeQueueNotificationHandle {
	cfg := DefaultNoticeQueueNotificationConfig()
	cfg.Type = NoticeQueueNotificationWarning
	cfg.Title, cfg.Description = title, desc
	return in.OpenNotification(cfg)
}

func (in *NoticeQueueInstance) NotifyError(title, desc string) *NoticeQueueNotificationHandle {
	cfg := DefaultNoticeQueueNotificationConfig()
	cfg.Type = NoticeQueueNotificationError
	cfg.Title, cfg.Description = title, desc
	return in.OpenNotification(cfg)
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func (in *NoticeQueueInstance) closeMessage(id uint64) {
	if in == nil {
		return
	}
	in.mu.Lock()
	idx := in.findMessageLocked(id)
	if idx < 0 {
		in.mu.Unlock()
		return
	}
	e := in.messages[idx]
	in.messages = append(in.messages[:idx], in.messages[idx+1:]...)
	in.resolveMessageHandleLocked(e, true)
	in.mu.Unlock()
}

func (in *NoticeQueueInstance) closeNotification(id uint64) {
	if in == nil {
		return
	}
	in.mu.Lock()
	idx := in.findNotificationLocked(id)
	if idx < 0 {
		in.mu.Unlock()
		return
	}
	e := in.notifications[idx]
	in.notifications = append(in.notifications[:idx], in.notifications[idx+1:]...)
	in.resolveNotificationHandleLocked(e, true)
	in.mu.Unlock()
}

// DestroyMessage closes one key or clears the message queue when key empty.
func (in *NoticeQueueInstance) DestroyMessage(key ...string) {
	if in == nil {
		return
	}
	var k string
	if len(key) > 0 {
		k = key[0]
	}
	for {
		in.mu.Lock()
		if len(in.messages) == 0 {
			in.mu.Unlock()
			return
		}
		if k == "" {
			top := in.messages[len(in.messages)-1]
			in.mu.Unlock()
			in.closeMessage(top.ID)
			continue
		}
		idx := in.findMessageByKeyLocked(k)
		if idx < 0 {
			in.mu.Unlock()
			return
		}
		id := in.messages[idx].ID
		in.mu.Unlock()
		in.closeMessage(id)
		return
	}
}

// DestroyNotification closes one key or clears all pools when key empty.
func (in *NoticeQueueInstance) DestroyNotification(key ...string) {
	if in == nil {
		return
	}
	var k string
	if len(key) > 0 {
		k = key[0]
	}
	for {
		in.mu.Lock()
		if len(in.notifications) == 0 {
			in.mu.Unlock()
			return
		}
		if k == "" {
			top := in.notifications[len(in.notifications)-1]
			in.mu.Unlock()
			in.closeNotification(top.ID)
			continue
		}
		idx := in.findNotificationByKeyLocked(k)
		if idx < 0 {
			in.mu.Unlock()
			return
		}
		id := in.notifications[idx].ID
		in.mu.Unlock()
		in.closeNotification(id)
		return
	}
}

// DestroyAll clears messages plus all notification pools top-down.
func (in *NoticeQueueInstance) DestroyAll() {
	if in == nil {
		return
	}
	for {
		in.mu.Lock()
		var mid, nid uint64
		hasM := len(in.messages) > 0
		hasN := len(in.notifications) > 0
		if hasM {
			mid = in.messages[len(in.messages)-1].ID
		}
		if hasN {
			nid = in.notifications[len(in.notifications)-1].ID
		}
		in.mu.Unlock()
		if !hasM && !hasN {
			return
		}
		if hasM {
			in.closeMessage(mid)
		}
		if hasN {
			in.closeNotification(nid)
		}
	}
}

// Tick advances the virtual clock; hovered pauseOnHover entries freeze.
func (in *NoticeQueueInstance) Tick(dt float64) {
	if in == nil || dt <= 0 {
		return
	}
	for {
		in.mu.Lock()
		var expireM uint64
		var expireN uint64
		for _, e := range in.messages {
			if e.Duration <= 0 {
				continue
			}
			if e.PauseOnHover && e.Hovered {
				continue
			}
			e.Remaining -= dt
			if e.Remaining <= 0 && expireM == 0 {
				expireM = e.ID
			}
		}
		for _, e := range in.notifications {
			if e.Duration <= 0 {
				continue
			}
			if e.PauseOnHover && e.Hovered {
				continue
			}
			e.Remaining -= dt
			if e.Remaining <= 0 && expireN == 0 {
				expireN = e.ID
			}
		}
		in.mu.Unlock()
		if expireM == 0 && expireN == 0 {
			// Clamp tiny negatives caused by overshoot for determinism.
			in.mu.Lock()
			for _, e := range in.messages {
				if e.Duration > 0 && e.Remaining < 0 {
					e.Remaining = 0
				}
			}
			for _, e := range in.notifications {
				if e.Duration > 0 && e.Remaining < 0 {
					e.Remaining = 0
				}
			}
			in.mu.Unlock()
			return
		}
		if expireM != 0 {
			in.closeMessage(expireM)
		}
		if expireN != 0 {
			in.closeNotification(expireN)
		}
	}
}

// SetMessageHovered freezes or resumes one message by key.
func (in *NoticeQueueInstance) SetMessageHovered(key string, hovered bool) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if idx := in.findMessageByKeyLocked(key); idx >= 0 {
		in.messages[idx].Hovered = hovered
	}
}

// SetNotificationHovered freezes or resumes one card by key.
func (in *NoticeQueueInstance) SetNotificationHovered(key string, hovered bool) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if idx := in.findNotificationByKeyLocked(key); idx >= 0 {
		in.notifications[idx].Hovered = hovered
	}
}

// ClickMessage triggers onClick without closing.
func (in *NoticeQueueInstance) ClickMessage(id uint64) {
	if in == nil {
		return
	}
	in.mu.Lock()
	idx := in.findMessageLocked(id)
	if idx < 0 {
		in.mu.Unlock()
		return
	}
	fn := in.messages[idx].OnClick
	in.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// ClickNotification triggers the card onClick without closing.
func (in *NoticeQueueInstance) ClickNotification(id uint64) {
	if in == nil {
		return
	}
	in.mu.Lock()
	idx := in.findNotificationLocked(id)
	if idx < 0 {
		in.mu.Unlock()
		return
	}
	fn := in.notifications[idx].OnClick
	in.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// ClickNotificationAction triggers one action callback without closing.
func (in *NoticeQueueInstance) ClickNotificationAction(id uint64, action int) {
	if in == nil {
		return
	}
	in.mu.Lock()
	idx := in.findNotificationLocked(id)
	if idx < 0 || action < 0 || action >= len(in.notifications[idx].Actions) {
		in.mu.Unlock()
		return
	}
	fn := in.notifications[idx].Actions[action].OnClick
	in.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// ClickNotificationClose closes via the close button when closable.
func (in *NoticeQueueInstance) ClickNotificationClose(id uint64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	idx := in.findNotificationLocked(id)
	if idx < 0 || !in.notifications[idx].Closable {
		in.mu.Unlock()
		return false
	}
	in.mu.Unlock()
	in.closeNotification(id)
	return true
}

// PressEsc closes the latest notification (message has no keyboard).
func (in *NoticeQueueInstance) PressEsc() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if len(in.notifications) == 0 {
		in.mu.Unlock()
		return false
	}
	id := in.notifications[len(in.notifications)-1].ID
	in.mu.Unlock()
	in.closeNotification(id)
	return true
}

// MessageCount reports open tips.
func (in *NoticeQueueInstance) MessageCount() int {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return len(in.messages)
}

// NotificationCount reports open cards, optionally filtered by pool.
func (in *NoticeQueueInstance) NotificationCount(pl ...NoticeQueuePlacement) int {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if len(pl) == 0 {
		return len(in.notifications)
	}
	n := 0
	for _, e := range in.notifications {
		if e.Placement == pl[0] {
			n++
		}
	}
	return n
}

// Messages returns a snapshot in open order.
func (in *NoticeQueueInstance) Messages() []*NoticeQueueMessageEntry {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	out := make([]*NoticeQueueMessageEntry, len(in.messages))
	copy(out, in.messages)
	return out
}

// Notifications returns a snapshot in open order.
func (in *NoticeQueueInstance) Notifications() []*NoticeQueueNotificationEntry {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	out := make([]*NoticeQueueNotificationEntry, len(in.notifications))
	copy(out, in.notifications)
	return out
}

// MessageProgress reports 1-remaining/duration frozen while hovered.
func (in *NoticeQueueInstance) MessageProgress(id uint64) float64 {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	idx := in.findMessageLocked(id)
	if idx < 0 {
		return 0
	}
	e := in.messages[idx]
	if e.Duration <= 0 {
		return 0
	}
	p := 1 - e.Remaining/e.Duration
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	return p
}

// NotificationProgress reports the showProgress fraction for one card.
func (in *NoticeQueueInstance) NotificationProgress(id uint64) float64 {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	idx := in.findNotificationLocked(id)
	if idx < 0 {
		return 0
	}
	e := in.notifications[idx]
	if e.Duration <= 0 {
		return 0
	}
	p := 1 - e.Remaining/e.Duration
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	return p
}

// HolderContent renders static-call holder copy through Ctx.
func (in *NoticeQueueInstance) HolderContent(kind string) string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	ctx := in.ctx
	in.mu.Unlock()
	if kind == "" {
		kind = "notice"
	}
	if ctx.HolderRender == nil {
		return kind + " holder"
	}
	return ctx.HolderRender(kind)
}
