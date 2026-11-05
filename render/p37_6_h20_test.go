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

// h20ReproRun 是 H20 的最小复现对：TestMem_T2_ResetAccelerator 重置 accelerator
// （关掉旧 GPUShared），TestP04_ApplyDropShadowGPU 随后走滤镜 GPU 腿。修复前
// 滤镜图全局钩子的闭包还捕获着被关掉的那份 shared——P04 的滤镜 op 复活它、在
// 上面装新设备并发布视图，读回却走当前 shared 的设备 ⇒ 跨设备句柄喂给
// CopyTextureToBuffer ⇒ wgpu-core "no longer alive" 非 unwind panic ⇒ 整进程
// SIGABRT。
const h20ReproRun = "^TestMem_T2_ResetAccelerator$|^TestP04_ApplyDropShadowGPU$"

// TestP376H20DeviceResetRunsToCompletion 把那条中止钉成门禁。
//
// 必须在**子进程**里跑（同 H19 钉的口径）：被钉的失败模式是 SIGABRT，同进程里
// 一旦发生整个测试二进制直接没了。子进程把「中止」变成可断言的外部事实。
//
// 负向（本轮实测）：把 Close 的钩子注销块与三处 closed 守卫一起摘掉 ⇒ 本钉红
// （输出里出现 no longer alive / 非展开 panic）；只摘其中一条 ⇒ 本钉绿（两条是
// 纵深：注销管「钩子不再指死 shared」，closed 守卫管「死了的 shared 拿不到
// 设备」），各自的单钉分别在 internal/gpu 的两枚钉上。
func TestP376H20DeviceResetRunsToCompletion(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("no WGPU_NATIVE_PATH：子进程起不了 webgpu-native 离屏设备")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("无 go 工具链，跑不了子进程复现: %v", err)
	}

	cmd := exec.Command(goBin, "test", "-count=1", "-v", "-timeout", "10m", "-run", h20ReproRun, "github.com/energye/gpui/render")
	cmd.Dir = "." // 包目录本身，模块根向上自动发现，不硬编码本机路径
	cmd.Env = os.Environ()
	out, runErr := cmd.CombinedOutput()
	text := string(out)

	if strings.Contains(text, "non-unwinding panic") || strings.Contains(text, "no longer alive") {
		t.Fatalf("H20 复发：重置 accelerator 后滤镜钩子又把已关闭的 shared 复活了\n--- 子进程输出尾 ---\n%s", h20TailOutput(text, 12))
	}
	if strings.Contains(text, "no test files") || strings.Contains(text, "no tests to run") {
		t.Fatal("子进程没跑到那两枚测试，钉是空的")
	}
	if strings.Contains(text, "--- SKIP:") {
		t.Fatalf("子进程里那两枚测试被跳过了，钉是空的（不能拿 skip 当过）:\n%s", h20TailOutput(text, 20))
	}
	if !strings.Contains(text, "--- PASS: TestMem_T2_ResetAccelerator") ||
		!strings.Contains(text, "--- PASS: TestP04_ApplyDropShadowGPU") {
		t.Fatalf("子进程里那两枚测试没双双真跑完，钉是空的:\n%s", h20TailOutput(text, 20))
	}
	if runErr != nil {
		t.Fatalf("子进程没跑绿: %v\n--- 子进程输出尾 ---\n%s", runErr, h20TailOutput(text, 20))
	}
	if !strings.Contains(text, "ok  \tgithub.com/energye/gpui/render") {
		t.Fatalf("子进程输出里没有 ok 行，无法证明那条序列真跑完: %s", h20TailOutput(text, 10))
	}
}

func h20TailOutput(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
