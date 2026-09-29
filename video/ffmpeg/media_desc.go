package ffmpeg

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// MediaDesc 媒体描述模块: 像素格式/声道/颜色/元数据查询全量导出.
//
// Say it plain: 问格式叫啥、多大、支不支持, 全是只读查询, 不动数据.

// MediaDesc holder.
type MediaDesc struct{ ptr unsafe.Pointer }

func (x *MediaDesc) Ptr() unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	if x == nil {
		return nil
	}
	return x.ptr
}

var (
	fAvAmbientViewingEnvironmentAlloc          func(size unsafe.Pointer) unsafe.Pointer
	fAvAmbientViewingEnvironmentCreateSideData func(frame unsafe.Pointer) unsafe.Pointer
	fAvChannelDescription                      func(buf unsafe.Pointer, buf_size uintptr, channel int32) int32
	fAvChannelDescriptionBprint                func(bp unsafe.Pointer, channel_id int32)
	fAvChannelFromString                       func(name unsafe.Pointer) int32
	fAvChannelLayoutAmbisonicOrder             func(channel_layout unsafe.Pointer) int32
	fAvChannelLayoutChannelFromIndex           func(channel_layout unsafe.Pointer, idx uint32) int32
	fAvChannelLayoutChannelFromString          func(channel_layout unsafe.Pointer, name unsafe.Pointer) int32
	fAvChannelLayoutCheck                      func(channel_layout unsafe.Pointer) int32
	fAvChannelLayoutCompare                    func(chl unsafe.Pointer, chl1 unsafe.Pointer) int32
	fAvChannelLayoutCopy                       func(dst unsafe.Pointer, src unsafe.Pointer) int32
	fAvChannelLayoutCustomInit                 func(channel_layout unsafe.Pointer, nb_channels int32) int32
	fAvChannelLayoutDefault                    func(ch_layout unsafe.Pointer, nb_channels int32)
	fAvChannelLayoutDescribe                   func(channel_layout unsafe.Pointer, buf unsafe.Pointer, buf_size uintptr) int32
	fAvChannelLayoutDescribeBprint             func(channel_layout unsafe.Pointer, bp unsafe.Pointer) int32
	fAvChannelLayoutFromMask                   func(channel_layout unsafe.Pointer, mask uint64) int32
	fAvChannelLayoutFromString                 func(channel_layout unsafe.Pointer, str unsafe.Pointer) int32
	fAvChannelLayoutIndexFromChannel           func(channel_layout unsafe.Pointer, channel int32) int32
	fAvChannelLayoutIndexFromString            func(channel_layout unsafe.Pointer, name unsafe.Pointer) int32
	fAvChannelLayoutRetype                     func(channel_layout unsafe.Pointer, order int32, flags int32) int32
	fAvChannelLayoutStandard                   func(opaque *unsafe.Pointer) unsafe.Pointer
	fAvChannelLayoutSubset                     func(channel_layout unsafe.Pointer, mask uint64) uint64
	fAvChannelLayoutUninit                     func(channel_layout unsafe.Pointer)
	fAvChannelName                             func(buf unsafe.Pointer, buf_size uintptr, channel int32) int32
	fAvChannelNameBprint                       func(bp unsafe.Pointer, channel_id int32)
	fAvColorPrimariesFromName                  func(name unsafe.Pointer) int32
	fAvColorPrimariesName                      func(primaries int32) unsafe.Pointer
	fAvColorRangeFromName                      func(name string) int32
	fAvColorRangeName                          func(rng int32) unsafe.Pointer
	fAvColorSpaceFromName                      func(name string) int32
	fAvColorSpaceName                          func(space int32) unsafe.Pointer
	fAvColorTransferFromName                   func(name unsafe.Pointer) int32
	fAvColorTransferName                       func(transfer int32) unsafe.Pointer
	fAvContentLightMetadataAlloc               func(size unsafe.Pointer) unsafe.Pointer
	fAvContentLightMetadataCreateSideData      func(frame unsafe.Pointer) unsafe.Pointer
	fAvDisplayMatrixFlip                       func(matrix unsafe.Pointer, hflip int32, vflip int32)
	fAvDisplayRotationGet                      func(matrix unsafe.Pointer) float64
	fAvDisplayRotationSet                      func(matrix unsafe.Pointer, angle float64)
	fAvDoviAlloc                               func(size unsafe.Pointer) unsafe.Pointer
	fAvDoviFindLevel                           func(data unsafe.Pointer, level uint8) unsafe.Pointer
	fAvDoviMetadataAlloc                       func(size unsafe.Pointer) unsafe.Pointer
	fAvDynamicHdrPlusAlloc                     func(size unsafe.Pointer) unsafe.Pointer
	fAvDynamicHdrPlusCreateSideData            func(frame unsafe.Pointer) unsafe.Pointer
	fAvDynamicHdrPlusFromT35                   func(s unsafe.Pointer, data unsafe.Pointer, size uintptr) int32
	fAvDynamicHdrPlusToT35                     func(s unsafe.Pointer, data *unsafe.Pointer, size unsafe.Pointer) int32
	fAvFilmGrainParamsAlloc                    func(size unsafe.Pointer) unsafe.Pointer
	fAvFilmGrainParamsCreateSideData           func(frame unsafe.Pointer) unsafe.Pointer
	fAvFilmGrainParamsSelect                   func(frame unsafe.Pointer) unsafe.Pointer
	fAvGetBitsPerPixel                         func(pixdesc unsafe.Pointer) int32
	fAvGetPixFmt                               func(name string) int32
	fAvGetPixFmtLoss                           func(dst_pix_fmt int32, src_pix_fmt int32, has_alpha int32) int32
	fAvGetPixFmtName                           func(pix_fmt int32) unsafe.Pointer
	fAvGetPixFmtString                         func(buf unsafe.Pointer, buf_size int32, pix_fmt int32) unsafe.Pointer
	fAvMasteringDisplayMetadataAlloc           func() unsafe.Pointer
	fAvMasteringDisplayMetadataAllocSize       func(size unsafe.Pointer) unsafe.Pointer
	fAvMasteringDisplayMetadataCreateSideData  func(frame unsafe.Pointer) unsafe.Pointer
	fAvParseColor                              func(rgba_color unsafe.Pointer, color_string unsafe.Pointer, slen int32, log_ctx unsafe.Pointer) int32
	fAvParseRatio                              func(q unsafe.Pointer, str unsafe.Pointer, max int32, log_offset int32, log_ctx unsafe.Pointer) int32
	fAvParseTime                               func(timeval unsafe.Pointer, timestr unsafe.Pointer, duration int32) int32
	fAvParseVideoRate                          func(rate unsafe.Pointer, str unsafe.Pointer) int32
	fAvParseVideoSize                          func(width_ptr unsafe.Pointer, height_ptr unsafe.Pointer, str unsafe.Pointer) int32
	fAvPixFmtDescGet                           func(pix_fmt int32) unsafe.Pointer
	fAvPixFmtDescGetId                         func(desc unsafe.Pointer) int32
	fAvPixFmtDescNext                          func(prev unsafe.Pointer) unsafe.Pointer
	fAvSphericalAlloc                          func(size unsafe.Pointer) unsafe.Pointer
	fAvSphericalFromName                       func(name unsafe.Pointer) int32
	fAvSphericalProjectionName                 func(projection int32) unsafe.Pointer
	fAvSphericalTileBounds                     func(mp unsafe.Pointer, width uintptr, height uintptr, left unsafe.Pointer, top unsafe.Pointer, right unsafe.Pointer, bottom unsafe.Pointer)
	fAvStereo3dAlloc                           func() unsafe.Pointer
	fAvStereo3dAllocSize                       func(size unsafe.Pointer) unsafe.Pointer
	fAvStereo3dCreateSideData                  func(frame unsafe.Pointer) unsafe.Pointer
	fAvStereo3dFromName                        func(name unsafe.Pointer) int32
	fAvStereo3dPrimaryEyeFromName              func(name unsafe.Pointer) int32
	fAvStereo3dPrimaryEyeName                  func(eye uint32) unsafe.Pointer
	fAvStereo3dTypeName                        func(typ uint32) unsafe.Pointer
	fAvStereo3dViewFromName                    func(name unsafe.Pointer) int32
	fAvStereo3dViewName                        func(view uint32) unsafe.Pointer
	fAvTimecodeAdjustNtscFramenum2             func(framenum int32, fps int32) int32
	fAvTimecodeCheckFrameRate                  func(rate AVRational) int32
	fAvTimecodeGetSmpte                        func(rate AVRational, drop int32, hh int32, mm int32, ss int32, ff int32) uint32
	fAvTimecodeGetSmpteFromFramenum            func(tc unsafe.Pointer, framenum int32) uint32
	fAvTimecodeInit                            func(tc unsafe.Pointer, rate AVRational, flags int32, frame_start int32, log_ctx unsafe.Pointer) int32
	fAvTimecodeInitFromComponents              func(tc unsafe.Pointer, rate AVRational, flags int32, hh int32, mm int32, ss int32, ff int32, log_ctx unsafe.Pointer) int32
	fAvTimecodeInitFromString                  func(tc unsafe.Pointer, rate AVRational, str unsafe.Pointer, log_ctx unsafe.Pointer) int32
	fAvTimecodeMakeMpegTcString                func(buf unsafe.Pointer, tc25bit uint32) unsafe.Pointer
	fAvTimecodeMakeSmpteTcString               func(buf unsafe.Pointer, tcsmpte uint32, prevent_df int32) unsafe.Pointer
	fAvTimecodeMakeSmpteTcString2              func(buf unsafe.Pointer, rate AVRational, tcsmpte uint32, prevent_df int32, skip_field int32) unsafe.Pointer
	fAvTimecodeMakeString                      func(tc unsafe.Pointer, buf unsafe.Pointer, framenum int32) unsafe.Pointer
)

// ensureModMediaDesc 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modMediaDescOnce sync.Once

func ensureModMediaDesc() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	modMediaDescOnce.Do(func() { registerMediaDesc(libHandle) })
	return nil
}

func registerMediaDesc(h uintptr) {
	purego.RegisterLibFunc(&fAvAmbientViewingEnvironmentAlloc, h, "av_ambient_viewing_environment_alloc")
	purego.RegisterLibFunc(&fAvAmbientViewingEnvironmentCreateSideData, h, "av_ambient_viewing_environment_create_side_data")
	purego.RegisterLibFunc(&fAvChannelDescription, h, "av_channel_description")
	purego.RegisterLibFunc(&fAvChannelDescriptionBprint, h, "av_channel_description_bprint")
	purego.RegisterLibFunc(&fAvChannelFromString, h, "av_channel_from_string")
	purego.RegisterLibFunc(&fAvChannelLayoutAmbisonicOrder, h, "av_channel_layout_ambisonic_order")
	purego.RegisterLibFunc(&fAvChannelLayoutChannelFromIndex, h, "av_channel_layout_channel_from_index")
	purego.RegisterLibFunc(&fAvChannelLayoutChannelFromString, h, "av_channel_layout_channel_from_string")
	purego.RegisterLibFunc(&fAvChannelLayoutCheck, h, "av_channel_layout_check")
	purego.RegisterLibFunc(&fAvChannelLayoutCompare, h, "av_channel_layout_compare")
	purego.RegisterLibFunc(&fAvChannelLayoutCopy, h, "av_channel_layout_copy")
	purego.RegisterLibFunc(&fAvChannelLayoutCustomInit, h, "av_channel_layout_custom_init")
	purego.RegisterLibFunc(&fAvChannelLayoutDefault, h, "av_channel_layout_default")
	purego.RegisterLibFunc(&fAvChannelLayoutDescribe, h, "av_channel_layout_describe")
	purego.RegisterLibFunc(&fAvChannelLayoutDescribeBprint, h, "av_channel_layout_describe_bprint")
	purego.RegisterLibFunc(&fAvChannelLayoutFromMask, h, "av_channel_layout_from_mask")
	purego.RegisterLibFunc(&fAvChannelLayoutFromString, h, "av_channel_layout_from_string")
	purego.RegisterLibFunc(&fAvChannelLayoutIndexFromChannel, h, "av_channel_layout_index_from_channel")
	purego.RegisterLibFunc(&fAvChannelLayoutIndexFromString, h, "av_channel_layout_index_from_string")
	purego.RegisterLibFunc(&fAvChannelLayoutRetype, h, "av_channel_layout_retype")
	purego.RegisterLibFunc(&fAvChannelLayoutStandard, h, "av_channel_layout_standard")
	purego.RegisterLibFunc(&fAvChannelLayoutSubset, h, "av_channel_layout_subset")
	purego.RegisterLibFunc(&fAvChannelLayoutUninit, h, "av_channel_layout_uninit")
	purego.RegisterLibFunc(&fAvChannelName, h, "av_channel_name")
	purego.RegisterLibFunc(&fAvChannelNameBprint, h, "av_channel_name_bprint")
	purego.RegisterLibFunc(&fAvColorPrimariesFromName, h, "av_color_primaries_from_name")
	purego.RegisterLibFunc(&fAvColorPrimariesName, h, "av_color_primaries_name")
	purego.RegisterLibFunc(&fAvColorRangeFromName, h, "av_color_range_from_name")
	purego.RegisterLibFunc(&fAvColorRangeName, h, "av_color_range_name")
	purego.RegisterLibFunc(&fAvColorSpaceFromName, h, "av_color_space_from_name")
	purego.RegisterLibFunc(&fAvColorSpaceName, h, "av_color_space_name")
	purego.RegisterLibFunc(&fAvColorTransferFromName, h, "av_color_transfer_from_name")
	purego.RegisterLibFunc(&fAvColorTransferName, h, "av_color_transfer_name")
	purego.RegisterLibFunc(&fAvContentLightMetadataAlloc, h, "av_content_light_metadata_alloc")
	purego.RegisterLibFunc(&fAvContentLightMetadataCreateSideData, h, "av_content_light_metadata_create_side_data")
	purego.RegisterLibFunc(&fAvDisplayMatrixFlip, h, "av_display_matrix_flip")
	purego.RegisterLibFunc(&fAvDisplayRotationGet, h, "av_display_rotation_get")
	purego.RegisterLibFunc(&fAvDisplayRotationSet, h, "av_display_rotation_set")
	purego.RegisterLibFunc(&fAvDoviAlloc, h, "av_dovi_alloc")
	purego.RegisterLibFunc(&fAvDoviFindLevel, h, "av_dovi_find_level")
	purego.RegisterLibFunc(&fAvDoviMetadataAlloc, h, "av_dovi_metadata_alloc")
	purego.RegisterLibFunc(&fAvDynamicHdrPlusAlloc, h, "av_dynamic_hdr_plus_alloc")
	purego.RegisterLibFunc(&fAvDynamicHdrPlusCreateSideData, h, "av_dynamic_hdr_plus_create_side_data")
	purego.RegisterLibFunc(&fAvDynamicHdrPlusFromT35, h, "av_dynamic_hdr_plus_from_t35")
	purego.RegisterLibFunc(&fAvDynamicHdrPlusToT35, h, "av_dynamic_hdr_plus_to_t35")
	purego.RegisterLibFunc(&fAvFilmGrainParamsAlloc, h, "av_film_grain_params_alloc")
	purego.RegisterLibFunc(&fAvFilmGrainParamsCreateSideData, h, "av_film_grain_params_create_side_data")
	purego.RegisterLibFunc(&fAvFilmGrainParamsSelect, h, "av_film_grain_params_select")
	purego.RegisterLibFunc(&fAvGetBitsPerPixel, h, "av_get_bits_per_pixel")
	purego.RegisterLibFunc(&fAvGetPixFmt, h, "av_get_pix_fmt")
	purego.RegisterLibFunc(&fAvGetPixFmtLoss, h, "av_get_pix_fmt_loss")
	purego.RegisterLibFunc(&fAvGetPixFmtName, h, "av_get_pix_fmt_name")
	purego.RegisterLibFunc(&fAvGetPixFmtString, h, "av_get_pix_fmt_string")
	purego.RegisterLibFunc(&fAvMasteringDisplayMetadataAlloc, h, "av_mastering_display_metadata_alloc")
	purego.RegisterLibFunc(&fAvMasteringDisplayMetadataAllocSize, h, "av_mastering_display_metadata_alloc_size")
	purego.RegisterLibFunc(&fAvMasteringDisplayMetadataCreateSideData, h, "av_mastering_display_metadata_create_side_data")
	purego.RegisterLibFunc(&fAvParseColor, h, "av_parse_color")
	purego.RegisterLibFunc(&fAvParseRatio, h, "av_parse_ratio")
	purego.RegisterLibFunc(&fAvParseTime, h, "av_parse_time")
	purego.RegisterLibFunc(&fAvParseVideoRate, h, "av_parse_video_rate")
	purego.RegisterLibFunc(&fAvParseVideoSize, h, "av_parse_video_size")
	purego.RegisterLibFunc(&fAvPixFmtDescGet, h, "av_pix_fmt_desc_get")
	purego.RegisterLibFunc(&fAvPixFmtDescGetId, h, "av_pix_fmt_desc_get_id")
	purego.RegisterLibFunc(&fAvPixFmtDescNext, h, "av_pix_fmt_desc_next")
	purego.RegisterLibFunc(&fAvSphericalAlloc, h, "av_spherical_alloc")
	purego.RegisterLibFunc(&fAvSphericalFromName, h, "av_spherical_from_name")
	purego.RegisterLibFunc(&fAvSphericalProjectionName, h, "av_spherical_projection_name")
	purego.RegisterLibFunc(&fAvSphericalTileBounds, h, "av_spherical_tile_bounds")
	purego.RegisterLibFunc(&fAvStereo3dAlloc, h, "av_stereo3d_alloc")
	purego.RegisterLibFunc(&fAvStereo3dAllocSize, h, "av_stereo3d_alloc_size")
	purego.RegisterLibFunc(&fAvStereo3dCreateSideData, h, "av_stereo3d_create_side_data")
	purego.RegisterLibFunc(&fAvStereo3dFromName, h, "av_stereo3d_from_name")
	purego.RegisterLibFunc(&fAvStereo3dPrimaryEyeFromName, h, "av_stereo3d_primary_eye_from_name")
	purego.RegisterLibFunc(&fAvStereo3dPrimaryEyeName, h, "av_stereo3d_primary_eye_name")
	purego.RegisterLibFunc(&fAvStereo3dTypeName, h, "av_stereo3d_type_name")
	purego.RegisterLibFunc(&fAvStereo3dViewFromName, h, "av_stereo3d_view_from_name")
	purego.RegisterLibFunc(&fAvStereo3dViewName, h, "av_stereo3d_view_name")
	purego.RegisterLibFunc(&fAvTimecodeAdjustNtscFramenum2, h, "av_timecode_adjust_ntsc_framenum2")
	purego.RegisterLibFunc(&fAvTimecodeCheckFrameRate, h, "av_timecode_check_frame_rate")
	purego.RegisterLibFunc(&fAvTimecodeGetSmpte, h, "av_timecode_get_smpte")
	purego.RegisterLibFunc(&fAvTimecodeGetSmpteFromFramenum, h, "av_timecode_get_smpte_from_framenum")
	purego.RegisterLibFunc(&fAvTimecodeInit, h, "av_timecode_init")
	purego.RegisterLibFunc(&fAvTimecodeInitFromComponents, h, "av_timecode_init_from_components")
	purego.RegisterLibFunc(&fAvTimecodeInitFromString, h, "av_timecode_init_from_string")
	purego.RegisterLibFunc(&fAvTimecodeMakeMpegTcString, h, "av_timecode_make_mpeg_tc_string")
	purego.RegisterLibFunc(&fAvTimecodeMakeSmpteTcString, h, "av_timecode_make_smpte_tc_string")
	purego.RegisterLibFunc(&fAvTimecodeMakeSmpteTcString2, h, "av_timecode_make_smpte_tc_string2")
	purego.RegisterLibFunc(&fAvTimecodeMakeString, h, "av_timecode_make_string")
}

// AmbientViewingEnvironmentAlloc 环境光元数据操作（对 av_ambient_viewing_environment_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) AmbientViewingEnvironmentAlloc(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvAmbientViewingEnvironmentAlloc(size)
}

// AmbientViewingEnvironmentCreateSideData 环境光元数据操作（对 av_ambient_viewing_environment_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) AmbientViewingEnvironmentCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvAmbientViewingEnvironmentCreateSideData(frame)
}

// ChannelDescription 查单个声道名或描述（对 av_channel_description；参数 buf、buf_size、channel(传 AVChannel 枚举数, 如 0=FL)；
// 回 (需字节数, error), 截断时回值大于 buf_size；buf 须是可写内存不可传 nil, channel 传 int32 不可传指针）.
func (x *MediaDesc) ChannelDescription(buf unsafe.Pointer, buf_size uintptr, channel int32) (int32, error) {
	if err := ensureModMediaDesc(); err != nil {
		return 0, err
	}
	if ret := fAvChannelDescription(buf, buf_size, channel); ret < 0 {
		return 0, codeErr("av_channel_description", ret)
	} else {
		return ret, nil
	}
}

// ChannelDescriptionBprint 查单个声道名或描述（对 av_channel_description_bprint；参数 bp(须是 NewBPrint 建的, 不可传 nil)、channel_id(AVChannel 枚举数)；无返回值，往 bp 里追加）.
func (x *MediaDesc) ChannelDescriptionBprint(bp unsafe.Pointer, channel_id int32) {
	mustUse(ensureModMediaDesc())
	fAvChannelDescriptionBprint(bp, channel_id)
}

// ChannelFromString 按名查声道号（对 av_channel_from_string；参数 name；
// 回声道号，非法名回 AV_CHAN_NONE(-1)；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelFromString(name unsafe.Pointer) int32 {
	mustUse(ensureModMediaDesc())
	return fAvChannelFromString(name)
}

// ChannelLayoutAmbisonicOrder 查声道布局的 ambisonic 阶数（对 av_channel_layout_ambisonic_order；参数 channel_layout(须是有效布局指针, 不可传 nil)；
// 回 (阶数, error), 非 ambisonic 布局回 error；无状态，可用零值直接调）.
func (x *MediaDesc) ChannelLayoutAmbisonicOrder(channel_layout unsafe.Pointer) (int32, error) {
	if err := ensureModMediaDesc(); err != nil {
		return 0, err
	}
	if ret := fAvChannelLayoutAmbisonicOrder(channel_layout); ret < 0 {
		return 0, codeErr("av_channel_layout_ambisonic_order", ret)
	} else {
		return ret, nil
	}
}

// ChannelLayoutChannelFromIndex 按下标查声道（对 av_channel_layout_channel_from_index；参数 channel_layout(有效布局指针)、idx；
// 回 AVChannel 枚举数, 失败回 AV_CHAN_NONE(-1)；无状态，可用零值直接调）.
func (x *MediaDesc) ChannelLayoutChannelFromIndex(channel_layout unsafe.Pointer, idx uint32) int32 {
	mustUse(ensureModMediaDesc())
	return fAvChannelLayoutChannelFromIndex(channel_layout, idx)
}

// ChannelLayoutChannelFromString 按名查布局里的声道（对 av_channel_layout_channel_from_string；参数 channel_layout(有效布局指针)、name(C 字符串指针)；
// 回 AVChannel 枚举数, 失败回 AV_CHAN_NONE(-1)；无状态，可用零值直接调）.
func (x *MediaDesc) ChannelLayoutChannelFromString(channel_layout unsafe.Pointer, name unsafe.Pointer) int32 {
	mustUse(ensureModMediaDesc())
	return fAvChannelLayoutChannelFromString(channel_layout, name)
}

// ChannelLayoutCheck 查声道布局是否有效（对 av_channel_layout_check；参数 channel_layout；
// C 回 1 有效、0 无效；有效回 nil，无效回 error；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutCheck(channel_layout unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvChannelLayoutCheck(channel_layout); ret == 0 {
		return codeErr("av_channel_layout_check", 0)
	}
	return nil
}

// ChannelLayoutCompare 比两个声道布局语义是否一样（对 av_channel_layout_compare；参数 chl、chl1(有效布局指针)；
// 回 0 表一样、1 表不一样, 负数是 AVERROR；回 (值, error), error 非 nil 表布局本身无效；无状态，可用零值直接调）.
func (x *MediaDesc) ChannelLayoutCompare(chl unsafe.Pointer, chl1 unsafe.Pointer) (int32, error) {
	if err := ensureModMediaDesc(); err != nil {
		return 0, err
	}
	if ret := fAvChannelLayoutCompare(chl, chl1); ret < 0 {
		return 0, codeErr("av_channel_layout_compare", ret)
	} else {
		return ret, nil
	}
}

// ChannelLayoutCopy 查或配声道布局（对 av_channel_layout_copy；参数 dst、src；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutCopy(dst unsafe.Pointer, src unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvChannelLayoutCopy(dst, src); ret < 0 {
		return codeErr("av_channel_layout_copy", ret)
	}
	return nil
}

// ChannelLayoutCustomInit 查或配声道布局（对 av_channel_layout_custom_init；参数 channel_layout、nb_channels；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutCustomInit(channel_layout unsafe.Pointer, nb_channels int32) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvChannelLayoutCustomInit(channel_layout, nb_channels); ret < 0 {
		return codeErr("av_channel_layout_custom_init", ret)
	}
	return nil
}

// ChannelLayoutDefault 查或配声道布局（对 av_channel_layout_default；参数 ch_layout、nb_channels；按签名取回值；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutDefault(ch_layout unsafe.Pointer, nb_channels int32) {
	mustUse(ensureModMediaDesc())
	fAvChannelLayoutDefault(ch_layout, nb_channels)
}

// ChannelLayoutDescribe 把声道布局拼成人话串（对 av_channel_layout_describe；参数 channel_layout(有效布局指针)、buf(可写内存)、buf_size；
// 回 (需字节数, error), 截断时回值大于 buf_size；无状态，可用零值直接调）.
func (x *MediaDesc) ChannelLayoutDescribe(channel_layout unsafe.Pointer, buf unsafe.Pointer, buf_size uintptr) (int32, error) {
	if err := ensureModMediaDesc(); err != nil {
		return 0, err
	}
	if ret := fAvChannelLayoutDescribe(channel_layout, buf, buf_size); ret < 0 {
		return 0, codeErr("av_channel_layout_describe", ret)
	} else {
		return ret, nil
	}
}

// ChannelLayoutDescribeBprint 查或配声道布局（对 av_channel_layout_describe_bprint；参数 channel_layout、bp；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutDescribeBprint(channel_layout unsafe.Pointer, bp unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvChannelLayoutDescribeBprint(channel_layout, bp); ret < 0 {
		return codeErr("av_channel_layout_describe_bprint", ret)
	}
	return nil
}

// ChannelLayoutFromMask 查或配声道布局（对 av_channel_layout_from_mask；参数 channel_layout、mask；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutFromMask(channel_layout unsafe.Pointer, mask uint64) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvChannelLayoutFromMask(channel_layout, mask); ret < 0 {
		return codeErr("av_channel_layout_from_mask", ret)
	}
	return nil
}

// ChannelLayoutFromString 查或配声道布局（对 av_channel_layout_from_string；参数 channel_layout、str；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutFromString(channel_layout unsafe.Pointer, str unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvChannelLayoutFromString(channel_layout, str); ret < 0 {
		return codeErr("av_channel_layout_from_string", ret)
	}
	return nil
}

// ChannelLayoutIndexFromChannel 查声道在布局里的下标（对 av_channel_layout_index_from_channel；参数 channel_layout(有效布局指针)、channel(AVChannel 枚举数)；
// 回 (下标, error), 声道不在布局里回 error；无状态，可用零值直接调）.
func (x *MediaDesc) ChannelLayoutIndexFromChannel(channel_layout unsafe.Pointer, channel int32) (int32, error) {
	if err := ensureModMediaDesc(); err != nil {
		return 0, err
	}
	if ret := fAvChannelLayoutIndexFromChannel(channel_layout, channel); ret < 0 {
		return 0, codeErr("av_channel_layout_index_from_channel", ret)
	} else {
		return ret, nil
	}
}

// ChannelLayoutIndexFromString 按名查声道在布局里的下标（对 av_channel_layout_index_from_string；参数 channel_layout(有效布局指针)、name(C 字符串指针)；
// 回 (下标, error), 名子不对或声道不在布局里回 error；无状态，可用零值直接调）.
func (x *MediaDesc) ChannelLayoutIndexFromString(channel_layout unsafe.Pointer, name unsafe.Pointer) (int32, error) {
	if err := ensureModMediaDesc(); err != nil {
		return 0, err
	}
	if ret := fAvChannelLayoutIndexFromString(channel_layout, name); ret < 0 {
		return 0, codeErr("av_channel_layout_index_from_string", ret)
	} else {
		return ret, nil
	}
}

// ChannelLayoutRetype 换声道布局的组织方式（对 av_channel_layout_retype；参数 channel_layout(有效布局指针, 原地改)、order(AVChannelOrder 枚举数: 0=UNSPEC/1=NATIVE/2=CUSTOM/3=AMBISONIC)、flags(0 或 1=LOSSLESS/2=CANONICAL)；
// 回 (0=无损转成, >0=有损但转成, error 非 nil=转不成)；无状态，可用零值直接调）.
func (x *MediaDesc) ChannelLayoutRetype(channel_layout unsafe.Pointer, order int32, flags int32) (int32, error) {
	if err := ensureModMediaDesc(); err != nil {
		return 0, err
	}
	if ret := fAvChannelLayoutRetype(channel_layout, order, flags); ret < 0 {
		return 0, codeErr("av_channel_layout_retype", ret)
	} else {
		return ret, nil
	}
}

// ChannelLayoutStandard 查或配声道布局（对 av_channel_layout_standard；参数 opaque；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutStandard(opaque *unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvChannelLayoutStandard(opaque)
}

// ChannelLayoutSubset 查或配声道布局（对 av_channel_layout_subset；参数 channel_layout、mask；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutSubset(channel_layout unsafe.Pointer, mask uint64) uint64 {
	mustUse(ensureModMediaDesc())
	return fAvChannelLayoutSubset(channel_layout, mask)
}

// ChannelLayoutUninit 查或配声道布局（对 av_channel_layout_uninit；参数 channel_layout；按签名取回值；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutUninit(channel_layout unsafe.Pointer) {
	mustUse(ensureModMediaDesc())
	fAvChannelLayoutUninit(channel_layout)
}

// ChannelName 查单个声道短名（对 av_channel_name；参数 buf(可写内存)、buf_size、channel(AVChannel 枚举数, 如 0=FL)；
// 回需字节数, 负数是 AVERROR；buf 不可传 nil；无状态，可用零值直接调）.
func (x *MediaDesc) ChannelName(buf unsafe.Pointer, buf_size uintptr, channel int32) int32 {
	mustUse(ensureModMediaDesc())
	return fAvChannelName(buf, buf_size, channel)
}

// ChannelNameBprint 查单个声道短名（对 av_channel_name_bprint；参数 bp(须是 NewBPrint 建的, 不可传 nil)、channel_id(AVChannel 枚举数)；往 bp 里追加，无返回值）.
func (x *MediaDesc) ChannelNameBprint(bp unsafe.Pointer, channel_id int32) {
	mustUse(ensureModMediaDesc())
	fAvChannelNameBprint(bp, channel_id)
}

// ColorPrimariesFromName 颜色主键和范围查名字或取值（对 av_color_primaries_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ColorPrimariesFromName(name unsafe.Pointer) int32 {
	mustUse(ensureModMediaDesc())
	return fAvColorPrimariesFromName(name)
}

// ColorPrimariesName 颜色主键和范围查名字或取值（对 av_color_primaries_name；参数 primaries；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ColorPrimariesName(primaries int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvColorPrimariesName(primaries)
}

// ColorRangeFromName 颜色主键和范围查名字或取值（对 av_color_range_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ColorRangeFromName(name string) int32 {
	mustUse(ensureModMediaDesc())
	return fAvColorRangeFromName(name)
}

// ColorRangeName 颜色主键和范围查名字或取值（对 av_color_range_name；参数 rng；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ColorRangeName(rng int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvColorRangeName(rng)
}

// ColorSpaceFromName 颜色主键和范围查名字或取值（对 av_color_space_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ColorSpaceFromName(name string) int32 {
	mustUse(ensureModMediaDesc())
	return fAvColorSpaceFromName(name)
}

// ColorSpaceName 颜色主键和范围查名字或取值（对 av_color_space_name；参数 space；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ColorSpaceName(space int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvColorSpaceName(space)
}

// ColorTransferFromName 颜色主键和范围查名字或取值（对 av_color_transfer_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ColorTransferFromName(name unsafe.Pointer) int32 {
	mustUse(ensureModMediaDesc())
	return fAvColorTransferFromName(name)
}

// ColorTransferName 按编号查传输函数名（对 av_color_transfer_name；参数 transfer(AVColorTransferCharacteristic 枚举数, 如 1=bt709)；
// 回 C 字符串指针, 编号越界回 nil；无状态，可用零值直接调）.
func (x *MediaDesc) ColorTransferName(transfer int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvColorTransferName(transfer)
}

// ContentLightMetadataAlloc HDR 内容亮度元数据操作（对 av_content_light_metadata_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ContentLightMetadataAlloc(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvContentLightMetadataAlloc(size)
}

// ContentLightMetadataCreateSideData HDR 内容亮度元数据操作（对 av_content_light_metadata_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ContentLightMetadataCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvContentLightMetadataCreateSideData(frame)
}

// DisplayMatrixFlip 原地翻转显示矩阵（对 av_display_matrix_flip；参数 matrix(须是 9 个 int32 共 36 字节的可写内存, 不可传 nil)、hflip、vflip(0/1)；无返回值）.
func (x *MediaDesc) DisplayMatrixFlip(matrix unsafe.Pointer, hflip int32, vflip int32) {
	mustUse(ensureModMediaDesc())
	fAvDisplayMatrixFlip(matrix, hflip, vflip)
}

// DisplayRotationGet 从显示矩阵读旋转角（对 av_display_rotation_get；参数 matrix(须是 9 个 int32 共 36 字节的有效矩阵, 不可传 nil)；
// 回角度 float64(-180 到 180), 矩阵奇异回 NaN；无状态，可用零值直接调）.
func (x *MediaDesc) DisplayRotationGet(matrix unsafe.Pointer) float64 {
	mustUse(ensureModMediaDesc())
	return fAvDisplayRotationGet(matrix)
}

// DisplayRotationSet 往显示矩阵写旋转角（对 av_display_rotation_set；参数 matrix(须是 9 个 int32 共 36 字节的可写内存, 不可传 nil)、angle(顺时针度数, Get 读出来是负的逆时针角)；整个矩阵会被重写）.
func (x *MediaDesc) DisplayRotationSet(matrix unsafe.Pointer, angle float64) {
	mustUse(ensureModMediaDesc())
	fAvDisplayRotationSet(matrix, angle)
}

// DoviAlloc 杜比视界元数据操作（对 av_dovi_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) DoviAlloc(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvDoviAlloc(size)
}

// DoviFindLevel 按 level 找杜比视界扩展块（对 av_dovi_find_level；参数 data(须是 DoviMetadataAlloc 建的, 不可传 nil)、level(0-255 的数值)；
// 回扩展块指针, 找不到回 nil；无状态，可用零值直接调）.
func (x *MediaDesc) DoviFindLevel(data unsafe.Pointer, level uint8) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvDoviFindLevel(data, level)
}

// DoviMetadataAlloc 杜比视界元数据操作（对 av_dovi_metadata_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) DoviMetadataAlloc(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvDoviMetadataAlloc(size)
}

// DynamicHdrPlusAlloc HDR10+ 动态元数据操作（对 av_dynamic_hdr_plus_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) DynamicHdrPlusAlloc(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvDynamicHdrPlusAlloc(size)
}

// DynamicHdrPlusCreateSideData HDR10+ 动态元数据操作（对 av_dynamic_hdr_plus_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) DynamicHdrPlusCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvDynamicHdrPlusCreateSideData(frame)
}

// DynamicHdrPlusFromT35 HDR10+ 动态元数据操作（对 av_dynamic_hdr_plus_from_t35；参数 s、data、size；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) DynamicHdrPlusFromT35(s unsafe.Pointer, data unsafe.Pointer, size uintptr) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvDynamicHdrPlusFromT35(s, data, size); ret < 0 {
		return codeErr("av_dynamic_hdr_plus_from_t35", ret)
	}
	return nil
}

// DynamicHdrPlusToT35 把 HDR10+ 元数据压成 T.35 字节串（对 av_dynamic_hdr_plus_to_t35；参数 s(须是 DynamicHdrPlusAlloc 建的、分数分母填好的有效结构, 不可传 nil；全零结构体分母是 0, C 里做除法会崩, 别直接调)、data(指向字节串指针的槽, 传 *unsafe.Pointer, *data 传 nil 表让 C 分配)、size(指向字节数的槽)；
// 回 (>=0=字节数, error), data 和 size 槽不可传 nil；C 分配的字节串记得 Mem.Free；无状态，可用零值直接调）.
func (x *MediaDesc) DynamicHdrPlusToT35(s unsafe.Pointer, data *unsafe.Pointer, size unsafe.Pointer) (int32, error) {
	if err := ensureModMediaDesc(); err != nil {
		return 0, err
	}
	if ret := fAvDynamicHdrPlusToT35(s, data, size); ret < 0 {
		return 0, codeErr("av_dynamic_hdr_plus_to_t35", ret)
	} else {
		return ret, nil
	}
}

// FilmGrainParamsAlloc 胶片颗粒参数操作（对 av_film_grain_params_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) FilmGrainParamsAlloc(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvFilmGrainParamsAlloc(size)
}

// FilmGrainParamsCreateSideData 胶片颗粒参数操作（对 av_film_grain_params_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) FilmGrainParamsCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvFilmGrainParamsCreateSideData(frame)
}

// FilmGrainParamsSelect 从帧里挑一套胶片颗粒参数（对 av_film_grain_params_select；参数 frame(须是真帧指针, 不可传 nil；空白帧没像素格式会诚实回 nil, 真解出来的帧才选得出)；
// 回参数指针, 挑不出回 nil；无状态，可用零值直接调）.
func (x *MediaDesc) FilmGrainParamsSelect(frame unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvFilmGrainParamsSelect(frame)
}

// GetBitsPerPixel 问像素格式平均每像素占几位（对 av_get_bits_per_pixel；参数 pixdesc(须是 PixFmtDescGet 回的有效描述指针, 不可传 nil, 传 nil 会崩)；
// 回位数 int32；yuv420p=12, rgb24=24；无状态，可用零值直接调）.
func (x *MediaDesc) GetBitsPerPixel(pixdesc unsafe.Pointer) int32 {
	mustUse(ensureModMediaDesc())
	return fAvGetBitsPerPixel(pixdesc)
}

// GetPixFmt 按名字找像素格式编号（对 av_get_pix_fmt；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) GetPixFmt(name string) int32 {
	mustUse(ensureModMediaDesc())
	return fAvGetPixFmt(name)
}

// GetPixFmtLoss 按名字找像素格式编号（对 av_get_pix_fmt_loss；参数 dst_pix_fmt、src_pix_fmt、has_alpha；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) GetPixFmtLoss(dst_pix_fmt int32, src_pix_fmt int32, has_alpha int32) int32 {
	mustUse(ensureModMediaDesc())
	return fAvGetPixFmtLoss(dst_pix_fmt, src_pix_fmt, has_alpha)
}

// GetPixFmtName 按名字找像素格式编号（对 av_get_pix_fmt_name；参数 pix_fmt；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) GetPixFmtName(pix_fmt int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvGetPixFmtName(pix_fmt)
}

// GetPixFmtString 按名字找像素格式编号（对 av_get_pix_fmt_string；参数 buf、buf_size、pix_fmt；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) GetPixFmtString(buf unsafe.Pointer, buf_size int32, pix_fmt int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvGetPixFmtString(buf, buf_size, pix_fmt)
}

// MasteringDisplayMetadataAlloc HDR 主显示元数据操作（对 av_mastering_display_metadata_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) MasteringDisplayMetadataAlloc() unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvMasteringDisplayMetadataAlloc()
}

// MasteringDisplayMetadataAllocSize HDR 主显示元数据操作（对 av_mastering_display_metadata_alloc_size；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) MasteringDisplayMetadataAllocSize(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvMasteringDisplayMetadataAllocSize(size)
}

// MasteringDisplayMetadataCreateSideData HDR 主显示元数据操作（对 av_mastering_display_metadata_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) MasteringDisplayMetadataCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvMasteringDisplayMetadataCreateSideData(frame)
}

// ParseColor 把颜色字符串解析成数值（对 av_parse_color；参数 rgba_color、color_string、slen、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ParseColor(rgba_color unsafe.Pointer, color_string unsafe.Pointer, slen int32, log_ctx unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvParseColor(rgba_color, color_string, slen, log_ctx); ret < 0 {
		return codeErr("av_parse_color", ret)
	}
	return nil
}

// ParseRatio 把分数的字符串解析成分子分母（对 av_parse_ratio；参数 q(指向 AVRational 共 8 字节的可写内存, 不可传 nil)、str(C 字符串指针)、max(分子分母上限, 传 0 会压成 0/1, 一般传 1001000)、log_offset、log_ctx(传 nil 关日志)；
// 回 nil 表成功, 失败回 error；q 槽不可传 nil；无状态，可用零值直接调）.
func (x *MediaDesc) ParseRatio(q unsafe.Pointer, str unsafe.Pointer, max int32, log_offset int32, log_ctx unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvParseRatio(q, str, max, log_offset, log_ctx); ret < 0 {
		return codeErr("av_parse_ratio", ret)
	}
	return nil
}

// ParseTime 把时间字符串解析成微秒（对 av_parse_time；参数 timeval、timestr、duration；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ParseTime(timeval unsafe.Pointer, timestr unsafe.Pointer, duration int32) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvParseTime(timeval, timestr, duration); ret < 0 {
		return codeErr("av_parse_time", ret)
	}
	return nil
}

// ParseVideoRate 把帧率字符串解析成分数（对 av_parse_video_rate；参数 rate、str；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ParseVideoRate(rate unsafe.Pointer, str unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvParseVideoRate(rate, str); ret < 0 {
		return codeErr("av_parse_video_rate", ret)
	}
	return nil
}

// ParseVideoSize 把宽高字符串解析成数字（对 av_parse_video_size；参数 width_ptr、height_ptr(各指向 int32 的可写内存, 不可传 nil)、str(C 字符串指针, 如 "640x480")；
// 回 nil 表成功, 失败回 error；无状态，可用零值直接调）.
func (x *MediaDesc) ParseVideoSize(width_ptr unsafe.Pointer, height_ptr unsafe.Pointer, str unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvParseVideoSize(width_ptr, height_ptr, str); ret < 0 {
		return codeErr("av_parse_video_size", ret)
	}
	return nil
}

// PixFmtDescGet 像素格式查名字查属性（对 av_pix_fmt_desc_get；参数 pix_fmt；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) PixFmtDescGet(pix_fmt int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvPixFmtDescGet(pix_fmt)
}

// PixFmtDescGetId 像素格式查名字查属性（对 av_pix_fmt_desc_get_id；参数 desc；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) PixFmtDescGetId(desc unsafe.Pointer) int32 {
	mustUse(ensureModMediaDesc())
	return fAvPixFmtDescGetId(desc)
}

// PixFmtDescNext 像素格式查名字查属性（对 av_pix_fmt_desc_next；参数 prev；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) PixFmtDescNext(prev unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvPixFmtDescNext(prev)
}

// SphericalAlloc 全景球面元数据操作（对 av_spherical_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) SphericalAlloc(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvSphericalAlloc(size)
}

// SphericalFromName 全景球面元数据操作（对 av_spherical_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) SphericalFromName(name unsafe.Pointer) int32 {
	mustUse(ensureModMediaDesc())
	return fAvSphericalFromName(name)
}

// SphericalProjectionName 按编号查全景投影名（对 av_spherical_projection_name；参数 projection(枚举数: 0=equirectangular/1=cubemap/2=tiled/3=half/4=rectilinear/5=fisheye)；
// 回 C 字符串指针, 越界回 "unknown" 不回 nil；无状态，可用零值直接调）.
func (x *MediaDesc) SphericalProjectionName(projection int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvSphericalProjectionName(projection)
}

// SphericalTileBounds 全景球面元数据操作（对 av_spherical_tile_bounds；参数 mp、width、height、left、top、right、bottom；按签名取回值；无状态，可用零值直接调）。
func (x *MediaDesc) SphericalTileBounds(mp unsafe.Pointer, width uintptr, height uintptr, left unsafe.Pointer, top unsafe.Pointer, right unsafe.Pointer, bottom unsafe.Pointer) {
	mustUse(ensureModMediaDesc())
	fAvSphericalTileBounds(mp, width, height, left, top, right, bottom)
}

// Stereo3dAlloc 立体视频元数据操作（对 av_stereo3d_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dAlloc() unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvStereo3dAlloc()
}

// Stereo3dAllocSize 立体视频元数据操作（对 av_stereo3d_alloc_size；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dAllocSize(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvStereo3dAllocSize(size)
}

// Stereo3dCreateSideData 立体视频元数据操作（对 av_stereo3d_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvStereo3dCreateSideData(frame)
}

// Stereo3dFromName 立体视频元数据操作（对 av_stereo3d_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dFromName(name unsafe.Pointer) int32 {
	mustUse(ensureModMediaDesc())
	return fAvStereo3dFromName(name)
}

// Stereo3dPrimaryEyeFromName 立体视频元数据操作（对 av_stereo3d_primary_eye_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dPrimaryEyeFromName(name unsafe.Pointer) int32 {
	mustUse(ensureModMediaDesc())
	return fAvStereo3dPrimaryEyeFromName(name)
}

// Stereo3dPrimaryEyeName 立体视频元数据操作（对 av_stereo3d_primary_eye_name；参数 eye；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dPrimaryEyeName(eye uint32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvStereo3dPrimaryEyeName(eye)
}

// Stereo3dTypeName 立体视频元数据操作（对 av_stereo3d_type_name；参数 typ；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dTypeName(typ uint32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvStereo3dTypeName(typ)
}

// Stereo3dViewFromName 立体视频元数据操作（对 av_stereo3d_view_from_name；参数 name；回数值；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dViewFromName(name unsafe.Pointer) int32 {
	mustUse(ensureModMediaDesc())
	return fAvStereo3dViewFromName(name)
}

// Stereo3dViewName 立体视频元数据操作（对 av_stereo3d_view_name；参数 view；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dViewName(view uint32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvStereo3dViewName(view)
}

// TimecodeAdjustNtscFramenum2 把 NTSC 丢帧时间码的帧号调准（对 av_timecode_adjust_ntsc_framenum2；参数 framenum、fps(须是 30 的倍数, 不是的话原样返回)；
// 回调准后的帧号 int32, 纯算术不报错；无状态，可用零值直接调）.
func (x *MediaDesc) TimecodeAdjustNtscFramenum2(framenum int32, fps int32) int32 {
	mustUse(ensureModMediaDesc())
	return fAvTimecodeAdjustNtscFramenum2(framenum, fps)
}

// TimecodeCheckFrameRate 时间码初始化换算和拼串（对 av_timecode_check_frame_rate；参数 rate；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeCheckFrameRate(rate AVRational) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvTimecodeCheckFrameRate(rate); ret < 0 {
		return codeErr("av_timecode_check_frame_rate", ret)
	}
	return nil
}

// TimecodeGetSmpte 时间码初始化换算和拼串（对 av_timecode_get_smpte；参数 rate、drop、hh、mm、ss、ff；回数值；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeGetSmpte(rate AVRational, drop int32, hh int32, mm int32, ss int32, ff int32) uint32 {
	mustUse(ensureModMediaDesc())
	return fAvTimecodeGetSmpte(rate, drop, hh, mm, ss, ff)
}

// TimecodeGetSmpteFromFramenum 时间码初始化换算和拼串（对 av_timecode_get_smpte_from_framenum；参数 tc、framenum；回数值；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeGetSmpteFromFramenum(tc unsafe.Pointer, framenum int32) uint32 {
	mustUse(ensureModMediaDesc())
	return fAvTimecodeGetSmpteFromFramenum(tc, framenum)
}

// TimecodeInit 时间码初始化换算和拼串（对 av_timecode_init；参数 tc、rate、flags、frame_start、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeInit(tc unsafe.Pointer, rate AVRational, flags int32, frame_start int32, log_ctx unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvTimecodeInit(tc, rate, flags, frame_start, log_ctx); ret < 0 {
		return codeErr("av_timecode_init", ret)
	}
	return nil
}

// TimecodeInitFromComponents 时间码初始化换算和拼串（对 av_timecode_init_from_components；参数 tc、rate、flags、hh、mm、ss、ff、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeInitFromComponents(tc unsafe.Pointer, rate AVRational, flags int32, hh int32, mm int32, ss int32, ff int32, log_ctx unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvTimecodeInitFromComponents(tc, rate, flags, hh, mm, ss, ff, log_ctx); ret < 0 {
		return codeErr("av_timecode_init_from_components", ret)
	}
	return nil
}

// TimecodeInitFromString 时间码初始化换算和拼串（对 av_timecode_init_from_string；参数 tc、rate、str、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeInitFromString(tc unsafe.Pointer, rate AVRational, str unsafe.Pointer, log_ctx unsafe.Pointer) error {
	if err := ensureModMediaDesc(); err != nil {
		return err
	}
	if ret := fAvTimecodeInitFromString(tc, rate, str, log_ctx); ret < 0 {
		return codeErr("av_timecode_init_from_string", ret)
	}
	return nil
}

// TimecodeMakeMpegTcString 时间码初始化换算和拼串（对 av_timecode_make_mpeg_tc_string；参数 buf、tc25bit；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeMakeMpegTcString(buf unsafe.Pointer, tc25bit uint32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvTimecodeMakeMpegTcString(buf, tc25bit)
}

// TimecodeMakeSmpteTcString 时间码初始化换算和拼串（对 av_timecode_make_smpte_tc_string；参数 buf、tcsmpte、prevent_df；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeMakeSmpteTcString(buf unsafe.Pointer, tcsmpte uint32, prevent_df int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvTimecodeMakeSmpteTcString(buf, tcsmpte, prevent_df)
}

// TimecodeMakeSmpteTcString2 时间码初始化换算和拼串（对 av_timecode_make_smpte_tc_string2；参数 buf、rate、tcsmpte、prevent_df、skip_field；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeMakeSmpteTcString2(buf unsafe.Pointer, rate AVRational, tcsmpte uint32, prevent_df int32, skip_field int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvTimecodeMakeSmpteTcString2(buf, rate, tcsmpte, prevent_df, skip_field)
}

// TimecodeMakeString 时间码初始化换算和拼串（对 av_timecode_make_string；参数 tc、buf、framenum；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeMakeString(tc unsafe.Pointer, buf unsafe.Pointer, framenum int32) unsafe.Pointer {
	mustUse(ensureModMediaDesc())
	return fAvTimecodeMakeString(tc, buf, framenum)
}
