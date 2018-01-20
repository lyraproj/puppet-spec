package pspec

import (
	"io/ioutil"
	"sync"
	"testing"

	. "github.com/puppetlabs/go-evaluator/eval"
	. "github.com/puppetlabs/go-evaluator/evaluator"
	. "github.com/puppetlabs/go-parser/parser"
	"github.com/puppetlabs/go-evaluator/pcore"
	"path/filepath"
	"os"
	"strings"
)

var baseLoader DefiningLoader
var baseLoaderLock sync.Mutex

func RunPspecTests(t *testing.T, testRoot string) {
	t.Helper()
	baseLoaderLock.Lock()
	logger := NewStdLogger()
	if baseLoader == nil {
		baseLoader = NewParentedLoader(pcore.NewPcore(logger).SystemLoader())
		ResolveResolvables(baseLoader, logger)
	}
	baseLoaderLock.Unlock()
	loader := NewParentedLoader(baseLoader)

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
	for _, testFile := range testFiles {
		tests = append(tests, NewSpecEvaluator(loader).CreateTests(parseTestContents(t, testFile), loader)...)
	}
	runTests(t, tests, nil)
}

func runTests(t *testing.T, tests []Test, parentContext *TestContext) {
	t.Helper()

	for _, test := range tests {
		ctx := &TestContext{
			parent: parentContext,
			tearDowns: make([]Housekeeping, 0),
			accessedValues: make(map[int64]PValue, 32),
			node: test.Node()}

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
