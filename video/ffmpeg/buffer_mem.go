package ffmpeg

import (
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
	if size <= 0 {
		return nil
	}
	return fMemMalloc(uintptr(size))
}

// AllocZ allocates zeroed size bytes.
func (m *Mem) AllocZ(size int) unsafe.Pointer {
	if size <= 0 {
		return nil
	}
	return fMemMallocZ(uintptr(size))
}

// Free releases a Mem.Alloc block (nil-safe).
func (m *Mem) Free(p unsafe.Pointer) {
	if p == nil {
		return
	}
	fMemFree(p)
}

// Dup copies size bytes (记得 Free).
func (m *Mem) Dup(p unsafe.Pointer, size int) unsafe.Pointer {
	if p == nil || size <= 0 {
		return nil
	}
	return fMemDup(p, uintptr(size))
}

// NewBuffer allocates a refcounted size-byte buffer.
func NewBuffer(size int) *Buffer {
	if size <= 0 {
		return nil
	}
	ptr := fBufAlloc(uintptr(size))
	if ptr == nil {
		return nil
	}
	return &Buffer{ptr: ptr}
}

// Ref adds one reference (记得 Unref 新引用).
func (b *Buffer) Ref() *Buffer {
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
	if b == nil || b.ptr == nil {
		return
	}
	ptr := b.ptr
	b.ptr = nil
	fBufUnref(&ptr)
}

// Replace swaps dst to src (引用计数, 老引用先丢).
func (b *Buffer) Replace(src *Buffer) error {
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
	if b == nil || b.ptr == nil {
		return false
	}
	return fBufIsWritable(b.ptr) > 0
}

// MakeWritable makes the buffer exclusive (写前调).
func (b *Buffer) MakeWritable() error {
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
	if b == nil || b.ptr == nil {
		return 0
	}
	return int(fBufRefCount(b.ptr))
}

// NewBufferPool builds a pool of size-byte buffers (记得 Uninit).
// alloc 传 nil 用默认分配器.
func NewBufferPool(size int, allocFn unsafe.Pointer) *BufferPool {
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
	if p == nil || p.ptr == nil {
		return nil
	}
	ptr := fBufPoolGet(p.ptr)
	if ptr == nil {
		return nil
	}
	return &Buffer{ptr: ptr}
}

// Uninit frees the pool and nils the holder.
func (p *BufferPool) Uninit() {
	if p == nil || p.ptr == nil {
		return
	}
	ptr := p.ptr
	p.ptr = nil
	fBufPoolUninit(&ptr)
}

// NewFifo builds an element queue (elemSize 字节 x nbElems 个, 记得 Freep).
func NewFifo(nbElems, elemSize int, flags uint32) *Fifo {
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
	if f == nil || f.ptr == nil {
		return 0
	}
	return int(fFifoCanRead(f.ptr))
}

// CanWrite reports writable slots without growing.
func (f *Fifo) CanWrite() int {
	if f == nil || f.ptr == nil {
		return 0
	}
	return int(fFifoCanWrite(f.ptr))
}

// Write appends nbElems elements from buf.
func (f *Fifo) Write(buf unsafe.Pointer, nbElems int) error {
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
	if f == nil {
		return errNilFifo
	}
	if ret := fFifoRead(f.ptr, buf, uintptr(nbElems)); ret < 0 {
		return codeErr("av_fifo_read", ret)
	}
	return nil
}

// Drain drops size elements from the head.
func (f *Fifo) Drain(size int) {
	if f == nil || f.ptr == nil {
		return
	}
	fFifoDrain2(f.ptr, uintptr(size))
}

// Reset empties the queue (内存留着复用).
func (f *Fifo) Reset() {
	if f == nil || f.ptr == nil {
		return
	}
	fFifoReset2(f.ptr)
}

// Freep frees the queue and nils the holder.
func (f *Fifo) Freep() {
	if f == nil || f.ptr == nil {
		return
	}
	ptr := f.ptr
	f.ptr = nil
	fFifoFreep2(&ptr)
}

// NewBPrint builds a print buffer (Init 挂上, 记得 Free).
func NewBPrint(sizeInit, sizeMax uint32) *BPrint {
	mem := fMemMalloc(BPrintSize)
	if mem == nil {
		return nil
	}
	fBpInit(mem, sizeInit, sizeMax)
	return &BPrint{ptr: mem}
}

// AppendData appends bytes.
func (b *BPrint) AppendData(s string) {
	if b == nil || b.ptr == nil {
		return
	}
	fBpAppendData(b.ptr, s, uint32(len(s)))
}

// Clear empties without freeing.
func (b *BPrint) Clear() {
	if b == nil || b.ptr == nil {
		return
	}
	fBpClear(b.ptr)
}

// Free finalizes and frees the blob.
func (b *BPrint) Free() {
	if b == nil || b.ptr == nil {
		return
	}
	fMemFree(b.ptr)
	b.ptr = nil
}
