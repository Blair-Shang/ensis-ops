// Package legacydata 在首次启动时把 1.x 的用户数据复制到 ensis-ops 目录。
//
// 旧目录和旧钥匙串条目都保留。新库或新目录里已经有内容时不覆盖。
package legacydata

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	// legacyDBName 是 1.x 本地库文件名。
	legacyDBName = "niuma.db"
	// currentDBName 是 2.x 本地库文件名，与 platform-core 的 dbFileName 一致。
	currentDBName = "ensis-ops.db"
	// markerName 表示这次用户目录已经做过一次旧数据导入。
	markerName   = "legacy-import.done"
	sqliteDriver = "sqlite"
)

// LegacyDataDir 返回 1.x 用户数据目录。
//
// Windows 为 %LOCALAPPDATA%\NiuMa\data，其他平台为 ~/.niuma/data。
// 无法确定用户目录时 ok 为 false。
func LegacyDataDir() (string, bool) {
	root, ok := legacyProductRoot()
	if !ok {
		return "", false
	}
	return filepath.Join(root, "data"), true
}

// LegacySkillsDir 返回 1.x 已安装 Skill 包目录。
//
// Windows 为 %LOCALAPPDATA%\NiuMa\skills，其他平台为 ~/.niuma/skills。
func LegacySkillsDir() (string, bool) {
	root, ok := legacyProductRoot()
	if !ok {
		return "", false
	}
	return filepath.Join(root, "skills"), true
}

// Finished 报告 newDir 是否已经完成过一次旧数据导入。
//
// 标记文件和 ensis-ops.db 都在时返回 true。只有标记、库被删掉时返回 false，以便重新导入。
func Finished(newDir string) bool {
	if _, err := os.Stat(filepath.Join(newDir, currentDBName)); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(newDir, markerName))
	return err == nil
}

// MarkFinished 写下导入完成标记。之后启动不再读取 1.x 目录。
func MarkFinished(newDir string) error {
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return fmt.Errorf("legacydata: mkdir %s: %w", newDir, err)
	}
	path := filepath.Join(newDir, markerName)
	if err := os.WriteFile(path, []byte("1\n"), 0o644); err != nil {
		return fmt.Errorf("legacydata: mark %s: %w", path, err)
	}
	return nil
}

// ImportData 在新库还不存在、且尚未标记完成时，把 legacy 用户数据导入 newDir。
//
// 数据库用 SQLite 快照写入，避免直接拷贝 WAL 时读到半截事务。
// components、plugins-disabled.txt 等其余文件在同一次导入里补齐。
// 旧目录保留。已标记完成时直接返回，不再访问旧目录。
func ImportData(newDir string) (bool, error) {
	if Finished(newDir) {
		return false, nil
	}
	legacyDir, ok := LegacyDataDir()
	if !ok {
		return false, nil
	}
	return importData(newDir, legacyDir)
}

// ImportTree 在 newDir 为空或不存在时，递归复制 legacyDir。
//
// 目标里已有任意一项时不复制。源目录不存在时视为没有旧数据。
func ImportTree(newDir, legacyDir string) (bool, error) {
	if legacyDir == "" || newDir == "" {
		return false, nil
	}
	if samePath(newDir, legacyDir) {
		return false, nil
	}
	if _, err := os.Stat(legacyDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("legacydata: stat %s: %w", legacyDir, err)
	}
	occupied, err := dirHasEntries(newDir)
	if err != nil {
		return false, err
	}
	if occupied {
		return false, nil
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return false, fmt.Errorf("legacydata: mkdir %s: %w", newDir, err)
	}
	if err := copyMissing(legacyDir, newDir, nil); err != nil {
		return false, err
	}
	return true, nil
}

func importData(newDir, legacyDir string) (bool, error) {
	if samePath(newDir, legacyDir) {
		return false, nil
	}
	legacyDB := filepath.Join(legacyDir, legacyDBName)
	if _, err := os.Stat(legacyDB); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("legacydata: stat %s: %w", legacyDB, err)
	}

	newDB := filepath.Join(newDir, currentDBName)
	imported := false
	if _, err := os.Stat(newDB); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			return false, fmt.Errorf("legacydata: mkdir %s: %w", newDir, err)
		}
		if err := snapshotSQLite(legacyDB, newDB); err != nil {
			_ = os.Remove(newDB)
			return false, err
		}
		imported = true
	} else if err != nil {
		return false, fmt.Errorf("legacydata: stat %s: %w", newDB, err)
	}

	if err := copyMissing(legacyDir, newDir, isLegacyDBFile); err != nil {
		return imported, err
	}
	return imported, nil
}

func legacyProductRoot() (string, bool) {
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "NiuMa"), true
		}
		return "", false
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	return filepath.Join(home, ".niuma"), true
}

func snapshotSQLite(src, dst string) error {
	db, err := sql.Open(sqliteDriver, sqliteReadURI(src))
	if err != nil {
		return fmt.Errorf("legacydata: open %s: %w", src, err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		return fmt.Errorf("legacydata: snapshot %s: %w", src, err)
	}
	quoted := strings.ReplaceAll(filepath.ToSlash(dst), "'", "''")
	if _, err := db.Exec("VACUUM INTO '" + quoted + "'"); err != nil {
		return fmt.Errorf("legacydata: snapshot %s: %w", src, err)
	}
	return nil
}

func sqliteReadURI(path string) string {
	slash := filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	u := url.URL{Scheme: "file", Path: slash, RawQuery: "mode=ro"}
	return u.String()
}

func isLegacyDBFile(name string) bool {
	switch name {
	case legacyDBName, legacyDBName + "-wal", legacyDBName + "-shm", legacyDBName + "-journal":
		return true
	default:
		return false
	}
}

func copyMissing(src, dst string, skipRoot func(string) bool) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if skipRoot != nil && filepath.Dir(rel) == "." && skipRoot(d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if _, statErr := os.Stat(target); statErr == nil {
			return nil
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("legacydata: copy %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("legacydata: copy %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("legacydata: copy %s: %w", dst, err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("legacydata: copy %s: %w", dst, err)
	}
	return nil
}

func dirHasEntries(path string) (bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("legacydata: open %s: %w", path, err)
	}
	defer f.Close()
	names, err := f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("legacydata: read %s: %w", path, err)
	}
	return len(names) > 0, nil
}

func samePath(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return false
	}
	aa = filepath.Clean(aa)
	bb = filepath.Clean(bb)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(aa, bb)
	}
	return aa == bb
}
