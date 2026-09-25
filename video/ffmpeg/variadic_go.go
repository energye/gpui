package ffmpeg

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
)

// 变参四件套的 Go 拼串版：C 的变参 purego 调不准，改在 Go 里拼好再调。
//
// 大白话：这 4 个函数都要先拼字符串再干活（拼地址、拼日志、写文件、
// 接字符串）。C 那边是边拼边干，Go 这边分成两步：先用 fmt.Sprintf
// 在 Go 里拼好，再调一个不带变参的函数干活。拼出来的东西和 C 拼的
// 一个样，6 个平台行为一致。
//
// 为啥不直调：purego 的 ...any 只是把 Go 参数一个个放到寄存器里，
// 不是 C 的变参。实测整数和字符串能混过去，浮点必错（1.5 变成 0.0），
// 因为 C 要求 float 先升成 double，还要置个数寄存器，purego 两样都
// 不做。4 个函数全是 printf 风格，早晚会收到浮点，直调就是埋错。

var (
	// 定长三参版 av_log：只传拼好的无百分号字符串，不带变参。
	fAvLogPlain func(unsafe.Pointer, int32, string)
	// 字符串版 av_strlcat：src 走 string，purego 负责转 C 字符串。
	fAvStrlcatStr func(unsafe.Pointer, string, uintptr) uintptr
)

func registerVariadicGo(h uintptr) {
	purego.RegisterLibFunc(&fAvLogPlain, h, "av_log")
	purego.RegisterLibFunc(&fAvStrlcatStr, h, "av_strlcat")
}

// Asprintf 拼出字符串（替 av_asprintf；C 版用 av_malloc 分配，Go 版
// 直接回 Go 字符串，不用管 C 内存）。
func Asprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

// Strlcatf 往 dst 尾巴上拼接拼好的字符串（替 av_strlcatf；dst 是 C
// 缓冲，size 是总容量，含结尾零；回值和 C 一样是拼接后的总长度）。
func (Util) Strlcatf(dst unsafe.Pointer, size uintptr, format string, args ...any) uintptr {
	if ensureLoaded() != nil || dst == nil || size == 0 {
		return 0
	}
	return fAvStrlcatStr(dst, fmt.Sprintf(format, args...), size)
}

// Printf 往动态输出流里写拼好的字节（替 avio_printf；调用方先
// OpenDynBuf 开流，写完必须 CloseDynBuf 收尾并取内容，CloseDynBuf
// 会把流连同缓冲一起管好，调用方最后用 Mem.Free 放缓冲，见单测）。
func (x *IOContext) Printf(format string, args ...any) int {
	if x == nil || x.ptr == nil {
		return 0
	}
	if ensureLoaded() != nil {
		return 0
	}
	b := []byte(fmt.Sprintf(format, args...))
	if len(b) == 0 {
		return 0
	}
	fAvioWrite(x.ptr, unsafe.Pointer(&b[0]), int32(len(b)))
	return len(b)
}

// Logf 发一条日志（替 av_log(fmt, ...)；拼好的字符串里有百分号就先
// 转成双百分号再传，C 那边不再读变参，就不会乱读寄存器）。
func (Log) Logf(avcl unsafe.Pointer, level int32, format string, args ...any) {
	if ensureLoaded() != nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if strings.Contains(msg, "%") {
		msg = strings.ReplaceAll(msg, "%", "%%")
	}
	fAvLogPlain(avcl, level, msg)
}

// Once 只打一次日志（替 av_log_once；state 传计数器指针，nil 当已打
// 过；第一次走 initialLevel，之后走 subsequentLevel，打完置 1）。
func (Log) Once(avcl unsafe.Pointer, initialLevel, subsequentLevel int32, state *int32, format string, args ...any) {
	if ensureLoaded() != nil {
		return
	}
	level := subsequentLevel
	if state == nil {
		fAvLogPlain(avcl, level, escapePercents(fmt.Sprintf(format, args...)))
		return
	}
	if *state == 0 {
		level = initialLevel
	}
	*state = 1
	fAvLogPlain(avcl, level, escapePercents(fmt.Sprintf(format, args...)))
}

// BprintfF 往打印缓冲里追加拼好的字符串（替 av_bprintf(buf, fmt, ...)；
// BPrint 走 AppendData，不经过 C 变参）。
func (b *BPrint) BprintfF(format string, args ...any) {
	if b == nil || b.ptr == nil {
		return
	}
	b.AppendData(fmt.Sprintf(format, args...))
}

// escapePercents 把拼好的字符串里的百分号 doubling，C 定长调用才安全。
func escapePercents(s string) string {
	if strings.Contains(s, "%") {
		return strings.ReplaceAll(s, "%", "%%")
	}
	return s
}
