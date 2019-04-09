package pspec

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/pcore/types"
	"github.com/lyraproj/puppet-evaluator/evaluator"
)

type (
	LazyValue interface {
		Get(tc *TestContext) px.Value
	}

	LazyComputedValue interface {
		LazyValue
		Id() int64
	}

	LazyValueGet struct {
		valueName string
	}

	LazyValueLet struct {
		valueName string
		value     LazyValue
	}

	lazyValue struct {
		id int64
	}

	GenericValue struct {
		lazyValue
		content px.Value
	}

	DirectoryValue struct {
		lazyValue
		content px.Value
	}

	FileValue struct {
		lazyValue
		content px.Value
	}

	FormatValue struct {
		lazyValue
		format    px.Value
		arguments []px.Value
	}

	LazyScope struct {
		evaluator.BasicScope
		ctx *TestContext
	}
)

var nextLazyId = int64(0)

func (lv *lazyValue) initialize() {
	lv.id = atomic.AddInt64(&nextLazyId, 1)
}

func (lv *lazyValue) Id() int64 {
	return lv.id
}

func newGenericValue(content px.Value) *GenericValue {
	d := &GenericValue{content: content}
	d.lazyValue.initialize()
	return d
}

func (lg *LazyValueGet) Get(tc *TestContext) px.Value {
	if lv, ok := tc.getLazyValue(lg.valueName); ok {
		if ng, ok := lv.(*LazyValueGet); ok {
			return ng.Get(tc)
		}
		return tc.Get(lv.(LazyComputedValue))
	}
	panic(px.Error(GetOfUnknownVariable, issue.H{`name`: lg.valueName}))
}

func (gv *GenericValue) Get(tc *TestContext) px.Value {
	return tc.resolveLazyValue(gv.content)
}

func newDirectoryValue(content px.Value) *DirectoryValue {
	d := &DirectoryValue{content: content}
	d.lazyValue.initialize()
	return d
}

func (dv *DirectoryValue) Get(tc *TestContext) px.Value {
	tmpDir, err := ioutil.TempDir(``, `pspec`)
	if err != nil {
		panic(err)
	}
	dir, ok := tc.resolveLazyValue(dv.content).(*types.Hash)
	if !ok {
		panic(px.Error(ValueNotHash, issue.H{`type`: `Directory`}))
	}
	makeDirectories(tmpDir, dir)
	tc.registerTearDown(func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			panic(err)
		}
	})
	return types.WrapString(tmpDir)
}

func newFileValue(content px.Value) *FileValue {
	d := &FileValue{content: content}
	d.lazyValue.initialize()
	return d
}

func (dv *FileValue) Get(tc *TestContext) px.Value {
	tmpFile, err := ioutil.TempFile(``, `pspec`)
	if err != nil {
		panic(err)
	}
	path := tmpFile.Name()
	writeFileValue(path, tc.resolveLazyValue(dv.content))
	tc.registerTearDown(func() {
		err := os.Remove(path)
		if err != nil {
			panic(err)
		}
	})
	return types.WrapString(path)
}

func newFormatValue(format px.Value, arguments []px.Value) *FormatValue {
	d := &FormatValue{format: format, arguments: arguments}
	d.lazyValue.initialize()
	return d
}

func (q *FormatValue) Get(tc *TestContext) px.Value {
	if format, ok := tc.resolveLazyValue(q.format).(px.StringValue); ok {
		return types.WrapString(types.PuppetSprintf(format.String(), tc.resolveLazyValues(types.WrapValues(q.arguments))...))
	}
	panic(px.Error(FormatNotString, issue.NoArgs))
}

func (ls *LazyScope) Get(name px.Value) (value px.Value, found bool) {
	return ls.Get2(name.String())
}

func (ls *LazyScope) Get2(name string) (value px.Value, found bool) {
	tc := ls.ctx
	if lv, ok := tc.getLazyValue(name); ok {
		if ng, ok := lv.(*LazyValueGet); ok {
			return ng.Get(tc), true
		}
		return tc.Get(lv.(LazyComputedValue)), true
	}
	return ls.BasicScope.Get2(name)
}

func (ls *LazyScope) State(name string) px.VariableState {
	return ls.BasicScope.State(name)
}

func makeDirectories(parent string, hash *types.Hash) {
	hash.EachPair(func(key, value px.Value) {
		name := key.String()
		path := filepath.Join(parent, name)
		if dir, ok := value.(*types.Hash); ok {
			err := os.Mkdir(path, 0755)
			if err != nil {
				panic(err)
			}
			makeDirectories(path, dir)
		} else {
			writeFileValue(path, value)
		}
	})
}

func writeFileValue(path string, value px.Value) {
	var err error
	switch value := value.(type) {
	case px.StringValue:
		err = ioutil.WriteFile(path, []byte(value.String()), 0644)
	case *types.Binary:
		err = ioutil.WriteFile(path, value.Bytes(), 0644)
	default:
		panic(px.Error(InvalidFileContent, issue.H{`value`: value}))
	}
	if err != nil {
		panic(err)
	}
}

func init() {
	px.NewGoConstructor(`PSpec::Directory`,
		func(d px.Dispatch) {
			d.Param(`Any`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(newDirectoryValue(args[0]))
			})
		})

	px.NewGoConstructor(`PSpec::File`,
		func(d px.Dispatch) {
			d.Param(`Any`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(newFileValue(args[0]))
			})
		})

	px.NewGoConstructor(`PSpec::Format`,
		func(d px.Dispatch) {
			d.Param(`Any`)
			d.RepeatedParam(`Any`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(newFormatValue(args[0], args[1:]))
			})
		})

	px.NewGoConstructor(`PSpec::Get`,
		func(d px.Dispatch) {
			d.Param(`String[1]`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&LazyValueGet{args[0].String()})
			})
		})

	px.NewGoConstructor(`PSpec::Let`,
		func(d px.Dispatch) {
			d.Param(`String[1]`)
			d.Param(`Any`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				v := args[1]
				var lv LazyValue
				r, ok := v.(*types.RuntimeValue)
				if ok {
					lv, ok = r.Interface().(LazyValue)
				}
				if !ok {
					lv = newGenericValue(v)
				}
				return types.WrapRuntime(&LazyValueLet{args[0].String(), lv})
			})
		})
}
