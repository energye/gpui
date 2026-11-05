//go:build linux && !nogpu

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// h19ReproRun 是 H19 的最小复现序列：先起一台 GL 呈现设备（真窗那条路），再起一台
// 离屏设备并让它建形状管线。修复前第二台会把第一台的共享 clip/mask bind group layout
// 喂给自己的 CreatePipelineLayout ⇒ wgpu-native 非展开 panic ⇒ 整进程 SIGABRT。
const h19ReproRun = "^TestP1GLPresentX11OnlineSmoke$|^TestP37ProjectionCounters$"

// TestP376H19DeviceSwapRunsToCompletion 把那条中止钉成门禁。
//
// 必须在**子进程**里跑：被钉的失败模式是 SIGABRT，在同进程里一旦发生，整个测试
// 二进制直接没了，后面的用例一起被带走（这正是逐文件口径看不见它的原因）。
// 子进程把「中止」变成一个可断言的外部事实：退出码 + 输出文本。
//
// 负向（本轮实测）：摘掉 initGPU 借设备分支里的 installDeviceGuardLocked 调用
// ⇒ 本钉红（输出里出现 invalid bind group layout / 非展开 panic）。
func TestP376H19DeviceSwapRunsToCompletion(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY：H19 那条路要先起一台真窗 GL 设备")
	}
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("no WGPU_NATIVE_PATH：子进程起不了 webgpu-native 离屏设备")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("无 go 工具链，跑不了子进程复现: %v", err)
	}

	cmd := exec.Command(goBin, "test", "-count=1", "-v", "-timeout", "10m", "-run", h19ReproRun, "github.com/energye/gpui/render")
	cmd.Dir = "." // 包目录本身，模块根向上自动发现，不硬编码本机路径
	cmd.Env = os.Environ()
	out, runErr := cmd.CombinedOutput()
	text := string(out)

	if strings.Contains(text, "non-unwinding panic") || strings.Contains(text, "invalid bind group layout") {
		t.Fatalf("H19 复发：同进程换设备又把旧设备的共享 layout 喂给了新设备\n--- 子进程输出尾 ---\n%s", h19TailOutput(text, 12))
	}
	if strings.Contains(text, "no test files") || strings.Contains(text, "no tests to run") {
		t.Fatal("子进程没跑到那两枚测试，钉是空的")
	}
	if strings.Contains(text, "--- SKIP:") {
		t.Fatalf("子进程里那两枚测试被跳过了，钉是空的（不能拿 skip 当过）:\n%s", h19TailOutput(text, 20))
	}
	if !strings.Contains(text, "--- PASS: TestP1GLPresentX11OnlineSmoke") ||
		!strings.Contains(text, "--- PASS: TestP37ProjectionCounters") {
		t.Fatalf("子进程里那两枚测试没双双真跑完，钉是空的:\n%s", h19TailOutput(text, 20))
	}
	if runErr != nil {
		t.Fatalf("子进程没跑绿: %v\n--- 子进程输出尾 ---\n%s", runErr, h19TailOutput(text, 20))
	}
	if !strings.Contains(text, "ok  \tgithub.com/energye/gpui/render") {
		t.Fatalf("子进程输出里没有 ok 行，无法证明那条序列真跑完: %s", h19TailOutput(text, 10))
	}
}

func h19TailOutput(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
