package servicesdk_test

import (
	"testing"

	"github.com/lyraproj/puppet-spec/pspec"

	// Ensure initialization of servicesdk
	_ "github.com/lyraproj/servicesdk/service"
)

func TestAll(t *testing.T) {
	pspec.RunPspecTests(t, `testdata`, nil)
}
