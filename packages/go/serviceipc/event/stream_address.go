package event

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	windowsStreamAddr = `\\.\pipe\ensis-ops.platform.stream`
	unixStreamName    = "ensis-ops.platform.stream.sock"
)

// StreamAddress 返回 Shell InvokeStream 原生分帧管道地址（与 ensis-ops.platform 同帧格式）。
func StreamAddress() string {
	if runtime.GOOS == "windows" {
		return windowsStreamAddr
	}
	return filepath.Join(os.TempDir(), unixStreamName)
}
