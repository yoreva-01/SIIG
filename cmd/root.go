package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yv3000/siig/cache"
	"github.com/yv3000/siig/internal/version"
	"github.com/yv3000/siig/matcher"
	"github.com/yv3000/siig/output"
	"github.com/yv3000/siig/rules"
	"github.com/yv3000/siig/scanner"
	"github.com/yv3000/siig/writer"
)

var (
	dryRun     bool
	targets    string
	verbose    bool
	offline    bool
	noCache    bool
	clearCache bool
)

// rootCmd is the base command — just run "siig" and it does the thing
var rootCmd = &cobra.Command{
	Use:   "siig",
	Short: "She Is Ignoring — auto-populate ignore files based on detected stack",
	Long: `SIIG scans your project directory, detects the tech stack,
and automatically populates .gitignore, .vercelignore, .dockerignore,
and .vscode/settings.json with the correct entries.

Zero config. One command. No more leaked secrets.`,
	Version:      version.Version,
	SilenceUsage: true,
	RunE:         runSiig,
}

func init() {
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be added without writing")
	rootCmd.Flags().StringVar(&targets, "targets", "", "Comma-separated targets to populate (git,vercel,docker,npm,vscode)")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "Print every file checked and rule applied")
	rootCmd.Flags().BoolVar(&offline, "offline", false, "Skip all network requests, use embedded rules and cache only")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false, "Ignore rules cached from earlier runs and fetch fresh ones")
	rootCmd.Flags().BoolVar(&clearCache, "clear-cache", false, "Delete rules cached from earlier runs and exit")
}

// Execute runs the root command — called from main.go
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// runSiig is the main logic. Caveman flow:
// 1. Load rules
// 2. Scan project
// 3. Match rules
// 4. Write files
// 5. Print what happened
func runSiig(cmd *cobra.Command, args []string) error {
	if clearCache {
		if err := cache.Clear(); err != nil {
			return fmt.Errorf("could not clear cached rules: %w", err)
		}
		fmt.Println("✓ Cached rules deleted")
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine current directory: %w", err)
	}

	allRules, err := rules.LoadAll()
	if err != nil {
		return fmt.Errorf("failed to load rules: %w", err)
	}

	scanResult, err := scanner.Scan(cwd, allRules)
	if err != nil {
		return fmt.Errorf("failed to scan project: %w", err)
	}

	// print detected stacks
	output.PrintDetected(scanResult)
	output.PrintScanErrors(scanResult)

	if len(scanResult.DetectedStacks) == 0 {
		// still apply global rules
		fmt.Println("Applying global rules only...")
	}

	var allowedTargets []matcher.Target
	if targets != "" {
		allowedTargets, err = matcher.ParseTargets(strings.Split(targets, ","))
		if err != nil {
			return err
		}
		if len(allowedTargets) == 0 {
			return fmt.Errorf("no valid targets in %q", targets)
		}
	}

	// 4. match detected stacks against rules (with online fetch if not offline)
	matchResult := matcher.Match(scanResult, allRules, allowedTargets, matcher.Options{
		ProjectRoot: cwd,
		Offline:     offline,
		UseCache:    !noCache,
	})

	// verbose output
	if verbose {
		output.PrintVerbose(scanResult, matchResult)
	}

	// 5. write to target files
	writeResults := writer.WriteAll(cwd, matchResult, dryRun)

	// 6. print results
	if dryRun {
		output.PrintDryRun(writeResults, matchResult)
	} else {
		// check if anything was actually written
		anyWritten := false
		for _, r := range writeResults {
			if !r.Skipped && r.EntriesAdded > 0 {
				anyWritten = true
				break
			}
		}

		if anyWritten {
			output.PrintResults(writeResults)
		} else {
			output.PrintNoChanges()
		}
	}
	return nil
}
