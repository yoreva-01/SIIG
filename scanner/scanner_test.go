package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yv3000/siig/rules"
)

func TestScan_emptyRoot(t *testing.T) {
	if _, err := Scan("", nil); err == nil {
		t.Fatal("expected error")
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

	ids := map[string]bool{}
	for _, s := range got.DetectedStacks {
		ids[s.Rule.ID] = true
	}
	if !ids["go"] || !ids["docker"] {
		t.Fatalf("detected %#v", ids)
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

func TestMatchRule_glob(t *testing.T) {
	rule := rules.StackRule{
		Detect: &rules.DetectConfig{Files: []string{"*.csproj"}},
	}
	ok, by := matchRule(rule, map[string]bool{"App.csproj": true}, nil, nil)
	if !ok || by != "App.csproj" {
		t.Fatalf("got %v %q", ok, by)
	}
}
