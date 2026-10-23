package version

import "testing"

func TestHeaderUsesVersion(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })

	Version = "9.9.9"
	if got, want := Header(), "# Added by SIIG v9.9.9"; got != want {
		t.Fatalf("Header() = %q, want %q", got, want)
	}
}

func TestDefaultVersionIsSemver(t *testing.T) {
	if Version == "" || Version[0] != '2' {
		t.Fatalf("unexpected default version %q", Version)
	}
}
