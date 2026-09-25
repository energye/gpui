package ffmpeg

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// DictOpt 字典配置模块: AVDictionary + AVOption 全量导出,
// 结构体方法直接可用.
//
// Say it plain: 字典就是打开解码器时塞进去的小纸条 (键值对,
// 比如超时时间、线程数); 配置项就是结构体上每个开关的名字,
// 上层按名字读写, 不用记偏移.

// Dictionary owns one AVDictionary* (nil-safe, 记得 Free).
type Dictionary struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle for cross-module calls (codec/format).
func (d *Dictionary) Ptr() unsafe.Pointer {
	if d == nil {
		return nil
	}
	return d.ptr
}

// Take returns the raw pointer and clears the holder (所有权交出去,
// 比如塞进 format open 的 options 里).
func (d *Dictionary) Take() unsafe.Pointer {
	if d == nil {
		return nil
	}
	p := d.ptr
	d.ptr = nil
	return p
}

// DictionaryEntry owns one AVDictionaryEntry* (跟着字典走, 不单独释放).
type DictionaryEntry struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (e *DictionaryEntry) Ptr() unsafe.Pointer {
	if e == nil {
		return nil
	}
	return e.ptr
}

// Key returns the entry key.
func (e *DictionaryEntry) Key() string {
	if e == nil || e.ptr == nil {
		return ""
	}
	return cstr(*(*unsafe.Pointer)(e.ptr))
}

// Value returns the entry value.
func (e *DictionaryEntry) Value() string {
	if e == nil || e.ptr == nil {
		return ""
	}
	return cstr(*(*unsafe.Pointer)(unsafe.Add(e.ptr, 8)))
}

// OptObject wraps any object carrying an AVClass (codec/format/filter
// 上下文都能当), 配置项按名字读写.
type OptObject struct{ ptr unsafe.Pointer }

// Opt wraps a target object pointer for option calls.
func Opt(ptr unsafe.Pointer) OptObject { return OptObject{ptr: ptr} }

// Ptr exposes the raw handle.
func (o OptObject) Ptr() unsafe.Pointer { return o.ptr }

// Option owns one AVOption* (跟着类走, 不单独释放).
type Option struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (o *Option) Ptr() unsafe.Pointer {
	if o == nil {
		return nil
	}
	return o.ptr
}

// Name returns the option name (AVOption 第一个字段).
func (o *Option) Name() string {
	if o == nil || o.ptr == nil {
		return ""
	}
	return cstr(*(*unsafe.Pointer)(o.ptr))
}

// OptionRanges owns one AVOptionRanges* (记得 FreeRanges).
type OptionRanges struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (r *OptionRanges) Ptr() unsafe.Pointer {
	if r == nil {
		return nil
	}
	return r.ptr
}

var (
	fDictCopy       func(*unsafe.Pointer, unsafe.Pointer, int32) int32
	fDictCount      func(unsafe.Pointer) int32
	fDictFree       func(*unsafe.Pointer)
	fDictGet        func(unsafe.Pointer, string, unsafe.Pointer, int32) unsafe.Pointer
	fDictGetString  func(unsafe.Pointer, *unsafe.Pointer, byte, byte) int32
	fDictIterate    func(unsafe.Pointer, unsafe.Pointer) unsafe.Pointer
	fDictParseStr   func(*unsafe.Pointer, string, string, string, int32) int32
	fDictSet        func(*unsafe.Pointer, string, string, int32) int32
	fDictSetInt     func(*unsafe.Pointer, string, int64, int32) int32
	fOptChildIter   func(unsafe.Pointer, *unsafe.Pointer) unsafe.Pointer
	fOptChildNext   func(unsafe.Pointer, unsafe.Pointer) unsafe.Pointer
	fOptCopy        func(unsafe.Pointer, unsafe.Pointer) int32
	fOptEvalDouble  func(unsafe.Pointer, unsafe.Pointer, string, *float64) int32
	fOptEvalFlags   func(unsafe.Pointer, unsafe.Pointer, string, *int32) int32
	fOptEvalFloat   func(unsafe.Pointer, unsafe.Pointer, string, *float32) int32
	fOptEvalInt     func(unsafe.Pointer, unsafe.Pointer, string, *int32) int32
	fOptEvalInt64   func(unsafe.Pointer, unsafe.Pointer, string, *int64) int32
	fOptEvalQ       func(unsafe.Pointer, unsafe.Pointer, string, *AVRational) int32
	fOptEvalUint    func(unsafe.Pointer, unsafe.Pointer, string, *uint32) int32
	fOptFind        func(unsafe.Pointer, string, string, int32, int32) unsafe.Pointer
	fOptFind2       func(unsafe.Pointer, string, string, int32, int32, *unsafe.Pointer) unsafe.Pointer
	fOptFlagIsSet   func(unsafe.Pointer, string, string) int32
	fOptFree        func(unsafe.Pointer)
	fOptFreepRanges func(*unsafe.Pointer)
	fOptGet         func(unsafe.Pointer, string, int32, *unsafe.Pointer) int32
	fOptGetArray    func(unsafe.Pointer, string, int32, uint32, uint32, int32, unsafe.Pointer) int32
	fOptGetArrSize  func(unsafe.Pointer, string, int32, *uint32) int32
	fOptGetChlayout func(unsafe.Pointer, string, int32, unsafe.Pointer) int32
	fOptGetDictVal  func(unsafe.Pointer, string, int32, *unsafe.Pointer) int32
	fOptGetDouble   func(unsafe.Pointer, string, int32, *float64) int32
	fOptGetImgSize  func(unsafe.Pointer, string, int32, *int32, *int32) int32
	fOptGetInt      func(unsafe.Pointer, string, int32, *int64) int32
	fOptGetKeyValue func(*unsafe.Pointer, string, string, uint32, *unsafe.Pointer, *unsafe.Pointer) int32
	fOptGetPixFmt   func(unsafe.Pointer, string, int32, *int32) int32
	fOptGetQ        func(unsafe.Pointer, string, int32, *AVRational) int32
	fOptGetSampleFm func(unsafe.Pointer, string, int32, *int32) int32
	fOptGetVideoRat func(unsafe.Pointer, string, int32, *AVRational) int32
	fOptIsDefault   func(unsafe.Pointer, unsafe.Pointer) int32
	fOptIsDefByName func(unsafe.Pointer, string, int32) int32
	fOptNext        func(unsafe.Pointer, unsafe.Pointer) unsafe.Pointer
	fOptPtr         func(unsafe.Pointer, unsafe.Pointer, string) unsafe.Pointer
	fOptQueryRanges func(*unsafe.Pointer, unsafe.Pointer, string, int32) int32
	fOptQueryDef    func(*unsafe.Pointer, unsafe.Pointer, string, int32) int32
	fOptSerialize   func(unsafe.Pointer, int32, int32, *unsafe.Pointer) int32
	fOptSet         func(unsafe.Pointer, string, string, int32) int32
	fOptSetArray    func(unsafe.Pointer, string, int32, uint32, int32, unsafe.Pointer) int32
	fOptSetBin      func(unsafe.Pointer, string, unsafe.Pointer, int32, int32) int32
	fOptSetChlayout func(unsafe.Pointer, string, unsafe.Pointer, int32) int32
	fOptSetDefaults func(unsafe.Pointer)
	fOptSetDefault2 func(unsafe.Pointer, int32, int32)
	fOptSetDict     func(unsafe.Pointer, *unsafe.Pointer) int32
	fOptSetDict2    func(unsafe.Pointer, *unsafe.Pointer, int32) int32
	fOptSetDictVal  func(unsafe.Pointer, string, unsafe.Pointer, int32) int32
	fOptSetDouble   func(unsafe.Pointer, string, float64, int32) int32
	fOptSetFromStr  func(unsafe.Pointer, string, string, string, string, int32) int32
	fOptSetImgSize  func(unsafe.Pointer, string, int32, int32, int32) int32
	fOptSetInt      func(unsafe.Pointer, string, int64, int32) int32
	fOptSetPixFmt   func(unsafe.Pointer, string, int32, int32) int32
	fOptSetQ        func(unsafe.Pointer, string, AVRational, int32) int32
	fOptSetSampleFm func(unsafe.Pointer, string, int32, int32) int32
	fOptSetVideoRat func(unsafe.Pointer, string, AVRational, int32) int32
	fOptShow2       func(unsafe.Pointer, string, int32, int32) int32
)

func registerDictOpt(h uintptr) {
	purego.RegisterLibFunc(&fDictCopy, h, "av_dict_copy")
	purego.RegisterLibFunc(&fDictCount, h, "av_dict_count")
	purego.RegisterLibFunc(&fDictFree, h, "av_dict_free")
	purego.RegisterLibFunc(&fDictGet, h, "av_dict_get")
	purego.RegisterLibFunc(&fDictGetString, h, "av_dict_get_string")
	purego.RegisterLibFunc(&fDictIterate, h, "av_dict_iterate")
	purego.RegisterLibFunc(&fDictParseStr, h, "av_dict_parse_string")
	purego.RegisterLibFunc(&fDictSet, h, "av_dict_set")
	purego.RegisterLibFunc(&fDictSetInt, h, "av_dict_set_int")
	purego.RegisterLibFunc(&fOptChildIter, h, "av_opt_child_class_iterate")
	purego.RegisterLibFunc(&fOptChildNext, h, "av_opt_child_next")
	purego.RegisterLibFunc(&fOptCopy, h, "av_opt_copy")
	purego.RegisterLibFunc(&fOptEvalDouble, h, "av_opt_eval_double")
	purego.RegisterLibFunc(&fOptEvalFlags, h, "av_opt_eval_flags")
	purego.RegisterLibFunc(&fOptEvalFloat, h, "av_opt_eval_float")
	purego.RegisterLibFunc(&fOptEvalInt, h, "av_opt_eval_int")
	purego.RegisterLibFunc(&fOptEvalInt64, h, "av_opt_eval_int64")
	purego.RegisterLibFunc(&fOptEvalQ, h, "av_opt_eval_q")
	purego.RegisterLibFunc(&fOptEvalUint, h, "av_opt_eval_uint")
	purego.RegisterLibFunc(&fOptFind, h, "av_opt_find")
	purego.RegisterLibFunc(&fOptFind2, h, "av_opt_find2")
	purego.RegisterLibFunc(&fOptFlagIsSet, h, "av_opt_flag_is_set")
	purego.RegisterLibFunc(&fOptFree, h, "av_opt_free")
	purego.RegisterLibFunc(&fOptFreepRanges, h, "av_opt_freep_ranges")
	purego.RegisterLibFunc(&fOptGet, h, "av_opt_get")
	purego.RegisterLibFunc(&fOptGetArray, h, "av_opt_get_array")
	purego.RegisterLibFunc(&fOptGetArrSize, h, "av_opt_get_array_size")
	purego.RegisterLibFunc(&fOptGetChlayout, h, "av_opt_get_chlayout")
	purego.RegisterLibFunc(&fOptGetDictVal, h, "av_opt_get_dict_val")
	purego.RegisterLibFunc(&fOptGetDouble, h, "av_opt_get_double")
	purego.RegisterLibFunc(&fOptGetImgSize, h, "av_opt_get_image_size")
	purego.RegisterLibFunc(&fOptGetInt, h, "av_opt_get_int")
	purego.RegisterLibFunc(&fOptGetKeyValue, h, "av_opt_get_key_value")
	purego.RegisterLibFunc(&fOptGetPixFmt, h, "av_opt_get_pixel_fmt")
	purego.RegisterLibFunc(&fOptGetQ, h, "av_opt_get_q")
	purego.RegisterLibFunc(&fOptGetSampleFm, h, "av_opt_get_sample_fmt")
	purego.RegisterLibFunc(&fOptGetVideoRat, h, "av_opt_get_video_rate")
	purego.RegisterLibFunc(&fOptIsDefault, h, "av_opt_is_set_to_default")
	purego.RegisterLibFunc(&fOptIsDefByName, h, "av_opt_is_set_to_default_by_name")
	purego.RegisterLibFunc(&fOptNext, h, "av_opt_next")
	purego.RegisterLibFunc(&fOptPtr, h, "av_opt_ptr")
	purego.RegisterLibFunc(&fOptQueryRanges, h, "av_opt_query_ranges")
	purego.RegisterLibFunc(&fOptQueryDef, h, "av_opt_query_ranges_default")
	purego.RegisterLibFunc(&fOptSerialize, h, "av_opt_serialize")
	purego.RegisterLibFunc(&fOptSet, h, "av_opt_set")
	purego.RegisterLibFunc(&fOptSetArray, h, "av_opt_set_array")
	purego.RegisterLibFunc(&fOptSetBin, h, "av_opt_set_bin")
	purego.RegisterLibFunc(&fOptSetChlayout, h, "av_opt_set_chlayout")
	purego.RegisterLibFunc(&fOptSetDefaults, h, "av_opt_set_defaults")
	purego.RegisterLibFunc(&fOptSetDefault2, h, "av_opt_set_defaults2")
	purego.RegisterLibFunc(&fOptSetDict, h, "av_opt_set_dict")
	purego.RegisterLibFunc(&fOptSetDict2, h, "av_opt_set_dict2")
	purego.RegisterLibFunc(&fOptSetDictVal, h, "av_opt_set_dict_val")
	purego.RegisterLibFunc(&fOptSetDouble, h, "av_opt_set_double")
	purego.RegisterLibFunc(&fOptSetFromStr, h, "av_opt_set_from_string")
	purego.RegisterLibFunc(&fOptSetImgSize, h, "av_opt_set_image_size")
	purego.RegisterLibFunc(&fOptSetInt, h, "av_opt_set_int")
	purego.RegisterLibFunc(&fOptSetPixFmt, h, "av_opt_set_pixel_fmt")
	purego.RegisterLibFunc(&fOptSetQ, h, "av_opt_set_q")
	purego.RegisterLibFunc(&fOptSetSampleFm, h, "av_opt_set_sample_fmt")
	purego.RegisterLibFunc(&fOptSetVideoRat, h, "av_opt_set_video_rate")
	purego.RegisterLibFunc(&fOptShow2, h, "av_opt_show2")
}

// NewDictionary builds an empty dictionary holder (Set 再用,
// 用完 Free; Take 交给 open 时记得不再碰).
func NewDictionary() *Dictionary { return &Dictionary{} }

// Set writes one key/value pair (value "" 删键, flags 一般 0).
func (d *Dictionary) Set(key, value string, flags int) error {
	if d == nil {
		return errNilDict
	}
	if ret := fDictSet(&d.ptr, key, value, int32(flags)); ret < 0 {
		return codeErr("av_dict_set", ret)
	}
	return nil
}

// SetInt writes one integer pair.
func (d *Dictionary) SetInt(key string, value int64, flags int) error {
	if d == nil {
		return errNilDict
	}
	if ret := fDictSetInt(&d.ptr, key, value, int32(flags)); ret < 0 {
		return codeErr("av_dict_set_int", ret)
	}
	return nil
}

// Get returns the entry for key (nil when absent, prev 传 nil 从头找).
func (d *Dictionary) Get(key string, prev *DictionaryEntry, flags int) *DictionaryEntry {
	if d == nil {
		return nil
	}
	var pv unsafe.Pointer
	if prev != nil {
		pv = prev.ptr
	}
	ptr := fDictGet(d.ptr, key, pv, int32(flags))
	if ptr == nil {
		return nil
	}
	return &DictionaryEntry{ptr: ptr}
}

// Count reports the number of pairs.
func (d *Dictionary) Count() int {
	if d == nil {
		return 0
	}
	return int(fDictCount(d.ptr))
}

// Copy duplicates src into dst (flags 一般 0).
func (d *Dictionary) Copy(src *Dictionary, flags int) error {
	if d == nil {
		return errNilDict
	}
	var sp unsafe.Pointer
	if src != nil {
		sp = src.ptr
	}
	if ret := fDictCopy(&d.ptr, sp, int32(flags)); ret < 0 {
		return codeErr("av_dict_copy", ret)
	}
	return nil
}

// ParseString parses "k=v:k=v" text into the dictionary.
func (d *Dictionary) ParseString(s, keySep, pairSep string, flags int) error {
	if d == nil {
		return errNilDict
	}
	if ret := fDictParseStr(&d.ptr, s, keySep, pairSep, int32(flags)); ret < 0 {
		return codeErr("av_dict_parse_string", ret)
	}
	return nil
}

// Free releases the dictionary and nils the holder.
func (d *Dictionary) Free() {
	if d == nil || d.ptr == nil {
		return
	}
	ptr := d.ptr
	d.ptr = nil
	fDictFree(&ptr)
}

// Iterate walks all pairs (prev 传 nil 从头开始, 返回 nil 到尾).
func (d *Dictionary) Iterate(prev *DictionaryEntry) *DictionaryEntry {
	if d == nil {
		return nil
	}
	var pv unsafe.Pointer
	if prev != nil {
		pv = prev.ptr
	}
	ptr := fDictIterate(d.ptr, pv)
	if ptr == nil {
		return nil
	}
	return &DictionaryEntry{ptr: ptr}
}

// Set writes a string option by name (flags 一般 0).
// 字符串能设任何选项: 数字、采样格式名 ("flt")、声道布局名
// ("stereo") 都按选项类型自动解析, 见 av_opt_set.
func (o OptObject) Set(name, value string, flags int) error {
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSet(o.ptr, name, value, int32(flags)); ret < 0 {
		return codeErr("av_opt_set", ret)
	}
	return nil
}

// SetInt writes an integer option by name.
func (o OptObject) SetInt(name string, value int64, flags int) error {
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetInt(o.ptr, name, value, int32(flags)); ret < 0 {
		return codeErr("av_opt_set_int", ret)
	}
	return nil
}

// SetDouble writes a float option by name.
func (o OptObject) SetDouble(name string, value float64, flags int) error {
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetDouble(o.ptr, name, value, int32(flags)); ret < 0 {
		return codeErr("av_opt_set_double", ret)
	}
	return nil
}

// SetQ writes a rational option by name.
func (o OptObject) SetQ(name string, value AVRational, flags int) error {
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetQ(o.ptr, name, value, int32(flags)); ret < 0 {
		return codeErr("av_opt_set_q", ret)
	}
	return nil
}

// GetInt reads an integer option by name.
func (o OptObject) GetInt(name string, flags int) (int64, error) {
	if o.ptr == nil {
		return 0, errNilOpt
	}
	var v int64
	if ret := fOptGetInt(o.ptr, name, int32(flags), &v); ret < 0 {
		return 0, codeErr("av_opt_get_int", ret)
	}
	return v, nil
}

// GetDouble reads a float option by name.
func (o OptObject) GetDouble(name string, flags int) (float64, error) {
	if o.ptr == nil {
		return 0, errNilOpt
	}
	var v float64
	if ret := fOptGetDouble(o.ptr, name, int32(flags), &v); ret < 0 {
		return 0, codeErr("av_opt_get_double", ret)
	}
	return v, nil
}

// GetQ reads a rational option by name.
func (o OptObject) GetQ(name string, flags int) (AVRational, error) {
	if o.ptr == nil {
		return AVRational{}, errNilOpt
	}
	var v AVRational
	if ret := fOptGetQ(o.ptr, name, int32(flags), &v); ret < 0 {
		return AVRational{}, codeErr("av_opt_get_q", ret)
	}
	return v, nil
}

// Find returns the option descriptor by name (nil when absent).
func (o OptObject) Find(name, unit string, optFlags, searchFlags int) *Option {
	if o.ptr == nil {
		return nil
	}
	ptr := fOptFind(o.ptr, name, unit, int32(optFlags), int32(searchFlags))
	if ptr == nil {
		return nil
	}
	return &Option{ptr: ptr}
}

// SetDefaults fills all options with defaults.
func (o OptObject) SetDefaults() {
	if o.ptr == nil {
		return
	}
	fOptSetDefaults(o.ptr)
}

// Copy copies option values from src.
func (o OptObject) Copy(src OptObject) error {
	if o.ptr == nil || src.ptr == nil {
		return errNilOpt
	}
	if ret := fOptCopy(o.ptr, src.ptr); ret < 0 {
		return codeErr("av_opt_copy", ret)
	}
	return nil
}

// FreeRanges releases an OptionRanges holder.
func (r *OptionRanges) FreeRanges() {
	if r == nil || r.ptr == nil {
		return
	}
	ptr := r.ptr
	r.ptr = nil
	fOptFreepRanges(&ptr)
}
