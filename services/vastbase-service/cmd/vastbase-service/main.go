// Command vastbase-service 是 ensis-ops Vastbase 能力服务（Layer 1）的独立进程入口。
//
// 源码位于 services/vastbase-service/，与 platform-core 分离；契约见
// docs/22-vastbase-module.md。本服务仅使用 Go + pgx，不引入 Java / JDBC。
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
	"ensis-ops/services/vastbase-service/internal/eventpub"
	"ensis-ops/services/vastbase-service/internal/handler"
	"ensis-ops/services/vastbase-service/internal/idgen"
)

const (
	windowsPipeAddr = `\\.\pipe\ensis-ops.vastbase`
	unixSocketName  = "ensis-ops.vastbase.sock"
)

func main() {
	if err := logutil.Init("vastbase-service"); err != nil {
		slog.Warn("file logging unavailable", "err", err)
	}
	if err := run(); err != nil {
		slog.Error("vastbase-service exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	idGen, err := idgen.NewSnowflake(0)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dispatcher := handler.New(idGen, eventpub.New())
	srv := server.New(ipcAddress(), dispatcher)
	return srv.Serve(ctx)
}

func ipcAddress() string {
	if runtime.GOOS == "windows" {
		return windowsPipeAddr
	}
	return filepath.Join(os.TempDir(), unixSocketName)
}
