package output

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/yv3000/siig/matcher"
	"github.com/yv3000/siig/rules"
	"github.com/yv3000/siig/scanner"
	"github.com/yv3000/siig/writer"
)

// TestMain disables ANSI output so assertions compare rendered text, not
// whichever terminal the test binary happens to write to.
func TestMain(m *testing.M) {
	color.NoColor = true
	os.Exit(m.Run())
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		os.Stdout = orig
		t.Fatal(err)
	}
	os.Stdout = orig

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertContains(t *testing.T, out string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Fatalf("output missing %q:\n%s", w, out)
		}
	}
}

func TestPrintDetected(t *testing.T) {
	out := captureStdout(t, func() {
		PrintDetected(&scanner.ScanResult{})
	})
	assertContains(t, out, "No stacks detected")

	out = captureStdout(t, func() {
		PrintDetected(&scanner.ScanResult{
			DetectedStacks: []scanner.DetectedStack{
				{Rule: rules.StackRule{Name: "Go"}},
				{Rule: rules.StackRule{Name: "Node.js"}},
			},
		})
	})
	assertContains(t, out, "Detected:", "Go, Node.js")
}

func TestPrintResults(t *testing.T) {
	out := captureStdout(t, func() {
		PrintResults([]writer.WriteResult{
			{Target: matcher.TargetGitignore, EntriesAdded: 2, Created: true},
			{Target: matcher.TargetVSCode, EntriesAdded: 1},
			{Target: matcher.TargetDockerignore, Skipped: true, SkipReason: "not applicable for this project"},
		})
	})

	assertContains(t, out,
		".gitignore", "2 entries created",
		".vscode/settings.json", "1 entries added",
		".dockerignore", "skipped (not applicable for this project)")
}

func TestPrintDryRun(t *testing.T) {
	out := captureStdout(t, func() {
		PrintDryRun([]writer.WriteResult{
			{Target: matcher.TargetDockerignore, EntriesAdded: 1},
			{Target: matcher.TargetNpmignore, Skipped: true, SkipReason: "no entries to add"},
		})
	})

	assertContains(t, out, "DRY RUN — nothing will be written", ".dockerignore", "1 entries would be added", "skipped (no entries to add)")
}

func TestPrintCheck(t *testing.T) {
	out := captureStdout(t, func() {
		PrintCheck([]writer.WriteResult{{Target: matcher.TargetGitignore, EntriesAdded: 3}})
	})

	assertContains(t, out, "CHECK — nothing will be written", ".gitignore", "3 entries would be added")
	if strings.Contains(out, "DRY RUN") {
		t.Fatalf("--check must not report a dry run:\n%s", out)
	}
}

func TestPrintNoChanges(t *testing.T) {
	out := captureStdout(t, PrintNoChanges)
	assertContains(t, out, "Everything already up to date")
}

func TestPrintScanErrors(t *testing.T) {
	out := captureStdout(t, func() { PrintScanErrors(&scanner.ScanResult{}) })
	if strings.TrimSpace(out) != "" {
		t.Fatalf("clean scan should print nothing, got %q", out)
	}

	many := make([]string, 9)
	for i := range many {
		many[i] = "blocked: permission denied"
	}
	out = captureStdout(t, func() { PrintScanErrors(&scanner.ScanResult{ScanErrors: many}) })
	assertContains(t, out, "9 path(s) could not be read", "blocked: permission denied", "... and 4 more")
}

func TestPrintVerbose_isStableAndOrdered(t *testing.T) {
	scan := &scanner.ScanResult{
		DetectedStacks: []scanner.DetectedStack{{Rule: rules.StackRule{Name: "Go"}, MatchedBy: "go.mod"}},
	}
	match := &matcher.MatchResult{
		Entries: map[matcher.Target][]string{
			matcher.TargetDockerignore: {"node_modules/"},
			matcher.TargetGitignore:    {".env"},
			matcher.TargetVSCode:       {"ignored"},
		},
		VSCodeExclude:     map[string]bool{"zz": true, "aa": true},
		ApplicableTargets: map[matcher.Target]bool{matcher.TargetGitignore: true, matcher.TargetDockerignore: true, matcher.TargetVSCode: true},
	}

	first := captureStdout(t, func() { PrintVerbose(scan, match) })
	second := captureStdout(t, func() { PrintVerbose(scan, match) })
	if first != second {
		t.Fatal("verbose output must not depend on map iteration order")
	}

	assertContains(t, first, "Detected Stacks:", "Go", "matched via: go.mod", ".env", "node_modules/", "vscode (files.exclude)")
	if strings.Index(first, "gitignore") > strings.Index(first, "dockerignore") {
		t.Fatalf("targets should follow writer order:\n%s", first)
	}
	if strings.Index(first, "aa") > strings.Index(first, "zz") {
		t.Fatalf("vscode keys should be sorted:\n%s", first)
	}
}

func TestTargetDisplayName(t *testing.T) {
	tests := []struct {
		target matcher.Target
		want   string
	}{
		{matcher.TargetGitignore, ".gitignore"},
		{matcher.TargetVercelignore, ".vercelignore"},
		{matcher.TargetDockerignore, ".dockerignore"},
		{matcher.TargetNpmignore, ".npmignore"},
		{matcher.TargetVSCode, ".vscode/settings.json"},
		{"custom", "custom"},
	}
	for _, tt := range tests {
		if got := targetDisplayName(tt.target); got != tt.want {
			t.Fatalf("targetDisplayName(%q) = %q, want %q", tt.target, got, tt.want)
		}
	}
}
