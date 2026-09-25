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

func (x *MediaDesc) AmbientViewingEnvironmentAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvAmbientViewingEnvironmentAlloc(size)
}

func (x *MediaDesc) AmbientViewingEnvironmentCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvAmbientViewingEnvironmentCreateSideData(frame)
}

func (x *MediaDesc) ChannelDescription(buf unsafe.Pointer, buf_size uintptr, channel unsafe.Pointer) error {
	if ret := fAvChannelDescription(buf, buf_size, channel); ret < 0 {
		return codeErr("av_channel_description", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelDescriptionBprint(bp unsafe.Pointer, channel_id unsafe.Pointer) {
	fAvChannelDescriptionBprint(bp, channel_id)
}

func (x *MediaDesc) ChannelFromString(name unsafe.Pointer) unsafe.Pointer {
	return fAvChannelFromString(name)
}

func (x *MediaDesc) ChannelLayoutAmbisonicOrder(channel_layout unsafe.Pointer) error {
	if ret := fAvChannelLayoutAmbisonicOrder(channel_layout); ret < 0 {
		return codeErr("av_channel_layout_ambisonic_order", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutChannelFromIndex(channel_layout unsafe.Pointer, idx uint32) unsafe.Pointer {
	return fAvChannelLayoutChannelFromIndex(channel_layout, idx)
}

func (x *MediaDesc) ChannelLayoutChannelFromString(channel_layout unsafe.Pointer, name unsafe.Pointer) unsafe.Pointer {
	return fAvChannelLayoutChannelFromString(channel_layout, name)
}

func (x *MediaDesc) ChannelLayoutCheck(channel_layout unsafe.Pointer) error {
	if ret := fAvChannelLayoutCheck(channel_layout); ret < 0 {
		return codeErr("av_channel_layout_check", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutCompare(chl unsafe.Pointer, chl1 unsafe.Pointer) error {
	if ret := fAvChannelLayoutCompare(chl, chl1); ret < 0 {
		return codeErr("av_channel_layout_compare", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutCopy(dst unsafe.Pointer, src unsafe.Pointer) error {
	if ret := fAvChannelLayoutCopy(dst, src); ret < 0 {
		return codeErr("av_channel_layout_copy", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutCustomInit(channel_layout unsafe.Pointer, nb_channels int32) error {
	if ret := fAvChannelLayoutCustomInit(channel_layout, nb_channels); ret < 0 {
		return codeErr("av_channel_layout_custom_init", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutDefault(ch_layout unsafe.Pointer, nb_channels int32) {
	fAvChannelLayoutDefault(ch_layout, nb_channels)
}

func (x *MediaDesc) ChannelLayoutDescribe(channel_layout unsafe.Pointer, buf unsafe.Pointer, buf_size uintptr) error {
	if ret := fAvChannelLayoutDescribe(channel_layout, buf, buf_size); ret < 0 {
		return codeErr("av_channel_layout_describe", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutDescribeBprint(channel_layout unsafe.Pointer, bp unsafe.Pointer) error {
	if ret := fAvChannelLayoutDescribeBprint(channel_layout, bp); ret < 0 {
		return codeErr("av_channel_layout_describe_bprint", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutFromMask(channel_layout unsafe.Pointer, mask uint64) error {
	if ret := fAvChannelLayoutFromMask(channel_layout, mask); ret < 0 {
		return codeErr("av_channel_layout_from_mask", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutFromString(channel_layout unsafe.Pointer, str unsafe.Pointer) error {
	if ret := fAvChannelLayoutFromString(channel_layout, str); ret < 0 {
		return codeErr("av_channel_layout_from_string", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutIndexFromChannel(channel_layout unsafe.Pointer, channel unsafe.Pointer) error {
	if ret := fAvChannelLayoutIndexFromChannel(channel_layout, channel); ret < 0 {
		return codeErr("av_channel_layout_index_from_channel", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutIndexFromString(channel_layout unsafe.Pointer, name unsafe.Pointer) error {
	if ret := fAvChannelLayoutIndexFromString(channel_layout, name); ret < 0 {
		return codeErr("av_channel_layout_index_from_string", ret)
	}
	return nil
}

func (x *MediaDesc) ChannelLayoutRetype(channel_layout unsafe.Pointer, order unsafe.Pointer, flags int32) unsafe.Pointer {
	return fAvChannelLayoutRetype(channel_layout, order, flags)
}

func (x *MediaDesc) ChannelLayoutStandard(opaque *unsafe.Pointer) unsafe.Pointer {
	return fAvChannelLayoutStandard(opaque)
}

func (x *MediaDesc) ChannelLayoutSubset(channel_layout unsafe.Pointer, mask uint64) uint64 {
	return fAvChannelLayoutSubset(channel_layout, mask)
}

func (x *MediaDesc) ChannelLayoutUninit(channel_layout unsafe.Pointer) {
	fAvChannelLayoutUninit(channel_layout)
}

func (x *MediaDesc) ChannelName(buf unsafe.Pointer, buf_size uintptr, channel unsafe.Pointer) int32 {
	return fAvChannelName(buf, buf_size, channel)
}

func (x *MediaDesc) ChannelNameBprint(bp unsafe.Pointer, channel_id unsafe.Pointer) {
	fAvChannelNameBprint(bp, channel_id)
}

func (x *MediaDesc) ColorPrimariesFromName(name unsafe.Pointer) int32 {
	return fAvColorPrimariesFromName(name)
}

func (x *MediaDesc) ColorPrimariesName(primaries int32) unsafe.Pointer {
	return fAvColorPrimariesName(primaries)
}

func (x *MediaDesc) ColorRangeFromName(name string) int32 {
	return fAvColorRangeFromName(name)
}

func (x *MediaDesc) ColorRangeName(rng int32) unsafe.Pointer {
	return fAvColorRangeName(rng)
}

func (x *MediaDesc) ColorSpaceFromName(name string) int32 {
	return fAvColorSpaceFromName(name)
}

func (x *MediaDesc) ColorSpaceName(space int32) unsafe.Pointer {
	return fAvColorSpaceName(space)
}

func (x *MediaDesc) ColorTransferFromName(name unsafe.Pointer) int32 {
	return fAvColorTransferFromName(name)
}

func (x *MediaDesc) ColorTransferName(transfer unsafe.Pointer) unsafe.Pointer {
	return fAvColorTransferName(transfer)
}

func (x *MediaDesc) ContentLightMetadataAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvContentLightMetadataAlloc(size)
}

func (x *MediaDesc) ContentLightMetadataCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvContentLightMetadataCreateSideData(frame)
}

func (x *MediaDesc) DisplayMatrixFlip(matrix int32, hflip int32, vflip int32) {
	fAvDisplayMatrixFlip(matrix, hflip, vflip)
}

func (x *MediaDesc) DisplayRotationGet(matrix int32) unsafe.Pointer {
	return fAvDisplayRotationGet(matrix)
}

func (x *MediaDesc) DisplayRotationSet(matrix int32, angle float64) {
	fAvDisplayRotationSet(matrix, angle)
}

func (x *MediaDesc) DoviAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvDoviAlloc(size)
}

func (x *MediaDesc) DoviFindLevel(data unsafe.Pointer, level unsafe.Pointer) unsafe.Pointer {
	return fAvDoviFindLevel(data, level)
}

func (x *MediaDesc) DoviMetadataAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvDoviMetadataAlloc(size)
}

func (x *MediaDesc) DynamicHdrPlusAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvDynamicHdrPlusAlloc(size)
}

func (x *MediaDesc) DynamicHdrPlusCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvDynamicHdrPlusCreateSideData(frame)
}

func (x *MediaDesc) DynamicHdrPlusFromT35(s unsafe.Pointer, data unsafe.Pointer, size uintptr) error {
	if ret := fAvDynamicHdrPlusFromT35(s, data, size); ret < 0 {
		return codeErr("av_dynamic_hdr_plus_from_t35", ret)
	}
	return nil
}

func (x *MediaDesc) DynamicHdrPlusToT35(s unsafe.Pointer, data *unsafe.Pointer, size unsafe.Pointer) unsafe.Pointer {
	return fAvDynamicHdrPlusToT35(s, data, size)
}

func (x *MediaDesc) FilmGrainParamsAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvFilmGrainParamsAlloc(size)
}

func (x *MediaDesc) FilmGrainParamsCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvFilmGrainParamsCreateSideData(frame)
}

func (x *MediaDesc) FilmGrainParamsSelect(frame unsafe.Pointer) unsafe.Pointer {
	return fAvFilmGrainParamsSelect(frame)
}

func (x *MediaDesc) GetBitsPerPixel(pixdesc unsafe.Pointer) unsafe.Pointer {
	return fAvGetBitsPerPixel(pixdesc)
}

func (x *MediaDesc) GetPixFmt(name string) int32 {
	return fAvGetPixFmt(name)
}

func (x *MediaDesc) GetPixFmtLoss(dst_pix_fmt int32, src_pix_fmt int32, has_alpha int32) int32 {
	return fAvGetPixFmtLoss(dst_pix_fmt, src_pix_fmt, has_alpha)
}

func (x *MediaDesc) GetPixFmtName(pix_fmt int32) unsafe.Pointer {
	return fAvGetPixFmtName(pix_fmt)
}

func (x *MediaDesc) GetPixFmtString(buf unsafe.Pointer, buf_size int32, pix_fmt int32) unsafe.Pointer {
	return fAvGetPixFmtString(buf, buf_size, pix_fmt)
}

func (x *MediaDesc) MasteringDisplayMetadataAlloc() unsafe.Pointer {
	return fAvMasteringDisplayMetadataAlloc()
}

func (x *MediaDesc) MasteringDisplayMetadataAllocSize(size unsafe.Pointer) unsafe.Pointer {
	return fAvMasteringDisplayMetadataAllocSize(size)
}

func (x *MediaDesc) MasteringDisplayMetadataCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvMasteringDisplayMetadataCreateSideData(frame)
}

func (x *MediaDesc) ParseColor(rgba_color unsafe.Pointer, color_string unsafe.Pointer, slen int32, log_ctx unsafe.Pointer) error {
	if ret := fAvParseColor(rgba_color, color_string, slen, log_ctx); ret < 0 {
		return codeErr("av_parse_color", ret)
	}
	return nil
}

func (x *MediaDesc) ParseRatio(q unsafe.Pointer, str unsafe.Pointer, max int32, log_offset int32, log_ctx unsafe.Pointer) unsafe.Pointer {
	return fAvParseRatio(q, str, max, log_offset, log_ctx)
}

func (x *MediaDesc) ParseTime(timeval unsafe.Pointer, timestr unsafe.Pointer, duration int32) error {
	if ret := fAvParseTime(timeval, timestr, duration); ret < 0 {
		return codeErr("av_parse_time", ret)
	}
	return nil
}

func (x *MediaDesc) ParseVideoRate(rate unsafe.Pointer, str unsafe.Pointer) error {
	if ret := fAvParseVideoRate(rate, str); ret < 0 {
		return codeErr("av_parse_video_rate", ret)
	}
	return nil
}

func (x *MediaDesc) ParseVideoSize(width_ptr unsafe.Pointer, height_ptr unsafe.Pointer, str unsafe.Pointer) unsafe.Pointer {
	return fAvParseVideoSize(width_ptr, height_ptr, str)
}

func (x *MediaDesc) PixFmtDescGet(pix_fmt int32) unsafe.Pointer {
	return fAvPixFmtDescGet(pix_fmt)
}

func (x *MediaDesc) PixFmtDescGetId(desc unsafe.Pointer) int32 {
	return fAvPixFmtDescGetId(desc)
}

func (x *MediaDesc) PixFmtDescNext(prev unsafe.Pointer) unsafe.Pointer {
	return fAvPixFmtDescNext(prev)
}

func (x *MediaDesc) SphericalAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvSphericalAlloc(size)
}

func (x *MediaDesc) SphericalFromName(name unsafe.Pointer) int32 {
	return fAvSphericalFromName(name)
}

func (x *MediaDesc) SphericalProjectionName(projection unsafe.Pointer) unsafe.Pointer {
	return fAvSphericalProjectionName(projection)
}

func (x *MediaDesc) SphericalTileBounds(mp unsafe.Pointer, width uintptr, height uintptr, left unsafe.Pointer, top unsafe.Pointer, right unsafe.Pointer, bottom unsafe.Pointer) {
	fAvSphericalTileBounds(mp, width, height, left, top, right, bottom)
}

func (x *MediaDesc) Stereo3dAlloc() unsafe.Pointer {
	return fAvStereo3dAlloc()
}

func (x *MediaDesc) Stereo3dAllocSize(size unsafe.Pointer) unsafe.Pointer {
	return fAvStereo3dAllocSize(size)
}

func (x *MediaDesc) Stereo3dCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvStereo3dCreateSideData(frame)
}

func (x *MediaDesc) Stereo3dFromName(name unsafe.Pointer) int32 {
	return fAvStereo3dFromName(name)
}

func (x *MediaDesc) Stereo3dPrimaryEyeFromName(name unsafe.Pointer) int32 {
	return fAvStereo3dPrimaryEyeFromName(name)
}

func (x *MediaDesc) Stereo3dPrimaryEyeName(eye uint32) unsafe.Pointer {
	return fAvStereo3dPrimaryEyeName(eye)
}

func (x *MediaDesc) Stereo3dTypeName(typ uint32) unsafe.Pointer {
	return fAvStereo3dTypeName(typ)
}

func (x *MediaDesc) Stereo3dViewFromName(name unsafe.Pointer) int32 {
	return fAvStereo3dViewFromName(name)
}

func (x *MediaDesc) Stereo3dViewName(view uint32) unsafe.Pointer {
	return fAvStereo3dViewName(view)
}

func (x *MediaDesc) TimecodeAdjustNtscFramenum2(framenum int32, fps int32) error {
	if ret := fAvTimecodeAdjustNtscFramenum2(framenum, fps); ret < 0 {
		return codeErr("av_timecode_adjust_ntsc_framenum2", ret)
	}
	return nil
}

func (x *MediaDesc) TimecodeCheckFrameRate(rate AVRational) error {
	if ret := fAvTimecodeCheckFrameRate(rate); ret < 0 {
		return codeErr("av_timecode_check_frame_rate", ret)
	}
	return nil
}

func (x *MediaDesc) TimecodeGetSmpte(rate AVRational, drop int32, hh int32, mm int32, ss int32, ff int32) uint32 {
	return fAvTimecodeGetSmpte(rate, drop, hh, mm, ss, ff)
}

func (x *MediaDesc) TimecodeGetSmpteFromFramenum(tc unsafe.Pointer, framenum int32) uint32 {
	return fAvTimecodeGetSmpteFromFramenum(tc, framenum)
}

func (x *MediaDesc) TimecodeInit(tc unsafe.Pointer, rate AVRational, flags int32, frame_start int32, log_ctx unsafe.Pointer) error {
	if ret := fAvTimecodeInit(tc, rate, flags, frame_start, log_ctx); ret < 0 {
		return codeErr("av_timecode_init", ret)
	}
	return nil
}

func (x *MediaDesc) TimecodeInitFromComponents(tc unsafe.Pointer, rate AVRational, flags int32, hh int32, mm int32, ss int32, ff int32, log_ctx unsafe.Pointer) error {
	if ret := fAvTimecodeInitFromComponents(tc, rate, flags, hh, mm, ss, ff, log_ctx); ret < 0 {
		return codeErr("av_timecode_init_from_components", ret)
	}
	return nil
}

func (x *MediaDesc) TimecodeInitFromString(tc unsafe.Pointer, rate AVRational, str unsafe.Pointer, log_ctx unsafe.Pointer) error {
	if ret := fAvTimecodeInitFromString(tc, rate, str, log_ctx); ret < 0 {
		return codeErr("av_timecode_init_from_string", ret)
	}
	return nil
}

func (x *MediaDesc) TimecodeMakeMpegTcString(buf unsafe.Pointer, tc25bit uint32) unsafe.Pointer {
	return fAvTimecodeMakeMpegTcString(buf, tc25bit)
}

func (x *MediaDesc) TimecodeMakeSmpteTcString(buf unsafe.Pointer, tcsmpte uint32, prevent_df int32) unsafe.Pointer {
	return fAvTimecodeMakeSmpteTcString(buf, tcsmpte, prevent_df)
}

func (x *MediaDesc) TimecodeMakeSmpteTcString2(buf unsafe.Pointer, rate AVRational, tcsmpte uint32, prevent_df int32, skip_field int32) unsafe.Pointer {
	return fAvTimecodeMakeSmpteTcString2(buf, rate, tcsmpte, prevent_df, skip_field)
}

func (x *MediaDesc) TimecodeMakeString(tc unsafe.Pointer, buf unsafe.Pointer, framenum int32) unsafe.Pointer {
	return fAvTimecodeMakeString(tc, buf, framenum)
}
