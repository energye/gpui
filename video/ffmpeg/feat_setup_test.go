package ffmpeg

// 功能演示小包装：只给本包单元测试用，不对外公开（小写开头）。
//
// 大白话：底层的 C 函数早就绑好了，但缺几个顺手步骤——
// 给编码器填宽高帧率、搭一条滤镜链。这文件就干这两件事，
// 不改引擎行为，测试跑通即使命完成。

import (
	"fmt"
	"unsafe"
)

// featCStr 把 Go 字符串拷进 av_malloc 的内存，调 C 接口传参用。
// 用完必须调回来的 free，不然漏内存。
func featCStr(s string) (unsafe.Pointer, func()) {
	raw := append([]byte(s), 0)
	var mem Mem
	buf := mem.Alloc(len(raw))
	if buf == nil {
		return nil, func() {}
	}
	copy(unsafe.Slice((*byte)(buf), len(raw)), raw)
	return buf, func() { mem.Free(buf) }
}

// openFeatEncoder 开一个视频编码器：按给的宽高、像素格式、时基、码率配好并打开。
// 大白话：编码器好比印片机，先告诉它纸多大（宽高）、墨什么色（像素格式）、
// 钟怎么走（时基），再开机。参数走 av_opt_set 系列，不碰结构体偏移。
func openFeatEncoder(codecID, w, h, pixFmt int32, tb AVRational, bitrate int64) (*CodecContext, error) {
	if err := ensureLoaded(); err != nil {
		return nil, err
	}
	enc := FindEncoder(codecID)
	if enc == nil {
		return nil, fmt.Errorf("ffmpeg: encoder %d missing (need full lib)", codecID)
	}
	ctx := enc.AllocContext()
	if ctx == nil || ctx.Ptr() == nil {
		return nil, fmt.Errorf("ffmpeg: alloc encoder ctx")
	}
	o := Opt(ctx.Ptr())
	// 注意名字跟直觉不一样（实测探出来的）：尺寸叫 video_size，
	// 码率叫 b，像素格式叫 pixel_format，时基走 SetQ。
	if err := o.Set("video_size", fmt.Sprintf("%dx%d", w, h), 0); err != nil {
		return nil, fmt.Errorf("ffmpeg: video_size: %w", err)
	}
	if err := o.SetInt("b", bitrate, 0); err != nil {
		return nil, fmt.Errorf("ffmpeg: bit_rate: %w", err)
	}
	if err := o.SetQ("time_base", tb, 0); err != nil {
		return nil, fmt.Errorf("ffmpeg: time_base: %w", err)
	}
	if err := o.SetInt("pixel_format", int64(pixFmt), 0); err != nil {
		return nil, fmt.Errorf("ffmpeg: pixel_format: %w", err)
	}
	if err := ctx.Open(enc, nil); err != nil {
		return nil, fmt.Errorf("ffmpeg: open encoder: %w", err)
	}
	return ctx, nil
}

// openFeatChain 手工搭一条线性滤镜链：buffer 输入 + 中间若干滤镜 + buffersink 输出。
// 大白话：描述串解析（parse2）那个绑定在本机构造的库上会崩，
// 这里绕开它，一个滤镜一个滤镜亲手创建再连起来，效果一样。
// bufArgs 是 buffer 头参数（宽高、像素格式、时基）；mids 是中间滤镜名和参数；
// 返回图和两端，后面推帧拉帧就靠它们。
func openFeatChain(bufArgs string, mids [][2]string) (graph, src, sink unsafe.Pointer, freeAll func(), err error) {
	if err := ensureLoaded(); err != nil {
		return nil, nil, nil, nil, err
	}
	freeAll = func() {}
	mkStr := func(s string) (unsafe.Pointer, func()) { return featCStr(s) }
	graph = fAvfilterGraphAlloc()
	if graph == nil {
		return nil, nil, nil, nil, fmt.Errorf("ffmpeg: graph alloc")
	}
	// 参数串在整图配好（config）之前不能释放：滤镜初始化可能拖到 config 才读参数。
	var argFrees []func()
	defer func() {
		for _, f := range argFrees {
			f()
		}
	}()
	freeAll = func() { fAvfilterGraphFree(&graph) }
	fail := func(format string, args ...any) (unsafe.Pointer, unsafe.Pointer, unsafe.Pointer, func(), error) {
		freeAll()
		return nil, nil, nil, nil, fmt.Errorf(format, args...)
	}
	newFilter := func(filter, name, args string) unsafe.Pointer {
		fn, ff := mkStr(filter)
		defer ff()
		filt := fAvfilterGetByName(fn)
		if filt == nil {
			return nil
		}
		nn, nfree := mkStr(name)
		defer nfree()
		var argPtr unsafe.Pointer
		if args != "" {
			var af func()
			argPtr, af = mkStr(args)
			argFrees = append(argFrees, af)
		}
		var ctx unsafe.Pointer
		if ret := fAvfilterGraphCreateFilter(&ctx, filt, nn, argPtr, nil, graph); ret < 0 {
			return nil
		}
		return ctx
	}
	src = newFilter("buffer", "src", bufArgs)
	if src == nil {
		return fail("ffmpeg: create buffer (%s)", bufArgs)
	}
	prev := src
	for i, m := range mids {
		ctx := newFilter(m[0], fmt.Sprintf("mid%d", i), m[1])
		if ctx == nil {
			return fail("ffmpeg: create %s (%s)", m[0], m[1])
		}
		if ret := fAvfilterLink(prev, 0, ctx, 0); ret < 0 {
			return fail("ffmpeg: link %s: %s", m[0], errText(ret))
		}
		prev = ctx
	}
	sink = newFilter("buffersink", "sink", "")
	if sink == nil {
		return fail("ffmpeg: create buffersink")
	}
	if ret := fAvfilterLink(prev, 0, sink, 0); ret < 0 {
		return fail("ffmpeg: link sink: %s", errText(ret))
	}
	if ret := fAvfilterGraphConfig(graph, nil); ret < 0 {
		return fail("ffmpeg: graph config: %s", errText(ret))
	}
	return graph, src, sink, freeAll, nil
}
