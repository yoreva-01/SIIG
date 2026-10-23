package writer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yv3000/siig/internal/version"
	"github.com/yv3000/siig/matcher"
)

// WriteResult holds what happened when writing to one target
type WriteResult struct {
	Target       matcher.Target
	FilePath     string
	EntriesAdded int
	Skipped      bool
	SkipReason   string
	Created      bool // file was created fresh
}

// WriteAll writes entries to all applicable target files.
// Caveman: read existing, figure out what's new, slap it on the end.
func WriteAll(projectRoot string, matchResult *matcher.MatchResult, dryRun bool) []WriteResult {
	var results []WriteResult

	// handle ignore-style files
	ignoreTargets := map[matcher.Target]string{
		matcher.TargetGitignore:    ".gitignore",
		matcher.TargetVercelignore: ".vercelignore",
		matcher.TargetDockerignore: ".dockerignore",
		matcher.TargetNpmignore:    ".npmignore",
	}

	for target, filename := range ignoreTargets {
		if !matchResult.ApplicableTargets[target] {
			results = append(results, WriteResult{
				Target:     target,
				FilePath:   filename,
				Skipped:    true,
				SkipReason: "not applicable for this project",
			})
			continue
		}

		entries := matchResult.Entries[target]
		if len(entries) == 0 {
			results = append(results, WriteResult{
				Target:     target,
				FilePath:   filename,
				Skipped:    true,
				SkipReason: "no entries to add",
			})
			continue
		}

		filePath := filepath.Join(projectRoot, filename)
		result := writeIgnoreFile(filePath, entries, target, dryRun)
		results = append(results, result)
	}

	// handle vscode settings
	if matchResult.ApplicableTargets[matcher.TargetVSCode] && len(matchResult.VSCodeExclude) > 0 {
		filePath := filepath.Join(projectRoot, ".vscode", "settings.json")
		result := writeVSCodeSettings(filePath, matchResult.VSCodeExclude, dryRun)
		results = append(results, result)
	} else {
		results = append(results, WriteResult{
			Target:     matcher.TargetVSCode,
			FilePath:   ".vscode/settings.json",
			Skipped:    true,
			SkipReason: "no .vscode directory found",
		})
	}

	return results
}

// writeIgnoreFile appends new entries to an ignore-style file
func writeIgnoreFile(filePath string, entries []string, target matcher.Target, dryRun bool) WriteResult {
	result := WriteResult{
		Target:   target,
		FilePath: filePath,
	}

	// read existing entries
	existing := readExistingEntries(filePath)

	// figure out what's actually new
	var newEntries []string
	for _, entry := range entries {
		if !existing[entry] {
			newEntries = append(newEntries, entry)
		}
	}

	if len(newEntries) == 0 {
		result.Skipped = true
		result.SkipReason = "all entries already present"
		return result
	}

	result.EntriesAdded = len(newEntries)

	if dryRun {
		return result
	}

	// check if file exists
	_, err := os.Stat(filePath)
	result.Created = os.IsNotExist(err)

	// build the content to append
	var builder strings.Builder
	builder.WriteString("\n")
	builder.WriteString(version.Header())
	builder.WriteString("\n")
	for _, entry := range newEntries {
		builder.WriteString(entry)
		builder.WriteString("\n")
	}

	// append or create
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("failed to open file: %v", err)
		return result
	}
	defer f.Close()

	_, err = f.WriteString(builder.String())
	if err != nil {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("failed to write: %v", err)
		return result
	}

	return result
}

// writeVSCodeSettings merges files.exclude into .vscode/settings.json
func writeVSCodeSettings(filePath string, excludes map[string]bool, dryRun bool) WriteResult {
	result := WriteResult{
		Target:   matcher.TargetVSCode,
		FilePath: filePath,
	}

	// read existing settings
	var settings map[string]interface{}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]interface{})
			result.Created = true
		} else {
			result.Skipped = true
			result.SkipReason = fmt.Sprintf("failed to read: %v", err)
			return result
		}
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			// file exists but isn't valid JSON — don't touch it
			result.Skipped = true
			result.SkipReason = "existing file is not valid JSON"
			return result
		}
	}

	// get or create files.exclude
	var filesExclude map[string]interface{}
	if existing, ok := settings["files.exclude"]; ok {
		if m, ok := existing.(map[string]interface{}); ok {
			filesExclude = m
		} else {
			filesExclude = make(map[string]interface{})
		}
	} else {
		filesExclude = make(map[string]interface{})
	}

	// count how many new entries we're adding
	added := 0
	for key, val := range excludes {
		if _, exists := filesExclude[key]; !exists {
			filesExclude[key] = val
			added++
		}
	}

	if added == 0 {
		result.Skipped = true
		result.SkipReason = "all entries already present"
		return result
	}

	result.EntriesAdded = added

	if dryRun {
		return result
	}

	settings["files.exclude"] = filesExclude

	// write back with nice formatting
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("failed to marshal JSON: %v", err)
		return result
	}

	// ensure .vscode directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("failed to create directory: %v", err)
		return result
	}

	if err := os.WriteFile(filePath, output, 0644); err != nil {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("failed to write: %v", err)
		return result
	}

	return result
}

// readExistingEntries reads an ignore file and returns a set of existing entries
func readExistingEntries(filePath string) map[string]bool {
	entries := make(map[string]bool)

	f, err := os.Open(filePath)
	if err != nil {
		return entries // file doesn't exist yet, empty set
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			entries[line] = true
		}
	}

	return entries
}
