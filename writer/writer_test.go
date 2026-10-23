package writer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yv3000/siig/internal/version"
	"github.com/yv3000/siig/matcher"
)

func TestWriteIgnoreFile_reusesExistingHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	first := version.Header() + "\nnode_modules/\n"
	if err := os.WriteFile(path, []byte(first), 0644); err != nil {
		t.Fatal(err)
	}

	got := writeIgnoreFile(path, []string{"node_modules/", "dist/"}, matcher.TargetGitignore, false)
	if got.Skipped || got.EntriesAdded != 1 || got.Created {
		t.Fatalf("%+v", got)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if n := strings.Count(text, version.Marker); n != 1 {
		t.Fatalf("expected one SIIG header, got %d in:\n%s", n, text)
	}
	if !strings.HasSuffix(text, "dist/\n") {
		t.Fatalf("new entry should be appended:\n%s", text)
	}
}

func TestWriteIgnoreFile_keepsUnterminatedLastLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("dist/"), 0644); err != nil {
		t.Fatal(err)
	}

	if got := writeIgnoreFile(path, []string{"build/"}, matcher.TargetGitignore, false); got.Skipped {
		t.Fatalf("%+v", got)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "dist/# Added by SIIG") {
		t.Fatalf("appended block glued onto the last line:\n%s", body)
	}
	if !strings.HasPrefix(string(body), "dist/\n") {
		t.Fatalf("existing entry changed:\n%s", body)
	}
	if !strings.Contains(string(body), "build/\n") {
		t.Fatalf("new entry missing:\n%s", body)
	}
}

func TestWriteAll_reportsTargetsInOrder(t *testing.T) {
	dir := t.TempDir()
	res := &matcher.MatchResult{
		Entries: map[matcher.Target][]string{
			matcher.TargetGitignore:    {"a/"},
			matcher.TargetVercelignore: {"b/"},
			matcher.TargetDockerignore: {"c/"},
			matcher.TargetNpmignore:    {"d/"},
		},
		VSCodeExclude:     map[string]bool{"e": true},
		ApplicableTargets: map[matcher.Target]bool{matcher.TargetVSCode: true},
	}
	for _, target := range []matcher.Target{matcher.TargetGitignore, matcher.TargetVercelignore, matcher.TargetDockerignore, matcher.TargetNpmignore} {
		res.ApplicableTargets[target] = true
	}
	want := []matcher.Target{
		matcher.TargetGitignore,
		matcher.TargetVercelignore,
		matcher.TargetDockerignore,
		matcher.TargetNpmignore,
		matcher.TargetVSCode,
	}

	for run := 0; run < 3; run++ {
		var got []matcher.Target
		for _, r := range WriteAll(dir, res, true) {
			got = append(got, r.Target)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d reported %v, want %v", run, got, want)
		}
	}
}

func TestWriteAll_vscodeSkipReason(t *testing.T) {
	dir := t.TempDir()
	res := &matcher.MatchResult{
		Entries:           map[matcher.Target][]string{matcher.TargetGitignore: {"a/"}},
		ApplicableTargets: map[matcher.Target]bool{matcher.TargetGitignore: true, matcher.TargetVSCode: true},
	}

	for _, r := range WriteAll(dir, res, true) {
		if r.Target == matcher.TargetVSCode {
			if r.SkipReason != "no entries to add" {
				t.Fatalf("applicable vscode target with no excludes: %q", r.SkipReason)
			}
			return
		}
	}
	t.Fatal("missing vscode result")
}

func TestWriteIgnoreFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte(".env\n"), 0644); err != nil {
		t.Fatal(err)
	}

	entries := []string{".env", "node_modules/", "dist/"}
	got := writeIgnoreFile(path, entries, matcher.TargetGitignore, false)
	if got.Skipped || got.EntriesAdded != 2 {
		t.Fatalf("got skipped=%v added=%d reason=%s", got.Skipped, got.EntriesAdded, got.SkipReason)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "node_modules/") || !strings.Contains(text, version.Header()) {
		t.Fatalf("missing new entries:\n%s", text)
	}
	if strings.Count(text, ".env") != 1 {
		t.Fatalf("should not duplicate .env:\n%s", text)
	}

	again := writeIgnoreFile(path, entries, matcher.TargetGitignore, false)
	if !again.Skipped {
		t.Fatal("second write should skip")
	}
}

func TestWriteIgnoreFile_dryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	got := writeIgnoreFile(path, []string{"dist/"}, matcher.TargetGitignore, true)
	if got.Skipped || got.EntriesAdded != 1 {
		t.Fatalf("%+v", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("dry-run must not create file")
	}
}

func TestWriteVSCodeSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".vscode", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"editor.tabSize": 2, "files.exclude": {"tmp": true}}`), 0644); err != nil {
		t.Fatal(err)
	}

	got := writeVSCodeSettings(path, map[string]bool{"tmp": true, "node_modules": true}, false)
	if got.Skipped || got.EntriesAdded != 1 {
		t.Fatalf("%+v", got)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	ex := settings["files.exclude"].(map[string]interface{})
	if _, ok := ex["node_modules"]; !ok {
		t.Fatalf("exclude missing node_modules: %#v", ex)
	}
	if settings["editor.tabSize"].(float64) != 2 {
		t.Fatal("must keep existing keys")
	}
}

func TestWriteVSCodeSettings_invalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	got := writeVSCodeSettings(path, map[string]bool{"a": true}, false)
	if !got.Skipped {
		t.Fatal("invalid JSON must be left alone")
	}
}

func TestWriteAll_skipsInapplicable(t *testing.T) {
	dir := t.TempDir()
	res := WriteAll(dir, &matcher.MatchResult{
		Entries:           map[matcher.Target][]string{matcher.TargetGitignore: {"dist/"}},
		ApplicableTargets: map[matcher.Target]bool{matcher.TargetGitignore: true},
	}, true)
	foundGit := false
	for _, r := range res {
		if r.Target == matcher.TargetGitignore && r.Skipped {
			t.Fatalf("gitignore should not skip: %+v", r)
		}
		if r.Target == matcher.TargetGitignore {
			foundGit = true
		}
	}
	if !foundGit {
		t.Fatal("missing gitignore result")
	}
}
