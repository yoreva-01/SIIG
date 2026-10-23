package output

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/yv3000/siig/matcher"
	"github.com/yv3000/siig/scanner"
	"github.com/yv3000/siig/writer"
)

var (
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
	dim    = color.New(color.Faint).SprintFunc()
)

// PrintDetected prints every stack SIIG applied rules for, including the ones
// fetched from siig-rules because they are not in the embedded DB.
func PrintDetected(result *matcher.MatchResult) {
	names := detectedLine(result)
	if len(names) == 0 {
		fmt.Printf("%s No stacks detected\n", yellow("⚠"))
		return
	}

	fmt.Printf("%s Detected: %s\n", green("✓"), bold(strings.Join(names, ", ")))
}

// detectedLine keeps embedded stacks first and marks the fetched ones, so a
// reader can tell offline rules from network rules.
func detectedLine(result *matcher.MatchResult) []string {
	names := make([]string, 0, len(result.DetectedNames)+len(result.OnlineNames))
	names = append(names, result.DetectedNames...)
	for _, name := range result.OnlineNames {
		names = append(names, name+" (online)")
	}
	return names
}

// PrintScanErrors warns about paths the scanner could not read, so a partial
// scan is never mistaken for a complete one.
func PrintScanErrors(result *scanner.ScanResult) {
	const maxListed = 5
	if len(result.ScanErrors) == 0 {
		return
	}

	fmt.Printf("%s %d path(s) could not be read — results may be incomplete\n", yellow("⚠"), len(result.ScanErrors))
	for i, e := range result.ScanErrors {
		if i == maxListed {
			fmt.Printf("  %s\n", dim(fmt.Sprintf("... and %d more", len(result.ScanErrors)-maxListed)))
			break
		}
		fmt.Printf("  %s\n", dim(e))
	}
}

// PrintResults prints the write results summary
func PrintResults(results []writer.WriteResult) {
	for _, r := range results {
		targetName := targetDisplayName(r.Target)

		if r.Skipped {
			fmt.Printf("%s %s — skipped (%s)\n", yellow("⚠"), targetName, r.SkipReason)
		} else {
			action := "added"
			if r.Created {
				action = "created +"
			}
			fmt.Printf("%s %s — %d entries %s\n", green("✓"), targetName, r.EntriesAdded, action)
		}
	}
}

// PrintDryRun prints what would happen without actually writing
func PrintDryRun(results []writer.WriteResult) {
	fmt.Printf("\n%s\n\n", bold("DRY RUN — nothing will be written"))
	printPlan(results)
}

// PrintCheck prints the pending plan for --check, which writes nothing but is
// not a dry run of a command the user asked to run.
func PrintCheck(results []writer.WriteResult) {
	fmt.Printf("\n%s\n\n", bold("CHECK — nothing will be written"))
	printPlan(results)
}

func printPlan(results []writer.WriteResult) {
	for _, r := range results {
		targetName := targetDisplayName(r.Target)

		if r.Skipped {
			fmt.Printf("%s %s — skipped (%s)\n", yellow("⚠"), targetName, r.SkipReason)
		} else {
			fmt.Printf("%s %s — %d entries would be added\n", green("✓"), targetName, r.EntriesAdded)
		}
	}
}

// PrintVerbose prints detailed info about every rule applied
func PrintVerbose(scanResult *scanner.ScanResult, matchResult *matcher.MatchResult) {
	fmt.Printf("\n%s\n", bold("─── Verbose Output ───"))

	// print scan details
	fmt.Printf("\n%s\n", cyan("Detected Stacks:"))
	for _, s := range scanResult.DetectedStacks {
		fmt.Printf("  • %s (matched via: %s)\n", s.Rule.Name, dim(s.MatchedBy))
	}
	for _, name := range matchResult.OnlineNames {
		fmt.Printf("  • %s (matched via: %s)\n", name, dim("fetched from siig-rules"))
	}

	// print entries per target, in the same fixed order the writer uses
	fmt.Printf("\n%s\n", cyan("Entries by Target:"))
	for _, target := range matcher.AllTargets {
		entries := matchResult.Entries[target]
		if !matchResult.ApplicableTargets[target] || len(entries) == 0 {
			continue
		}
		fmt.Printf("\n  %s:\n", bold(string(target)))
		for _, entry := range entries {
			fmt.Printf("    %s\n", entry)
		}
	}

	if len(matchResult.VSCodeExclude) > 0 && matchResult.ApplicableTargets[matcher.TargetVSCode] {
		keys := make([]string, 0, len(matchResult.VSCodeExclude))
		for key := range matchResult.VSCodeExclude {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		fmt.Printf("\n  %s:\n", bold("vscode (files.exclude)"))
		for _, key := range keys {
			fmt.Printf("    %s: true\n", key)
		}
	}

	fmt.Printf("\n%s\n\n", bold("──────────────────────"))
}

// PrintNoChanges prints when everything is already up to date
func PrintNoChanges() {
	fmt.Printf("%s Everything already up to date — no changes needed\n", green("✓"))
}

// targetDisplayName returns a nice display name for a target
func targetDisplayName(target matcher.Target) string {
	switch target {
	case matcher.TargetGitignore:
		return ".gitignore"
	case matcher.TargetVercelignore:
		return ".vercelignore"
	case matcher.TargetDockerignore:
		return ".dockerignore"
	case matcher.TargetNpmignore:
		return ".npmignore"
	case matcher.TargetVSCode:
		return ".vscode/settings.json"
	default:
		return string(target)
	}
}
