// Command platform-core 是 ensis-ops Platform 层（Layer 2）的常驻进程入口。
//
// 职责：解析本地 SQLite 库路径 → 打开库并启用 WAL → 执行结构迁移 →
// 在应用 IPC 地址（Windows 命名管道 / 其他平台 UDS）上提供服务，处理来自
// C++ 壳层透传过来的 platform.* 请求，直到进程被终止。
//
// 壳层不含任何业务逻辑：持久化与方法裁决全部发生在本进程。
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"ensis-ops/pkg/buildinfo"
	"ensis-ops/pkg/logutil"
	"ensis-ops/pkg/serviceipc/event"
	"ensis-ops/platform/internal/ai"
	"ensis-ops/platform/internal/ai/skill"
	"ensis-ops/platform/internal/appupdate"
	"ensis-ops/platform/internal/components"
	"ensis-ops/platform/internal/eventhub"
	"ensis-ops/platform/internal/handler"
	"ensis-ops/platform/internal/idgen"
	"ensis-ops/platform/internal/legacydata"
	"ensis-ops/platform/internal/migrate"
	"ensis-ops/platform/internal/server"
	"ensis-ops/platform/internal/store"
	"ensis-ops/platform/internal/streamregistry"
	"ensis-ops/platform/internal/streamserver"
	"ensis-ops/platform/internal/supervisor"

	_ "modernc.org/sqlite"
)

const (
	// dbFileName 是本地库文件名。
	dbFileName = "ensis-ops.db"
	// sqliteDriver 是 modernc.org/sqlite 注册的驱动名。
	sqliteDriver = "sqlite"
	// windowsPipeAddr 是 Windows 应用 IPC 命名管道地址，须与壳层保持一致。
	windowsPipeAddr = `\\.\pipe\ensis-ops.platform`
	// unixSocketName 是非 Windows 平台 UDS 文件名。
	unixSocketName = "ensis-ops.platform.sock"
	// defaultWorkerID 是单进程桌面端的 Snowflake worker ID。
	defaultWorkerID = 0
)

func main() {
	if err := logutil.Init("platform-core"); err != nil {
		slog.Warn("file logging unavailable", "err", err)
	}
	slog.Info("platform-core starting", "version", buildinfo.Version, "build", buildinfo.BuildID)

	if err := run(); err != nil {
		slog.Error("platform-core exited", "err", err)
		os.Exit(1)
	}
}

// run 组装依赖并启动服务，返回时表示进程应退出。
func run() error {
	dbPath, err := resolveDBPath()
	if err != nil {
		return err
	}
	dataRoot := filepath.Dir(dbPath)
	legacyPending := !legacydata.Finished(dataRoot)
	if legacyPending {
		if imported, impErr := legacydata.ImportData(dataRoot); impErr != nil {
			return fmt.Errorf("platform: import legacy data: %w", impErr)
		} else if imported {
			slog.Info("imported legacy user data", "dir", dataRoot)
		}
		importLegacySkills()
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("platform: create data dir: %w", err)
	}

	db, err := openDatabase(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// 进程收到中断信号时取消 ctx，触发服务端优雅关闭。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := migrate.Run(ctx, db); err != nil {
		return err
	}
	slog.Info("database ready", "path", dbPath)

	idGen, err := idgen.NewSnowflake(defaultWorkerID)
	if err != nil {
		return fmt.Errorf("platform: init id generator: %w", err)
	}

	installDir, err := supervisor.ResolveServicesDir()
	if err != nil {
		slog.Warn("capability services lazy start may fail", "err", err)
	}
	var capabilities *handler.CapabilityRegistry
	var sup *supervisor.Supervisor
	var streamReg *streamregistry.Registry
	if installDir != "" {
		if s, supErr := supervisor.New(installDir); supErr != nil {
			slog.Warn("supervisor init failed", "err", supErr)
		} else {
			sup = s
			defer sup.Shutdown()
			if reg, regErr := streamregistry.Load(sup); regErr != nil {
				slog.Warn("stream registry init failed", "err", regErr)
			} else {
				streamReg = reg
			}
			if reg, regErr := handler.NewCapabilityRegistry(sup); regErr != nil {
				slog.Warn("capability registry init failed", "err", regErr)
			} else {
				capabilities = reg
			}
		}
	}

	eventHub := eventhub.New()
	if sup != nil {
		sup.SetEventSink(eventHub.Publish)
	}
	if streamReg == nil {
		streamReg, _ = streamregistry.Load(nil)
	}
	streamSrv := streamserver.New(event.StreamAddress(), streamReg)
	eventHub.SetStreamDeliverer(streamSrv)
	fileEditor := handler.NewFileEditorCoordinator(eventHub)

	// VaultStore：AES-256-GCM 加密密文存 SQLite，OS Keychain 仅保留一条主密钥。
	// 向后兼容旧版 KeychainStore：首次读取时自动迁移旧条目。
	keychain := store.NewKeychainStore()
	if legacyPending {
		if adopted, adoptErr := store.AdoptLegacyMasterKey(keychain); adoptErr != nil {
			return fmt.Errorf("platform: import legacy master key: %w", adoptErr)
		} else if adopted {
			slog.Info("imported legacy master key")
		}
		if err := legacydata.MarkFinished(dataRoot); err != nil {
			return err
		}
	}
	secrets := store.NewVaultStore(db, keychain)
	settingStore := store.NewSettingStore(db)

	var componentRegistry *components.Registry
	if componentsDir, compErr := components.ResolveDir(); compErr != nil {
		slog.Warn("tool components registry unavailable", "err", compErr)
	} else if dataRoot, dataErr := dataDir(); dataErr != nil {
		slog.Warn("tool components registry unavailable", "err", dataErr)
	} else if reg, regErr := components.NewRegistry(componentsDir, settingStore, dataRoot); regErr != nil {
		slog.Warn("tool components registry init failed", "err", regErr)
	} else {
		componentRegistry = reg
		slog.Info("tool components registry ready", "dir", componentsDir)
		components.BindComponentEnv(sup, componentRegistry)
	}

	aiService := ai.NewService(ai.Deps{
		Providers:     store.NewAIProviderStore(db),
		Conversations: store.NewAIConversationStore(db),
		MCP:           store.NewAIMCPStore(db),
		Skills:        store.NewAISkillStore(db),
		Secrets:       secrets,
		IDs:           idGen,
		Events:        eventHub,
	})
	updateHosts := strings.Split(os.Getenv("ENSIS_UPDATE_DOWNLOAD_HOSTS"), ",")
	appUpdateMgr := appupdate.New(updateHosts)
	dispatcher := handler.New(handler.Deps{
		Settings:     settingStore,
		Connections:  store.NewConnectionStore(db),
		Organization: store.NewOrganizationStore(db),
		Credentials:  store.NewCredentialStore(db),
		Secrets:      secrets,
		IDs:          idGen,
		Capabilities: capabilities,
		FileEditor:   fileEditor,
		Components:   componentRegistry,
		AppUpdate:    appUpdateMgr,
		AI:           aiService,
		APIHistory:   store.NewAPIHistoryStore(db),
		APICatalog:   store.NewAPICatalogStore(db),
		Events:       eventHub,
	})
	aiService.BindHost(dispatcher.HostRuntime())
	go aiService.SoftDiscoverBuiltinMCP(context.Background())
	srv := server.New(ipcAddress(), dispatcher)
	go func() {
		if err := eventHub.Serve(ctx); err != nil && ctx.Err() == nil {
			slog.Error("event hub error", "err", err)
		}
	}()
	go func() {
		if err := streamSrv.Serve(ctx); err != nil && ctx.Err() == nil {
			slog.Error("stream server error", "err", err)
		}
	}()
	return srv.Serve(ctx)
}

// openDatabase 打开 SQLite 库并启用适合桌面单文件场景的 PRAGMA。
func openDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open(sqliteDriver, dbPath)
	if err != nil {
		return nil, fmt.Errorf("platform: open sqlite: %w", err)
	}

	// 单连接即可满足配置 KV 的低并发负载，同时彻底规避 WAL 下的写锁竞争。
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;", // 毫秒
		"PRAGMA foreign_keys=OFF;",  // 本库不使用物理外键
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("platform: apply %q: %w", p, err)
		}
	}
	return db, nil
}

// importLegacySkills 把 1.x 的 Skill 目录补到新目录。失败只记日志，不阻断启动。
func importLegacySkills() {
	skillsDir, err := skill.SkillsRootDir()
	if err != nil {
		slog.Warn("skills dir unavailable", "err", err)
		return
	}
	legacyDir, ok := legacydata.LegacySkillsDir()
	if !ok {
		return
	}
	copied, err := legacydata.ImportTree(skillsDir, legacyDir)
	if err != nil {
		slog.Warn("legacy skills import failed", "err", err)
		return
	}
	if copied {
		slog.Info("imported legacy skills", "dir", skillsDir)
	}
}

// resolveDBPath 计算本地 SQLite 库的绝对路径。
//
// Windows：%LOCALAPPDATA%\ensis-ops\data\ensis-ops.db（与壳层用户数据目录一致）。
// 其他平台：~/.ensis-ops/data/ensis-ops.db；无法确定 HOME 时回退到可执行文件同级的
// data 目录。
func resolveDBPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, dbFileName), nil
}

// dataDir 返回用户数据目录（存放 ensis-ops.db 等）。
func dataDir() (string, error) {
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "ensis-ops", "data"), nil
		}
		return installFallbackDir()
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".ensis-ops", "data"), nil
	}
	return installFallbackDir()
}

// installFallbackDir 在无法确定用户目录时，回退到可执行文件同级的 data 目录。
func installFallbackDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("platform: resolve executable dir: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "data"), nil
}

// ipcAddress 返回本平台的应用 IPC 监听地址。
func ipcAddress() string {
	if runtime.GOOS == "windows" {
		return windowsPipeAddr
	}
	return filepath.Join(os.TempDir(), unixSocketName)
}
