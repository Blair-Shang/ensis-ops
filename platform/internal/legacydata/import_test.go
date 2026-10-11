package legacydata

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestImportData_CopiesDatabaseAndSidecars(t *testing.T) {
	legacy := t.TempDir()
	dest := filepath.Join(t.TempDir(), "data")
	legacyDB := filepath.Join(legacy, legacyDBName)
	writeLegacyDB(t, legacyDB)

	if err := os.MkdirAll(filepath.Join(legacy, "components", "bundle"), 0o755); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(legacy, "components", "bundle", "tool.txt")
	if err := os.WriteFile(tool, []byte("vsql"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "plugins-disabled.txt"), []byte("old-plugin\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	imported, err := importData(dest, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if !imported {
		t.Fatal("expected import")
	}

	db, err := sql.Open("sqlite", filepath.Join(dest, currentDBName))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name string
	if err := db.QueryRow(`SELECT name FROM conn WHERE id=1`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "prod" {
		t.Fatalf("row = %q", name)
	}
	body, err := os.ReadFile(filepath.Join(dest, "components", "bundle", "tool.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "vsql" {
		t.Fatalf("tool = %q", body)
	}
	if _, err := os.Stat(filepath.Join(dest, legacyDBName)); !os.IsNotExist(err) {
		t.Fatalf("legacy db name should not be copied, stat err=%v", err)
	}
	if _, err := os.Stat(legacyDB); err != nil {
		t.Fatal(err)
	}

	again, err := importData(dest, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("second import must not snapshot over the new database")
	}
	if err := MarkFinished(dest); err != nil {
		t.Fatal(err)
	}
	if !Finished(dest) {
		t.Fatal("expected finished marker")
	}
	extra := filepath.Join(legacy, "later.txt")
	if err := os.WriteFile(extra, []byte("later"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportData(dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "later.txt")); !os.IsNotExist(err) {
		t.Fatalf("finished import must not read the legacy dir again, stat err=%v", err)
	}
}

func TestImportTree_SkipsOccupiedDir(t *testing.T) {
	legacy := t.TempDir()
	if err := os.WriteFile(filepath.Join(legacy, "pack.md"), []byte("skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "keep.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	copied, err := ImportTree(dest, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if copied {
		t.Fatal("occupied destination must be left unchanged")
	}
	if _, err := os.Stat(filepath.Join(dest, "pack.md")); !os.IsNotExist(err) {
		t.Fatalf("pack.md should not appear, stat err=%v", err)
	}
}

func writeLegacyDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE conn (id INTEGER PRIMARY KEY, name TEXT); INSERT INTO conn (id, name) VALUES (1, 'prod');`); err != nil {
		t.Fatal(err)
	}
}
