package parser

import (
	"testing"

	. "github.com/puppetlabs/go-pspec/pspec"
)

func TestPrimitives(t *testing.T) {
	RunPspecTests(t, `basic.pspec`)
}

func TestArithmetic(t *testing.T) {
	RunPspecTests(t, `arithmetic.pspec`)
}

func TestComparison(t *testing.T) {
	RunPspecTests(t, `comparison.pspec`)
}

func TestLogical(t *testing.T) {
	RunPspecTests(t, `logical.pspec`)
}

func TestVariables(t *testing.T) {
	RunPspecTests(t, `variables.pspec`)
}

func TestFunctions(t *testing.T) {
	RunPspecTests(t, `functions.pspec`)
}

func TestConstructors(t *testing.T) {
	RunPspecTests(t, `constructors.pspec`)
}

func TestTypes(t *testing.T) {
	RunPspecTests(t, `types.pspec`)
}

func TestSemver(t *testing.T) {
	RunPspecTests(t, `semver.pspec`)
}

func TestStringFormat(t *testing.T) {
	RunPspecTests(t, `stringformat.pspec`)
}

func TestFlowControl(t *testing.T) {
	RunPspecTests(t, `flowcontrol.pspec`)
}

func TestBreak(t *testing.T) {
	RunPspecTests(t, `break.pspec`)
}

func TestNext(t *testing.T) {
	RunPspecTests(t, `next.pspec`)
}

func TestReturn(t *testing.T) {
	RunPspecTests(t, `return.pspec`)
}
