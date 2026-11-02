//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import (
	"sync"
	"time"

	"github.com/energye/gpui/ui/scheduler"
)

// PipelineOwner flushes layout and paint for a root RenderObject (Flutter subset).
type PipelineOwner struct {
	mu sync.Mutex

	root RenderObject

	// Counters for tests / metrics (reset with ResetCounts).
	LayoutCount int64
	PaintCount  int64

	layoutDirty bool
	paintDirty  bool

	// boundaryCache is the process-lifetime Picture cache for RepaintBoundary
	// nodes on this tree. Shared across presents so skip is observable.
	boundaryCache *BoundaryCache

	// tickers 是本棵树的「按 dt 推进显示态」参与者注册表。契约与实现都直接用
	// ui/scheduler 那一份（不再自建同形状接口）：参与者的 Tick(dt) 返回 false
	// 即由注册表自动摘除。光标闪烁、动图逐帧都注册到这里，嵌道层的泵把它接进
	// 所属窗口的帧调度器；表空时这条链路一分钱不花。
	tickers *scheduler.TickerRegistry
	// tickerCtl 由嵌道层装入：注册表在空与非空之间翻转时通知它，按需挂/卸泵
	// ticker（没有参与者就不占帧调度器的一位）。
	tickerCtl func(active bool)
}

// SetTickerCtl 安装嵌道层的泵挂卸回调。传 nil 表示不挂（单测自建 owner 的形态）。
func (o *PipelineOwner) SetTickerCtl(fn func(active bool)) {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.tickerCtl = fn
	o.mu.Unlock()
}

// tickerRegistryLocked 惰性建表：调用方须持有 o.mu（零值 owner 也要可用）。
func (o *PipelineOwner) tickerRegistryLocked() *scheduler.TickerRegistry {
	if o.tickers == nil {
		o.tickers = &scheduler.TickerRegistry{}
	}
	return o.tickers
}

// AddTicker 把参与者注册进本棵树（幂等）。表由空转非空时通知 tickerCtl 把泵挂上。
func (o *PipelineOwner) AddTicker(t scheduler.Ticker) {
	if o == nil || t == nil {
		return
	}
	var ctl func(active bool)
	var notify bool
	o.mu.Lock()
	reg := o.tickerRegistryLocked()
	wasEmpty := !reg.HasActive()
	reg.Add(t)
	notify = wasEmpty && reg.HasActive()
	ctl = o.tickerCtl
	o.mu.Unlock()
	if notify && ctl != nil {
		ctl(true)
	}
}

// RemoveTicker 注销参与者（幂等）。表转为空时通知 tickerCtl 把泵摘掉。
// 参与者也可以在自己的 Tick 里返回 false 让注册表摘掉自己，两条路等价。
func (o *PipelineOwner) RemoveTicker(t scheduler.Ticker) {
	if o == nil || t == nil {
		return
	}
	var ctl func(active bool)
	var notify bool
	o.mu.Lock()
	if o.tickers != nil {
		wasActive := o.tickers.HasActive()
		o.tickers.Remove(t)
		notify = wasActive && !o.tickers.HasActive()
	}
	ctl = o.tickerCtl
	o.mu.Unlock()
	if notify && ctl != nil {
		ctl(false)
	}
}

// HasTickers 报告本棵树是否还有参与者需要按 dt 推进。
func (o *PipelineOwner) HasTickers() bool {
	if o == nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.tickers != nil && o.tickers.HasActive()
}

// NextTickerWake 当场问一遍在册参与者，返回最早的下一次可见变化截止点。
// false 表示没人报出截止点（没注册，或处于稳定态），事件循环转而等事件。
func (o *PipelineOwner) NextTickerWake() (time.Duration, bool) {
	if o == nil {
		return 0, false
	}
	o.mu.Lock()
	reg := o.tickers
	o.mu.Unlock()
	if reg == nil {
		return 0, false
	}
	return reg.PollNextWake()
}

// TickAllTickers 在 UI 线程推进全部在册参与者；返回 false 的由注册表自动摘除。
// 参与者的回调在持有 owner 锁之外执行（回调里标脏会再进 owner，持锁回调必死锁）。
func (o *PipelineOwner) TickAllTickers(dt float64) {
	if o == nil {
		return
	}
	o.mu.Lock()
	reg := o.tickers
	o.mu.Unlock()
	if reg == nil {
		return
	}
	reg.TickAll(dt)
}

// NewPipelineOwner creates an owner with optional root.
func NewPipelineOwner(root RenderObject) *PipelineOwner {
	o := &PipelineOwner{
		root:          root,
		boundaryCache: NewBoundaryCache(),
	}
	if root != nil {
		attachOwner(root, o)
	}
	return o
}

// BoundaryCache returns the tree's Picture-backed RepaintBoundary cache (never nil
// for a non-nil owner). Survives across frames so clean boundaries can skip.
func (o *PipelineOwner) BoundaryCache() *BoundaryCache {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.boundaryCache == nil {
		o.boundaryCache = NewBoundaryCache()
	}
	return o.boundaryCache
}

// SetRoot replaces the root and attaches owner.
func (o *PipelineOwner) SetRoot(root RenderObject) {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.root = root
	o.layoutDirty = true
	o.paintDirty = true
	o.mu.Unlock()
	if root != nil {
		attachOwner(root, o)
	}
}

// Root returns the current root.
func (o *PipelineOwner) Root() RenderObject {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.root
}

func (o *PipelineOwner) noteLayoutDirty() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.layoutDirty = true
	o.mu.Unlock()
}

func (o *PipelineOwner) notePaintDirty() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.paintDirty = true
	o.mu.Unlock()
}

// ResetCounts zeroes layout/paint counters.
func (o *PipelineOwner) ResetCounts() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.LayoutCount = 0
	o.PaintCount = 0
	o.mu.Unlock()
}

// FlushLayout lays out the root under tight viewport constraints when dirty
// or when force is true. Returns whether layout work ran.
// A second flush with no dirty flags is a no-op (layout_count unchanged).
func (o *PipelineOwner) FlushLayout(viewport Size, force bool) bool {
	if o == nil {
		return false
	}
	o.mu.Lock()
	root := o.root
	dirty := o.layoutDirty || force
	o.mu.Unlock()
	if root == nil {
		return false
	}
	if !force && !dirty && !root.NeedsLayout() {
		return false
	}
	_ = root.Layout(Tight(viewport.Width, viewport.Height))
	o.mu.Lock()
	o.LayoutCount++
	o.layoutDirty = false
	o.mu.Unlock()
	return true
}

// FlushPaint paints the root when paint-dirty or force. pc must be non-nil.
// When force is false and the tree is only partially dirty, CompositeOnly is set
// so clean subtrees are skipped.
// Returns whether paint ran.
func (o *PipelineOwner) FlushPaint(pc *PaintContext, force bool) bool {
	if o == nil || pc == nil {
		return false
	}
	o.mu.Lock()
	root := o.root
	dirty := o.paintDirty || force
	o.mu.Unlock()
	if root == nil {
		return false
	}
	if !dirty && !root.NeedsPaint() && !SubtreeNeedsPaint(root) && !force {
		return false
	}
	if !force {
		pc.CompositeOnly = true
	} else {
		pc.CompositeOnly = false
	}
	root.Paint(pc)
	o.mu.Lock()
	o.PaintCount++
	o.paintDirty = false
	o.mu.Unlock()
	return true
}

// UpdateCompositingBits runs F04 propagation from the root.
func (o *PipelineOwner) UpdateCompositingBits() {
	if o == nil || o.root == nil {
		return
	}
	if b, ok := baseOf(o.root); ok {
		b.UpdateCompositingBits()
	}
}

// ConsumeNeedsPaint clears all paint-dirty flags without drawing. Used by the
// retained textured-composite path: content is captured into the layer
// tree (BuildLayerTree records leaf display lists) and rasterized to cached
// textures, so no live FlushPaint runs and the needs-paint marks must not keep
// scheduling frames forever.
func (o *PipelineOwner) ConsumeNeedsPaint() {
	if o == nil || o.root == nil {
		return
	}
	o.mu.Lock()
	o.paintDirty = false
	o.mu.Unlock()
	var walk func(n RenderObject)
	walk = func(n RenderObject) {
		if n == nil {
			return
		}
		if b, ok := baseOf(n); ok {
			b.clearPaintDirty()
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(o.root)
}

// NeedsFrame reports layout or paint dirty.
func (o *PipelineOwner) NeedsFrame() bool {
	if o == nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.layoutDirty || o.paintDirty {
		return true
	}
	if o.root != nil && (o.root.NeedsLayout() || o.root.NeedsPaint()) {
		return true
	}
	return false
}

func attachOwner(n RenderObject, o *PipelineOwner) {
	if n == nil {
		return
	}
	if b, ok := baseOf(n); ok {
		b.SetOwner(o)
	}
	for _, c := range n.Children() {
		attachOwner(c, o)
	}
}
