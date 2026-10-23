package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/yv3000/siig/internal/version"
	"github.com/yv3000/siig/rules"
)

func mapFS(files map[string]string) fstest.MapFS {
	m := make(fstest.MapFS, len(files))
	for name, body := range files {
		m[name] = &fstest.MapFile{Data: []byte(body), Mode: 0644}
	}
	return m
}

func ids(result *ScanResult) map[string]bool {
	got := make(map[string]bool)
	for _, s := range result.DetectedStacks {
		got[s.Rule.ID] = true
	}
	return got
}

func TestScan_emptyRoot(t *testing.T) {
	if _, err := Scan("", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_rootIsAFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(file, nil); err == nil {
		t.Fatal("expected error for non-directory root")
	}
}

func TestScan_depthAndSkips(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("go.mod", "module example")
	mustWrite("Dockerfile", "FROM scratch")
	mustWrite(filepath.Join("node_modules", "pkg", "index.js"), "x")
	mustWrite(filepath.Join(".git", "config"), "x")
	mustWrite(filepath.Join(".vscode", "settings.json"), "{}")
	mustWrite("vercel.json", "{}")
	mustWrite(filepath.Join("a", "b", "c", "d", "secret.xyz"), "too deep")

	all, err := rules.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Scan(root, all)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasDockerfile || !got.HasVSCodeDir || !got.HasVercelJSON {
		t.Fatalf("flags: docker=%v vscode=%v vercel=%v", got.HasDockerfile, got.HasVSCodeDir, got.HasVercelJSON)
	}

	found := ids(got)
	if !found["go"] || !found["docker"] {
		t.Fatalf("detected %#v", found)
	}
	if len(got.ScanErrors) != 0 {
		t.Fatalf("clean tree should report no scan errors: %v", got.ScanErrors)
	}
}

func TestScan_npmLib(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "package.json")
	if err := os.WriteFile(pkg, []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.HasNpmLib {
		t.Fatal("app package.json is not a lib")
	}

	if err := os.WriteFile(pkg, []byte(`{"main":"index.js","exports":{}}`), 0644); err != nil {
		t.Fatal(err)
	}
	got, err = Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasNpmLib {
		t.Fatal("main+exports should mark library")
	}
}

func TestScan_projectDirNameIsNotASignal(t *testing.T) {
	// A checkout whose folder happens to be named "supabase" must not detect the
	// Supabase stack from that name alone.
	root := filepath.Join(t.TempDir(), "supabase")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	all, err := rules.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Scan(root, all)
	if err != nil {
		t.Fatal(err)
	}
	if ids(got)["supabase"] {
		t.Fatal("project root directory name must not trigger detection")
	}
}

func TestScanFS_surfacesUnreadablePaths(t *testing.T) {
	mem := mapFS(map[string]string{"go.mod": "module example", "blocked/inner.txt": "x"})
	broken := failingFS{MapFS: mem, blocked: "blocked"}

	got, err := scanFS(broken, []rules.StackRule{{
		ID:     "go",
		Detect: &rules.DetectConfig{Files: []string{"go.mod"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ScanErrors) == 0 {
		t.Fatal("unreadable directory must be reported, not dropped")
	}
	if !ids(got)["go"] {
		t.Fatalf("readable parts should still be scanned: %#v", ids(got))
	}
}

type failingFS struct {
	fstest.MapFS
	blocked string
}

func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.blocked {
		return nil, fmt.Errorf("permission denied")
	}
	return f.MapFS.Open(name)
}

func (f failingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == f.blocked {
		return nil, fmt.Errorf("permission denied")
	}
	return f.MapFS.ReadDir(name)
}

func TestScanFS_noisyDetectNamesNeedRoot(t *testing.T) {
	express := []rules.StackRule{{
		ID:     "express",
		Detect: &rules.DetectConfig{Files: []string{"index.js"}},
	}}

	nested, err := scanFS(mapFS(map[string]string{"src/index.js": "x"}), express)
	if err != nil {
		t.Fatal(err)
	}
	if ids(nested)["express"] {
		t.Fatal("src/index.js is not evidence of an Express server")
	}

	root, err := scanFS(mapFS(map[string]string{"index.js": "x"}), express)
	if err != nil {
		t.Fatal(err)
	}
	if !ids(root)["express"] {
		t.Fatal("root index.js should still detect Express")
	}
}

func TestScanFS_vscodeDirAloneIsNotAnExtension(t *testing.T) {
	all, err := rules.LoadAll()
	if err != nil {
		t.Fatal(err)
	}

	plain, err := scanFS(mapFS(map[string]string{".vscode/settings.json": "{}"}), all)
	if err != nil {
		t.Fatal(err)
	}
	if ids(plain)["vscode-extension"] {
		t.Fatal("a plain .vscode folder must not select the VS Code Extension stack")
	}
	if !plain.HasVSCodeDir {
		t.Fatal("HasVSCodeDir should still be true for the vscode target")
	}

	ext, err := scanFS(mapFS(map[string]string{".vscodeignore": "x"}), all)
	if err != nil {
		t.Fatal(err)
	}
	if !ids(ext)["vscode-extension"] {
		t.Fatal(".vscodeignore should detect a VS Code extension")
	}
}

func TestScanFS_siigAuthoredIgnoreFilesAreNotSignals(t *testing.T) {
	all, err := rules.LoadAll()
	if err != nil {
		t.Fatal(err)
	}

	authored := version.Header() + "\nnode_modules/\n"
	first, err := scanFS(mapFS(map[string]string{".vercelignore": authored}), all)
	if err != nil {
		t.Fatal(err)
	}
	if ids(first)["vercel"] {
		t.Fatal("SIIG-written .vercelignore must not make the next run detect Vercel")
	}

	handwritten, err := scanFS(mapFS(map[string]string{".vercelignore": "dist/\n"}), all)
	if err != nil {
		t.Fatal(err)
	}
	if !ids(handwritten)["vercel"] {
		t.Fatal("a hand-written .vercelignore should still detect Vercel")
	}
}

func TestMatchRule_glob(t *testing.T) {
	idx := newIndex()
	idx.files["App.csproj"] = true
	idx.rootFiles["App.csproj"] = true

	rule := rules.StackRule{Detect: &rules.DetectConfig{Files: []string{"*.csproj"}}}
	ok, by := matchRule(rule, idx, nil)
	if !ok || by != "App.csproj" {
		t.Fatalf("got %v %q", ok, by)
	}
}

func TestDetectNpmLib_malformedJSON(t *testing.T) {
	if detectNpmLib(mapFS(map[string]string{"package.json": "not json"})) {
		t.Fatal("malformed package.json must not count as a library")
	}
}
