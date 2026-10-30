//go:build windows

package local

import (
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func canReadVersion(path string) bool {
	if strings.ToLower(filepath.Ext(path)) != ".exe" {
		return false
	}
	return windowsConsole(path)
}

func consoleVersionCommand(ctx context.Context, path string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, path, "-version")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
		HideWindow:    true,
	}
	return cmd
}

func windowsConsole(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	hdr := make([]byte, 64)
	if _, err := f.Read(hdr); err != nil {
		return false
	}
	if hdr[0] != 'M' || hdr[1] != 'Z' {
		return false
	}
	peOff := int64(binary.LittleEndian.Uint32(hdr[0x3C:]))
	magic := make([]byte, 2)
	if _, err := f.ReadAt(magic, peOff+24); err != nil {
		return false
	}
	// Subsystem 在可选头固定偏移 68：3 是控制台，2 是窗口程序。
	sub := make([]byte, 2)
	if _, err := f.ReadAt(sub, peOff+24+68); err != nil {
		return false
	}
	kind := binary.LittleEndian.Uint16(magic)
	if kind != 0x10b && kind != 0x20b {
		return false
	}
	return binary.LittleEndian.Uint16(sub) == 3
}
