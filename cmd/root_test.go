package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunSiig_dryRunOffline(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n"), 0644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	dryRun = true
	offline = true
	targets = ""
	verbose = false
	noCache = false
	if err := runSiig(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRunSiig_badTargets(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	dryRun = true
	offline = true
	targets = "nope"
	err = runSiig(rootCmd, nil)
	if err == nil {
		t.Fatal("expected unknown target error")
	}
}
