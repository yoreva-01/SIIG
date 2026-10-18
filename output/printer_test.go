package output

import (
	"testing"

	"github.com/yv3000/siig/matcher"
	"github.com/yv3000/siig/rules"
	"github.com/yv3000/siig/scanner"
	"github.com/yv3000/siig/writer"
)

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
