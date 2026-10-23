package version

// Version is stamped at release time with
// -ldflags "-X github.com/yv3000/siig/internal/version.Version=vX.Y.Z".
var Version = "2.0.0"

// Marker is the version-independent prefix of SIIG's block header. Readers use
// it to recognise files SIIG wrote earlier.
const Marker = "# Added by SIIG"

// Header is the marker line SIIG writes above the entries it appends.
func Header() string {
	return Marker + " v" + Version
}
