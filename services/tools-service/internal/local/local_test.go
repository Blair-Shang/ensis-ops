package local

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func useHome(t *testing.T, dir string) {
	t.Helper()
	prev := userHomeDir
	userHomeDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { userHomeDir = prev })
}

func TestUnderHomeRejectsEscape(t *testing.T) {
	home := t.TempDir()
	useHome(t, home)
	outside := filepath.Join(home, "..", "outside.txt")
	if _, err := ReadFile(outside); err == nil {
		t.Fatal("path outside home was accepted")
	}
	if _, err := WriteFile(filepath.Join(home, "..", "outside.txt"), "no"); err == nil {
		t.Fatal("write outside home was accepted")
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	home := t.TempDir()
	useHome(t, home)
	got, err := WriteFile("notes.txt", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if got.Bytes != 5 {
		t.Fatalf("bytes = %d", got.Bytes)
	}
	read, err := ReadFile(got.Path)
	if err != nil {
		t.Fatal(err)
	}
	if read.Text != "hello" {
		t.Fatalf("text = %q", read.Text)
	}
}

func TestWhichRejectsShell(t *testing.T) {
	for _, name := range []string{"java & calc", "git;rm", `..\notepad`, "", "java/bin"} {
		if _, err := Which(context.Background(), name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}

func TestVersionSkipsWindowPrograms(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("console gate is checked against Windows programs")
	}
	bat := filepath.Join(t.TempDir(), "evil.bat")
	if err := os.WriteFile(bat, []byte("@echo off\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if canReadVersion(bat) {
		t.Fatal("batch file would be executed")
	}
	if canReadVersion(`C:\Windows\System32\notepad.exe`) {
		t.Fatal("notepad would be launched")
	}
	if !canReadVersion(`C:\Windows\System32\where.exe`) {
		t.Fatal("console program was rejected")
	}
}

func TestSymlinkOutsideHome(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, "link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Skip("symlink not permitted")
	}
	useHome(t, home)
	if _, err := ReadFile(link); err == nil {
		t.Fatal("symlink to outside home was read")
	}
}

func TestJunctionOutsideHome(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junction")
	}
	home := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, "link")
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, outside)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("junction: %v %s", err, out)
	}
	useHome(t, home)
	target := filepath.Join(link, "secret.txt")
	full, err := underHome(target)
	if err == nil {
		t.Fatalf("junction to outside home was read via %s", full)
	}
}
