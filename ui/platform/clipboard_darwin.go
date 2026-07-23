//go:build darwin

package platform

import (
	"bytes"
	"os/exec"
	"strings"
)

// NewSystemClipboard returns the macOS system clipboard adapter
// (pbcopy/pbpaste + memory fallback).
func NewSystemClipboard() Clipboard {
	return NewFallbackClipboard(&darwinClipPrimary{})
}

type darwinClipPrimary struct{}

func (darwinClipPrimary) ReadText() (string, bool) {
	cmd := exec.Command("pbpaste")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func (darwinClipPrimary) WriteText(s string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(s)
	if err := cmd.Run(); err != nil {
		// rare: try with explicit encoding
		cmd2 := exec.Command("pbcopy")
		cmd2.Stdin = bytes.NewReader([]byte(s))
		return cmd2.Run()
	}
	return nil
}

// NewXClipClipboard is a cross-build alias (Linux name) → system clipboard.
func NewXClipClipboard() Clipboard { return NewSystemClipboard() }
