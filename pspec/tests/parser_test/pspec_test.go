package parser_test

import (
	"github.com/lyraproj/puppet-spec/pspec"
	"testing"
)

func TestAll(t *testing.T) {
	pspec.RunPspecTests(t, `testdata`, nil)
}
