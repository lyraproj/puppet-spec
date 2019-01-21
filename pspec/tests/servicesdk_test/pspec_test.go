package servicesdk_test

import (
	"github.com/lyraproj/puppet-spec/pspec"
	"testing"

	// Ensure initialization of servicesdk
	_ "github.com/lyraproj/servicesdk/annotation"
)

func TestAll(t *testing.T) {
	pspec.RunPspecTests(t, `testdata`, nil)
}
