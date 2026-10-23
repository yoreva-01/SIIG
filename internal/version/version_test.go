package version

import (
	"regexp"
	"testing"
)

// semver is the shape a source build must fall back to; a release tag is stamped
// on top of it with -ldflags and is allowed to carry the leading "v".
var semver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func TestDefaultVersionIsSemver(t *testing.T) {
	if !semver.MatchString(Version) {
		t.Fatalf("default Version %q is not X.Y.Z", Version)
	}
}

func TestHeaderUsesVersion(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })

	Version = "9.9.9"
	if got, want := Header(), "# Added by SIIG v9.9.9"; got != want {
		t.Fatalf("Header() = %q, want %q", got, want)
	}
}

func TestMarkerIsVersionIndependent(t *testing.T) {
	if Header()[:len(Marker)] != Marker {
		t.Fatalf("Header %q must start with the marker %q", Header(), Marker)
	}
}
