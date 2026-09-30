//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package ffmpeg

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// FilterGraph 滤镜模块: FilterGraph/FilterContext/Filter/FilterSink/FilterSource 全量导出, 结构体方法直接可用.
//
// Say it plain: 把解出来的帧丢进滤镜图里加工 (缩放裁剪调色混音), 图配好推帧进去拉帧出来.

// FilterGraph holder.
type FilterGraph struct{ ptr unsafe.Pointer }

func (x *FilterGraph) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

// FilterContext holder.
type FilterContext struct{ ptr unsafe.Pointer }

func (x *FilterContext) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

// Filter holder.
type Filter struct{ ptr unsafe.Pointer }

// FilterSink owns one sink AVFilterContext* (buffersink 端, 拉帧出来).
type FilterSink struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (x *FilterSink) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

// FilterSource owns one source AVFilterContext* (buffersrc 端, 推帧进去).
type FilterSource struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (x *FilterSource) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}
func (x *Filter) Ptr() unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return nil
	}
	return x.ptr
}

var (
	fAvBuffersinkGetChannels           func(ctx unsafe.Pointer) int32
	fAvBuffersinkGetChLayout           func(ctx unsafe.Pointer, ch_layout unsafe.Pointer) int32
	fAvBuffersinkGetColorRange         func(ctx unsafe.Pointer) int32
	fAvBuffersinkGetColorspace         func(ctx unsafe.Pointer) int32
	fAvBuffersinkGetFormat             func(ctx unsafe.Pointer) int32
	fAvBuffersinkGetFrame              func(ctx unsafe.Pointer, frame unsafe.Pointer) int32
	fAvBuffersinkGetFrameFlags         func(ctx unsafe.Pointer, frame unsafe.Pointer, flags int32) int32
	fAvBuffersinkGetFrameRate          func(ctx unsafe.Pointer) AVRational
	fAvBuffersinkGetH                  func(ctx unsafe.Pointer) int32
	fAvBuffersinkGetHwFramesCtx        func(ctx unsafe.Pointer) unsafe.Pointer
	fAvBuffersinkGetSampleAspectRatio  func(ctx unsafe.Pointer) AVRational
	fAvBuffersinkGetSampleRate         func(ctx unsafe.Pointer) int32
	fAvBuffersinkGetSamples            func(ctx unsafe.Pointer, frame unsafe.Pointer, nb_samples int32) int32
	fAvBuffersinkGetTimeBase           func(ctx unsafe.Pointer) AVRational
	fAvBuffersinkGetType               func(ctx unsafe.Pointer) int32
	fAvBuffersinkGetW                  func(ctx unsafe.Pointer) int32
	fAvBuffersinkSetFrameSize          func(ctx unsafe.Pointer, frame_size uint32)
	fAvBuffersrcAddFrame               func(ctx unsafe.Pointer, frame unsafe.Pointer) int32
	fAvBuffersrcAddFrameFlags          func(buffer_src unsafe.Pointer, frame unsafe.Pointer, flags int32) int32
	fAvBuffersrcClose                  func(ctx unsafe.Pointer, pts int64, flags uint32) int32
	fAvBuffersrcGetNbFailedRequests    func(buffer_src unsafe.Pointer) uint32
	fAvBuffersrcGetStatus              func(ctx unsafe.Pointer) int32
	fAvBuffersrcParametersAlloc        func() unsafe.Pointer
	fAvBuffersrcParametersSet          func(ctx unsafe.Pointer, param unsafe.Pointer) int32
	fAvBuffersrcWriteFrame             func(ctx unsafe.Pointer, frame unsafe.Pointer) int32
	fAvfilterConfigLinks               func(filter unsafe.Pointer) int32
	fAvfilterConfiguration             func() unsafe.Pointer
	fAvfilterFilterPadCount            func(filter unsafe.Pointer, is_output int32) uint32
	fAvfilterFree                      func(filter unsafe.Pointer)
	fAvfilterGetByName                 func(name unsafe.Pointer) unsafe.Pointer
	fAvfilterGetClass                  func() unsafe.Pointer
	fAvfilterGraphAlloc                func() unsafe.Pointer
	fAvfilterGraphAllocFilter          func(graph unsafe.Pointer, filter unsafe.Pointer, name unsafe.Pointer) unsafe.Pointer
	fAvfilterGraphConfig               func(graphctx unsafe.Pointer, log_ctx unsafe.Pointer) int32
	fAvfilterGraphCreateFilter         func(filt_ctx *unsafe.Pointer, filt unsafe.Pointer, name unsafe.Pointer, args unsafe.Pointer, opaque unsafe.Pointer, graph_ctx unsafe.Pointer) int32
	fAvfilterGraphDump                 func(graph unsafe.Pointer, options unsafe.Pointer) unsafe.Pointer
	fAvfilterGraphFree                 func(graph *unsafe.Pointer)
	fAvfilterGraphGetFilter            func(graph unsafe.Pointer, name unsafe.Pointer) unsafe.Pointer
	fAvfilterGraphParse                func(graph unsafe.Pointer, filters unsafe.Pointer, inputs unsafe.Pointer, outputs unsafe.Pointer, log_ctx unsafe.Pointer) int32
	fAvfilterGraphParse2               func(graph unsafe.Pointer, filters unsafe.Pointer, inputs *unsafe.Pointer, outputs *unsafe.Pointer) int32
	fAvfilterGraphParsePtr             func(graph unsafe.Pointer, filters unsafe.Pointer, inputs *unsafe.Pointer, outputs *unsafe.Pointer, log_ctx unsafe.Pointer) int32
	fAvfilterGraphQueueCommand         func(graph unsafe.Pointer, target unsafe.Pointer, cmd unsafe.Pointer, arg unsafe.Pointer, flags int32, ts float64) int32
	fAvfilterGraphRequestOldest        func(graph unsafe.Pointer) int32
	fAvfilterGraphSegmentApply         func(seg unsafe.Pointer, flags int32, inputs *unsafe.Pointer, outputs *unsafe.Pointer) int32
	fAvfilterGraphSegmentApplyOpts     func(seg unsafe.Pointer, flags int32) int32
	fAvfilterGraphSegmentCreateFilters func(seg unsafe.Pointer, flags int32) int32
	fAvfilterGraphSegmentFree          func(seg *unsafe.Pointer)
	fAvfilterGraphSegmentInit          func(seg unsafe.Pointer, flags int32) int32
	fAvfilterGraphSegmentLink          func(seg unsafe.Pointer, flags int32, inputs *unsafe.Pointer, outputs *unsafe.Pointer) int32
	fAvfilterGraphSegmentParse         func(graph unsafe.Pointer, graph_str unsafe.Pointer, flags int32, seg *unsafe.Pointer) int32
	fAvfilterGraphSendCommand          func(graph unsafe.Pointer, target unsafe.Pointer, cmd unsafe.Pointer, arg unsafe.Pointer, res unsafe.Pointer, res_len int32, flags int32) int32
	fAvfilterGraphSetAutoConvert       func(graph unsafe.Pointer, flags uint32)
	fAvfilterInitDict                  func(ctx unsafe.Pointer, options *unsafe.Pointer) int32
	fAvfilterInitStr                   func(ctx unsafe.Pointer, args unsafe.Pointer) int32
	fAvfilterInoutAlloc                func() unsafe.Pointer
	fAvfilterInoutFree                 func(inout *unsafe.Pointer)
	fAvfilterInsertFilter              func(link unsafe.Pointer, filt unsafe.Pointer, filt_srcpad_idx uint32, filt_dstpad_idx uint32) int32
	fAvfilterLicense                   func() unsafe.Pointer
	fAvfilterLink                      func(src unsafe.Pointer, srcpad uint32, dst unsafe.Pointer, dstpad uint32) int32
	fAvfilterLinkFree                  func(link *unsafe.Pointer)
	fAvfilterPadGetName                func(pads unsafe.Pointer, pad_idx int32) unsafe.Pointer
	fAvfilterPadGetType                func(pads unsafe.Pointer, pad_idx int32) int32
	fAvfilterProcessCommand            func(filter unsafe.Pointer, cmd unsafe.Pointer, arg unsafe.Pointer, res unsafe.Pointer, res_len int32, flags int32) int32
	fAvfilterVersion                   func() uint32
)

// ensureModFilterGraph 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modFilterGraphOnce sync.Once

func ensureModFilterGraph() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	modFilterGraphOnce.Do(func() { registerFilterGraph(libHandle) })
	return nil
}

func registerFilterGraph(h uintptr) {
	purego.RegisterLibFunc(&fAvBuffersinkGetChannels, h, "av_buffersink_get_channels")
	purego.RegisterLibFunc(&fAvBuffersinkGetChLayout, h, "av_buffersink_get_ch_layout")
	purego.RegisterLibFunc(&fAvBuffersinkGetColorRange, h, "av_buffersink_get_color_range")
	purego.RegisterLibFunc(&fAvBuffersinkGetColorspace, h, "av_buffersink_get_colorspace")
	purego.RegisterLibFunc(&fAvBuffersinkGetFormat, h, "av_buffersink_get_format")
	purego.RegisterLibFunc(&fAvBuffersinkGetFrame, h, "av_buffersink_get_frame")
	purego.RegisterLibFunc(&fAvBuffersinkGetFrameFlags, h, "av_buffersink_get_frame_flags")
	purego.RegisterLibFunc(&fAvBuffersinkGetFrameRate, h, "av_buffersink_get_frame_rate")
	purego.RegisterLibFunc(&fAvBuffersinkGetH, h, "av_buffersink_get_h")
	purego.RegisterLibFunc(&fAvBuffersinkGetHwFramesCtx, h, "av_buffersink_get_hw_frames_ctx")
	purego.RegisterLibFunc(&fAvBuffersinkGetSampleAspectRatio, h, "av_buffersink_get_sample_aspect_ratio")
	purego.RegisterLibFunc(&fAvBuffersinkGetSampleRate, h, "av_buffersink_get_sample_rate")
	purego.RegisterLibFunc(&fAvBuffersinkGetSamples, h, "av_buffersink_get_samples")
	purego.RegisterLibFunc(&fAvBuffersinkGetTimeBase, h, "av_buffersink_get_time_base")
	purego.RegisterLibFunc(&fAvBuffersinkGetType, h, "av_buffersink_get_type")
	purego.RegisterLibFunc(&fAvBuffersinkGetW, h, "av_buffersink_get_w")
	purego.RegisterLibFunc(&fAvBuffersinkSetFrameSize, h, "av_buffersink_set_frame_size")
	purego.RegisterLibFunc(&fAvBuffersrcAddFrame, h, "av_buffersrc_add_frame")
	purego.RegisterLibFunc(&fAvBuffersrcAddFrameFlags, h, "av_buffersrc_add_frame_flags")
	purego.RegisterLibFunc(&fAvBuffersrcClose, h, "av_buffersrc_close")
	purego.RegisterLibFunc(&fAvBuffersrcGetNbFailedRequests, h, "av_buffersrc_get_nb_failed_requests")
	purego.RegisterLibFunc(&fAvBuffersrcGetStatus, h, "av_buffersrc_get_status")
	purego.RegisterLibFunc(&fAvBuffersrcParametersAlloc, h, "av_buffersrc_parameters_alloc")
	purego.RegisterLibFunc(&fAvBuffersrcParametersSet, h, "av_buffersrc_parameters_set")
	purego.RegisterLibFunc(&fAvBuffersrcWriteFrame, h, "av_buffersrc_write_frame")
	purego.RegisterLibFunc(&fAvfilterConfigLinks, h, "avfilter_config_links")
	purego.RegisterLibFunc(&fAvfilterConfiguration, h, "avfilter_configuration")
	purego.RegisterLibFunc(&fAvfilterFilterPadCount, h, "avfilter_filter_pad_count")
	purego.RegisterLibFunc(&fAvfilterFree, h, "avfilter_free")
	purego.RegisterLibFunc(&fAvfilterGetByName, h, "avfilter_get_by_name")
	purego.RegisterLibFunc(&fAvfilterGetClass, h, "avfilter_get_class")
	purego.RegisterLibFunc(&fAvfilterGraphAlloc, h, "avfilter_graph_alloc")
	purego.RegisterLibFunc(&fAvfilterGraphAllocFilter, h, "avfilter_graph_alloc_filter")
	purego.RegisterLibFunc(&fAvfilterGraphConfig, h, "avfilter_graph_config")
	purego.RegisterLibFunc(&fAvfilterGraphCreateFilter, h, "avfilter_graph_create_filter")
	purego.RegisterLibFunc(&fAvfilterGraphDump, h, "avfilter_graph_dump")
	purego.RegisterLibFunc(&fAvfilterGraphFree, h, "avfilter_graph_free")
	purego.RegisterLibFunc(&fAvfilterGraphGetFilter, h, "avfilter_graph_get_filter")
	purego.RegisterLibFunc(&fAvfilterGraphParse, h, "avfilter_graph_parse")
	purego.RegisterLibFunc(&fAvfilterGraphParse2, h, "avfilter_graph_parse2")
	purego.RegisterLibFunc(&fAvfilterGraphParsePtr, h, "avfilter_graph_parse_ptr")
	purego.RegisterLibFunc(&fAvfilterGraphQueueCommand, h, "avfilter_graph_queue_command")
	purego.RegisterLibFunc(&fAvfilterGraphRequestOldest, h, "avfilter_graph_request_oldest")
	purego.RegisterLibFunc(&fAvfilterGraphSegmentApply, h, "avfilter_graph_segment_apply")
	purego.RegisterLibFunc(&fAvfilterGraphSegmentApplyOpts, h, "avfilter_graph_segment_apply_opts")
	purego.RegisterLibFunc(&fAvfilterGraphSegmentCreateFilters, h, "avfilter_graph_segment_create_filters")
	purego.RegisterLibFunc(&fAvfilterGraphSegmentFree, h, "avfilter_graph_segment_free")
	purego.RegisterLibFunc(&fAvfilterGraphSegmentInit, h, "avfilter_graph_segment_init")
	purego.RegisterLibFunc(&fAvfilterGraphSegmentLink, h, "avfilter_graph_segment_link")
	purego.RegisterLibFunc(&fAvfilterGraphSegmentParse, h, "avfilter_graph_segment_parse")
	purego.RegisterLibFunc(&fAvfilterGraphSendCommand, h, "avfilter_graph_send_command")
	purego.RegisterLibFunc(&fAvfilterGraphSetAutoConvert, h, "avfilter_graph_set_auto_convert")
	purego.RegisterLibFunc(&fAvfilterInitDict, h, "avfilter_init_dict")
	purego.RegisterLibFunc(&fAvfilterInitStr, h, "avfilter_init_str")
	purego.RegisterLibFunc(&fAvfilterInoutAlloc, h, "avfilter_inout_alloc")
	purego.RegisterLibFunc(&fAvfilterInoutFree, h, "avfilter_inout_free")
	purego.RegisterLibFunc(&fAvfilterInsertFilter, h, "avfilter_insert_filter")
	purego.RegisterLibFunc(&fAvfilterLicense, h, "avfilter_license")
	purego.RegisterLibFunc(&fAvfilterLink, h, "avfilter_link")
	purego.RegisterLibFunc(&fAvfilterLinkFree, h, "avfilter_link_free")
	purego.RegisterLibFunc(&fAvfilterPadGetName, h, "avfilter_pad_get_name")
	purego.RegisterLibFunc(&fAvfilterPadGetType, h, "avfilter_pad_get_type")
	purego.RegisterLibFunc(&fAvfilterProcessCommand, h, "avfilter_process_command")
	purego.RegisterLibFunc(&fAvfilterVersion, h, "avfilter_version")
}

// GetChannels 从滤镜出口问参数或取帧（对 av_buffersink_get_channels；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterContext) GetChannels() int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersinkGetChannels(x.ptr)
}

// GetChLayout 从滤镜出口问参数或取帧（对 av_buffersink_get_ch_layout；参数 ch_layout；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetChLayout(ch_layout unsafe.Pointer) int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersinkGetChLayout(x.ptr, ch_layout)
}

// GetColorRange 从滤镜出口问参数或取帧（对 av_buffersink_get_color_range；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetColorRange() int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return -1
	}
	return fAvBuffersinkGetColorRange(x.ptr)
}

// GetColorspace 从滤镜出口问参数或取帧（对 av_buffersink_get_colorspace；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetColorspace() int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return -1
	}
	return fAvBuffersinkGetColorspace(x.ptr)
}

// GetFormat 从滤镜出口问参数或取帧（对 av_buffersink_get_format；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetFormat() int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersinkGetFormat(x.ptr)
}

// GetFrame 从滤镜出口问参数或取帧（对 av_buffersink_get_frame；参数 frame；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetFrame(frame unsafe.Pointer) int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersinkGetFrame(x.ptr, frame)
}

// GetFrameFlags 从滤镜出口取帧（对 av_buffersink_get_frame_flags；参数 frame(须是 NewFrame 建的真帧, 不可传 nil)、flags；
// 回 0=成功, 负数是 AVERROR；无状态调用前图要先配好）.
func (x *FilterSink) GetFrameFlags(frame unsafe.Pointer, flags int32) int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersinkGetFrameFlags(x.ptr, frame, flags)
}

// GetFrameRate 从滤镜出口问参数或取帧（对 av_buffersink_get_frame_rate；无参数；回分数（分子分母）；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetFrameRate() AVRational {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return AVRational{}
	}
	return fAvBuffersinkGetFrameRate(x.ptr)
}

// GetH 从滤镜出口问参数或取帧（对 av_buffersink_get_h；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetH() int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersinkGetH(x.ptr)
}

// GetHwFramesCtx 从滤镜出口问参数或取帧（对 av_buffersink_get_hw_frames_ctx；无参数；回 C 指针，失败回 nil；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetHwFramesCtx() unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return nil
	}
	return fAvBuffersinkGetHwFramesCtx(x.ptr)
}

// GetSampleAspectRatio 从滤镜出口问参数或取帧（对 av_buffersink_get_sample_aspect_ratio；无参数；回分数（分子分母）；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetSampleAspectRatio() AVRational {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return AVRational{}
	}
	return fAvBuffersinkGetSampleAspectRatio(x.ptr)
}

// GetSampleRate 从滤镜出口问参数或取帧（对 av_buffersink_get_sample_rate；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetSampleRate() int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersinkGetSampleRate(x.ptr)
}

// GetSamples 从滤镜出口问参数或取帧（对 av_buffersink_get_samples；参数 frame、nb_samples；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetSamples(frame unsafe.Pointer, nb_samples int32) int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersinkGetSamples(x.ptr, frame, nb_samples)
}

// GetTimeBase 从滤镜出口问参数或取帧（对 av_buffersink_get_time_base；无参数；回分数（分子分母）；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetTimeBase() AVRational {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return AVRational{}
	}
	return fAvBuffersinkGetTimeBase(x.ptr)
}

// GetType 问出口是视频还是音频（对 av_buffersink_get_type；无参数；回媒体类型枚举数: 0=视频/1=音频, 负数是 AVERROR；nil 接收器回 0，不崩）.
func (x *FilterSink) GetType() int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersinkGetType(x.ptr)
}

// GetW 从滤镜出口问参数或取帧（对 av_buffersink_get_w；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterSink) GetW() int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersinkGetW(x.ptr)
}

// SetFrameSize 设出口每次吐几帧的量（对 av_buffersink_set_frame_size；参数 frame_size；无返回值，无报错；nil 接收器直接回，不崩）.
func (x *FilterSink) SetFrameSize(frame_size uint32) {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return
	}
	fAvBuffersinkSetFrameSize(x.ptr, frame_size)
}

// AddFrame 往滤镜入口推帧（对 av_buffersrc_add_frame；参数 frame；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterSource) AddFrame(frame unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvBuffersrcAddFrame(x.ptr, frame); ret < 0 {
		return codeErr("av_buffersrc_add_frame", ret)
	}
	return nil
}

// AddFrameFlags 往滤镜入口推帧（对 av_buffersrc_add_frame_flags；参数 frame、flags；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterContext) AddFrameFlags(frame unsafe.Pointer, flags int32) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvBuffersrcAddFrameFlags(x.ptr, frame, flags); ret < 0 {
		return codeErr("av_buffersrc_add_frame_flags", ret)
	}
	return nil
}

// Close 往滤镜入口推帧（对 av_buffersrc_close；参数 pts、flags；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterContext) Close(pts int64, flags uint32) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvBuffersrcClose(x.ptr, pts, flags); ret < 0 {
		return codeErr("av_buffersrc_close", ret)
	}
	return nil
}

// GetNbFailedRequests 往滤镜入口推帧（对 av_buffersrc_get_nb_failed_requests；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterContext) GetNbFailedRequests() uint32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersrcGetNbFailedRequests(x.ptr)
}

// GetStatus 往滤镜入口推帧（对 av_buffersrc_get_status；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *FilterSource) GetStatus() int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvBuffersrcGetStatus(x.ptr)
}

// ParametersAlloc 往滤镜入口推帧（对 av_buffersrc_parameters_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；nil 接收器直接回零值，不崩）。
func (x *FilterSource) ParametersAlloc() unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	return fAvBuffersrcParametersAlloc()
}

// ParametersSet 往滤镜入口推帧（对 av_buffersrc_parameters_set；参数 param；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterSource) ParametersSet(param unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvBuffersrcParametersSet(x.ptr, param); ret < 0 {
		return codeErr("av_buffersrc_parameters_set", ret)
	}
	return nil
}

// WriteFrame 往滤镜入口推帧（对 av_buffersrc_write_frame；参数 frame；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterContext) WriteFrame(frame unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvBuffersrcWriteFrame(x.ptr, frame); ret < 0 {
		return codeErr("av_buffersrc_write_frame", ret)
	}
	return nil
}

// ConfigLinks 让连好的图协商参数（对 avfilter_config_links；参数 filter(须是连好的滤镜实例, 不可传 nil)；
// 成功回 nil，失败回 error；nil 接收器回错，不崩）.
func (x *FilterContext) ConfigLinks() error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterConfigLinks(x.ptr); ret < 0 {
		return codeErr("avfilter_config_links", ret)
	}
	return nil
}

// Configuration 问滤镜库编译配置（对 avfilter_configuration；无参数；回 C 指针，失败回 nil；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) Configuration() unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	return fAvfilterConfiguration()
}

// FilterPadCount 问滤镜有几个端口（对 avfilter_filter_pad_count；参数 is_output；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *Filter) FilterPadCount(is_output int32) uint32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvfilterFilterPadCount(x.ptr, is_output)
}

// Free 释放滤镜实例（对 avfilter_free；无参数；按签名取回值；nil 接收器直接回零值，不崩）。
func (x *FilterContext) Free() {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return
	}
	fAvfilterFree(x.ptr)
}

// GetByName 按名字找滤镜（对 avfilter_get_by_name；参数 name；成功回 C 指针，失败回 nil；新建的记得调对应 Free；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GetByName(name unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	return fAvfilterGetByName(name)
}

// GetClass 取滤镜的选项类（对 avfilter_get_class；无参数；回 C 指针，失败回 nil；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GetClass() unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	return fAvfilterGetClass()
}

// GraphAlloc 建图配图连图跑图（对 avfilter_graph_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphAlloc() unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	return fAvfilterGraphAlloc()
}

// GraphAllocFilter 建图配图连图跑图（对 avfilter_graph_alloc_filter；参数 filter(须是 GetByName 回的有效滤镜, 不可传 nil)、name(实例名 C 字符串)；
// 成功回滤镜实例指针，失败回 nil；注意：必须调在真图上 (GraphAlloc 回的)，零值 FilterGraph 调等于传空图会崩；
// 建出来的实例记得 Free；nil 接收器回 nil，不崩）.
func (x *FilterGraph) GraphAllocFilter(filter unsafe.Pointer, name unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return nil
	}
	return fAvfilterGraphAllocFilter(x.ptr, filter, name)
}

// GraphConfig 建图配图连图跑图（对 avfilter_graph_config；参数 log_ctx；回数值；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphConfig(log_ctx unsafe.Pointer) int32 {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return 0
	}
	return fAvfilterGraphConfig(x.ptr, log_ctx)
}

// GraphCreateFilter 建图配图连图跑图（对 avfilter_graph_create_filter；参数 filt_ctx、filt、name、args、opaque、graph_ctx；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphCreateFilter(filt_ctx *unsafe.Pointer, filt unsafe.Pointer, name unsafe.Pointer, args unsafe.Pointer, opaque unsafe.Pointer, graph_ctx unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if ret := fAvfilterGraphCreateFilter(filt_ctx, filt, name, args, opaque, graph_ctx); ret < 0 {
		return codeErr("avfilter_graph_create_filter", ret)
	}
	return nil
}

// GraphDump 建图配图连图跑图（对 avfilter_graph_dump；参数 options；回 C 指针，失败回 nil；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphDump(options unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return nil
	}
	return fAvfilterGraphDump(x.ptr, options)
}

// GraphFree 建图配图连图跑图（对 avfilter_graph_free；参数 graph；按签名取回值；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphFree(graph *unsafe.Pointer) {
	mustUse(ensureModFilterGraph())
	fAvfilterGraphFree(graph)
}

// GraphGetFilter 建图配图连图跑图（对 avfilter_graph_get_filter；参数 name；回 C 指针，失败回 nil；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphGetFilter(name unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return nil
	}
	return fAvfilterGraphGetFilter(x.ptr, name)
}

// GraphParse 建图配图连图跑图（对 avfilter_graph_parse；参数 filters、inputs、outputs、log_ctx；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphParse(filters unsafe.Pointer, inputs unsafe.Pointer, outputs unsafe.Pointer, log_ctx unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterGraphParse(x.ptr, filters, inputs, outputs, log_ctx); ret < 0 {
		return codeErr("avfilter_graph_parse", ret)
	}
	return nil
}

// GraphParse2 建图配图连图跑图（对 avfilter_graph_parse2；参数 filters、inputs、outputs；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphParse2(filters unsafe.Pointer, inputs *unsafe.Pointer, outputs *unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterGraphParse2(x.ptr, filters, inputs, outputs); ret < 0 {
		return codeErr("avfilter_graph_parse2", ret)
	}
	return nil
}

// GraphParsePtr 建图配图连图跑图（对 avfilter_graph_parse_ptr；参数 filters、inputs、outputs、log_ctx；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphParsePtr(filters unsafe.Pointer, inputs *unsafe.Pointer, outputs *unsafe.Pointer, log_ctx unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterGraphParsePtr(x.ptr, filters, inputs, outputs, log_ctx); ret < 0 {
		return codeErr("avfilter_graph_parse_ptr", ret)
	}
	return nil
}

// GraphQueueCommand 建图配图连图跑图（对 avfilter_graph_queue_command；参数 target、cmd、arg、flags、ts；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphQueueCommand(target unsafe.Pointer, cmd unsafe.Pointer, arg unsafe.Pointer, flags int32, ts float64) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterGraphQueueCommand(x.ptr, target, cmd, arg, flags, ts); ret < 0 {
		return codeErr("avfilter_graph_queue_command", ret)
	}
	return nil
}

// GraphRequestOldest 建图配图连图跑图（对 avfilter_graph_request_oldest；无参数；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphRequestOldest() error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterGraphRequestOldest(x.ptr); ret < 0 {
		return codeErr("avfilter_graph_request_oldest", ret)
	}
	return nil
}

// GraphSegmentApply 建图配图连图跑图（对 avfilter_graph_segment_apply；参数 seg、flags、inputs、outputs；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (x *FilterGraph) GraphSegmentApply(seg unsafe.Pointer, flags int32, inputs *unsafe.Pointer, outputs *unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if ret := fAvfilterGraphSegmentApply(seg, flags, inputs, outputs); ret < 0 {
		return codeErr("avfilter_graph_segment_apply", ret)
	}
	return nil
}

// GraphSegmentApplyOpts 建图配图连图跑图（对 avfilter_graph_segment_apply_opts；参数 seg、flags；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (x *FilterGraph) GraphSegmentApplyOpts(seg unsafe.Pointer, flags int32) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if ret := fAvfilterGraphSegmentApplyOpts(seg, flags); ret < 0 {
		return codeErr("avfilter_graph_segment_apply_opts", ret)
	}
	return nil
}

// GraphSegmentCreateFilters 建图配图连图跑图（对 avfilter_graph_segment_create_filters；参数 seg、flags；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (x *FilterGraph) GraphSegmentCreateFilters(seg unsafe.Pointer, flags int32) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if ret := fAvfilterGraphSegmentCreateFilters(seg, flags); ret < 0 {
		return codeErr("avfilter_graph_segment_create_filters", ret)
	}
	return nil
}

// GraphSegmentFree 建图配图连图跑图（对 avfilter_graph_segment_free；参数 seg；按签名取回值；无状态调用）。
func (x *FilterGraph) GraphSegmentFree(seg *unsafe.Pointer) {
	mustUse(ensureModFilterGraph())
	fAvfilterGraphSegmentFree(seg)
}

// GraphSegmentInit 建图配图连图跑图（对 avfilter_graph_segment_init；参数 seg、flags；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (x *FilterGraph) GraphSegmentInit(seg unsafe.Pointer, flags int32) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if ret := fAvfilterGraphSegmentInit(seg, flags); ret < 0 {
		return codeErr("avfilter_graph_segment_init", ret)
	}
	return nil
}

// GraphSegmentLink 建图配图连图跑图（对 avfilter_graph_segment_link；参数 seg、flags、inputs、outputs；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphSegmentLink(seg unsafe.Pointer, flags int32, inputs *unsafe.Pointer, outputs *unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if ret := fAvfilterGraphSegmentLink(seg, flags, inputs, outputs); ret < 0 {
		return codeErr("avfilter_graph_segment_link", ret)
	}
	return nil
}

// GraphSegmentParse 建图配图连图跑图（对 avfilter_graph_segment_parse；参数 graph_str、flags、seg；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphSegmentParse(graph_str unsafe.Pointer, flags int32, seg *unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterGraphSegmentParse(x.ptr, graph_str, flags, seg); ret < 0 {
		return codeErr("avfilter_graph_segment_parse", ret)
	}
	return nil
}

// GraphSendCommand 建图配图连图跑图（对 avfilter_graph_send_command；参数 target、cmd、arg、res、res_len、flags；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphSendCommand(target unsafe.Pointer, cmd unsafe.Pointer, arg unsafe.Pointer, res unsafe.Pointer, res_len int32, flags int32) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterGraphSendCommand(x.ptr, target, cmd, arg, res, res_len, flags); ret < 0 {
		return codeErr("avfilter_graph_send_command", ret)
	}
	return nil
}

// GraphSetAutoConvert 建图配图连图跑图（对 avfilter_graph_set_auto_convert；参数 flags；按签名取回值；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) GraphSetAutoConvert(flags uint32) {
	mustUse(ensureModFilterGraph())
	if x == nil {
		return
	}
	fAvfilterGraphSetAutoConvert(x.ptr, flags)
}

// InitDict 初始化滤镜实例（对 avfilter_init_dict；参数 options；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterContext) InitDict(options *unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterInitDict(x.ptr, options); ret < 0 {
		return codeErr("avfilter_init_dict", ret)
	}
	return nil
}

// InitStr 初始化滤镜实例（对 avfilter_init_str；参数 args；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterContext) InitStr(args unsafe.Pointer) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterInitStr(x.ptr, args); ret < 0 {
		return codeErr("avfilter_init_str", ret)
	}
	return nil
}

// InoutAlloc 分配或释放图的进出口端点（对 avfilter_inout_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (x *FilterGraph) InoutAlloc() unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	return fAvfilterInoutAlloc()
}

// InoutFree 分配或释放图的进出口端点（对 avfilter_inout_free；参数 inout；按签名取回值；无状态调用）。
func (x *FilterGraph) InoutFree(inout *unsafe.Pointer) {
	mustUse(ensureModFilterGraph())
	fAvfilterInoutFree(inout)
}

// InsertFilter 在连好的链中间插一个滤镜（对 avfilter_insert_filter；参数 link、filt、filt_srcpad_idx、filt_dstpad_idx；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) InsertFilter(link unsafe.Pointer, filt unsafe.Pointer, filt_srcpad_idx uint32, filt_dstpad_idx uint32) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if ret := fAvfilterInsertFilter(link, filt, filt_srcpad_idx, filt_dstpad_idx); ret < 0 {
		return codeErr("avfilter_insert_filter", ret)
	}
	return nil
}

// License 问滤镜库许可证（对 avfilter_license；无参数；回 C 指针，失败回 nil；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) License() unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	return fAvfilterLicense()
}

// Link 把两个滤镜连起来（对 avfilter_link；参数 srcpad、dst、dstpad；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *FilterContext) Link(srcpad uint32, dst unsafe.Pointer, dstpad uint32) error {
	if err := ensureModFilterGraph(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvfilterLink(x.ptr, srcpad, dst, dstpad); ret < 0 {
		return codeErr("avfilter_link", ret)
	}
	return nil
}

// LinkFree 释放一条连线（对 avfilter_link_free；参数 link(指向连线指针的槽, 传 *unsafe.Pointer；C 会把槽置空；空槽直接回)；无返回值；无状态调用）.
func (x *FilterGraph) LinkFree(link *unsafe.Pointer) {
	mustUse(ensureModFilterGraph())
	fAvfilterLinkFree(link)
}

// PadGetName 问滤镜端口名字或类型（对 avfilter_pad_get_name；参数 pads、pad_idx；成功回 C 指针，失败回 nil；新建的记得调对应 Free；nil 接收器直接回零值，不崩）。
func (x *FilterGraph) PadGetName(pads unsafe.Pointer, pad_idx int32) unsafe.Pointer {
	mustUse(ensureModFilterGraph())
	return fAvfilterPadGetName(pads, pad_idx)
}

// PadGetType 问滤镜端口是视频还是音频（对 avfilter_pad_get_type；参数 pads(AVFilter 头 16/24 字节处的端口数组, 不可传 nil)、pad_idx；
// 回媒体类型枚举数: 0=视频/1=音频；无状态调用）.
func (x *FilterGraph) PadGetType(pads unsafe.Pointer, pad_idx int32) int32 {
	mustUse(ensureModFilterGraph())
	return fAvfilterPadGetType(pads, pad_idx)
}

// ProcessCommand 给跑着的滤镜发命令（对 avfilter_process_command；参数 cmd、arg(C 字符串指针)、res(可写回执内存, 传 nil 表不要回执)、res_len、flags；
// 回 (>=0=成功, error 非 nil=失败)；nil 接收器回错，不崩）.
func (x *FilterContext) ProcessCommand(cmd unsafe.Pointer, arg unsafe.Pointer, res unsafe.Pointer, res_len int32, flags int32) (int32, error) {
	if err := ensureModFilterGraph(); err != nil {
		return 0, err
	}
	if x == nil {
		return 0, errNilFF
	}
	if ret := fAvfilterProcessCommand(x.ptr, cmd, arg, res, res_len, flags); ret < 0 {
		return 0, codeErr("avfilter_process_command", ret)
	} else {
		return ret, nil
	}
}

// Version 问滤镜库版本号（对 avfilter_version；无参数；回版本号 uint32(大端 16 位是主版本, 10 表 7.x)；无状态调用）.
func (x *FilterGraph) Version() uint32 {
	mustUse(ensureModFilterGraph())
	return fAvfilterVersion()
}
