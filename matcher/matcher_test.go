package matcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/yv3000/siig/cache"
	"github.com/yv3000/siig/rules"
	"github.com/yv3000/siig/scanner"
)

// stubFetch replaces the network lookup for the duration of a test and returns
// the ids it was asked for, in the order they were resolved.
func stubFetch(t *testing.T, handler func(stackID string) (*rules.StackRule, error)) *[]string {
	t.Helper()

	var (
		mu    sync.Mutex
		calls []string
	)
	original := fetchStack
	fetchStack = func(stackID string) (*rules.StackRule, error) {
		mu.Lock()
		calls = append(calls, stackID)
		mu.Unlock()
		return handler(stackID)
	}
	t.Cleanup(func() { fetchStack = original })

	return &calls
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

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

	res := Match(scan, []StackRule{global}, nil, Options{Offline: true})
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
	onlyGit := Match(scan, []StackRule{global}, filtered, Options{Offline: true})
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

	got := buildCandidates(dir, &scanner.ScanResult{}, []StackRule{{
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

func TestBuildCandidates_isOrderedAndIgnoresUnreadableRoot(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zeta.toml", "alpha.toml", "mid.toml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got := buildCandidates(dir, &scanner.ScanResult{}, nil)
	want := []string{"alpha", "mid", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates = %v, want %v", got, want)
	}

	if missing := buildCandidates(filepath.Join(dir, "nope"), &scanner.ScanResult{}, nil); missing != nil {
		t.Fatalf("unreadable root should yield no candidates, got %v", missing)
	}
}

func TestMatch_offlineNeverFetches(t *testing.T) {
	calls := stubFetch(t, func(id string) (*rules.StackRule, error) {
		return nil, errors.New("must not be called")
	})

	scan := &scanner.ScanResult{}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gleam.toml"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	Match(scan, nil, nil, Options{ProjectRoot: dir, Offline: true})
	if len(*calls) != 0 {
		t.Fatalf("offline run fetched %v", *calls)
	}
}

func TestMatch_mergesOnlineRulesInCandidateOrder(t *testing.T) {
	calls := []string{}
	var mu sync.Mutex
	original := fetchStack
	fetchStack = func(id string) (*rules.StackRule, error) {
		mu.Lock()
		calls = append(calls, id)
		mu.Unlock()
		return &rules.StackRule{ID: id, Name: upperFirst(id), Rules: rules.StackRulesList{
			Gitignore: []string{id + "/build/"},
		}}, nil
	}
	t.Cleanup(func() { fetchStack = original })

	dir := t.TempDir()
	for _, name := range []string{"zeta.toml", "alpha.toml", "mid.toml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	res := Match(&scanner.ScanResult{}, nil, nil, Options{ProjectRoot: dir})

	wantNames := []string{"Alpha (online)", "Mid (online)", "Zeta (online)"}
	if !reflect.DeepEqual(res.DetectedNames, wantNames) {
		t.Fatalf("DetectedNames = %v, want %v", res.DetectedNames, wantNames)
	}
	wantEntries := []string{"alpha/build/", "mid/build/", "zeta/build/"}
	if !reflect.DeepEqual(res.Entries[TargetGitignore], wantEntries) {
		t.Fatalf("entries = %v, want %v", res.Entries[TargetGitignore], wantEntries)
	}
	if len(calls) != 3 {
		t.Fatalf("expected 3 lookups, got %v", calls)
	}
}

func TestMatch_capsOnlineFetches(t *testing.T) {
	calls := stubFetch(t, func(id string) (*rules.StackRule, error) {
		return nil, nil
	})

	dir := t.TempDir()
	for i := 0; i < maxOnlineFetches+5; i++ {
		name := fmt.Sprintf("stack%02d.toml", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	Match(&scanner.ScanResult{}, nil, nil, Options{ProjectRoot: dir})

	if len(*calls) != maxOnlineFetches {
		t.Fatalf("fetched %d candidates, want at most %d", len(*calls), maxOnlineFetches)
	}
	asked := append([]string(nil), *calls...)
	sort.Strings(asked)
	if asked[0] != "stack00" {
		t.Fatalf("cap should keep the first candidates in order, got %v", asked)
	}
}

func TestResolveStack_skipsCacheWhenDisabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cached := &rules.StackRule{ID: "gleam", Name: "FromCache"}
	if err := cache.Set("gleam", cached); err != nil {
		t.Fatal(err)
	}

	calls := stubFetch(t, func(id string) (*rules.StackRule, error) {
		return &rules.StackRule{ID: id, Name: "FromNetwork"}, nil
	})

	got, err := resolveStack("gleam", true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "FromCache" || len(*calls) != 0 {
		t.Fatalf("cache hit expected, got %+v calls=%v", got, *calls)
	}

	fresh, err := resolveStack("gleam", false)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Name != "FromNetwork" || len(*calls) != 1 {
		t.Fatalf("--no-cache must bypass the cache, got %+v calls=%v", fresh, *calls)
	}
}

func TestResolveStack_propagatesFetchError(t *testing.T) {
	stubFetch(t, func(id string) (*rules.StackRule, error) {
		return nil, errors.New("boom")
	})

	got, err := resolveStack("gleam", false)
	if err == nil || got != nil {
		t.Fatalf("expected error, got %+v %v", got, err)
	}
}
