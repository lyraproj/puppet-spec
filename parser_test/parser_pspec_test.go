package parser

import (
	. "github.com/puppetlabs/go-pspec/pspec"
	"testing"
)

func TestPrimitives(t *testing.T) {
	RunPspecTests(t, `primitives.pspec`)
}
