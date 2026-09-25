// Command ffmpeg-recipe 打印 gpui 自建单文件库的两档配方（基础版 base / 高级版 full）。
//
// 大白话：编库的开关太多，散在脚本里容易改错，这里是唯一真源。
// 基础版只管看片（解码+拆盒+常用滤镜），高级版在基础版上加写文件
// （复用+编码+烧字）。两档都不碰 GPL（x264/x265/fdk 一个不开），
// 许可保持 LGPL 2.1。实际编译在 docker 和 GitHub Actions 里跑，
// 本文件只管配方字符串，拼好给脚本用。
//
// 用法：go run ./tools/ffmpeg-recipe -variant base|full -target linux-x64
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// 基础版解码器：现 28 个 + 老片声音字幕补齐（全原生，无外库）。
// 新加的是以前播老片网盘片会打不开的那堆。
var baseDecoders = []string{
	// 现有的 28 个，一个不少。
	"h264", "hevc", "vp9", "av1", "mpeg4", "mpeg2video", "vp8", "theora",
	"mjpeg", "vc1", "wmv3", "dvvideo", "prores", "dnxhd",
	"aac", "mp3", "opus", "vorbis", "flac", "ac3", "alac", "pcm_s16le",
	"png", "gif", "webp", "srt", "ass", "webvtt",
	// 新增视频（老片编码，全原生）。
	"mpeg1video", "h261", "h263", "msmpeg4v2", "msmpeg4v3", "wmv1", "wmv2",
	"vp6", "vp6a", "vp6f", "flashsv", "flashsv2", "rv10", "rv20", "rv30", "rv40",
	"huffyuv", "ffv1", "qtrle", "roq", "cinepak", "fraps", "smc", "cyuv", "rpza",
	"v210", "v410", "speedhq", "hap", "snow", "utvideo", "dirac",
	"exr", "tiff", "jpegls", "jpeg2000", "pixlet", "pictor",
	// 新增声音（老片音轨，全原生）。
	"eac3", "mlp", "truehd", "dca", "amrnb", "amrwb", "ape", "mpc7", "mpc8",
	"wavpack", "tta", "wmalossless", "wmavoice", "qdm2", "qdmc", "ralf",
	"aptx", "sbc", "g729", "ilbc", "sipr", "sonic", "imc", "bonk", "dfpwm",
	// 新增字幕轨（只读不烧，全原生）。
	"movtext", "dvbsub", "dvdsub", "pgssub", "xsub",
}

// 基础版拆盒器：现 25 个 + 老片盒子补齐（全原生）。
var baseDemuxers = []string{
	// 现有的 25 个，一个不少。
	"mov", "matroska", "mpegts", "flv", "hls", "avi", "asf", "mpegps",
	"image2", "concat", "flac", "ape", "mpc", "dts", "mxf",
	"srt", "ass", "webvtt", "wav", "ogg", "mp3", "aac", "rtsp", "rtp", "dash",
	// 新增盒子（老片网盘片，全原生）。
	"rm", "aiff", "caf", "au", "tta", "wv", "w64", "dsf", "r3d", "gxf", "nut",
	"oma", "pva", "wtv", "yuv4mpegpipe", "rawvideo", "mjpeg", "m4v",
	"h261", "h263", "h264", "hevc", "ivf", "amr", "amrnb", "amrwb",
	"mm", "mmf", "nuv", "nsv", "vqf", "wc3", "wve", "xmv", "xwma",
	"filmstrip", "gif", "v210", "vag", "vc1", "pmp", "gxf", "mxf",
	"aax", "aa", "acm", "adf", "adx", "aea", "afc", "aix", "alp", "anm",
	"apng", "aptx", "aqtitle", "argo_asf", "bethsoftvid", "bfi", "bink",
	"cdg", "cdxl", "cine", "dcstr", "dfa", "dhav", "dirac", "dnxhd",
	"dsicin", "dss", "dv", "dvbsub", "dxa", "ea", "eac3", "ffmetadata",
	"fits", "flic", "g726", "g729", "gsm", "hca", "hcom", "hnm", "ico",
	"idcin", "iff", "ingenient", "ipmovie", "ipu", "iv8", "ivr", "jv",
	"kux", "kvag", "lmlm4", "loas", "lrc", "lvf", "lxf", "mgsts",
	"microdvd", "mpl2", "mpsub", "msf", "mtv", "musx", "mv", "mvi",
	"mxg", "nc", "nistsphere", "nsp", "nut", "obu", "ogg", "oma",
	"paf", "qcp", "redspark", "rl2", "roq", "rpl", "rsd", "rso",
	"s337m", "sami", "sbg", "scc", "sdns", "ser", "siff", "sln",
	"smjpeg", "smush", "sol", "sox", "spdif", "stl", "subviewer", "subviewer1",
	"sup", "svag", "svs", "tak", "tedcaptions", "thp", "tmv", "truehd",
	"tty", "txd", "ty", "vag", "vividas", "vpk", "vplayer", "wsaud", "wsd",
	"xa", "xvag", "yop",
}

// 基础版切帧器：解码器加了，切帧器得跟上（全原生）。
var baseParsers = []string{
	"h264", "hevc", "vp9", "av1", "mpegaudio", "opus", "vorbis",
	"mpeg4video", "mpegvideo", "aac", "vp8", "mjpeg", "png",
	"flac", "dca", "ac3", "mlp", "amr", "g729",
	"gsm", "h261", "h263", "mjpeg", "dirac", "dnxhd", "dvbsub", "dvdsub",
}

// 基础版码流过滤：字幕轨识别要用的加上（全原生）。
var baseBSFs = []string{
	"h264_mp4toannexb", "hevc_mp4toannexb", "aac_adtstoasc",
	"vp9_superframe", "av1_frame_merge", "extract_extradata", "dump_extradata",
	"dts2pts", "noise", "mov2textsub", "text2movsub", "null",
}

// 基础版滤镜：现有 38 个不动，只管看片。
var baseFilters = []string{
	"crop", "pad", "overlay", "rotate", "hue",
	"yadif", "bwdif", "thumbnail", "split", "select", "trim", "concat",
	"fps", "scale", "format", "transpose", "unsharp", "gblur", "noise", "deband",
	"deshake", "tonemap",
	"aresample", "aformat", "hwupload", "hwdownload", "hwmap",
	"loudnorm", "acompressor", "amix", "pan", "volume",
	"showspectrum", "volumedetect", "sine", "anoisesrc", "aevalsrc",
}

// 基础版协议：现有 13 个 + 直播加密补齐（全原生）。
var baseProtocols = []string{
	"file", "http", "https", "tcp", "udp", "tls", "data", "pipe",
	"cache", "concat", "subfile", "ftp", "rtmp",
	"mmsh", "mmst", "crypto", "rtp",
}

// 高级版在基础版上加的三层（全原生 + BSD/LGPL 外库，GPL 三个一个不开）。
// 写盒 181 个全开：configure 不支持 --enable-muxer=all？支持，写 all。
// 原生编码 182 个全开：--enable-encoder=all 会把外库编码也选上吗？
// 不会，没开的外库 configure 直接跳过，只开原生的，所以用 all 安全。
// GPL 三个（libx264/libx265/libfdk_aac）连开关都不加，许可保持 LGPL。
// 烧字四个（freetype/harfbuzz/fontconfig/libass）+ fribidi 建议 + openh264：
// 全静态编进单文件，用户机器不用装。
var fullExtraDecoders = []string{} // 基础版已是全原生解码，高级版不用再加。
var fullExtraDemuxers = []string{} // 同上。

func join(prefix string, names []string) string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, prefix+n)
	}
	return strings.Join(out, " ")
}

func baseFlags() string {
	return strings.Join([]string{
		"--disable-everything --disable-programs --disable-doc",
		"--disable-encoders --disable-muxers", // 基础版只管看，不管写。
		"--enable-avdevice --enable-avfilter --enable-network --enable-zlib",
		join("--enable-decoder=", baseDecoders),
		// 图片编码 4 个留着（截图用，不算写片）。
		"--enable-encoder=png --enable-encoder=mjpeg --enable-encoder=bmp --enable-encoder=gif",
		join("--enable-demuxer=", baseDemuxers),
		join("--enable-parser=", baseParsers),
		"--enable-protocol=file --enable-protocol=http --enable-protocol=https",
		"--enable-protocol=tcp --enable-protocol=udp --enable-protocol=tls",
		"--enable-protocol=data --enable-protocol=pipe --enable-protocol=cache",
		"--enable-protocol=concat --enable-protocol=subfile --enable-protocol=ftp",
		"--enable-protocol=rtmp --enable-protocol=mmsh --enable-protocol=mmst",
		"--enable-protocol=crypto --enable-protocol=rtp",
		join("--enable-bsf=", baseBSFs),
		join("--enable-filter=", baseFilters),
		"--enable-indev=lavfi",
		"--disable-x86asm --enable-pic --enable-small",
		"--disable-autodetect --disable-debug",
	}, " ")
}

func fullFlags() string {
	return strings.Join([]string{
		"--disable-everything --disable-programs --disable-doc",
		// 高级版开写：复用器全开（181 个，原生），编码器全开（没开的外库自动跳过）。
		"--enable-muxer=all --enable-encoder=all",
		"--enable-avdevice --enable-avfilter --enable-network --enable-zlib",
		join("--enable-decoder=", baseDecoders),
		join("--enable-demuxer=", baseDemuxers),
		join("--enable-parser=", baseParsers),
		"--enable-protocol=file --enable-protocol=http --enable-protocol=https",
		"--enable-protocol=tcp --enable-protocol=udp --enable-protocol=tls",
		"--enable-protocol=data --enable-protocol=pipe --enable-protocol=cache",
		"--enable-protocol=concat --enable-protocol=subfile --enable-protocol=ftp",
		"--enable-protocol=rtmp --enable-protocol=mmsh --enable-protocol=mmst",
		"--enable-protocol=crypto --enable-protocol=rtp",
		join("--enable-bsf=", baseBSFs),
		join("--enable-filter=", baseFilters),
		// 烧字两个滤镜：drawtext/subtitles（ass 跟 subtitles 一起进）。
		"--enable-filter=drawtext --enable-filter=subtitles --enable-filter=ass",
		"--enable-indev=lavfi",
		// 烧字外库（LGPL/BSD/ISC，许可干净）：freetype+harfbuzz 必须，
		// fontconfig+fribidi 建议（中东文字排版），libass 烧字幕必须，
		// openh264 做 H264 编码（BSD）。
		"--enable-libfreetype --enable-libharfbuzz --enable-libfontconfig",
		"--enable-libfribidi --enable-libass --enable-libopenh264",
		"--disable-x86asm --enable-pic --enable-small",
		"--disable-autodetect --disable-debug",
	}, " ")
}

func main() {
	variant := flag.String("variant", "base", "base|full")
	flag.Parse()
	switch *variant {
	case "base":
		fmt.Println(baseFlags())
	case "full":
		fmt.Println(fullFlags())
	default:
		fmt.Fprintf(os.Stderr, "unknown variant %q\n", *variant)
		os.Exit(1)
	}
}
