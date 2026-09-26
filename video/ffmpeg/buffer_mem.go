package ffmpeg

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// BufferMem 缓冲内存模块: 引用计数缓冲 + 内存分配 + 队列 + 打印缓存,
// 结构体方法直接可用.
//
// Say it plain: 这里全是小零件——内存块谁分配谁释放, 缓冲区多处
// 引用不断链, 队列先进先出, 打印缓存拼字符串. 上层按需直调.

// Mem holder for raw allocation (无状态, 方法挂在这).
type Mem struct{}

// Buffer owns one AVBufferRef* (nil-safe, 记得 Unref).
type Buffer struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (b *Buffer) Ptr() unsafe.Pointer {
	if b == nil {
		return nil
	}
	return b.ptr
}

// BufferPool owns one AVBufferPool* (nil-safe, 记得 Uninit).
type BufferPool struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (p *BufferPool) Ptr() unsafe.Pointer {
	if p == nil {
		return nil
	}
	return p.ptr
}

// Fifo owns one AVFifo* (nil-safe, 记得 Freep).
type Fifo struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (f *Fifo) Ptr() unsafe.Pointer {
	if f == nil {
		return nil
	}
	return f.ptr
}

// BPrint owns one AVBPrint blob (1024 字节, 见 bprint.h
// FF_PAD_STRUCTURE(AVBPrint, 1024, ...), 记得 Free).
type BPrint struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (b *BPrint) Ptr() unsafe.Pointer {
	if b == nil {
		return nil
	}
	return b.ptr
}

// BPrintSize is the AVBPrint struct size (bprint.h 钉死 1024).
const BPrintSize = 1024

var (
	fBufAlloc      func(uintptr) unsafe.Pointer
	fBufAllocZ     func(uintptr) unsafe.Pointer
	fBufCreate     func(unsafe.Pointer, uintptr, unsafe.Pointer, unsafe.Pointer, int32) unsafe.Pointer
	fBufDefaultFre func(unsafe.Pointer, unsafe.Pointer)
	fBufGetOpaque  func(unsafe.Pointer) unsafe.Pointer
	fBufRefCount   func(unsafe.Pointer) int32
	fBufIsWritable func(unsafe.Pointer) int32
	fBufMakeWritab func(*unsafe.Pointer) int32
	fBufPoolOpaque func(unsafe.Pointer) unsafe.Pointer
	fBufPoolGet    func(unsafe.Pointer) unsafe.Pointer
	fBufPoolInit   func(uintptr, unsafe.Pointer) unsafe.Pointer
	fBufPoolInit2  func(uintptr, unsafe.Pointer, unsafe.Pointer, unsafe.Pointer) unsafe.Pointer
	fBufPoolUninit func(*unsafe.Pointer)
	fBufRealloc    func(*unsafe.Pointer, uintptr) int32
	fBufRef        func(unsafe.Pointer) unsafe.Pointer
	fBufUnref      func(*unsafe.Pointer)
	fBufReplace    func(*unsafe.Pointer, unsafe.Pointer) int32
	fMemMalloc     func(uintptr) unsafe.Pointer
	fMemMallocArr  func(uintptr, uintptr) unsafe.Pointer
	fMemMallocZ    func(uintptr) unsafe.Pointer
	fMemRealloc    func(unsafe.Pointer, uintptr) unsafe.Pointer
	fMemReallocArr func(unsafe.Pointer, uintptr, uintptr) unsafe.Pointer
	fMemReallocF   func(unsafe.Pointer, uintptr, uintptr) unsafe.Pointer
	fMemReallocP   func(unsafe.Pointer, uintptr) int32
	fMemReallocPAr func(unsafe.Pointer, uintptr, uintptr) int32
	fMemFree       func(unsafe.Pointer)
	fMemFreep      func(unsafe.Pointer)
	fMemDup        func(unsafe.Pointer, uintptr) unsafe.Pointer
	fMemCpyBack    func(unsafe.Pointer, int32, int32)
	fFifoAlloc2    func(uintptr, uintptr, uint32) unsafe.Pointer
	fFifoGrowLimit func(unsafe.Pointer, uintptr)
	fFifoCanRead   func(unsafe.Pointer) uintptr
	fFifoCanWrite  func(unsafe.Pointer) uintptr
	fFifoDrain2    func(unsafe.Pointer, uintptr)
	fFifoElemSize  func(unsafe.Pointer) uintptr
	fFifoFreep2    func(*unsafe.Pointer)
	fFifoGrow2     func(unsafe.Pointer, uintptr) int32
	fFifoPeek      func(unsafe.Pointer, unsafe.Pointer, uintptr, uintptr) int32
	fFifoPeekCb    func(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer, *uintptr, uintptr) int32
	fFifoRead      func(unsafe.Pointer, unsafe.Pointer, uintptr) int32
	fFifoReadCb    func(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer, *uintptr) int32
	fFifoReset2    func(unsafe.Pointer)
	fFifoWrite     func(unsafe.Pointer, unsafe.Pointer, uintptr) int32
	fFifoWriteCb   func(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer, *uintptr) int32
	fBpAppendData  func(unsafe.Pointer, string, uint32)
	fBpChars       func(unsafe.Pointer, byte, uint32)
	fBpClear       func(unsafe.Pointer)
	fBpEscape      func(unsafe.Pointer, string, string, int32, int32)
	fBpFinalize    func(unsafe.Pointer, *unsafe.Pointer) int32
	fBpGetBuffer   func(unsafe.Pointer, uint32, *unsafe.Pointer, *uint32)
	fBpInit        func(unsafe.Pointer, uint32, uint32)
	fBpInitBuf     func(unsafe.Pointer, unsafe.Pointer, uint32)
	fBpStrftime    func(unsafe.Pointer, string, unsafe.Pointer)
)

// ensureModBufferMem 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modBufferMemOnce sync.Once

func ensureModBufferMem() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	modBufferMemOnce.Do(func() { registerBufferMem(libHandle) })
	return nil
}

func registerBufferMem(h uintptr) {
	purego.RegisterLibFunc(&fBufAlloc, h, "av_buffer_alloc")
	purego.RegisterLibFunc(&fBufAllocZ, h, "av_buffer_allocz")
	purego.RegisterLibFunc(&fBufCreate, h, "av_buffer_create")
	purego.RegisterLibFunc(&fBufDefaultFre, h, "av_buffer_default_free")
	purego.RegisterLibFunc(&fBufGetOpaque, h, "av_buffer_get_opaque")
	purego.RegisterLibFunc(&fBufRefCount, h, "av_buffer_get_ref_count")
	purego.RegisterLibFunc(&fBufIsWritable, h, "av_buffer_is_writable")
	purego.RegisterLibFunc(&fBufMakeWritab, h, "av_buffer_make_writable")
	purego.RegisterLibFunc(&fBufPoolOpaque, h, "av_buffer_pool_buffer_get_opaque")
	purego.RegisterLibFunc(&fBufPoolGet, h, "av_buffer_pool_get")
	purego.RegisterLibFunc(&fBufPoolInit, h, "av_buffer_pool_init")
	purego.RegisterLibFunc(&fBufPoolInit2, h, "av_buffer_pool_init2")
	purego.RegisterLibFunc(&fBufPoolUninit, h, "av_buffer_pool_uninit")
	purego.RegisterLibFunc(&fBufRealloc, h, "av_buffer_realloc")
	purego.RegisterLibFunc(&fBufRef, h, "av_buffer_ref")
	purego.RegisterLibFunc(&fBufUnref, h, "av_buffer_unref")
	purego.RegisterLibFunc(&fBufReplace, h, "av_buffer_replace")
	purego.RegisterLibFunc(&fMemMalloc, h, "av_malloc")
	purego.RegisterLibFunc(&fMemMallocArr, h, "av_malloc_array")
	purego.RegisterLibFunc(&fMemMallocZ, h, "av_mallocz")
	purego.RegisterLibFunc(&fMemRealloc, h, "av_realloc")
	purego.RegisterLibFunc(&fMemReallocArr, h, "av_realloc_array")
	purego.RegisterLibFunc(&fMemReallocF, h, "av_realloc_f")
	purego.RegisterLibFunc(&fMemReallocP, h, "av_reallocp")
	purego.RegisterLibFunc(&fMemReallocPAr, h, "av_reallocp_array")
	purego.RegisterLibFunc(&fMemFree, h, "av_free")
	purego.RegisterLibFunc(&fMemFreep, h, "av_freep")
	purego.RegisterLibFunc(&fMemDup, h, "av_memdup")
	purego.RegisterLibFunc(&fMemCpyBack, h, "av_memcpy_backptr")
	purego.RegisterLibFunc(&fFifoAlloc2, h, "av_fifo_alloc2")
	purego.RegisterLibFunc(&fFifoGrowLimit, h, "av_fifo_auto_grow_limit")
	purego.RegisterLibFunc(&fFifoCanRead, h, "av_fifo_can_read")
	purego.RegisterLibFunc(&fFifoCanWrite, h, "av_fifo_can_write")
	purego.RegisterLibFunc(&fFifoDrain2, h, "av_fifo_drain2")
	purego.RegisterLibFunc(&fFifoElemSize, h, "av_fifo_elem_size")
	purego.RegisterLibFunc(&fFifoFreep2, h, "av_fifo_freep2")
	purego.RegisterLibFunc(&fFifoGrow2, h, "av_fifo_grow2")
	purego.RegisterLibFunc(&fFifoPeek, h, "av_fifo_peek")
	purego.RegisterLibFunc(&fFifoPeekCb, h, "av_fifo_peek_to_cb")
	purego.RegisterLibFunc(&fFifoRead, h, "av_fifo_read")
	purego.RegisterLibFunc(&fFifoReadCb, h, "av_fifo_read_to_cb")
	purego.RegisterLibFunc(&fFifoReset2, h, "av_fifo_reset2")
	purego.RegisterLibFunc(&fFifoWrite, h, "av_fifo_write")
	purego.RegisterLibFunc(&fFifoWriteCb, h, "av_fifo_write_from_cb")
	purego.RegisterLibFunc(&fBpAppendData, h, "av_bprint_append_data")
	purego.RegisterLibFunc(&fBpChars, h, "av_bprint_chars")
	purego.RegisterLibFunc(&fBpClear, h, "av_bprint_clear")
	purego.RegisterLibFunc(&fBpEscape, h, "av_bprint_escape")
	purego.RegisterLibFunc(&fBpFinalize, h, "av_bprint_finalize")
	purego.RegisterLibFunc(&fBpGetBuffer, h, "av_bprint_get_buffer")
	purego.RegisterLibFunc(&fBpInit, h, "av_bprint_init")
	purego.RegisterLibFunc(&fBpInitBuf, h, "av_bprint_init_for_buffer")
	purego.RegisterLibFunc(&fBpStrftime, h, "av_bprint_strftime")
}

// Alloc allocates size bytes (记得 Free, 对齐 32).
func (m *Mem) Alloc(size int) unsafe.Pointer {
	mustUse(ensureModBufferMem())
	if size <= 0 {
		return nil
	}
	return fMemMalloc(uintptr(size))
}

// AllocZ allocates zeroed size bytes.
func (m *Mem) AllocZ(size int) unsafe.Pointer {
	mustUse(ensureModBufferMem())
	if size <= 0 {
		return nil
	}
	return fMemMallocZ(uintptr(size))
}

// Free releases a Mem.Alloc block (nil-safe).
func (m *Mem) Free(p unsafe.Pointer) {
	mustUse(ensureModBufferMem())
	if p == nil {
		return
	}
	fMemFree(p)
}

// Dup copies size bytes (记得 Free).
func (m *Mem) Dup(p unsafe.Pointer, size int) unsafe.Pointer {
	mustUse(ensureModBufferMem())
	if p == nil || size <= 0 {
		return nil
	}
	return fMemDup(p, uintptr(size))
}

// AllocArray allocates nmemb*size bytes with overflow check
// (av_malloc_array; 0 元素回 nil).
func (m *Mem) AllocArray(nmemb, size int) unsafe.Pointer {
	mustUse(ensureModBufferMem())
	if nmemb <= 0 || size <= 0 {
		return nil
	}
	return fMemMallocArr(uintptr(nmemb), uintptr(size))
}

// Realloc grows/shrinks a Mem.Alloc block (av_realloc; nil 指针当 Alloc 用).
func (m *Mem) Realloc(p unsafe.Pointer, size int) unsafe.Pointer {
	mustUse(ensureModBufferMem())
	if size < 0 {
		return nil
	}
	return fMemRealloc(p, uintptr(size))
}

// ReallocArray grows with overflow check (av_realloc_array).
func (m *Mem) ReallocArray(p unsafe.Pointer, nmemb, size int) unsafe.Pointer {
	mustUse(ensureModBufferMem())
	if nmemb < 0 || size < 0 {
		return nil
	}
	return fMemReallocArr(p, uintptr(nmemb), uintptr(size))
}

// ReallocF frees the old block on failure (av_realloc_f; 比 Realloc 更安全).
func (m *Mem) ReallocF(p unsafe.Pointer, nelem, elsize int) unsafe.Pointer {
	mustUse(ensureModBufferMem())
	if nelem < 0 || elsize < 0 {
		return nil
	}
	return fMemReallocF(p, uintptr(nelem), uintptr(elsize))
}

// ReallocP reallocs through a pointer slot (av_reallocp; 0 表释放).
func (m *Mem) ReallocP(pp unsafe.Pointer, size int) error {
	mustUse(ensureModBufferMem())
	if ret := fMemReallocP(pp, uintptr(size)); ret < 0 {
		return codeErr("av_reallocp", ret)
	}
	return nil
}

// ReallocPArray reallocs an array through a slot with overflow check
// (av_reallocp_array).
func (m *Mem) ReallocPArray(pp unsafe.Pointer, nmemb, size int) error {
	mustUse(ensureModBufferMem())
	if ret := fMemReallocPAr(pp, uintptr(nmemb), uintptr(size)); ret < 0 {
		return codeErr("av_reallocp_array", ret)
	}
	return nil
}

// Freep frees through a slot and nils it (av_freep; 比 Free 多清指针).
func (m *Mem) Freep(pp unsafe.Pointer) {
	mustUse(ensureModBufferMem())
	if pp == nil {
		return
	}
	fMemFreep(pp)
}

// MemcpyBackptr repeats the last back bytes forward (av_memcpy_backptr;
// 解码器内部回拷, 一般用不上).
func (m *Mem) MemcpyBackptr(dst unsafe.Pointer, back, cnt int32) {
	mustUse(ensureModBufferMem())
	if dst == nil {
		return
	}
	fMemCpyBack(dst, back, cnt)
}

// NewBuffer allocates a refcounted size-byte buffer.
func NewBuffer(size int) *Buffer {
	mustUse(ensureModBufferMem())
	if size <= 0 {
		return nil
	}
	ptr := fBufAlloc(uintptr(size))
	if ptr == nil {
		return nil
	}
	return &Buffer{ptr: ptr}
}

// NewBufferZeroed allocates a zeroed refcounted buffer (av_buffer_allocz).
func NewBufferZeroed(size int) *Buffer {
	mustUse(ensureModBufferMem())
	if size <= 0 {
		return nil
	}
	ptr := fBufAllocZ(uintptr(size))
	if ptr == nil {
		return nil
	}
	return &Buffer{ptr: ptr}
}

// WrapBuffer wraps external memory without copying (av_buffer_create;
// free 传 nil 表 ffmpeg 不接管, 传 DefaultFree 走默认释放;
// 调用后别再碰 data, 归引用计数管).
func WrapBuffer(data unsafe.Pointer, size int, free, opaque unsafe.Pointer, flags int32) *Buffer {
	mustUse(ensureModBufferMem())
	if data == nil || size <= 0 {
		return nil
	}
	ptr := fBufCreate(data, uintptr(size), free, opaque, flags)
	if ptr == nil {
		return nil
	}
	return &Buffer{ptr: ptr}
}

// DefaultFree is the stock release for WrapBuffer (av_buffer_default_free).
func DefaultFree(opaque, data unsafe.Pointer) {
	mustUse(ensureModBufferMem())
	fBufDefaultFre(opaque, data)
}

// Opaque returns the opaque set at WrapBuffer time
// (av_buffer_get_opaque).
func (b *Buffer) Opaque() unsafe.Pointer {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return nil
	}
	return fBufGetOpaque(b.ptr)
}

// ReallocBuffer grows/shrinks a refcounted buffer (av_buffer_realloc;
// 独占引用才能改, 共享的先 MakeWritable).
func ReallocBuffer(buf **Buffer, size int) error {
	mustUse(ensureModBufferMem())
	if buf == nil || *buf == nil {
		return errNilBuffer
	}
	var ptr unsafe.Pointer = (*buf).ptr
	if ret := fBufRealloc(&ptr, uintptr(size)); ret < 0 {
		return codeErr("av_buffer_realloc", ret)
	}
	(*buf).ptr = ptr
	return nil
}

// Ref adds one reference (记得 Unref 新引用).
func (b *Buffer) Ref() *Buffer {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return nil
	}
	ptr := fBufRef(b.ptr)
	if ptr == nil {
		return nil
	}
	return &Buffer{ptr: ptr}
}

// Unref drops one reference (归零自动释放).
func (b *Buffer) Unref() {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return
	}
	ptr := b.ptr
	b.ptr = nil
	fBufUnref(&ptr)
}

// Replace swaps dst to src (引用计数, 老引用先丢).
func (b *Buffer) Replace(src *Buffer) error {
	mustUse(ensureModBufferMem())
	if b == nil {
		return errNilBuffer
	}
	var sp unsafe.Pointer
	if src != nil {
		sp = src.ptr
	}
	if ret := fBufReplace(&b.ptr, sp); ret < 0 {
		return codeErr("av_buffer_replace", ret)
	}
	return nil
}

// IsWritable reports exclusive ownership.
func (b *Buffer) IsWritable() bool {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return false
	}
	return fBufIsWritable(b.ptr) > 0
}

// MakeWritable makes the buffer exclusive (写前调).
func (b *Buffer) MakeWritable() error {
	mustUse(ensureModBufferMem())
	if b == nil {
		return errNilBuffer
	}
	if ret := fBufMakeWritab(&b.ptr); ret < 0 {
		return codeErr("av_buffer_make_writable", ret)
	}
	return nil
}

// RefCount reports current references.
func (b *Buffer) RefCount() int {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return 0
	}
	return int(fBufRefCount(b.ptr))
}

// NewBufferPool builds a pool of size-byte buffers (记得 Uninit).
// alloc 传 nil 用默认分配器.
func NewBufferPool(size int, allocFn unsafe.Pointer) *BufferPool {
	mustUse(ensureModBufferMem())
	if size <= 0 {
		return nil
	}
	ptr := fBufPoolInit(uintptr(size), allocFn)
	if ptr == nil {
		return nil
	}
	return &BufferPool{ptr: ptr}
}

// Get borrows one buffer from the pool (记得 Unref).
func (p *BufferPool) Get() *Buffer {
	mustUse(ensureModBufferMem())
	if p == nil || p.ptr == nil {
		return nil
	}
	ptr := fBufPoolGet(p.ptr)
	if ptr == nil {
		return nil
	}
	return &Buffer{ptr: ptr}
}

// NewBufferPoolCustom builds a pool with a custom allocator
// (av_buffer_pool_init2; alloc 传 nil 用默认, poolFree 传 nil 不管;
// opaque 会在每次 Get 的引用上透出, 见 PoolOpaque).
func NewBufferPoolCustom(size int, opaque, alloc, poolFree unsafe.Pointer) *BufferPool {
	mustUse(ensureModBufferMem())
	if size <= 0 {
		return nil
	}
	ptr := fBufPoolInit2(uintptr(size), opaque, alloc, poolFree)
	if ptr == nil {
		return nil
	}
	return &BufferPool{ptr: ptr}
}

// PoolOpaque returns the custom opaque behind a pooled reference
// (av_buffer_pool_buffer_get_opaque).
func (b *Buffer) PoolOpaque() unsafe.Pointer {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return nil
	}
	return fBufPoolOpaque(b.ptr)
}

// Uninit frees the pool and nils the holder.
func (p *BufferPool) Uninit() {
	mustUse(ensureModBufferMem())
	if p == nil || p.ptr == nil {
		return
	}
	ptr := p.ptr
	p.ptr = nil
	fBufPoolUninit(&ptr)
}

// NewFifo builds an element queue (elemSize 字节 x nbElems 个, 记得 Freep).
func NewFifo(nbElems, elemSize int, flags uint32) *Fifo {
	mustUse(ensureModBufferMem())
	if nbElems <= 0 || elemSize <= 0 {
		return nil
	}
	ptr := fFifoAlloc2(uintptr(nbElems), uintptr(elemSize), flags)
	if ptr == nil {
		return nil
	}
	return &Fifo{ptr: ptr}
}

// CanRead reports readable elements.
func (f *Fifo) CanRead() int {
	mustUse(ensureModBufferMem())
	if f == nil || f.ptr == nil {
		return 0
	}
	return int(fFifoCanRead(f.ptr))
}

// CanWrite reports writable slots without growing.
func (f *Fifo) CanWrite() int {
	mustUse(ensureModBufferMem())
	if f == nil || f.ptr == nil {
		return 0
	}
	return int(fFifoCanWrite(f.ptr))
}

// Write appends nbElems elements from buf.
func (f *Fifo) Write(buf unsafe.Pointer, nbElems int) error {
	mustUse(ensureModBufferMem())
	if f == nil {
		return errNilFifo
	}
	if ret := fFifoWrite(f.ptr, buf, uintptr(nbElems)); ret < 0 {
		return codeErr("av_fifo_write", ret)
	}
	return nil
}

// Read pops nbElems elements into buf.
func (f *Fifo) Read(buf unsafe.Pointer, nbElems int) error {
	mustUse(ensureModBufferMem())
	if f == nil {
		return errNilFifo
	}
	if ret := fFifoRead(f.ptr, buf, uintptr(nbElems)); ret < 0 {
		return codeErr("av_fifo_read", ret)
	}
	return nil
}

// SetGrowLimit caps auto-growth at maxElems (av_fifo_auto_grow_limit;
// 0 表不限, 超限的 Write 报错不涨).
func (f *Fifo) SetGrowLimit(maxElems int) {
	mustUse(ensureModBufferMem())
	if f == nil || f.ptr == nil {
		return
	}
	fFifoGrowLimit(f.ptr, uintptr(maxElems))
}

// ElemSize reports the per-element byte size (av_fifo_elem_size).
func (f *Fifo) ElemSize() int {
	mustUse(ensureModBufferMem())
	if f == nil || f.ptr == nil {
		return 0
	}
	return int(fFifoElemSize(f.ptr))
}

// Grow2 reserves room for inc more elements (av_fifo_grow2).
func (f *Fifo) Grow2(inc int) error {
	mustUse(ensureModBufferMem())
	if f == nil {
		return errNilFifo
	}
	if ret := fFifoGrow2(f.ptr, uintptr(inc)); ret < 0 {
		return codeErr("av_fifo_grow2", ret)
	}
	return nil
}

// Peek copies nbElems at offset without popping (av_fifo_peek).
func (f *Fifo) Peek(buf unsafe.Pointer, nbElems, offset int) error {
	mustUse(ensureModBufferMem())
	if f == nil {
		return errNilFifo
	}
	if ret := fFifoPeek(f.ptr, buf, uintptr(nbElems), uintptr(offset)); ret < 0 {
		return codeErr("av_fifo_peek", ret)
	}
	return nil
}

// PeekToCallback peeks through a Go callback (av_fifo_peek_to_cb;
// cb 传 purego.NewCallback 做的指针, 不用传 nil; nbElems 传 nil 表全读).
func (f *Fifo) PeekToCallback(cb, opaque unsafe.Pointer, nbElems *uintptr, offset int) error {
	mustUse(ensureModBufferMem())
	if f == nil {
		return errNilFifo
	}
	if ret := fFifoPeekCb(f.ptr, cb, opaque, nbElems, uintptr(offset)); ret < 0 {
		return codeErr("av_fifo_peek_to_cb", ret)
	}
	return nil
}

// ReadToCallback pops through a Go callback (av_fifo_read_to_cb).
func (f *Fifo) ReadToCallback(cb, opaque unsafe.Pointer, nbElems *uintptr) error {
	mustUse(ensureModBufferMem())
	if f == nil {
		return errNilFifo
	}
	if ret := fFifoReadCb(f.ptr, cb, opaque, nbElems); ret < 0 {
		return codeErr("av_fifo_read_to_cb", ret)
	}
	return nil
}

// WriteFromCallback pushes through a Go callback (av_fifo_write_from_cb).
func (f *Fifo) WriteFromCallback(cb, opaque unsafe.Pointer, nbElems *uintptr) error {
	mustUse(ensureModBufferMem())
	if f == nil {
		return errNilFifo
	}
	if ret := fFifoWriteCb(f.ptr, cb, opaque, nbElems); ret < 0 {
		return codeErr("av_fifo_write_from_cb", ret)
	}
	return nil
}

// Drain drops size elements from the head.
func (f *Fifo) Drain(size int) {
	mustUse(ensureModBufferMem())
	if f == nil || f.ptr == nil {
		return
	}
	fFifoDrain2(f.ptr, uintptr(size))
}

// Reset empties the queue (内存留着复用).
func (f *Fifo) Reset() {
	mustUse(ensureModBufferMem())
	if f == nil || f.ptr == nil {
		return
	}
	fFifoReset2(f.ptr)
}

// Freep frees the queue and nils the holder.
func (f *Fifo) Freep() {
	mustUse(ensureModBufferMem())
	if f == nil || f.ptr == nil {
		return
	}
	ptr := f.ptr
	f.ptr = nil
	fFifoFreep2(&ptr)
}

// NewBPrint builds a print buffer (Init 挂上, 记得 Free).
func NewBPrint(sizeInit, sizeMax uint32) *BPrint {
	mustUse(ensureModBufferMem())
	mem := fMemMalloc(BPrintSize)
	if mem == nil {
		return nil
	}
	fBpInit(mem, sizeInit, sizeMax)
	return &BPrint{ptr: mem}
}

// AppendData appends bytes.
func (b *BPrint) AppendData(s string) {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return
	}
	fBpAppendData(b.ptr, s, uint32(len(s)))
}

// Clear empties without freeing.
func (b *BPrint) Clear() {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return
	}
	fBpClear(b.ptr)
}

// Free finalizes and frees the blob.
func (b *BPrint) Free() {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return
	}
	fMemFree(b.ptr)
	b.ptr = nil
}

// AppendChar appends one byte n times (av_bprint_chars).
func (b *BPrint) AppendChar(c byte, n uint32) {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return
	}
	fBpChars(b.ptr, c, n)
}

// Escape appends src escaping specialChars (av_bprint_escape;
// mode 用 EscapeMode* 常量, flags 传 0).
func (b *BPrint) Escape(src, specialChars string, mode, flags int32) {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return
	}
	fBpEscape(b.ptr, src, specialChars, mode, flags)
}

// Finalize seals the buffer and hands out the C string
// (av_bprint_finalize; 返回的指针用 Mem.Free 放).
func (b *BPrint) Finalize() (unsafe.Pointer, error) {
	if err := ensureModBufferMem(); err != nil {
		var z1 unsafe.Pointer
		return z1, err
	}
	if b == nil || b.ptr == nil {
		return nil, errNilFF
	}
	var out unsafe.Pointer
	if ret := fBpFinalize(b.ptr, &out); ret < 0 {
		return nil, codeErr("av_bprint_finalize", ret)
	}
	return out, nil
}

// GetBuffer reserves size bytes and reports the write pointer
// (av_bprint_get_buffer; actualSize 由包内写).
func (b *BPrint) GetBuffer(size uint32, actualSize *uint32) unsafe.Pointer {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return nil
	}
	var mem unsafe.Pointer
	fBpGetBuffer(b.ptr, size, &mem, actualSize)
	return mem
}

// InitForBuffer reuses external memory as the backing store
// (av_bprint_init_for_buffer; 别 Free 外部内存, 归调用方管).
func (b *BPrint) InitForBuffer(buf unsafe.Pointer, size uint32) {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return
	}
	fBpInitBuf(b.ptr, buf, size)
}

// AppendTime formats tm with fmt (av_bprint_strftime; tm 传 *time.Time
// 的 C 镜像指针, 一般直接传 nil 用当前时间 — 见 av_bprint_strftime).
func (b *BPrint) AppendTime(fmtStr string, tm unsafe.Pointer) {
	mustUse(ensureModBufferMem())
	if b == nil || b.ptr == nil {
		return
	}
	fBpStrftime(b.ptr, fmtStr, tm)
}
