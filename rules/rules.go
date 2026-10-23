package rules

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"unicode"
)

//go:embed db/*.json
var rulesFS embed.FS

// DetectConfig tells the scanner what files/dirs indicate this stack
type DetectConfig struct {
	Files []string `json:"files"`
	Dirs  []string `json:"dirs"`
}

// VSCodeSettings holds vscode-specific settings like files.exclude
type VSCodeSettings struct {
	FilesExclude map[string]bool `json:"files.exclude,omitempty"`
}

// StackRule is one stack's complete rule definition loaded from JSON
type StackRule struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Detect *DetectConfig  `json:"detect"` // nil for global
	Rules  StackRulesList `json:"rules"`
}

// StackRulesList holds all the ignore/config entries for a stack
type StackRulesList struct {
	Gitignore    []string        `json:"gitignore,omitempty"`
	Vercelignore []string        `json:"vercelignore,omitempty"`
	Dockerignore []string        `json:"dockerignore,omitempty"`
	Npmignore    []string        `json:"npmignore,omitempty"`
	VSCode       *VSCodeSettings `json:"vscode,omitempty"`
}

// LoadAll reads every JSON file from the embedded rules/db/ directory
// and returns a slice of StackRule.
func LoadAll() ([]StackRule, error) {
	var rules []StackRule

	entries, err := fs.ReadDir(rulesFS, "db")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded rules dir: %w", err)
	}

	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := rulesFS.ReadFile("db/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read rule file %s: %w", entry.Name(), err)
		}

		rule, err := decodeRule(entry.Name(), data, seen)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// decodeRule parses one rule file and rejects the mistakes that would silently
// break detection: no id, or an id already claimed by another file.
func decodeRule(name string, data []byte, seen map[string]bool) (StackRule, error) {
	var rule StackRule
	if err := json.Unmarshal(data, &rule); err != nil {
		return StackRule{}, fmt.Errorf("failed to parse rule file %s: %w", name, err)
	}
	if strings.TrimSpace(rule.ID) == "" {
		return StackRule{}, fmt.Errorf("rule file %s: missing id", name)
	}
	if seen[rule.ID] {
		return StackRule{}, fmt.Errorf("rule file %s: duplicate id %q", name, rule.ID)
	}
	seen[rule.ID] = true

	return rule, nil
}

// GetGlobal returns the global rule (detect == nil, always applied)
func GetGlobal(rules []StackRule) *StackRule {
	for i := range rules {
		if rules[i].Detect == nil {
			return &rules[i]
		}
	}
	return nil
}

// GetDetectable returns all rules that have detection config (not global)
func GetDetectable(rules []StackRule) []StackRule {
	var detectable []StackRule
	for _, r := range rules {
		if r.Detect != nil {
			detectable = append(detectable, r)
		}
	}
	return detectable
}

// ValidateStackID rejects empty, path-like, or otherwise unsafe stack IDs.
func ValidateStackID(stackID string) error {
	if stackID == "" {
		return fmt.Errorf("stack id is required")
	}
	if len(stackID) > 64 {
		return fmt.Errorf("stack id too long")
	}
	if strings.ContainsAny(stackID, `/\`) || strings.Contains(stackID, "..") {
		return fmt.Errorf("invalid stack id")
	}
	for _, r := range stackID {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("invalid stack id")
	}
	return nil
}
