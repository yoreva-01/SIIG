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

func TestDecodeRule(t *testing.T) {
	seen := map[string]bool{"go": true}

	if _, err := decodeRule("dup.json", []byte(`{"id":"go"}`), seen); err == nil {
		t.Fatal("duplicate id should be rejected")
	}
	if _, err := decodeRule("blank.json", []byte(`{"id":"   "}`), seen); err == nil {
		t.Fatal("missing id should be rejected")
	}
	if _, err := decodeRule("broken.json", []byte(`{"id":`), seen); err == nil {
		t.Fatal("malformed json should be rejected")
	}

	rule, err := decodeRule("rust.json", []byte(`{"id":"rust","name":"Rust"}`), seen)
	if err != nil {
		t.Fatal(err)
	}
	if rule.ID != "rust" || !seen["rust"] {
		t.Fatalf("rule %+v, seen %v", rule, seen)
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
