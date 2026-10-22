package version_test

import (
	"regexp"
	"testing"

	"mdcaster/internal/version"
)

// reSemVer valida un número MAJOR.MINOR.PATCH sin pre-release ni build.
var reSemVer = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func TestVersion_IsSemVer(t *testing.T) {
	if !reSemVer.MatchString(version.Version) {
		t.Errorf("Version = %q; se esperaba SemVer MAJOR.MINOR.PATCH", version.Version)
	}
}
