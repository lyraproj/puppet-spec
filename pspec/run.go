package pspec

import (
	"io/ioutil"
	"path/filepath"
	"sync"
	"testing"

	. "github.com/puppetlabs/go-evaluator/eval"
	. "github.com/puppetlabs/go-evaluator/evaluator"
	. "github.com/puppetlabs/go-parser/parser"
)

var baseLoader DefiningLoader
var baseLoaderLock sync.Mutex

func RunPspecTests(t *testing.T, pattern string) {
	t.Helper()
	baseLoaderLock.Lock()
	if baseLoader == nil {
		baseLoader = NewParentedLoader(pcore.Loader())
		ResolveGoFunctions(baseLoader, NewStdLogger())
	}
	baseLoaderLock.Unlock()
	loader := NewParentedLoader(baseLoader)
	ResolveGoFunctions(loader, NewStdLogger())

	testFiles, err := filepath.Glob(pattern)
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	tests := make([]Test, 0, 100)
	for _, testFile := range testFiles {
		tests = append(tests, NewSpecEvaluator(loader).CreateTests(parseTestContents(t, testFile), loader)...)
	}
	runTests(t, tests)
}

func runTests(t *testing.T, tests []Test) {
	t.Helper()
	for _, test := range tests {
		if testExec, ok := test.(*TestExecutable); ok {
			t.Run(testExec.Name(), func(s *testing.T) {
				testExec.Executable()(&assertions{s})
			})
		} else if testGroup, ok := test.(*TestGroup); ok {
			t.Run(testGroup.Name(), func(s *testing.T) {
				runTests(s, testGroup.Tests())
			})
		}
	}
}

func parseTestContents(t *testing.T, path string) Expression {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	expr, err := CreatePspecParser().Parse(path, string(content), false, false)
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
	if !Equals(expected, actual) {
		a.t.Errorf("expected %T '%v', got %T '%v'\n", expected, expected, actual, actual)
	}
}
