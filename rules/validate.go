package rules

import (
	"fmt"
	"strings"
)

// Limits applied to rules that arrive from outside the embedded DB.
const (
	MaxNameLength     = 80
	MaxEntriesPerKind = 512
	MaxEntryLength    = 256
	MaxVSCodeKeys     = 256
)

// Validate reports whether a rule is safe to merge into a user's ignore files.
//
// Remote and cached rules are untrusted input: they are appended verbatim to
// .gitignore and friends, so an entry carrying a newline could inject lines the
// user never asked for, and an oversized payload could bloat the file.
// wantID is the stack the caller asked for; a rule that answers with a
// different id is treated as corrupt rather than trusted.
func Validate(rule *StackRule, wantID string) error {
	if rule == nil {
		return fmt.Errorf("rule is required")
	}
	if err := ValidateStackID(rule.ID); err != nil {
		return err
	}
	if wantID != "" && rule.ID != wantID {
		return fmt.Errorf("rule id %q does not match requested %q", rule.ID, wantID)
	}
	if strings.TrimSpace(rule.Name) == "" {
		return fmt.Errorf("rule %s: name is required", rule.ID)
	}
	if len(rule.Name) > MaxNameLength {
		return fmt.Errorf("rule %s: name longer than %d characters", rule.ID, MaxNameLength)
	}

	lists := []struct {
		kind    string
		entries []string
	}{
		{"gitignore", rule.Rules.Gitignore},
		{"vercelignore", rule.Rules.Vercelignore},
		{"dockerignore", rule.Rules.Dockerignore},
		{"npmignore", rule.Rules.Npmignore},
	}
	for _, list := range lists {
		if err := validateEntries(rule.ID, list.kind, list.entries); err != nil {
			return err
		}
	}

	if rule.Rules.VSCode == nil {
		return nil
	}
	if len(rule.Rules.VSCode.FilesExclude) > MaxVSCodeKeys {
		return fmt.Errorf("rule %s: vscode has more than %d exclude keys", rule.ID, MaxVSCodeKeys)
	}
	for key := range rule.Rules.VSCode.FilesExclude {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("rule %s: vscode exclude key is empty", rule.ID)
		}
		if len(key) > MaxEntryLength || strings.ContainsAny(key, "\r\n") {
			return fmt.Errorf("rule %s: invalid vscode exclude key %q", rule.ID, truncate(key))
		}
	}
	return nil
}

func validateEntries(id, kind string, entries []string) error {
	if len(entries) > MaxEntriesPerKind {
		return fmt.Errorf("rule %s: %s has more than %d entries", id, kind, MaxEntriesPerKind)
	}
	for i, entry := range entries {
		if strings.TrimSpace(entry) == "" {
			return fmt.Errorf("rule %s: %s entry %d is empty", id, kind, i)
		}
		if len(entry) > MaxEntryLength {
			return fmt.Errorf("rule %s: %s entry %d is longer than %d characters", id, kind, i, MaxEntryLength)
		}
		if strings.ContainsAny(entry, "\r\n") {
			return fmt.Errorf("rule %s: %s entry %d contains a newline", id, kind, i)
		}
	}
	return nil
}

func truncate(s string) string {
	if len(s) > 24 {
		return s[:24] + "..."
	}
	return s
}
