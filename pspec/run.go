package pspec

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/puppetlabs/go-evaluator/eval"
	"github.com/puppetlabs/go-evaluator/impl"
	"github.com/puppetlabs/go-evaluator/pcore"
	"github.com/puppetlabs/go-parser/parser"
)

var baseLoader eval.DefiningLoader
var baseLoaderLock sync.Mutex

func RunPspecTests(t *testing.T, testRoot string, initializer func() eval.DefiningLoader) {
	t.Helper()
	pcore.InitializePuppet()
	baseLoaderLock.Lock()
	logger := eval.NewStdLogger()
	if baseLoader == nil {
		baseLoader = eval.NewParentedLoader(eval.Puppet.SystemLoader())
		impl.ResolveResolvables(baseLoader, logger)
	}
	baseLoaderLock.Unlock()

	if initializer != nil {
		eval.Puppet.ResolveResolvables(initializer())
	}

	loader := eval.NewParentedLoader(baseLoader)

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
	runTests(t, loader, tests, nil)
}

func runTests(t *testing.T, loader eval.Loader, tests []Test, parentContext *TestContext) {
	t.Helper()

	for _, test := range tests {
		ctx := &TestContext{
			parent:         parentContext,
			tearDowns:      make([]Housekeeping, 0),
			accessedValues: make(map[int64]eval.PValue, 32),
			node:           test.Node(),
			loader:         loader}

		if testExec, ok := test.(*TestExecutable); ok {
			t.Run(testExec.Name(), func(s *testing.T) {
				testExec.Run(ctx, &assertions{s})
			})
		} else if testGroup, ok := test.(*TestGroup); ok {
			t.Run(testGroup.Name(), func(s *testing.T) {
				runTests(s, loader, testGroup.Tests(), ctx)
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
	if !eval.Equals(expected, actual) {
		a.t.Errorf("expected %T '%v', got %T '%v'\n", expected, expected, actual, actual)
	}
}
