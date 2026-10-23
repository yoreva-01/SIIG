package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yv3000/siig/cache"
	"github.com/yv3000/siig/internal/version"
	"github.com/yv3000/siig/rules"
)

// resetFlags clears the package-level flag vars after a test so one case cannot
// leak its flags into the next.
func resetFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		dryRun, targets, verbose = false, "", false
		offline, noCache, clearCache = false, false, false
	})
}

// isolateHome points the cache at a throwaway directory.
func isolateHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
}

func TestRootCmdUsesSharedVersion(t *testing.T) {
	if rootCmd.Version != version.Version {
		t.Fatalf("rootCmd.Version = %q, want %q", rootCmd.Version, version.Version)
	}
}

func TestRunSiig_dryRunOffline(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n"), 0644); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	resetFlags(t)
	dryRun = true
	offline = true
	if err := runSiig(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err == nil {
		t.Fatal("dry-run must not write .gitignore")
	}
}

func TestRunSiig_badTargets(t *testing.T) {
	chdir(t, t.TempDir())

	resetFlags(t)
	dryRun = true
	offline = true
	targets = "nope"
	if err := runSiig(rootCmd, nil); err == nil {
		t.Fatal("expected unknown target error")
	}
}

func TestRunSiig_clearCache(t *testing.T) {
	isolateHome(t)
	if err := cache.Set("gleam", &rules.StackRule{ID: "gleam", Name: "Gleam"}); err != nil {
		t.Fatal(err)
	}
	if !cache.IsCached("gleam") {
		t.Fatal("test setup: expected a cached rule")
	}

	resetFlags(t)
	clearCache = true
	if err := runSiig(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
	if cache.IsCached("gleam") {
		t.Fatal("--clear-cache should remove cached rules")
	}
}

func TestRunSiig_noCacheKeepsCacheOnDisk(t *testing.T) {
	isolateHome(t)
	chdir(t, t.TempDir())
	if err := cache.Set("gleam", &rules.StackRule{ID: "gleam", Name: "Gleam"}); err != nil {
		t.Fatal(err)
	}

	resetFlags(t)
	dryRun = true
	offline = true
	noCache = true
	if err := runSiig(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !cache.IsCached("gleam") {
		t.Fatal("--no-cache should ignore the cache, not delete it")
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}
