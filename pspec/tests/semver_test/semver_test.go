package semver_test

import (
"testing"
"github.com/lyraproj/puppet-spec/pspec"
)

func TestAll(t *testing.T) {
	pspec.RunPspecTests(t, `testdata`, nil)
}
