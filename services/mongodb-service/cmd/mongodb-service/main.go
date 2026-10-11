// Command mongodb-service 是 ensis-ops MongoDB 能力服务（Layer 1）的独立进程入口。
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
	"ensis-ops/services/mongodb-service/internal/eventpub"
	"ensis-ops/services/mongodb-service/internal/handler"
	"ensis-ops/services/mongodb-service/internal/idgen"
)

const (
	windowsPipeAddr = `\\.\pipe\ensis-ops.mongodb`
	unixSocketName  = "ensis-ops.mongodb.sock"
)

func main() {
	if err := logutil.Init("mongodb-service"); err != nil {
		slog.Warn("file logging unavailable", "err", err)
	}
	if err := run(); err != nil {
		slog.Error("mongodb-service exited", "err", err)
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
