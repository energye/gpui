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
func Opt(ptr unsafe.Pointer) OptObject {
	return OptObject{ptr: ptr}
}

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
	mustUse(ensureModDictOpt())
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
	fOptFind        func(unsafe.Pointer, string, unsafe.Pointer, int32, int32) unsafe.Pointer
	fOptFind2       func(unsafe.Pointer, string, unsafe.Pointer, int32, int32, *unsafe.Pointer) unsafe.Pointer
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
	fOptSerialize   func(unsafe.Pointer, int32, int32, *unsafe.Pointer, byte, byte) int32
	fOptSet         func(unsafe.Pointer, string, string, int32) int32
	fOptSetArray    func(unsafe.Pointer, string, int32, uint32, uint32, int32, unsafe.Pointer) int32
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
	fOptShow2       func(unsafe.Pointer, unsafe.Pointer, int32, int32) int32
)

// ensureModDictOpt 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modDictOptOnce sync.Once

func ensureModDictOpt() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	modDictOptOnce.Do(func() { registerDictOpt(libHandle) })
	return nil
}

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
func NewDictionary() *Dictionary {
	return &Dictionary{}
}

// Set writes one key/value pair (value "" 删键, flags 一般 0).
func (d *Dictionary) Set(key, value string, flags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
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
	if err := ensureModDictOpt(); err != nil {
		return err
	}
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
	mustUse(ensureModDictOpt())
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
	mustUse(ensureModDictOpt())
	if d == nil {
		return 0
	}
	return int(fDictCount(d.ptr))
}

// Copy duplicates src into dst (flags 一般 0).
func (d *Dictionary) Copy(src *Dictionary, flags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
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
	if err := ensureModDictOpt(); err != nil {
		return err
	}
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
	mustUse(ensureModDictOpt())
	if d == nil || d.ptr == nil {
		return
	}
	ptr := d.ptr
	d.ptr = nil
	fDictFree(&ptr)
}

// Iterate walks all pairs (prev 传 nil 从头开始, 返回 nil 到尾).
func (d *Dictionary) Iterate(prev *DictionaryEntry) *DictionaryEntry {
	mustUse(ensureModDictOpt())
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
	if err := ensureModDictOpt(); err != nil {
		return err
	}
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
	if err := ensureModDictOpt(); err != nil {
		return err
	}
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
	if err := ensureModDictOpt(); err != nil {
		return err
	}
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
	if err := ensureModDictOpt(); err != nil {
		return err
	}
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
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
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
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
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
	if err := ensureModDictOpt(); err != nil {
		var z1 AVRational
		return z1, err
	}
	if o.ptr == nil {
		return AVRational{}, errNilOpt
	}
	var v AVRational
	if ret := fOptGetQ(o.ptr, name, int32(flags), &v); ret < 0 {
		return AVRational{}, codeErr("av_opt_get_q", ret)
	}
	return v, nil
}

// featCStrTmp builds a throwaway NUL-terminated C string on the Mem heap
// (caller frees via the returned func; for hot paths hoist it out).
func featCStrTmp(s string) (unsafe.Pointer, func()) {
	raw := append([]byte(s), 0)
	var mem Mem
	buf := mem.Alloc(len(raw))
	if buf == nil {
		return nil, func() {}
	}
	copy(unsafe.Slice((*byte)(buf), len(raw)), raw)
	return buf, func() { mem.Free(buf) }
}

// Find returns the option descriptor by name (nil when absent;
// unit 传 "" 表只找普通选项 (C 里空串是正经 unit 名, 不是通配),
// 传 "\x00" 表不过滤 (C 的 NULL); optFlags/searchFlags 一般 0/AV_OPT_SEARCH_CHILDREN).
func (o OptObject) Find(name, unit string, optFlags, searchFlags int) *Option {
	mustUse(ensureModDictOpt())
	if o.ptr == nil {
		return nil
	}
	var up unsafe.Pointer
	if unit == "\x00" {
		up = nil
	} else {
		cu, freeCu := featCStrTmp(unit)
		defer freeCu()
		up = cu
	}
	ptr := fOptFind(o.ptr, name, up, int32(optFlags), int32(searchFlags))
	if ptr == nil {
		return nil
	}
	return &Option{ptr: ptr}
}

// SetDefaults fills all options with defaults.
func (o OptObject) SetDefaults() {
	mustUse(ensureModDictOpt())
	if o.ptr == nil {
		return
	}
	fOptSetDefaults(o.ptr)
}

// Copy copies option values from src.
func (o OptObject) Copy(src OptObject) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
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
	mustUse(ensureModDictOpt())
	if r == nil || r.ptr == nil {
		return
	}
	ptr := r.ptr
	r.ptr = nil
	fOptFreepRanges(&ptr)
}

// GetString serializes the whole dictionary to "k=v,k=v"
// (av_dict_get_string; sep 传 ','/'=' 最常见, out 用 Mem.Free 放).
func (d *Dictionary) GetString(keySep, pairSep byte) (unsafe.Pointer, error) {
	if err := ensureModDictOpt(); err != nil {
		return nil, err
	}
	if d == nil {
		return nil, errNilDict
	}
	var out unsafe.Pointer
	if ret := fDictGetString(d.ptr, &out, keySep, pairSep); ret < 0 {
		return nil, codeErr("av_dict_get_string", ret)
	}
	return out, nil
}

// EvalInt parses val as int through the option's expression engine
// (av_opt_eval_int; o 传 Find 到的 Option).
func (o OptObject) EvalInt(opt *Option, val string) (int32, error) {
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
	if o.ptr == nil || opt == nil {
		return 0, errNilOpt
	}
	var out int32
	if ret := fOptEvalInt(o.ptr, opt.ptr, val, &out); ret < 0 {
		return 0, codeErr("av_opt_eval_int", ret)
	}
	return out, nil
}

// EvalInt64 parses val as int64 (av_opt_eval_int64).
func (o OptObject) EvalInt64(opt *Option, val string) (int64, error) {
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
	if o.ptr == nil || opt == nil {
		return 0, errNilOpt
	}
	var out int64
	if ret := fOptEvalInt64(o.ptr, opt.ptr, val, &out); ret < 0 {
		return 0, codeErr("av_opt_eval_int64", ret)
	}
	return out, nil
}

// EvalUint parses val as uint (av_opt_eval_uint).
func (o OptObject) EvalUint(opt *Option, val string) (uint32, error) {
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
	if o.ptr == nil || opt == nil {
		return 0, errNilOpt
	}
	var out uint32
	if ret := fOptEvalUint(o.ptr, opt.ptr, val, &out); ret < 0 {
		return 0, codeErr("av_opt_eval_uint", ret)
	}
	return out, nil
}

// EvalFloat parses val as float (av_opt_eval_float).
func (o OptObject) EvalFloat(opt *Option, val string) (float32, error) {
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
	if o.ptr == nil || opt == nil {
		return 0, errNilOpt
	}
	var out float32
	if ret := fOptEvalFloat(o.ptr, opt.ptr, val, &out); ret < 0 {
		return 0, codeErr("av_opt_eval_float", ret)
	}
	return out, nil
}

// EvalDouble parses val as double (av_opt_eval_double).
func (o OptObject) EvalDouble(opt *Option, val string) (float64, error) {
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
	if o.ptr == nil || opt == nil {
		return 0, errNilOpt
	}
	var out float64
	if ret := fOptEvalDouble(o.ptr, opt.ptr, val, &out); ret < 0 {
		return 0, codeErr("av_opt_eval_double", ret)
	}
	return out, nil
}

// EvalQ parses val as rational (av_opt_eval_q).
func (o OptObject) EvalQ(opt *Option, val string) (AVRational, error) {
	if err := ensureModDictOpt(); err != nil {
		var z1 AVRational
		return z1, err
	}
	if o.ptr == nil || opt == nil {
		return AVRational{}, errNilOpt
	}
	var out AVRational
	if ret := fOptEvalQ(o.ptr, opt.ptr, val, &out); ret < 0 {
		return AVRational{}, codeErr("av_opt_eval_q", ret)
	}
	return out, nil
}

// EvalFlags parses val as flags (av_opt_eval_flags).
func (o OptObject) EvalFlags(opt *Option, val string) (int32, error) {
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
	if o.ptr == nil || opt == nil {
		return 0, errNilOpt
	}
	var out int32
	if ret := fOptEvalFlags(o.ptr, opt.ptr, val, &out); ret < 0 {
		return 0, codeErr("av_opt_eval_flags", ret)
	}
	return out, nil
}

// Find2 looks up an option with unit + flags (av_opt_find2;
// unit 语义同 Find ("" 只找普通选项, "\x00" 不过滤);
// target 传 nil 表不取容器).
func (o OptObject) Find2(name, unit string, optFlags, searchFlags int, target *unsafe.Pointer) *Option {
	mustUse(ensureModDictOpt())
	if o.ptr == nil {
		return nil
	}
	var up unsafe.Pointer
	if unit == "\x00" {
		up = nil
	} else {
		cu, freeCu := featCStrTmp(unit)
		defer freeCu()
		up = cu
	}
	ptr := fOptFind2(o.ptr, name, up, int32(optFlags), int32(searchFlags), target)
	if ptr == nil {
		return nil
	}
	return &Option{ptr: ptr}
}

// FlagIsSet reports whether a flags field has value set
// (av_opt_flag_is_set; fieldName 如 "flags", value 传位值).
func (o OptObject) FlagIsSet(fieldName, value string) bool {
	mustUse(ensureModDictOpt())
	if o.ptr == nil {
		return false
	}
	return fOptFlagIsSet(o.ptr, fieldName, value) != 0
}

// FreeOptions frees an option struct allocated by the library
// (av_opt_free; 一般 frame/codec 上下文不用调, Close 包办).
func FreeOptions(obj unsafe.Pointer) {
	mustUse(ensureModDictOpt())
	if obj == nil {
		return
	}
	fOptFree(obj)
}

// Get reads any option as bytes (av_opt_get; out 用 Mem.Free 放,
// 数字会转成字符串, 再用 Eval* 解析).
func (o OptObject) Get(name string, searchFlags int) (unsafe.Pointer, error) {
	if err := ensureModDictOpt(); err != nil {
		return nil, err
	}
	if o.ptr == nil {
		return nil, errNilOpt
	}
	var out unsafe.Pointer
	if ret := fOptGet(o.ptr, name, int32(searchFlags), &out); ret < 0 {
		return nil, codeErr("av_opt_get", ret)
	}
	return out, nil
}

// GetArray reads array elements [start, start+count) (av_opt_get_array;
// outType 传元素类型码, out 传足够大的缓冲).
func (o OptObject) GetArray(name string, searchFlags int, start, count uint32, outType int32, out unsafe.Pointer) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptGetArray(o.ptr, name, int32(searchFlags), start, count, outType, out); ret < 0 {
		return codeErr("av_opt_get_array", ret)
	}
	return nil
}

// GetArraySize reports an array option's element count
// (av_opt_get_array_size).
func (o OptObject) GetArraySize(name string, searchFlags int) (uint32, error) {
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
	if o.ptr == nil {
		return 0, errNilOpt
	}
	var n uint32
	if ret := fOptGetArrSize(o.ptr, name, int32(searchFlags), &n); ret < 0 {
		return 0, codeErr("av_opt_get_array_size", ret)
	}
	return n, nil
}

// GetChlayout reads a channel-layout option (av_opt_get_chlayout;
// layout 传 32 字节 AVChannelLayout 缓冲, 见 ChannelLayoutDefault).
func (o OptObject) GetChlayout(name string, searchFlags int, layout unsafe.Pointer) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptGetChlayout(o.ptr, name, int32(searchFlags), layout); ret < 0 {
		return codeErr("av_opt_get_chlayout", ret)
	}
	return nil
}

// GetDictVal reads a dict-valued option entry (av_opt_get_dict_val).
func (o OptObject) GetDictVal(name string, searchFlags int) (unsafe.Pointer, error) {
	if err := ensureModDictOpt(); err != nil {
		return nil, err
	}
	if o.ptr == nil {
		return nil, errNilOpt
	}
	var out unsafe.Pointer
	if ret := fOptGetDictVal(o.ptr, name, int32(searchFlags), &out); ret < 0 {
		return nil, codeErr("av_opt_get_dict_val", ret)
	}
	return out, nil
}

// GetImageSize reads a WxH option (av_opt_get_image_size).
func (o OptObject) GetImageSize(name string, searchFlags int) (w, h int32, err error) {
	if err := ensureModDictOpt(); err != nil {
		return 0, 0, err
	}
	if o.ptr == nil {
		return 0, 0, errNilOpt
	}
	if ret := fOptGetImgSize(o.ptr, name, int32(searchFlags), &w, &h); ret < 0 {
		return 0, 0, codeErr("av_opt_get_image_size", ret)
	}
	return w, h, nil
}

// GetKeyValue parses "k=v" into key/value buffers (av_opt_get_key_value;
// ropts 传待解析串的指针槽, keySep/pairSep 传 "="/"、", key/val 用 Mem.Free 放).
func GetKeyValue(ropts *unsafe.Pointer, keySep, pairSep string, flags uint32, key, val *unsafe.Pointer) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if ret := fOptGetKeyValue(ropts, keySep, pairSep, flags, key, val); ret < 0 {
		return codeErr("av_opt_get_key_value", ret)
	}
	return nil
}

// GetPixFmt reads a pixel-format option (av_opt_get_pixel_fmt).
func (o OptObject) GetPixFmt(name string, searchFlags int) (int32, error) {
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
	if o.ptr == nil {
		return PixFmtNone, errNilOpt
	}
	var out int32
	if ret := fOptGetPixFmt(o.ptr, name, int32(searchFlags), &out); ret < 0 {
		return PixFmtNone, codeErr("av_opt_get_pixel_fmt", ret)
	}
	return out, nil
}

// GetSampleFmt reads a sample-format option (av_opt_get_sample_fmt).
func (o OptObject) GetSampleFmt(name string, searchFlags int) (int32, error) {
	if err := ensureModDictOpt(); err != nil {
		return 0, err
	}
	if o.ptr == nil {
		return 0, errNilOpt
	}
	var out int32
	if ret := fOptGetSampleFm(o.ptr, name, int32(searchFlags), &out); ret < 0 {
		return 0, codeErr("av_opt_get_sample_fmt", ret)
	}
	return out, nil
}

// GetVideoRate reads a framerate option (av_opt_get_video_rate).
func (o OptObject) GetVideoRate(name string, searchFlags int) (AVRational, error) {
	if err := ensureModDictOpt(); err != nil {
		var z1 AVRational
		return z1, err
	}
	if o.ptr == nil {
		return AVRational{}, errNilOpt
	}
	var out AVRational
	if ret := fOptGetVideoRat(o.ptr, name, int32(searchFlags), &out); ret < 0 {
		return AVRational{}, codeErr("av_opt_get_video_rate", ret)
	}
	return out, nil
}

// IsDefault reports whether the option still holds its default
// (av_opt_is_set_to_default).
func (o OptObject) IsDefault(opt *Option) bool {
	mustUse(ensureModDictOpt())
	if o.ptr == nil || opt == nil {
		return true
	}
	return fOptIsDefault(o.ptr, opt.ptr) != 0
}

// IsDefaultByName reports default-ness by option name
// (av_opt_is_set_to_default_by_name; searchFlags 传 0).
func (o OptObject) IsDefaultByName(name string, searchFlags int) bool {
	mustUse(ensureModDictOpt())
	if o.ptr == nil {
		return true
	}
	return fOptIsDefByName(o.ptr, name, int32(searchFlags)) != 0
}

// NextOption walks every option on the object (av_opt_next;
// prev 传 nil 开头, 返回 nil 表走完).
func (o OptObject) NextOption(prev *Option) *Option {
	mustUse(ensureModDictOpt())
	if o.ptr == nil {
		return nil
	}
	var pv unsafe.Pointer
	if prev != nil {
		pv = prev.ptr
	}
	ptr := fOptNext(o.ptr, pv)
	if ptr == nil {
		return nil
	}
	return &Option{ptr: ptr}
}

// FieldPtr returns the address of a named field (av_opt_ptr; 第一个参数
// 是对象的 AVClass 表 (对象头 8 字节), 不是对象本身, 传对象会把内存当表读;
// o 须是带 AVClass 的真对象, 表空直接回 nil 不进 C).
func (o OptObject) FieldPtr(name string) unsafe.Pointer {
	mustUse(ensureModDictOpt())
	if o.ptr == nil {
		return nil
	}
	class := *(*unsafe.Pointer)(o.ptr)
	if class == nil {
		return nil
	}
	return fOptPtr(class, o.ptr, name)
}

// QueryRanges lists an option's allowed range (av_opt_query_ranges;
// 返回的 OptionRanges 记得 FreeRanges).
func (o OptObject) QueryRanges(name string, searchFlags int) (*OptionRanges, error) {
	if err := ensureModDictOpt(); err != nil {
		return nil, err
	}
	if o.ptr == nil {
		return nil, errNilOpt
	}
	var r unsafe.Pointer
	if ret := fOptQueryRanges(&r, o.ptr, name, int32(searchFlags)); ret < 0 {
		return nil, codeErr("av_opt_query_ranges", ret)
	}
	if r == nil {
		return nil, errNilOpt
	}
	return &OptionRanges{ptr: r}, nil
}

// QueryRangesDefault lists the default range (av_opt_query_ranges_default).
func (o OptObject) QueryRangesDefault(name string, searchFlags int) (*OptionRanges, error) {
	if err := ensureModDictOpt(); err != nil {
		return nil, err
	}
	if o.ptr == nil {
		return nil, errNilOpt
	}
	var r unsafe.Pointer
	if ret := fOptQueryDef(&r, o.ptr, name, int32(searchFlags)); ret < 0 {
		return nil, codeErr("av_opt_query_ranges_default", ret)
	}
	if r == nil {
		return nil, errNilOpt
	}
	return &OptionRanges{ptr: r}, nil
}

// Serialize dumps all set options to "k=v,k=v" (av_opt_serialize;
// keySep/pairSep 传 '='、"," 最常见, 传 '\0'/相同/反斜杠 C 直接报错;
// out 用 Mem.Free 放).
func (o OptObject) Serialize(optFlags, flags int32, keySep, pairSep byte) (unsafe.Pointer, error) {
	if err := ensureModDictOpt(); err != nil {
		return nil, err
	}
	var out unsafe.Pointer
	if o.ptr == nil {
		return nil, errNilOpt
	}
	if ret := fOptSerialize(o.ptr, optFlags, flags, &out, keySep, pairSep); ret < 0 {
		return nil, codeErr("av_opt_serialize", ret)
	}
	return out, nil
}

// SetArray writes array elements [start, start+count) (av_opt_set_array;
// valType 传元素类型码, val 传元素首地址).
func (o OptObject) SetArray(name string, searchFlags int, start, count uint32, valType int32, val unsafe.Pointer) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetArray(o.ptr, name, int32(searchFlags), start, count, valType, val); ret < 0 {
		return codeErr("av_opt_set_array", ret)
	}
	return nil
}

// SetBin writes raw bytes (av_opt_set_bin).
func (o OptObject) SetBin(name string, data unsafe.Pointer, size, searchFlags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetBin(o.ptr, name, data, int32(size), int32(searchFlags)); ret < 0 {
		return codeErr("av_opt_set_bin", ret)
	}
	return nil
}

// SetDefaults2 resets options matching a mask (av_opt_set_defaults2;
// mask 上为 1 的位才重置, 0 全重置).
func (o OptObject) SetDefaults2(mask, flags int32) {
	mustUse(ensureModDictOpt())
	if o.ptr == nil {
		return
	}
	fOptSetDefault2(o.ptr, mask, flags)
}

// SetDict applies a whole dictionary at once (av_opt_set_dict;
// C 吃掉整个字典再把没吃掉的装回去: 调完 holder 里是野指针, 别再碰它,
// 需要的话重建; 没吃掉的进 options, 吃完的字典会清空).
func (o OptObject) SetDict(options *Dictionary) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	var dp unsafe.Pointer
	if options != nil {
		dp = options.ptr
	}
	if ret := fOptSetDict(o.ptr, &dp); ret < 0 {
		return codeErr("av_opt_set_dict", ret)
	}
	if options != nil {
		options.ptr = nil
	}
	return nil
}

// SetDict2 applies a dictionary with flags (av_opt_set_dict2;
// 所有权语义同 SetDict: 调完 holder 作废).
func (o OptObject) SetDict2(options *Dictionary, searchFlags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	var dp unsafe.Pointer
	if options != nil {
		dp = options.ptr
	}
	if ret := fOptSetDict2(o.ptr, &dp, int32(searchFlags)); ret < 0 {
		return codeErr("av_opt_set_dict2", ret)
	}
	if options != nil {
		options.ptr = nil
	}
	return nil
}

// SetDictVal writes one dict-valued entry (av_opt_set_dict_val).
func (o OptObject) SetDictVal(name string, val unsafe.Pointer, searchFlags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetDictVal(o.ptr, name, val, int32(searchFlags)); ret < 0 {
		return codeErr("av_opt_set_dict_val", ret)
	}
	return nil
}

// SetFromString parses "k=v,k=v" onto the object (av_opt_set_from_string;
// shorthand 传 "" 不用简写, keySep/pairSep 传 "="/"、",".
func (o OptObject) SetFromString(opts, shorthand, keySep, pairSep string, searchFlags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetFromStr(o.ptr, opts, shorthand, keySep, pairSep, int32(searchFlags)); ret < 0 {
		return codeErr("av_opt_set_from_string", ret)
	}
	return nil
}

// SetImageSize writes a WxH option (av_opt_set_image_size).
func (o OptObject) SetImageSize(name string, w, h, searchFlags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetImgSize(o.ptr, name, int32(w), int32(h), int32(searchFlags)); ret < 0 {
		return codeErr("av_opt_set_image_size", ret)
	}
	return nil
}

// SetChlayout writes a channel-layout option (av_opt_set_chlayout;
// layout 传 32 字节 AVChannelLayout 缓冲, 见 ChannelLayoutDefault).
func (o OptObject) SetChlayout(name string, layout unsafe.Pointer, searchFlags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetChlayout(o.ptr, name, layout, int32(searchFlags)); ret < 0 {
		return codeErr("av_opt_set_chlayout", ret)
	}
	return nil
}

// SetSampleFmt writes a sample-format option (av_opt_set_sample_fmt).
func (o OptObject) SetSampleFmt(name string, sampleFmt, searchFlags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetSampleFm(o.ptr, name, int32(sampleFmt), int32(searchFlags)); ret < 0 {
		return codeErr("av_opt_set_sample_fmt", ret)
	}
	return nil
}

// SetPixFmt writes a pixel-format option (av_opt_set_pixel_fmt).
func (o OptObject) SetPixFmt(name string, pixFmt, searchFlags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetPixFmt(o.ptr, name, int32(pixFmt), int32(searchFlags)); ret < 0 {
		return codeErr("av_opt_set_pixel_fmt", ret)
	}
	return nil
}

// SetVideoRate writes a framerate option (av_opt_set_video_rate).
func (o OptObject) SetVideoRate(name string, rate AVRational, searchFlags int) error {
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if o.ptr == nil {
		return errNilOpt
	}
	if ret := fOptSetVideoRat(o.ptr, name, rate, int32(searchFlags)); ret < 0 {
		return codeErr("av_opt_set_video_rate", ret)
	}
	return nil
}

// ShowOptions dumps the option list for debugging (av_opt_show2;
// reqFlags 传想要的, rejFlags 传不要的, 都传 0 表全打).
func (o OptObject) ShowOptions(logObj unsafe.Pointer, reqFlags, rejFlags int32) {
	mustUse(ensureModDictOpt())
	if o.ptr == nil {
		return
	}
	fOptShow2(o.ptr, logObj, reqFlags, rejFlags)
}

// ChildClassIterate walks child option classes (av_opt_child_class_iterate;
// 第一个参数是父对象的 AVClass 表 (对象头 8 字节), 不是对象本身;
// opaque 传 nil 开头; C 内不判空, 表空直接回 nil 不进 C).
func ChildClassIterate(parent OptObject, opaque *unsafe.Pointer) unsafe.Pointer {
	if ensureModDictOpt() != nil || parent.ptr == nil {
		return nil
	}
	class := *(*unsafe.Pointer)(parent.ptr)
	if class == nil {
		return nil
	}
	return fOptChildIter(class, opaque)
}

// ChildNext walks the next child object (av_opt_child_next;
// prev 传 nil 开头).
func (o OptObject) ChildNext(prev unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModDictOpt())
	if o.ptr == nil {
		return nil
	}
	return fOptChildNext(o.ptr, prev)
}
