package supervisor_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"ensis-ops/platform/internal/supervisor"
)

func TestEnsure_FtpService(t *testing.T) {
	if os.Getenv("ENSIS_INTEGRATION") == "" {
		t.Skip("set ENSIS_INTEGRATION=1 to run")
	}

	dir := filepath.Join("..", "..", "..", "build", "shell", "Release", "services")
	binName := "ensis-ftp-service.exe"
	if runtime.GOOS != "windows" {
		dir = filepath.Join("..", "..", "..", "build", "shell-x64", "services")
		binName = "ensis-ftp-service"
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", binName)); err != nil {
		t.Skipf("staged %s not found", binName)
	}

	sup, err := supervisor.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := sup.Ensure(ctx, "com.ensisops.ftp"); err != nil {
		t.Fatal(err)
	}
}
