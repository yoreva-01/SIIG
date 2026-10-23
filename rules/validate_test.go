package rules

import (
	"strings"
	"testing"
)

func validRule() *StackRule {
	return &StackRule{
		ID:   "gleam",
		Name: "Gleam",
		Rules: StackRulesList{
			Gitignore: []string{"erl_crash.dump"},
			VSCode:    &VSCodeSettings{FilesExclude: map[string]bool{"_build": true}},
		},
	}
}

func TestValidate_acceptsGoodRule(t *testing.T) {
	if err := Validate(validRule(), "gleam"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := Validate(validRule(), ""); err != nil {
		t.Fatalf("wantID is optional: %v", err)
	}
}

func TestValidate_rejects(t *testing.T) {
	longEntry := strings.Repeat("a", MaxEntryLength+1)
	longName := strings.Repeat("n", MaxNameLength+1)
	manyEntries := make([]string, MaxEntriesPerKind+1)
	for i := range manyEntries {
		manyEntries[i] = "x"
	}
	manyKeys := make(map[string]bool, MaxVSCodeKeys+1)
	for i := 0; i <= MaxVSCodeKeys; i++ {
		manyKeys[string(rune('a'+i%26))+string(rune('0'+i/26))+string(rune('A'+i/13))] = true
	}

	tests := []struct {
		name    string
		rule    *StackRule
		wantID  string
		wantErr string
	}{
		{name: "nil rule", rule: nil, wantErr: "rule is required"},
		{name: "path traversal id", rule: &StackRule{ID: "../etc", Name: "X"}, wantErr: "invalid stack id"},
		{name: "id mismatch", rule: validRule(), wantID: "other", wantErr: "does not match requested"},
		{name: "missing name", rule: &StackRule{ID: "gleam"}, wantErr: "name is required"},
		{name: "blank name", rule: &StackRule{ID: "gleam", Name: "  "}, wantErr: "name is required"},
		{name: "long name", rule: &StackRule{ID: "gleam", Name: longName}, wantErr: "name longer"},
		{name: "newline entry", rule: &StackRule{ID: "g", Name: "G", Rules: StackRulesList{Gitignore: []string{"a\nb"}}}, wantErr: "contains a newline"},
		{name: "crlf entry", rule: &StackRule{ID: "g", Name: "G", Rules: StackRulesList{Dockerignore: []string{"a\r\n"}}}, wantErr: "contains a newline"},
		{name: "empty entry", rule: &StackRule{ID: "g", Name: "G", Rules: StackRulesList{Npmignore: []string{"   "}}}, wantErr: "entry 0 is empty"},
		{name: "oversized entry", rule: &StackRule{ID: "g", Name: "G", Rules: StackRulesList{Vercelignore: []string{longEntry}}}, wantErr: "longer than"},
		{name: "too many entries", rule: &StackRule{ID: "g", Name: "G", Rules: StackRulesList{Gitignore: manyEntries}}, wantErr: "more than 512"},
		{name: "blank vscode key", rule: &StackRule{ID: "g", Name: "G", Rules: StackRulesList{VSCode: &VSCodeSettings{FilesExclude: map[string]bool{"": true}}}}, wantErr: "vscode exclude key is empty"},
		{name: "too many vscode keys", rule: &StackRule{ID: "g", Name: "G", Rules: StackRulesList{VSCode: &VSCodeSettings{FilesExclude: manyKeys}}}, wantErr: "more than 256"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.rule, tt.wantID)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}
