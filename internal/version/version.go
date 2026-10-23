package version

// Version is stamped at release time with
// -ldflags "-X github.com/yv3000/siig/internal/version.Version=vX.Y.Z".
var Version = "2.0.0"

// Header is the marker line SIIG writes above the entries it appends.
func Header() string {
	return "# Added by SIIG v" + Version
}
