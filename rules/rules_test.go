package rules

import (
	"testing"
)

func TestLoadAll(t *testing.T) {
	all, err := LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 50 {
		t.Fatalf("too few rules: %d", len(all))
	}
	g := GetGlobal(all)
	if g == nil || g.ID != "global" {
		t.Fatalf("global = %+v", g)
	}
	d := GetDetectable(all)
	if len(d) != len(all)-1 {
		t.Fatalf("detectable %d vs total %d", len(d), len(all))
	}
	seen := map[string]bool{}
	for _, r := range all {
		if seen[r.ID] {
			t.Fatalf("duplicate id %s", r.ID)
		}
		seen[r.ID] = true
	}
}

func TestValidateStackID(t *testing.T) {
	ok := []string{"go", "next-js", "node.js", "c_sharp"}
	for _, id := range ok {
		if err := ValidateStackID(id); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	bad := []string{"", "../etc", "a/b", `a\b`, "id with space", string(make([]byte, 65))}
	for _, id := range bad {
		if err := ValidateStackID(id); err == nil {
			t.Fatalf("expected error for %q", id)
		}
	}
}
