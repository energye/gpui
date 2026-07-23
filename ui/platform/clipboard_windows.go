//go:build windows

package platform

import (
	"bytes"
	"os/exec"
	"strings"
)

// NewSystemClipboard returns the Windows system clipboard adapter
// (PowerShell Get/Set-Clipboard + memory fallback).
func NewSystemClipboard() Clipboard {
	return NewFallbackClipboard(&windowsClipPrimary{})
}

type windowsClipPrimary struct{}

func (windowsClipPrimary) ReadText() (string, bool) {
	// PowerShell is available on modern Windows; avoids linking user32 in the stub host.
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard -Raw")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	// PowerShell may append trailing CRLF depending on version.
	s := string(out)
	s = strings.TrimSuffix(s, "\r\n")
	s = strings.TrimSuffix(s, "\n")
	return s, true
}

func (windowsClipPrimary) WriteText(s string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Set-Clipboard -Value ([Console]::In.ReadToEnd())")
	cmd.Stdin = strings.NewReader(s)
	if err := cmd.Run(); err == nil {
		return nil
	}
	// Fallback: clip.exe accepts stdin (write-only; no read).
	cmd2 := exec.Command("clip")
	cmd2.Stdin = bytes.NewReader([]byte(s))
	return cmd2.Run()
}

// NewXClipClipboard is a cross-build alias (Linux name) → system clipboard.
func NewXClipClipboard() Clipboard { return NewSystemClipboard() }
