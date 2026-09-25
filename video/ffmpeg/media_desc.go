package ffmpeg

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// MediaDesc 媒体描述模块: 像素格式/声道/颜色/元数据查询全量导出.
//
// Say it plain: 问格式叫啥、多大、支不支持, 全是只读查询, 不动数据.

// MediaDesc holder.
type MediaDesc struct{ ptr unsafe.Pointer }

func (x *MediaDesc) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

var (
	fAvAmbientViewingEnvironmentAlloc          func(size unsafe.Pointer) unsafe.Pointer
	fAvAmbientViewingEnvironmentCreateSideData func(frame unsafe.Pointer) unsafe.Pointer
	fAvChannelDescription                      func(buf unsafe.Pointer, buf_size uintptr, channel unsafe.Pointer) int32
	fAvChannelDescriptionBprint                func(bp unsafe.Pointer, channel_id unsafe.Pointer)
	fAvChannelFromString                       func(name unsafe.Pointer) unsafe.Pointer
	fAvChannelLayoutAmbisonicOrder             func(channel_layout unsafe.Pointer) int32
	fAvChannelLayoutChannelFromIndex           func(channel_layout unsafe.Pointer, idx uint32) unsafe.Pointer
	fAvChannelLayoutChannelFromString          func(channel_layout unsafe.Pointer, name unsafe.Pointer) unsafe.Pointer
	fAvChannelLayoutCheck                      func(channel_layout unsafe.Pointer) int32
	fAvChannelLayoutCompare                    func(chl unsafe.Pointer, chl1 unsafe.Pointer) int32
	fAvChannelLayoutCopy                       func(dst unsafe.Pointer, src unsafe.Pointer) int32
	fAvChannelLayoutCustomInit                 func(channel_layout unsafe.Pointer, nb_channels int32) int32
	fAvChannelLayoutDefault                    func(ch_layout unsafe.Pointer, nb_channels int32)
	fAvChannelLayoutDescribe                   func(channel_layout unsafe.Pointer, buf unsafe.Pointer, buf_size uintptr) int32
	fAvChannelLayoutDescribeBprint             func(channel_layout unsafe.Pointer, bp unsafe.Pointer) int32
	fAvChannelLayoutFromMask                   func(channel_layout unsafe.Pointer, mask uint64) int32
	fAvChannelLayoutFromString                 func(channel_layout unsafe.Pointer, str unsafe.Pointer) int32
	fAvChannelLayoutIndexFromChannel           func(channel_layout unsafe.Pointer, channel unsafe.Pointer) int32
	fAvChannelLayoutIndexFromString            func(channel_layout unsafe.Pointer, name unsafe.Pointer) int32
	fAvChannelLayoutRetype                     func(channel_layout unsafe.Pointer, order unsafe.Pointer, flags int32) unsafe.Pointer
	fAvChannelLayoutStandard                   func(opaque *unsafe.Pointer) unsafe.Pointer
	fAvChannelLayoutSubset                     func(channel_layout unsafe.Pointer, mask uint64) uint64
	fAvChannelLayoutUninit                     func(channel_layout unsafe.Pointer)
	fAvChannelName                             func(buf unsafe.Pointer, buf_size uintptr, channel unsafe.Pointer) int32
	fAvChannelNameBprint                       func(bp unsafe.Pointer, channel_id unsafe.Pointer)
	fAvColorPrimariesFromName                  func(name unsafe.Pointer) int32
	fAvColorPrimariesName                      func(primaries int32) unsafe.Pointer
	fAvColorRangeFromName                      func(name string) int32
	fAvColorRangeName                          func(rng int32) unsafe.Pointer
	fAvColorSpaceFromName                      func(name string) int32
	fAvColorSpaceName                          func(space int32) unsafe.Pointer
	fAvColorTransferFromName                   func(name unsafe.Pointer) int32
	fAvColorTransferName                       func(transfer unsafe.Pointer) unsafe.Pointer
	fAvContentLightMetadataAlloc               func(size unsafe.Pointer) unsafe.Pointer
	fAvContentLightMetadataCreateSideData      func(frame unsafe.Pointer) unsafe.Pointer
	fAvDisplayMatrixFlip                       func(matrix int32, hflip int32, vflip int32)
	fAvDisplayRotationGet                      func(matrix int32) unsafe.Pointer
	fAvDisplayRotationSet                      func(matrix int32, angle float64)
	fAvDoviAlloc                               func(size unsafe.Pointer) unsafe.Pointer
	fAvDoviFindLevel                           func(data unsafe.Pointer, level unsafe.Pointer) unsafe.Pointer
	fAvDoviMetadataAlloc                       func(size unsafe.Pointer) unsafe.Pointer
	fAvDynamicHdrPlusAlloc                     func(size unsafe.Pointer) unsafe.Pointer
	fAvDynamicHdrPlusCreateSideData            func(frame unsafe.Pointer) unsafe.Pointer
	fAvDynamicHdrPlusFromT35                   func(s unsafe.Pointer, data unsafe.Pointer, size uintptr) int32
	fAvDynamicHdrPlusToT35                     func(s unsafe.Pointer, data *unsafe.Pointer, size unsafe.Pointer) unsafe.Pointer
	fAvFilmGrainParamsAlloc                    func(size unsafe.Pointer) unsafe.Pointer
	fAvFilmGrainParamsCreateSideData           func(frame unsafe.Pointer) unsafe.Pointer
	fAvFilmGrainParamsSelect                   func(frame unsafe.Pointer) unsafe.Pointer
	fAvGetBitsPerPixel                         func(pixdesc unsafe.Pointer) unsafe.Pointer
	fAvGetPixFmt                               func(name string) int32
	fAvGetPixFmtLoss                           func(dst_pix_fmt int32, src_pix_fmt int32, has_alpha int32) int32
	fAvGetPixFmtName                           func(pix_fmt int32) unsafe.Pointer
	fAvGetPixFmtString                         func(buf unsafe.Pointer, buf_size int32, pix_fmt int32) unsafe.Pointer
	fAvMasteringDisplayMetadataAlloc           func() unsafe.Pointer
	fAvMasteringDisplayMetadataAllocSize       func(size unsafe.Pointer) unsafe.Pointer
	fAvMasteringDisplayMetadataCreateSideData  func(frame unsafe.Pointer) unsafe.Pointer
	fAvParseColor                              func(rgba_color unsafe.Pointer, color_string unsafe.Pointer, slen int32, log_ctx unsafe.Pointer) int32
	fAvParseRatio                              func(q unsafe.Pointer, str unsafe.Pointer, max int32, log_offset int32, log_ctx unsafe.Pointer) unsafe.Pointer
	fAvParseTime                               func(timeval unsafe.Pointer, timestr unsafe.Pointer, duration int32) int32
	fAvParseVideoRate                          func(rate unsafe.Pointer, str unsafe.Pointer) int32
	fAvParseVideoSize                          func(width_ptr unsafe.Pointer, height_ptr unsafe.Pointer, str unsafe.Pointer) unsafe.Pointer
	fAvPixFmtDescGet                           func(pix_fmt int32) unsafe.Pointer
	fAvPixFmtDescGetId                         func(desc unsafe.Pointer) int32
	fAvPixFmtDescNext                          func(prev unsafe.Pointer) unsafe.Pointer
	fAvSphericalAlloc                          func(size unsafe.Pointer) unsafe.Pointer
	fAvSphericalFromName                       func(name unsafe.Pointer) int32
	fAvSphericalProjectionName                 func(projection unsafe.Pointer) unsafe.Pointer
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
	return fAvAmbientViewingEnvironmentAlloc(size)
}

// AmbientViewingEnvironmentCreateSideData 环境光元数据操作（对 av_ambient_viewing_environment_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) AmbientViewingEnvironmentCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvAmbientViewingEnvironmentCreateSideData(frame)
}

// ChannelDescription 查单个声道名或描述（对 av_channel_description；参数 buf、buf_size、channel；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelDescription(buf unsafe.Pointer, buf_size uintptr, channel unsafe.Pointer) error {
	if ret := fAvChannelDescription(buf, buf_size, channel); ret < 0 {
		return codeErr("av_channel_description", ret)
	}
	return nil
}

// ChannelDescriptionBprint 查单个声道名或描述（对 av_channel_description_bprint；参数 bp、channel_id；按签名取回值；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelDescriptionBprint(bp unsafe.Pointer, channel_id unsafe.Pointer) {
	fAvChannelDescriptionBprint(bp, channel_id)
}

// ChannelFromString 查单个声道名或描述（对 av_channel_from_string；参数 name；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelFromString(name unsafe.Pointer) unsafe.Pointer {
	return fAvChannelFromString(name)
}

// ChannelLayoutAmbisonicOrder 查或配声道布局（对 av_channel_layout_ambisonic_order；参数 channel_layout；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutAmbisonicOrder(channel_layout unsafe.Pointer) error {
	if ret := fAvChannelLayoutAmbisonicOrder(channel_layout); ret < 0 {
		return codeErr("av_channel_layout_ambisonic_order", ret)
	}
	return nil
}

// ChannelLayoutChannelFromIndex 查或配声道布局（对 av_channel_layout_channel_from_index；参数 channel_layout、idx；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutChannelFromIndex(channel_layout unsafe.Pointer, idx uint32) unsafe.Pointer {
	return fAvChannelLayoutChannelFromIndex(channel_layout, idx)
}

// ChannelLayoutChannelFromString 查或配声道布局（对 av_channel_layout_channel_from_string；参数 channel_layout、name；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutChannelFromString(channel_layout unsafe.Pointer, name unsafe.Pointer) unsafe.Pointer {
	return fAvChannelLayoutChannelFromString(channel_layout, name)
}

// ChannelLayoutCheck 查或配声道布局（对 av_channel_layout_check；参数 channel_layout；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutCheck(channel_layout unsafe.Pointer) error {
	if ret := fAvChannelLayoutCheck(channel_layout); ret < 0 {
		return codeErr("av_channel_layout_check", ret)
	}
	return nil
}

// ChannelLayoutCompare 查或配声道布局（对 av_channel_layout_compare；参数 chl、chl1；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutCompare(chl unsafe.Pointer, chl1 unsafe.Pointer) error {
	if ret := fAvChannelLayoutCompare(chl, chl1); ret < 0 {
		return codeErr("av_channel_layout_compare", ret)
	}
	return nil
}

// ChannelLayoutCopy 查或配声道布局（对 av_channel_layout_copy；参数 dst、src；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutCopy(dst unsafe.Pointer, src unsafe.Pointer) error {
	if ret := fAvChannelLayoutCopy(dst, src); ret < 0 {
		return codeErr("av_channel_layout_copy", ret)
	}
	return nil
}

// ChannelLayoutCustomInit 查或配声道布局（对 av_channel_layout_custom_init；参数 channel_layout、nb_channels；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutCustomInit(channel_layout unsafe.Pointer, nb_channels int32) error {
	if ret := fAvChannelLayoutCustomInit(channel_layout, nb_channels); ret < 0 {
		return codeErr("av_channel_layout_custom_init", ret)
	}
	return nil
}

// ChannelLayoutDefault 查或配声道布局（对 av_channel_layout_default；参数 ch_layout、nb_channels；按签名取回值；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutDefault(ch_layout unsafe.Pointer, nb_channels int32) {
	fAvChannelLayoutDefault(ch_layout, nb_channels)
}

// ChannelLayoutDescribe 查或配声道布局（对 av_channel_layout_describe；参数 channel_layout、buf、buf_size；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutDescribe(channel_layout unsafe.Pointer, buf unsafe.Pointer, buf_size uintptr) error {
	if ret := fAvChannelLayoutDescribe(channel_layout, buf, buf_size); ret < 0 {
		return codeErr("av_channel_layout_describe", ret)
	}
	return nil
}

// ChannelLayoutDescribeBprint 查或配声道布局（对 av_channel_layout_describe_bprint；参数 channel_layout、bp；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutDescribeBprint(channel_layout unsafe.Pointer, bp unsafe.Pointer) error {
	if ret := fAvChannelLayoutDescribeBprint(channel_layout, bp); ret < 0 {
		return codeErr("av_channel_layout_describe_bprint", ret)
	}
	return nil
}

// ChannelLayoutFromMask 查或配声道布局（对 av_channel_layout_from_mask；参数 channel_layout、mask；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutFromMask(channel_layout unsafe.Pointer, mask uint64) error {
	if ret := fAvChannelLayoutFromMask(channel_layout, mask); ret < 0 {
		return codeErr("av_channel_layout_from_mask", ret)
	}
	return nil
}

// ChannelLayoutFromString 查或配声道布局（对 av_channel_layout_from_string；参数 channel_layout、str；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutFromString(channel_layout unsafe.Pointer, str unsafe.Pointer) error {
	if ret := fAvChannelLayoutFromString(channel_layout, str); ret < 0 {
		return codeErr("av_channel_layout_from_string", ret)
	}
	return nil
}

// ChannelLayoutIndexFromChannel 查或配声道布局（对 av_channel_layout_index_from_channel；参数 channel_layout、channel；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutIndexFromChannel(channel_layout unsafe.Pointer, channel unsafe.Pointer) error {
	if ret := fAvChannelLayoutIndexFromChannel(channel_layout, channel); ret < 0 {
		return codeErr("av_channel_layout_index_from_channel", ret)
	}
	return nil
}

// ChannelLayoutIndexFromString 查或配声道布局（对 av_channel_layout_index_from_string；参数 channel_layout、name；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutIndexFromString(channel_layout unsafe.Pointer, name unsafe.Pointer) error {
	if ret := fAvChannelLayoutIndexFromString(channel_layout, name); ret < 0 {
		return codeErr("av_channel_layout_index_from_string", ret)
	}
	return nil
}

// ChannelLayoutRetype 查或配声道布局（对 av_channel_layout_retype；参数 channel_layout、order、flags；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutRetype(channel_layout unsafe.Pointer, order unsafe.Pointer, flags int32) unsafe.Pointer {
	return fAvChannelLayoutRetype(channel_layout, order, flags)
}

// ChannelLayoutStandard 查或配声道布局（对 av_channel_layout_standard；参数 opaque；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutStandard(opaque *unsafe.Pointer) unsafe.Pointer {
	return fAvChannelLayoutStandard(opaque)
}

// ChannelLayoutSubset 查或配声道布局（对 av_channel_layout_subset；参数 channel_layout、mask；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutSubset(channel_layout unsafe.Pointer, mask uint64) uint64 {
	return fAvChannelLayoutSubset(channel_layout, mask)
}

// ChannelLayoutUninit 查或配声道布局（对 av_channel_layout_uninit；参数 channel_layout；按签名取回值；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelLayoutUninit(channel_layout unsafe.Pointer) {
	fAvChannelLayoutUninit(channel_layout)
}

// ChannelName 查单个声道名或描述（对 av_channel_name；参数 buf、buf_size、channel；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelName(buf unsafe.Pointer, buf_size uintptr, channel unsafe.Pointer) int32 {
	return fAvChannelName(buf, buf_size, channel)
}

// ChannelNameBprint 查单个声道名或描述（对 av_channel_name_bprint；参数 bp、channel_id；按签名取回值；无状态，可用零值直接调）。
func (x *MediaDesc) ChannelNameBprint(bp unsafe.Pointer, channel_id unsafe.Pointer) {
	fAvChannelNameBprint(bp, channel_id)
}

// ColorPrimariesFromName 颜色主键和范围查名字或取值（对 av_color_primaries_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ColorPrimariesFromName(name unsafe.Pointer) int32 {
	return fAvColorPrimariesFromName(name)
}

// ColorPrimariesName 颜色主键和范围查名字或取值（对 av_color_primaries_name；参数 primaries；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ColorPrimariesName(primaries int32) unsafe.Pointer {
	return fAvColorPrimariesName(primaries)
}

// ColorRangeFromName 颜色主键和范围查名字或取值（对 av_color_range_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ColorRangeFromName(name string) int32 {
	return fAvColorRangeFromName(name)
}

// ColorRangeName 颜色主键和范围查名字或取值（对 av_color_range_name；参数 rng；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ColorRangeName(rng int32) unsafe.Pointer {
	return fAvColorRangeName(rng)
}

// ColorSpaceFromName 颜色主键和范围查名字或取值（对 av_color_space_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ColorSpaceFromName(name string) int32 {
	return fAvColorSpaceFromName(name)
}

// ColorSpaceName 颜色主键和范围查名字或取值（对 av_color_space_name；参数 space；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ColorSpaceName(space int32) unsafe.Pointer {
	return fAvColorSpaceName(space)
}

// ColorTransferFromName 颜色主键和范围查名字或取值（对 av_color_transfer_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) ColorTransferFromName(name unsafe.Pointer) int32 {
	return fAvColorTransferFromName(name)
}

// ColorTransferName 颜色主键和范围查名字或取值（对 av_color_transfer_name；参数 transfer；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ColorTransferName(transfer unsafe.Pointer) unsafe.Pointer {
	return fAvColorTransferName(transfer)
}

// ContentLightMetadataAlloc HDR 内容亮度元数据操作（对 av_content_light_metadata_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ContentLightMetadataAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvContentLightMetadataAlloc(size)
}

// ContentLightMetadataCreateSideData HDR 内容亮度元数据操作（对 av_content_light_metadata_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) ContentLightMetadataCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvContentLightMetadataCreateSideData(frame)
}

// DisplayMatrixFlip 显示矩阵翻转或读旋转角（对 av_display_matrix_flip；参数 matrix、hflip、vflip；按签名取回值；无状态，可用零值直接调）。
func (x *MediaDesc) DisplayMatrixFlip(matrix int32, hflip int32, vflip int32) {
	fAvDisplayMatrixFlip(matrix, hflip, vflip)
}

// DisplayRotationGet 显示矩阵翻转或读旋转角（对 av_display_rotation_get；参数 matrix；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) DisplayRotationGet(matrix int32) unsafe.Pointer {
	return fAvDisplayRotationGet(matrix)
}

// DisplayRotationSet 显示矩阵翻转或读旋转角（对 av_display_rotation_set；参数 matrix、angle；按签名取回值；无状态，可用零值直接调）。
func (x *MediaDesc) DisplayRotationSet(matrix int32, angle float64) {
	fAvDisplayRotationSet(matrix, angle)
}

// DoviAlloc 杜比视界元数据操作（对 av_dovi_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) DoviAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvDoviAlloc(size)
}

// DoviFindLevel 杜比视界元数据操作（对 av_dovi_find_level；参数 data、level；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) DoviFindLevel(data unsafe.Pointer, level unsafe.Pointer) unsafe.Pointer {
	return fAvDoviFindLevel(data, level)
}

// DoviMetadataAlloc 杜比视界元数据操作（对 av_dovi_metadata_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) DoviMetadataAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvDoviMetadataAlloc(size)
}

// DynamicHdrPlusAlloc HDR10+ 动态元数据操作（对 av_dynamic_hdr_plus_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) DynamicHdrPlusAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvDynamicHdrPlusAlloc(size)
}

// DynamicHdrPlusCreateSideData HDR10+ 动态元数据操作（对 av_dynamic_hdr_plus_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) DynamicHdrPlusCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvDynamicHdrPlusCreateSideData(frame)
}

// DynamicHdrPlusFromT35 HDR10+ 动态元数据操作（对 av_dynamic_hdr_plus_from_t35；参数 s、data、size；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) DynamicHdrPlusFromT35(s unsafe.Pointer, data unsafe.Pointer, size uintptr) error {
	if ret := fAvDynamicHdrPlusFromT35(s, data, size); ret < 0 {
		return codeErr("av_dynamic_hdr_plus_from_t35", ret)
	}
	return nil
}

// DynamicHdrPlusToT35 HDR10+ 动态元数据操作（对 av_dynamic_hdr_plus_to_t35；参数 s、data、size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) DynamicHdrPlusToT35(s unsafe.Pointer, data *unsafe.Pointer, size unsafe.Pointer) unsafe.Pointer {
	return fAvDynamicHdrPlusToT35(s, data, size)
}

// FilmGrainParamsAlloc 胶片颗粒参数操作（对 av_film_grain_params_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) FilmGrainParamsAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvFilmGrainParamsAlloc(size)
}

// FilmGrainParamsCreateSideData 胶片颗粒参数操作（对 av_film_grain_params_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) FilmGrainParamsCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvFilmGrainParamsCreateSideData(frame)
}

// FilmGrainParamsSelect 胶片颗粒参数操作（对 av_film_grain_params_select；参数 frame；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) FilmGrainParamsSelect(frame unsafe.Pointer) unsafe.Pointer {
	return fAvFilmGrainParamsSelect(frame)
}

// GetBitsPerPixel 问像素格式平均每像素占几位（对 av_get_bits_per_pixel；参数 pixdesc；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) GetBitsPerPixel(pixdesc unsafe.Pointer) unsafe.Pointer {
	return fAvGetBitsPerPixel(pixdesc)
}

// GetPixFmt 按名字找像素格式编号（对 av_get_pix_fmt；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) GetPixFmt(name string) int32 {
	return fAvGetPixFmt(name)
}

// GetPixFmtLoss 按名字找像素格式编号（对 av_get_pix_fmt_loss；参数 dst_pix_fmt、src_pix_fmt、has_alpha；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) GetPixFmtLoss(dst_pix_fmt int32, src_pix_fmt int32, has_alpha int32) int32 {
	return fAvGetPixFmtLoss(dst_pix_fmt, src_pix_fmt, has_alpha)
}

// GetPixFmtName 按名字找像素格式编号（对 av_get_pix_fmt_name；参数 pix_fmt；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) GetPixFmtName(pix_fmt int32) unsafe.Pointer {
	return fAvGetPixFmtName(pix_fmt)
}

// GetPixFmtString 按名字找像素格式编号（对 av_get_pix_fmt_string；参数 buf、buf_size、pix_fmt；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) GetPixFmtString(buf unsafe.Pointer, buf_size int32, pix_fmt int32) unsafe.Pointer {
	return fAvGetPixFmtString(buf, buf_size, pix_fmt)
}

// MasteringDisplayMetadataAlloc HDR 主显示元数据操作（对 av_mastering_display_metadata_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) MasteringDisplayMetadataAlloc() unsafe.Pointer {
	return fAvMasteringDisplayMetadataAlloc()
}

// MasteringDisplayMetadataAllocSize HDR 主显示元数据操作（对 av_mastering_display_metadata_alloc_size；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) MasteringDisplayMetadataAllocSize(size unsafe.Pointer) unsafe.Pointer {
	return fAvMasteringDisplayMetadataAllocSize(size)
}

// MasteringDisplayMetadataCreateSideData HDR 主显示元数据操作（对 av_mastering_display_metadata_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) MasteringDisplayMetadataCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvMasteringDisplayMetadataCreateSideData(frame)
}

// ParseColor 把颜色字符串解析成数值（对 av_parse_color；参数 rgba_color、color_string、slen、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ParseColor(rgba_color unsafe.Pointer, color_string unsafe.Pointer, slen int32, log_ctx unsafe.Pointer) error {
	if ret := fAvParseColor(rgba_color, color_string, slen, log_ctx); ret < 0 {
		return codeErr("av_parse_color", ret)
	}
	return nil
}

// ParseRatio 把分数的字符串解析成分子分母（对 av_parse_ratio；参数 q、str、max、log_offset、log_ctx；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) ParseRatio(q unsafe.Pointer, str unsafe.Pointer, max int32, log_offset int32, log_ctx unsafe.Pointer) unsafe.Pointer {
	return fAvParseRatio(q, str, max, log_offset, log_ctx)
}

// ParseTime 把时间字符串解析成微秒（对 av_parse_time；参数 timeval、timestr、duration；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ParseTime(timeval unsafe.Pointer, timestr unsafe.Pointer, duration int32) error {
	if ret := fAvParseTime(timeval, timestr, duration); ret < 0 {
		return codeErr("av_parse_time", ret)
	}
	return nil
}

// ParseVideoRate 把帧率字符串解析成分数（对 av_parse_video_rate；参数 rate、str；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) ParseVideoRate(rate unsafe.Pointer, str unsafe.Pointer) error {
	if ret := fAvParseVideoRate(rate, str); ret < 0 {
		return codeErr("av_parse_video_rate", ret)
	}
	return nil
}

// ParseVideoSize 把宽高字符串解析成数字（对 av_parse_video_size；参数 width_ptr、height_ptr、str；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) ParseVideoSize(width_ptr unsafe.Pointer, height_ptr unsafe.Pointer, str unsafe.Pointer) unsafe.Pointer {
	return fAvParseVideoSize(width_ptr, height_ptr, str)
}

// PixFmtDescGet 像素格式查名字查属性（对 av_pix_fmt_desc_get；参数 pix_fmt；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) PixFmtDescGet(pix_fmt int32) unsafe.Pointer {
	return fAvPixFmtDescGet(pix_fmt)
}

// PixFmtDescGetId 像素格式查名字查属性（对 av_pix_fmt_desc_get_id；参数 desc；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) PixFmtDescGetId(desc unsafe.Pointer) int32 {
	return fAvPixFmtDescGetId(desc)
}

// PixFmtDescNext 像素格式查名字查属性（对 av_pix_fmt_desc_next；参数 prev；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) PixFmtDescNext(prev unsafe.Pointer) unsafe.Pointer {
	return fAvPixFmtDescNext(prev)
}

// SphericalAlloc 全景球面元数据操作（对 av_spherical_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) SphericalAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvSphericalAlloc(size)
}

// SphericalFromName 全景球面元数据操作（对 av_spherical_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) SphericalFromName(name unsafe.Pointer) int32 {
	return fAvSphericalFromName(name)
}

// SphericalProjectionName 全景球面元数据操作（对 av_spherical_projection_name；参数 projection；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) SphericalProjectionName(projection unsafe.Pointer) unsafe.Pointer {
	return fAvSphericalProjectionName(projection)
}

// SphericalTileBounds 全景球面元数据操作（对 av_spherical_tile_bounds；参数 mp、width、height、left、top、right、bottom；按签名取回值；无状态，可用零值直接调）。
func (x *MediaDesc) SphericalTileBounds(mp unsafe.Pointer, width uintptr, height uintptr, left unsafe.Pointer, top unsafe.Pointer, right unsafe.Pointer, bottom unsafe.Pointer) {
	fAvSphericalTileBounds(mp, width, height, left, top, right, bottom)
}

// Stereo3dAlloc 立体视频元数据操作（对 av_stereo3d_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dAlloc() unsafe.Pointer {
	return fAvStereo3dAlloc()
}

// Stereo3dAllocSize 立体视频元数据操作（对 av_stereo3d_alloc_size；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dAllocSize(size unsafe.Pointer) unsafe.Pointer {
	return fAvStereo3dAllocSize(size)
}

// Stereo3dCreateSideData 立体视频元数据操作（对 av_stereo3d_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvStereo3dCreateSideData(frame)
}

// Stereo3dFromName 立体视频元数据操作（对 av_stereo3d_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dFromName(name unsafe.Pointer) int32 {
	return fAvStereo3dFromName(name)
}

// Stereo3dPrimaryEyeFromName 立体视频元数据操作（对 av_stereo3d_primary_eye_from_name；参数 name；回数值或个数；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dPrimaryEyeFromName(name unsafe.Pointer) int32 {
	return fAvStereo3dPrimaryEyeFromName(name)
}

// Stereo3dPrimaryEyeName 立体视频元数据操作（对 av_stereo3d_primary_eye_name；参数 eye；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dPrimaryEyeName(eye uint32) unsafe.Pointer {
	return fAvStereo3dPrimaryEyeName(eye)
}

// Stereo3dTypeName 立体视频元数据操作（对 av_stereo3d_type_name；参数 typ；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dTypeName(typ uint32) unsafe.Pointer {
	return fAvStereo3dTypeName(typ)
}

// Stereo3dViewFromName 立体视频元数据操作（对 av_stereo3d_view_from_name；参数 name；回数值；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dViewFromName(name unsafe.Pointer) int32 {
	return fAvStereo3dViewFromName(name)
}

// Stereo3dViewName 立体视频元数据操作（对 av_stereo3d_view_name；参数 view；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (x *MediaDesc) Stereo3dViewName(view uint32) unsafe.Pointer {
	return fAvStereo3dViewName(view)
}

// TimecodeAdjustNtscFramenum2 时间码初始化换算和拼串（对 av_timecode_adjust_ntsc_framenum2；参数 framenum、fps；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeAdjustNtscFramenum2(framenum int32, fps int32) error {
	if ret := fAvTimecodeAdjustNtscFramenum2(framenum, fps); ret < 0 {
		return codeErr("av_timecode_adjust_ntsc_framenum2", ret)
	}
	return nil
}

// TimecodeCheckFrameRate 时间码初始化换算和拼串（对 av_timecode_check_frame_rate；参数 rate；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeCheckFrameRate(rate AVRational) error {
	if ret := fAvTimecodeCheckFrameRate(rate); ret < 0 {
		return codeErr("av_timecode_check_frame_rate", ret)
	}
	return nil
}

// TimecodeGetSmpte 时间码初始化换算和拼串（对 av_timecode_get_smpte；参数 rate、drop、hh、mm、ss、ff；回数值；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeGetSmpte(rate AVRational, drop int32, hh int32, mm int32, ss int32, ff int32) uint32 {
	return fAvTimecodeGetSmpte(rate, drop, hh, mm, ss, ff)
}

// TimecodeGetSmpteFromFramenum 时间码初始化换算和拼串（对 av_timecode_get_smpte_from_framenum；参数 tc、framenum；回数值；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeGetSmpteFromFramenum(tc unsafe.Pointer, framenum int32) uint32 {
	return fAvTimecodeGetSmpteFromFramenum(tc, framenum)
}

// TimecodeInit 时间码初始化换算和拼串（对 av_timecode_init；参数 tc、rate、flags、frame_start、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeInit(tc unsafe.Pointer, rate AVRational, flags int32, frame_start int32, log_ctx unsafe.Pointer) error {
	if ret := fAvTimecodeInit(tc, rate, flags, frame_start, log_ctx); ret < 0 {
		return codeErr("av_timecode_init", ret)
	}
	return nil
}

// TimecodeInitFromComponents 时间码初始化换算和拼串（对 av_timecode_init_from_components；参数 tc、rate、flags、hh、mm、ss、ff、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeInitFromComponents(tc unsafe.Pointer, rate AVRational, flags int32, hh int32, mm int32, ss int32, ff int32, log_ctx unsafe.Pointer) error {
	if ret := fAvTimecodeInitFromComponents(tc, rate, flags, hh, mm, ss, ff, log_ctx); ret < 0 {
		return codeErr("av_timecode_init_from_components", ret)
	}
	return nil
}

// TimecodeInitFromString 时间码初始化换算和拼串（对 av_timecode_init_from_string；参数 tc、rate、str、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeInitFromString(tc unsafe.Pointer, rate AVRational, str unsafe.Pointer, log_ctx unsafe.Pointer) error {
	if ret := fAvTimecodeInitFromString(tc, rate, str, log_ctx); ret < 0 {
		return codeErr("av_timecode_init_from_string", ret)
	}
	return nil
}

// TimecodeMakeMpegTcString 时间码初始化换算和拼串（对 av_timecode_make_mpeg_tc_string；参数 buf、tc25bit；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeMakeMpegTcString(buf unsafe.Pointer, tc25bit uint32) unsafe.Pointer {
	return fAvTimecodeMakeMpegTcString(buf, tc25bit)
}

// TimecodeMakeSmpteTcString 时间码初始化换算和拼串（对 av_timecode_make_smpte_tc_string；参数 buf、tcsmpte、prevent_df；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeMakeSmpteTcString(buf unsafe.Pointer, tcsmpte uint32, prevent_df int32) unsafe.Pointer {
	return fAvTimecodeMakeSmpteTcString(buf, tcsmpte, prevent_df)
}

// TimecodeMakeSmpteTcString2 时间码初始化换算和拼串（对 av_timecode_make_smpte_tc_string2；参数 buf、rate、tcsmpte、prevent_df、skip_field；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeMakeSmpteTcString2(buf unsafe.Pointer, rate AVRational, tcsmpte uint32, prevent_df int32, skip_field int32) unsafe.Pointer {
	return fAvTimecodeMakeSmpteTcString2(buf, rate, tcsmpte, prevent_df, skip_field)
}

// TimecodeMakeString 时间码初始化换算和拼串（对 av_timecode_make_string；参数 tc、buf、framenum；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (x *MediaDesc) TimecodeMakeString(tc unsafe.Pointer, buf unsafe.Pointer, framenum int32) unsafe.Pointer {
	return fAvTimecodeMakeString(tc, buf, framenum)
}
