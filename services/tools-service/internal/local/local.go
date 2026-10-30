// Package local 是 AI 助手对「这台电脑」的只读检查和受控写文件。
// 不进工具页目录，也不提供任意命令。
package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	maxReadBytes  = 64 * 1024
	maxWriteBytes = 256 * 1024
	versionWait   = 4 * time.Second
)

var userHomeDir = os.UserHomeDir

var programName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)

// HostInfo 是这台电脑的概要，不含账号口令。
type HostInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
	CPU      int    `json:"cpu"`
	Home     string `json:"home"`
}

// WhichResult 说明某个程序是否在 PATH 上，以及它自己打印的版本。
type WhichResult struct {
	Name    string `json:"name"`
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ReadResult 是家目录下的一段文本。
type ReadResult struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// WriteResult 确认写入落在家目录内。
type WriteResult struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

// Info 读取操作系统、CPU、主机名和用户主目录。
func Info() (HostInfo, error) {
	home, err := homeDir()
	if err != nil {
		return HostInfo{}, err
	}
	host, _ := os.Hostname()
	return HostInfo{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: host,
		CPU:      runtime.NumCPU(),
		Home:     home,
	}, nil
}

// Which 只接受单个程序名，不用 shell，也不接受路径。
func Which(ctx context.Context, name string) (WhichResult, error) {
	name = strings.TrimSpace(name)
	if !programName.MatchString(name) {
		return WhichResult{}, fmt.Errorf("program name must be a single token such as java or git")
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return WhichResult{Name: name, Found: false}, nil
	}
	out := WhichResult{Name: name, Found: true, Path: path}
	if !canReadVersion(path) {
		return out, nil
	}
	verCtx, cancel := context.WithTimeout(ctx, versionWait)
	defer cancel()
	cmd := consoleVersionCommand(verCtx, path)
	raw, runErr := cmd.CombinedOutput()
	text := trimOutput(raw)
	if text != "" {
		out.Version = text
	}
	if runErr != nil && text == "" {
		out.Error = runErr.Error()
	}
	return out, nil
}

// ReadFile 只读用户主目录里的普通文本文件。
func ReadFile(path string) (ReadResult, error) {
	full, err := underHome(path)
	if err != nil {
		return ReadResult{}, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return ReadResult{}, err
	}
	if !info.Mode().IsRegular() {
		return ReadResult{}, fmt.Errorf("not a regular file")
	}
	if info.Size() > maxReadBytes {
		return ReadResult{}, fmt.Errorf("file is larger than %dKB", maxReadBytes/1024)
	}
	body, err := os.ReadFile(full)
	if err != nil {
		return ReadResult{}, err
	}
	if bytes.IndexByte(body, 0) >= 0 {
		return ReadResult{}, fmt.Errorf("not a text file")
	}
	return ReadResult{Path: full, Text: string(body)}, nil
}

// WriteFile 只在用户主目录里新建或覆盖普通文本文件。父目录必须已经存在。
func WriteFile(path, content string) (WriteResult, error) {
	if len(content) > maxWriteBytes {
		return WriteResult{}, fmt.Errorf("content is larger than %dKB", maxWriteBytes/1024)
	}
	if bytes.IndexByte([]byte(content), 0) >= 0 {
		return WriteResult{}, fmt.Errorf("not a text file")
	}
	full, err := underHome(path)
	if err != nil {
		return WriteResult{}, err
	}
	if info, statErr := os.Lstat(full); statErr == nil && !info.Mode().IsRegular() {
		return WriteResult{}, fmt.Errorf("not a regular file")
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return WriteResult{}, err
	}
	return WriteResult{Path: full, Bytes: len(content)}, nil
}

func homeDir() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	home, err = filepath.Abs(home)
	if err != nil {
		return "", err
	}
	if resolved, err := canonical(home); err == nil {
		home = resolved
	}
	return home, nil
}

// underHome 把路径收进用户主目录。符号链接按最终目标判断，越界直接拒绝。
func underHome(path string) (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(cleaned) {
		cleaned = filepath.Join(home, cleaned)
	}
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", err
	}
	resolved, err := resolvePath(abs)
	if err != nil {
		return "", err
	}
	if !inside(home, resolved) {
		return "", fmt.Errorf("path must stay under the user home directory")
	}
	return resolved, nil
}

func resolvePath(abs string) (string, error) {
	if resolved, err := canonical(abs); err == nil {
		return resolved, nil
	}
	parent := filepath.Dir(abs)
	parentResolved, err := canonical(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("parent directory does not exist")
		}
		return "", err
	}
	return filepath.Join(parentResolved, filepath.Base(abs)), nil
}

func inside(home, path string) bool {
	if runtime.GOOS == "windows" {
		home = strings.ToLower(home)
		path = strings.ToLower(path)
	}
	rel, err := filepath.Rel(home, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func trimOutput(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if len(text) > 4000 {
		text = text[:4000]
	}
	return text
}
