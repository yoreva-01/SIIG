package fetcher

import "testing"

func TestFetchStack_badID(t *testing.T) {
	if _, err := FetchStack("../passwd"); err == nil {
		t.Fatal("expected error")
	}
}
