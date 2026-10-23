package output

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/yv3000/siig/matcher"
	"github.com/yv3000/siig/rules"
	"github.com/yv3000/siig/scanner"
	"github.com/yv3000/siig/writer"
)

func TestPrintScanErrors(t *testing.T) {
	PrintScanErrors(&scanner.ScanResult{})
	PrintScanErrors(&scanner.ScanResult{ScanErrors: []string{"a: denied"}})
	many := make([]string, 9)
	for i := range many {
		many[i] = "x: denied"
	}
	PrintScanErrors(&scanner.ScanResult{ScanErrors: many})
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
	if strings.Index(first, "gitignore") > strings.Index(first, "dockerignore") {
		t.Fatalf("targets should follow writer order:\n%s", first)
	}
	if strings.Index(first, "aa") > strings.Index(first, "zz") {
		t.Fatalf("vscode keys should be sorted:\n%s", first)
	}
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

func TestPrinters(t *testing.T) {
	empty := &scanner.ScanResult{}
	PrintDetected(empty)
	PrintDetected(&scanner.ScanResult{
		DetectedStacks: []scanner.DetectedStack{{Rule: rules.StackRule{Name: "Go"}, MatchedBy: "go.mod"}},
	})
	PrintNoChanges()
	PrintResults([]writer.WriteResult{
		{Target: matcher.TargetGitignore, Skipped: true, SkipReason: "no entries to add"},
		{Target: matcher.TargetGitignore, EntriesAdded: 2, Created: true},
	})
	PrintDryRun([]writer.WriteResult{
		{Target: matcher.TargetDockerignore, EntriesAdded: 1},
	}, &matcher.MatchResult{})
	PrintVerbose(&scanner.ScanResult{
		DetectedStacks: []scanner.DetectedStack{{Rule: rules.StackRule{Name: "Go"}, MatchedBy: "go.mod"}},
	}, &matcher.MatchResult{
		Entries:           map[matcher.Target][]string{matcher.TargetGitignore: {".env"}},
		ApplicableTargets: map[matcher.Target]bool{matcher.TargetGitignore: true},
	})
	if targetDisplayName(matcher.TargetGitignore) != ".gitignore" {
		t.Fatal("display name")
	}
}
