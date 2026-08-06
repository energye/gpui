package hint

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ftexpBin 解析 FT 度量衡二进制路径，优先级：
//
//  1. $FTEXP_BIN 环境变量（CI/换机可指定已构建路径）；
//  2. testdata/ftexp/ftexp（若仓库内已有构建产物）；
//  3. 拷贝 testdata/ftexp 源码到临时目录，go build 自动重建。
//
// 任一路径不可用都返回空串，调用方据此 t.Skipf（禁止假绿）。
func ftexpBin(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("FTEXP_BIN"); p != "" {
		return p
	}
	const srcRel = "testdata/ftexp"
	if st, err := os.Stat(filepath.Join(srcRel, "ftexp")); err == nil && !st.IsDir() {
		return filepath.Join(srcRel, "ftexp")
	}
	tmp := t.TempDir()
	for _, name := range []string{"main.go", "go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(srcRel, name))
		if err != nil {
			t.Skipf("ftexp source missing (%s): %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(tmp, name), data, 0o644); err != nil {
			t.Fatalf("write ftexp source %s: %v", name, err)
		}
	}
	bin := filepath.Join(tmp, "ftexp")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ftexp rebuild failed (need go + purego): %v %s", err, out)
	}
	return bin
}