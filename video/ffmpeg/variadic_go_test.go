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
	"testing"
	"unsafe"
)

// 大白话：C 那边是边拼边干，Go 这边先拼好再干。这里验拼出来的
// 东西和 C 干出来的一个样，浮点也不错。
func TestVariadicGo(t *testing.T) {
	if !Available() {
		t.Skipf("lib missing: %s", LibPath())
	}
	// Asprintf：纯拼串，浮点也对。
	if got := Asprintf("%d %s", 42, "hi"); got != "42 hi" {
		t.Fatalf("Asprintf = %q", got)
	}
	if got := Asprintf("%.2f", 1.5); got != "1.50" {
		t.Fatalf("Asprintf float = %q", got)
	}
	// Strlcatf：往 C 缓冲尾巴上拼。
	var m Mem
	buf := m.Alloc(64)
	if buf == nil {
		t.Fatal("alloc nil")
	}
	defer m.Free(buf)
	*(*byte)(buf) = 0
	var u Util
	if n := u.Strlcatf(buf, 64, "ab,%d", 7); n != 4 {
		t.Fatalf("Strlcatf n = %d", n)
	}
	if got := cstr(buf); got != "ab,7" {
		t.Fatalf("Strlcatf buf = %q", got)
	}
	// BprintfF：往打印缓冲里追加。
	bp := NewBPrint(64, 4096)
	if bp == nil {
		t.Fatal("bprint nil")
	}
	defer bp.Free()
	bp.BprintfF("%d %s %.1f", 42, "hi", 1.5)
	out, err := bp.Finalize()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Free(out)
	if got := cstr(out); got != "42 hi 1.5" {
		t.Fatalf("BprintfF = %q", got)
	}
	// Printf：往动态流里写，CloseDynBuf 收尾取内容。
	var fc FormatContext
	var ioptr unsafe.Pointer
	if err := fc.OpenDynBuf(&ioptr); err != nil {
		t.Fatal(err)
	}
	io := &IOContext{}
	*(*unsafe.Pointer)(unsafe.Pointer(io)) = ioptr
	if wrote := io.Printf("%d %s", 42, "hi"); wrote != 5 {
		t.Fatalf("Printf wrote = %d", wrote)
	}
	var cbuf unsafe.Pointer
	size, err := io.CloseDynBuf(&cbuf)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Free(cbuf)
	if size != 5 {
		t.Fatalf("dyn size = %d", size)
	}
	if got := string(unsafe.Slice((*byte)(cbuf), size)); got != "42 hi" {
		t.Fatalf("dyn content = %q", got)
	}
	// Once/Logf：只验不崩，日志走测试输出。
	var lg Log
	var st int32
	lg.Once(nil, LogError, LogError, &st, "once %d", 1)
	if st != 1 {
		t.Fatalf("Once state = %d", st)
	}
	lg.Once(nil, LogError, LogError, &st, "once %d", 2)
	lg.Logf(nil, LogError, "plain %s", "msg")
	lg.Logf(nil, LogError, "100%% sure %d%%", 50)
}
