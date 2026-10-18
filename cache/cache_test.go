package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yv3000/siig/rules"
)

func isolateHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
}

func TestSetGetIsCached(t *testing.T) {
	isolateHome(t)
	rule := &rules.StackRule{ID: "gleam", Name: "Gleam"}
	if IsCached("gleam") {
		t.Fatal("empty cache")
	}
	if err := Set("gleam", rule); err != nil {
		t.Fatal(err)
	}
	if !IsCached("gleam") {
		t.Fatal("expected cache hit")
	}
	got, err := Get("gleam")
	if err != nil || got == nil || got.Name != "Gleam" {
		t.Fatalf("Get = %+v err=%v", got, err)
	}
}

func TestGet_stale(t *testing.T) {
	isolateHome(t)
	rule := &rules.StackRule{ID: "old", Name: "Old"}
	if err := Set("old", rule); err != nil {
		t.Fatal(err)
	}
	meta := loadMetadata()
	meta.Fetched["old"] = time.Now().Add(-staleDuration - time.Hour)
	if err := saveMetadata(meta); err != nil {
		t.Fatal(err)
	}
	got, err := Get("old")
	if err != nil || got != nil {
		t.Fatalf("stale should miss, got %+v err=%v", got, err)
	}
	if IsCached("old") {
		t.Fatal("stale is not cached")
	}
}

func TestGet_missingFile(t *testing.T) {
	isolateHome(t)
	if err := Set("x", &rules.StackRule{ID: "x"}); err != nil {
		t.Fatal(err)
	}
	dir, err := cacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "x.json")); err != nil {
		t.Fatal(err)
	}
	got, err := Get("x")
	if err != nil || got != nil {
		t.Fatalf("missing file should miss: %+v %v", got, err)
	}
}

func TestSet_rejectsBadID(t *testing.T) {
	isolateHome(t)
	if err := Set("../etc", &rules.StackRule{ID: "x"}); err == nil {
		t.Fatal("expected invalid id error")
	}
}

func TestClear(t *testing.T) {
	isolateHome(t)
	if err := Set("a", &rules.StackRule{ID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	if IsCached("a") {
		t.Fatal("cleared")
	}
}
