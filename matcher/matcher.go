package matcher

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/yv3000/siig/cache"
	"github.com/yv3000/siig/fetcher"
	"github.com/yv3000/siig/internal/log"
	"github.com/yv3000/siig/rules"
	"github.com/yv3000/siig/scanner"
)

// Target represents one output file target (gitignore, vercelignore, etc.)
type Target string

const (
	TargetGitignore    Target = "gitignore"
	TargetVercelignore Target = "vercelignore"
	TargetDockerignore Target = "dockerignore"
	TargetNpmignore    Target = "npmignore"
	TargetVSCode       Target = "vscode"
)

// AllTargets is the full list of supported targets
var AllTargets = []Target{
	TargetGitignore,
	TargetVercelignore,
	TargetDockerignore,
	TargetNpmignore,
	TargetVSCode,
}

// MatchResult holds the consolidated entries for each target
type MatchResult struct {
	// Entries per target — deduplicated across all detected stacks
	Entries map[Target][]string
	// VSCode settings merged from all stacks
	VSCodeExclude map[string]bool
	// Which targets are applicable based on scan result
	ApplicableTargets map[Target]bool
	// Names of detected stacks for display
	DetectedNames []string
	// OnlineNames are stacks whose rules were fetched from siig-rules because
	// they are not in the embedded DB.
	OnlineNames []string
}

// Options controls one Match call.
type Options struct {
	// ProjectRoot is the directory being populated.
	ProjectRoot string
	// Offline forbids outbound requests. Rules already in the cache still apply.
	Offline bool
	// UseCache is false when cached rules must be ignored for this run.
	UseCache bool
}

// fetchStack is the remote lookup for unknown stacks; tests replace it.
var fetchStack = fetcher.FetchStack

// maxOnlineFetches bounds how many unrecognised stacks a single run asks about.
const maxOnlineFetches = 10

// maxConcurrentFetches bounds parallel outbound requests.
const maxConcurrentFetches = 5

// Match takes scan results and all rules, produces a consolidated MatchResult
// by merging the global rule and every detected stack.
// If opts.Offline is false, also tries fetching unknown stacks from siig-rules.
func Match(scanResult *scanner.ScanResult, allRules []StackRule, allowedTargets []Target, opts Options) *MatchResult {
	result := &MatchResult{
		Entries:           make(map[Target][]string),
		VSCodeExclude:     make(map[string]bool),
		ApplicableTargets: make(map[Target]bool),
	}

	// figure out which targets are applicable
	result.ApplicableTargets[TargetGitignore] = true // always
	result.ApplicableTargets[TargetVercelignore] = scanResult.HasVercelJSON || hasStack(scanResult, "nextjs") || hasStack(scanResult, "vercel")
	result.ApplicableTargets[TargetDockerignore] = scanResult.HasDockerfile || hasStack(scanResult, "docker")
	result.ApplicableTargets[TargetVSCode] = scanResult.HasVSCodeDir
	result.ApplicableTargets[TargetNpmignore] = scanResult.HasNpmLib

	// if allowedTargets specified, filter down
	if len(allowedTargets) > 0 {
		filtered := make(map[Target]bool)
		for _, t := range allowedTargets {
			if result.ApplicableTargets[t] {
				filtered[t] = true
			}
		}
		result.ApplicableTargets = filtered
	}

	// collect entries: start with global, then each detected stack
	globalRule := rules.GetGlobal(convertRules(allRules))
	if globalRule != nil {
		mergeRuleEntries(result, globalRule)
	}

	for _, detected := range scanResult.DetectedStacks {
		result.DetectedNames = append(result.DetectedNames, detected.Rule.Name)
		mergeRuleEntries(result, &detected.Rule)
	}

	// unknown stacks: cached rules apply even offline, the network does not
	if !opts.Offline || opts.UseCache {
		candidates := buildCandidates(opts.ProjectRoot, scanResult, allRules)
		if len(candidates) > maxOnlineFetches {
			candidates = candidates[:maxOnlineFetches]
		}
		if len(candidates) > 0 {
			fetchAndMerge(result, candidates, opts)
		}
	}

	// deduplicate entries per target
	for target := range result.Entries {
		result.Entries[target] = dedupe(result.Entries[target])
	}

	return result
}

// StackRule re-export for convenience (avoiding circular import)
type StackRule = rules.StackRule

// mergeRuleEntries adds all entries from a rule into the result
func mergeRuleEntries(result *MatchResult, rule *rules.StackRule) {
	if len(rule.Rules.Gitignore) > 0 {
		result.Entries[TargetGitignore] = append(result.Entries[TargetGitignore], rule.Rules.Gitignore...)
	}
	if len(rule.Rules.Vercelignore) > 0 {
		result.Entries[TargetVercelignore] = append(result.Entries[TargetVercelignore], rule.Rules.Vercelignore...)
	}
	if len(rule.Rules.Dockerignore) > 0 {
		result.Entries[TargetDockerignore] = append(result.Entries[TargetDockerignore], rule.Rules.Dockerignore...)
	}
	if len(rule.Rules.Npmignore) > 0 {
		result.Entries[TargetNpmignore] = append(result.Entries[TargetNpmignore], rule.Rules.Npmignore...)
	}
	if rule.Rules.VSCode != nil {
		for k, v := range rule.Rules.VSCode.FilesExclude {
			result.VSCodeExclude[k] = v
		}
	}
}

// convertRules converts []StackRule to pass to rules.GetGlobal
// (they're the same type, this is just for the function signature)
func convertRules(allRules []StackRule) []rules.StackRule {
	return allRules
}

// hasStack checks if a stack ID was detected
func hasStack(scanResult *scanner.ScanResult, stackID string) bool {
	for _, s := range scanResult.DetectedStacks {
		if s.Rule.ID == stackID {
			return true
		}
	}
	return false
}

// dedupe removes duplicate strings while preserving order
func dedupe(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// ParseTargets converts user-provided target strings to Target values.
// Repeated names collapse into one entry, keeping the order they first appeared.
func ParseTargets(targets []string) ([]Target, error) {
	var result []Target
	seen := make(map[Target]bool, len(targets))
	add := func(target Target) {
		if !seen[target] {
			seen[target] = true
			result = append(result, target)
		}
	}

	for _, raw := range targets {
		t := strings.TrimSpace(strings.ToLower(raw))
		if t == "" {
			continue
		}
		switch t {
		case "git", "gitignore":
			add(TargetGitignore)
		case "vercel", "vercelignore":
			add(TargetVercelignore)
		case "docker", "dockerignore":
			add(TargetDockerignore)
		case "npm", "npmignore":
			add(TargetNpmignore)
		case "vscode":
			add(TargetVSCode)
		default:
			return nil, fmt.Errorf("unknown target %q (want git, vercel, docker, npm, vscode)", raw)
		}
	}
	return result, nil
}

// genericNames are file/dir names too common to be stack identifiers
var genericNames = map[string]bool{
	"makefile": true, "readme": true, "license": true, "dockerfile": true,
	"index": true, "main": true, "app": true, "src": true, "package": true,
	"config": true, "test": true, "build": true, "dist": true, "lib": true,
	"bin": true, "docs": true, "doc": true, "tmp": true, "temp": true,
	"vendor": true, "public": true, "static": true, "assets": true,
	"changelog": true, "contributing": true, "gitignore": true,
}

// buildCandidates extracts probable stack IDs from the project root that did
// not match any embedded rule: the lowercased file name without its extension.
func buildCandidates(root string, scanResult *scanner.ScanResult, allRules []StackRule) []string {
	// collect all detect files/dirs from embedded rules to know what's "matched"
	matchedFiles := make(map[string]bool)
	matchedDirs := make(map[string]bool)
	for _, rule := range allRules {
		if rule.Detect == nil {
			continue
		}
		for _, f := range rule.Detect.Files {
			matchedFiles[strings.ToLower(f)] = true
		}
		for _, d := range rule.Detect.Dirs {
			matchedDirs[strings.ToLower(d)] = true
		}
	}

	// also collect IDs of already-detected stacks to skip
	detectedIDs := make(map[string]bool)
	for _, s := range scanResult.DetectedStacks {
		detectedIDs[s.Rule.ID] = true
	}

	// scan the project root for candidate IDs
	candidates := make(map[string]bool)

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		name := entry.Name()
		nameLower := strings.ToLower(name)

		// skip hidden files/dirs
		if strings.HasPrefix(name, ".") {
			continue
		}

		// extract candidate ID: name without extension, lowercased
		candidate := strings.TrimSuffix(nameLower, filepath.Ext(nameLower))

		// skip if it's a generic name
		if genericNames[candidate] {
			continue
		}

		// skip if the file is already matched by embedded rules
		if matchedFiles[nameLower] {
			continue
		}
		if entry.IsDir() && matchedDirs[nameLower] {
			continue
		}

		// skip if this ID is already detected
		if detectedIDs[candidate] {
			continue
		}

		// skip very short candidates (1-2 chars are noise)
		if len(candidate) < 3 {
			continue
		}

		candidates[candidate] = true
	}

	// convert to slice, ordered
	result := make([]string, 0, len(candidates))
	for c := range candidates {
		result = append(result, c)
	}
	sort.Strings(result)
	return result
}

// fetchAndMerge resolves rules for candidate stack IDs concurrently and merges
// them into the result, in candidate order so output does not depend on which
// request finished first.
func fetchAndMerge(result *MatchResult, candidates []string, opts Options) {
	resolved := fetchConcurrently(candidates, opts)

	for _, id := range candidates {
		rule := resolved[id]
		if rule == nil {
			continue
		}
		result.OnlineNames = append(result.OnlineNames, rule.Name)
		mergeRuleEntries(result, rule)
	}
}

// fetchConcurrently looks each candidate up in the cache and, when allowed,
// online. At most maxConcurrentFetches requests are in flight.
func fetchConcurrently(candidates []string, opts Options) map[string]*rules.StackRule {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		sem      = make(chan struct{}, maxConcurrentFetches)
		resolved = make(map[string]*rules.StackRule, len(candidates))
	)

	for _, candidate := range candidates {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore slot

		go func(stackID string) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore slot

			rule, err := resolveStack(stackID, opts)
			if err != nil {
				return // network error — silently skip
			}
			if rule == nil {
				return // 404 — stack doesn't exist in siig-rules
			}

			mu.Lock()
			resolved[stackID] = rule
			mu.Unlock()
		}(candidate)
	}

	wg.Wait()
	return resolved
}

// resolveStack returns the rule for a stack id from cache, then from the
// network — unless the run is offline, in which case a cache miss is simply a
// miss.
func resolveStack(stackID string, opts Options) (*rules.StackRule, error) {
	if opts.UseCache {
		rule, err := cache.Get(stackID)
		if err != nil {
			log.Warn("cached rule unreadable", "stack", stackID, "err", err)
		} else if rule != nil {
			return rule, nil
		}
	}

	if opts.Offline {
		return nil, nil
	}

	rule, err := fetchStack(stackID)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, nil
	}

	if opts.UseCache {
		if cacheErr := cache.Set(stackID, rule); cacheErr != nil {
			log.Warn("rule cache write failed", "stack", stackID, "err", cacheErr)
		}
	}

	return rule, nil
}
