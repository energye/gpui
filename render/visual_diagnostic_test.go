package render

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func visualRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if filepath.Base(wd) == "render" {
		return filepath.Dir(wd)
	}
	return wd
}

func runVisualCommand(t *testing.T, repoRoot string, pkg string, goArgs []string, out string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	args := append([]string{"run"}, goArgs...)
	args = append(args, pkg, "-out", out)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go %s failed: %v\nstdout:\n%s\nstderr:\n%s",
			strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	if ctx.Err() != nil {
		t.Fatalf("go %s timed out", strings.Join(args, " "))
	}
	return strings.TrimSpace(stdout.String() + "\n" + stderr.String())
}
