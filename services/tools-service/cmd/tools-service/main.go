// Command tools-service 是本机工具能力服务（Layer 1）的独立进程入口。
//
// 格式化与文本对比都在这个进程里执行，不占用 platform-core。
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"ensis-ops/pkg/logutil"
	"ensis-ops/pkg/serviceipc/server"
	"ensis-ops/services/tools-service/internal/handler"
)

const (
	windowsPipeAddr = `\\.\pipe\ensis-ops.tools`
	unixSocketName  = "ensis-ops.tools.sock"
)

func main() {
	if err := logutil.Init("tools-service"); err != nil {
		slog.Warn("file logging unavailable", "err", err)
	}
	if err := run(); err != nil {
		slog.Error("tools-service exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := server.New(ipcAddress(), handler.New())
	return srv.Serve(ctx)
}

func ipcAddress() string {
	if runtime.GOOS == "windows" {
		return windowsPipeAddr
	}
	return filepath.Join(os.TempDir(), unixSocketName)
}
