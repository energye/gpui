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
	"unsafe"
)

// 数据常量模块：so 里剩下 16 个数据符号，全是只读数据，不是函数。
//
// 大白话：前面 991 个都是函数，直接调就行。剩下这 16 个是库里放好的
// 数字和字符串，比如加解密上下文要占多少字节、每块库的版本号。
// 函数用 RegisterLibFunc 绑，数据用 Dlsym 取地址再读，两条路都能用。

// dataAddr 按名字取数据符号的地址，拿不到回 nil。
func dataAddr(name string) unsafe.Pointer {
	if ensureLoaded() != nil {
		return nil
	}
	loadMu.Lock()
	h := libHandle
	loadMu.Unlock()
	if h == 0 {
		return nil
	}
	addr, err := symAddr(h, name)
	if err != nil || addr == 0 {
		return nil
	}
	// 同 Log.SetGoCallback 的写法：经变量中转，不直转 uintptr，
	// 过 go vet 的 unsafeptr 检查。
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}

// dataInt32 读 const int 数据，拿不到回 0。
func dataInt32(name string) int32 {
	p := dataAddr(name)
	if p == nil {
		return 0
	}
	return *(*int32)(p)
}

// dataString 读 const char[] 数据，拿不到回空串。
func dataString(name string) string { return cstr(dataAddr(name)) }

// AesSize 是 AES 上下文占多少字节（av_aes_size，配 av_aes_alloc 用）。
func (Crypto) AesSize() int { return int(dataInt32("av_aes_size")) }

// CamelliaSize 是 Camellia 上下文占多少字节（av_camellia_size）。
func (Crypto) CamelliaSize() int { return int(dataInt32("av_camellia_size")) }

// Cast5Size 是 CAST5 上下文占多少字节（av_cast5_size）。
func (Crypto) Cast5Size() int { return int(dataInt32("av_cast5_size")) }

// TeaSize 是 TEA 上下文占多少字节（av_tea_size）。
func (Crypto) TeaSize() int { return int(dataInt32("av_tea_size")) }

// TwofishSize 是 Twofish 上下文占多少字节（av_twofish_size）。
func (Crypto) TwofishSize() int { return int(dataInt32("av_twofish_size")) }

// Md5Size 是 MD5 上下文占多少字节（av_md5_size）。
func (Crypto) Md5Size() int { return int(dataInt32("av_md5_size")) }

// RipemdSize 是 RIPEMD 上下文占多少字节（av_ripemd_size）。
func (Crypto) RipemdSize() int { return int(dataInt32("av_ripemd_size")) }

// ShaSize 是 SHA 上下文占多少字节（av_sha_size）。
func (Crypto) ShaSize() int { return int(dataInt32("av_sha_size")) }

// Sha512Size 是 SHA512 上下文占多少字节（av_sha512_size）。
func (Crypto) Sha512Size() int { return int(dataInt32("av_sha512_size")) }

// TreeNodeSize 是一个树节点占多少字节（av_tree_node_size，配树操作看内存用）。
func (Util) TreeNodeSize() int { return int(dataInt32("av_tree_node_size")) }

// CodecFfversion 是编解码库的版本串（av_codec_ffversion，如 FFmpeg version 7.1.5）。
func (Library) CodecFfversion() string { return dataString("av_codec_ffversion") }

// DeviceFfversion 是设备库的版本串（av_device_ffversion）。
func (Library) DeviceFfversion() string { return dataString("av_device_ffversion") }

// FilterFfversion 是滤镜库的版本串（av_filter_ffversion）。
func (Library) FilterFfversion() string { return dataString("av_filter_ffversion") }

// FormatFfversion 是封装库的版本串（av_format_ffversion）。
func (Library) FormatFfversion() string { return dataString("av_format_ffversion") }

// UtilFfversion 是工具库的版本串（av_util_ffversion）。
func (Library) UtilFfversion() string { return dataString("av_util_ffversion") }

// SwrFfversion 是重采样库的版本串（swr_ffversion）。
func (Library) SwrFfversion() string { return dataString("swr_ffversion") }
