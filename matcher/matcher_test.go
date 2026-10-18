package matcher

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yv3000/siig/rules"
	"github.com/yv3000/siig/scanner"
)

func TestDedupe(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "empty", in: nil, want: []string{}},
		{name: "unique", in: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "dups keep first", in: []string{"a", "b", "a", "c", "b"}, want: []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupe(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("dedupe() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseTargets(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []Target
		wantErr bool
	}{
		{name: "aliases", in: []string{"git", "vercelignore", "DOCKER", " npm ", "vscode"}, want: []Target{TargetGitignore, TargetVercelignore, TargetDockerignore, TargetNpmignore, TargetVSCode}},
		{name: "unknown", in: []string{"git", "nope"}, wantErr: true},
		{name: "blanks skipped", in: []string{"", "git"}, want: []Target{TargetGitignore}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTargets(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v want %#v", got, tt.want)
			}
		})
	}
}

func TestMatch_targetsAndDedupe(t *testing.T) {
	global := StackRule{
		ID: "global",
		Rules: rules.StackRulesList{
			Gitignore: []string{".env", "node_modules/"},
		},
	}
	next := scanner.DetectedStack{
		Rule: rules.StackRule{
			ID:   "nextjs",
			Name: "Next.js",
			Rules: rules.StackRulesList{
				Gitignore:    []string{"node_modules/", ".next/"},
				Vercelignore: []string{".next/"},
				Npmignore:    []string{"src/"},
			},
		},
	}

	scan := &scanner.ScanResult{
		DetectedStacks: []scanner.DetectedStack{next},
		HasVercelJSON:  true,
		HasDockerfile:  false,
		HasVSCodeDir:   false,
		HasNpmLib:      true,
	}

	res := Match(scan, []StackRule{global}, nil, true)
	if !res.ApplicableTargets[TargetGitignore] {
		t.Fatal("gitignore always applicable")
	}
	if !res.ApplicableTargets[TargetVercelignore] {
		t.Fatal("vercel should apply")
	}
	if res.ApplicableTargets[TargetDockerignore] {
		t.Fatal("docker should not apply")
	}
	if !res.ApplicableTargets[TargetNpmignore] {
		t.Fatal("npm lib should apply")
	}

	got := res.Entries[TargetGitignore]
	want := []string{".env", "node_modules/", ".next/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gitignore entries = %#v, want %#v", got, want)
	}

	filtered, err := ParseTargets([]string{"git"})
	if err != nil {
		t.Fatal(err)
	}
	onlyGit := Match(scan, []StackRule{global}, filtered, true)
	if !onlyGit.ApplicableTargets[TargetGitignore] || onlyGit.ApplicableTargets[TargetVercelignore] {
		t.Fatalf("allowed targets not filtered: %#v", onlyGit.ApplicableTargets)
	}
}

func TestHasStack(t *testing.T) {
	scan := &scanner.ScanResult{
		DetectedStacks: []scanner.DetectedStack{
			{Rule: rules.StackRule{ID: "docker"}},
		},
	}
	if !hasStack(scan, "docker") || hasStack(scan, "nextjs") {
		t.Fatal("hasStack mismatch")
	}
}

func TestBuildCandidates(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"gleam.toml", "README.md", "ab", "go.mod"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	got := buildCandidates(&scanner.ScanResult{}, []StackRule{{
		ID:     "go",
		Detect: &rules.DetectConfig{Files: []string{"go.mod"}},
	}})
	found := map[string]bool{}
	for _, c := range got {
		found[c] = true
	}
	if !found["gleam"] {
		t.Fatalf("want gleam candidate, got %v", got)
	}
	if found["readme"] || found["ab"] || found["go"] {
		t.Fatalf("generic/short/matched leaked: %v", got)
	}
}
