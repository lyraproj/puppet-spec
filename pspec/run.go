package pspec

import (
	"github.com/lyraproj/puppet-evaluator/evaluator"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lyraproj/pcore/pcore"
	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/puppet-parser/parser"

	// Ensure that all functions are loaded
	_ "github.com/lyraproj/puppet-evaluator/functions"
)

func RunPspecTests(t *testing.T, testRoot string, initializer func() px.DefiningLoader) {
	t.Helper()

	if initializer != nil {
		err := pcore.Try(func(c px.Context) error {
			c.DoWithLoader(initializer(), func() { px.ResolveResolvables(c) })
			return nil
		})
		if err != nil {
			panic(err)
		}
	}

	testFiles := make([]string, 0, 64)
	err := filepath.Walk(testRoot, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			if !info.IsDir() && strings.HasSuffix(path, `.pspec`) {
				testFiles = append(testFiles, path)
			}
		}
		return err
	})
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	tests := make([]Test, 0, 100)
	c := evaluator.NewContext(NewSpecEvaluator, px.NewParentedLoader(pcore.SystemLoader()), pcore.Logger())
	for _, testFile := range testFiles {
		tests = append(tests, CreateTests(c, parseTestContents(t, testFile))...)
	}
	runTests(t, tests, nil)
}

func runTests(t *testing.T, tests []Test, parentContext *TestContext) {
	t.Helper()

	for _, test := range tests {
		ctx := &TestContext{
			parent:         parentContext,
			tearDowns:      make([]Housekeeping, 0),
			accessedValues: make(map[int64]px.Value, 32),
			node:           test.Node()}

		if testExec, ok := test.(*TestExecutable); ok {
			t.Run(testExec.Name(), func(s *testing.T) {
				testExec.Run(ctx, &assertions{s})
			})
		} else if testGroup, ok := test.(*TestGroup); ok {
			t.Run(testGroup.Name(), func(s *testing.T) {
				runTests(s, testGroup.Tests(), ctx)
			})
		}
	}
}

func parseTestContents(t *testing.T, path string) parser.Expression {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	expr, err := parser.CreatePspecParser().Parse(path, string(content), false)
	if err != nil {
		t.Fatal(err.Error())
	}
	return expr
}

type assertions struct {
	t *testing.T
}

func (a *assertions) Fail(message string) {
	a.t.Error(message)
	a.t.FailNow()
}

func (a *assertions) AssertEquals(expected interface{}, actual interface{}) {
	if !px.Equals(expected, actual) {
		a.t.Errorf("expected %T '%v', got %T '%v'\n", expected, expected, actual, actual)
	}
}
