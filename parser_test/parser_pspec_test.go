package parser

import (
	"testing"

	. "github.com/puppetlabs/go-pspec/pspec"
)

func TestPrimitives(t *testing.T) {
	RunPspecTests(t, `testdata`)
}
