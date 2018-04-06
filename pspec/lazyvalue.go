package pspec

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/puppetlabs/go-evaluator/eval"
	"github.com/puppetlabs/go-evaluator/impl"
	"github.com/puppetlabs/go-evaluator/types"
	"github.com/puppetlabs/go-parser/issue"
)

type (
	LazyValue interface {
		Get(tc *TestContext) eval.PValue
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
		content eval.PValue
	}

	DirectoryValue struct {
		lazyValue
		content eval.PValue
	}

	FileValue struct {
		lazyValue
		content eval.PValue
	}

	FormatValue struct {
		lazyValue
		format    eval.PValue
		arguments []eval.PValue
	}

	LazyScope struct {
		impl.BasicScope
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

func newGenericValue(content eval.PValue) *GenericValue {
	d := &GenericValue{content: content}
	d.lazyValue.initialize()
	return d
}

func (lg *LazyValueGet) Get(tc *TestContext) eval.PValue {
	if lv, ok := tc.getLazyValue(lg.valueName); ok {
		if ng, ok := lv.(*LazyValueGet); ok {
			return ng.Get(tc)
		}
		return tc.Get(lv.(LazyComputedValue))
	}
	panic(eval.Error(nil, PSPEC_GET_OF_UNKNOWN_VARIABLE, issue.H{`name`: lg.valueName}))
}

func (gv *GenericValue) Get(tc *TestContext) eval.PValue {
	return tc.resolveLazyValue(gv.content)
}

func newDirectoryValue(content eval.PValue) *DirectoryValue {
	d := &DirectoryValue{content: content}
	d.lazyValue.initialize()
	return d
}

func (dv *DirectoryValue) Get(tc *TestContext) eval.PValue {
	tmpDir, err := ioutil.TempDir(``, `pspec`)
	if err != nil {
		panic(err)
	}
	dir, ok := tc.resolveLazyValue(dv.content).(*types.HashValue)
	if !ok {
		panic(eval.Error(nil, PSPEC_VALUE_NOT_HASH, issue.H{`type`: `Directory`}))
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

func newFileValue(content eval.PValue) *FileValue {
	d := &FileValue{content: content}
	d.lazyValue.initialize()
	return d
}

func (dv *FileValue) Get(tc *TestContext) eval.PValue {
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

func newFormatValue(format eval.PValue, arguments []eval.PValue) *FormatValue {
	d := &FormatValue{format: format, arguments: arguments}
	d.lazyValue.initialize()
	return d
}

func (q *FormatValue) Get(tc *TestContext) eval.PValue {
	if format, ok := tc.resolveLazyValue(q.format).(*types.StringValue); ok {
		return types.WrapString(types.PuppetSprintf(format.String(), tc.resolveLazyValues(types.WrapArray(q.arguments))...))
	}
	panic(eval.Error(nil, PSPEC_FORMAT_NOT_STRING, issue.NO_ARGS))
}

func (ls *LazyScope) Get(name string) (value eval.PValue, found bool) {
	tc := ls.ctx
	if lv, ok := tc.getLazyValue(name); ok {
		if ng, ok := lv.(*LazyValueGet); ok {
			return ng.Get(tc), true
		}
		return tc.Get(lv.(LazyComputedValue)), true
	}
	return ls.BasicScope.Get(name)
}

func makeDirectories(parent string, hash *types.HashValue) {
	hash.EachPair(func(key, value eval.PValue) {
		name := key.String()
		path := filepath.Join(parent, name)
		if dir, ok := value.(*types.HashValue); ok {
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

func writeFileValue(path string, value eval.PValue) {
	var err error
	switch value.(type) {
	case *types.StringValue:
		err = ioutil.WriteFile(path, []byte(value.String()), 0644)
	case *types.BinaryValue:
		err = ioutil.WriteFile(path, value.(*types.BinaryValue).Bytes(), 0644)
	default:
		panic(eval.Error(nil, PSPEC_INVALID_FILE_CONTENT, issue.H{`value`: value}))
	}
	if err != nil {
		panic(err)
	}
}

func init() {
	eval.NewGoConstructor(`PSpec::Directory`,
		func(d eval.Dispatch) {
			d.Param(`Any`)
			d.Function(func(c eval.Context, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(newDirectoryValue(args[0]))
			})
		})

	eval.NewGoConstructor(`PSpec::File`,
		func(d eval.Dispatch) {
			d.Param(`Any`)
			d.Function(func(c eval.Context, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(newFileValue(args[0]))
			})
		})

	eval.NewGoConstructor(`PSpec::Format`,
		func(d eval.Dispatch) {
			d.Param(`Any`)
			d.RepeatedParam(`Any`)
			d.Function(func(c eval.Context, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(newFormatValue(args[0], args[1:]))
			})
		})

	eval.NewGoConstructor(`PSpec::Get`,
		func(d eval.Dispatch) {
			d.Param(`String[1]`)
			d.Function(func(c eval.Context, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&LazyValueGet{args[0].String()})
			})
		})

	eval.NewGoConstructor(`PSpec::Let`,
		func(d eval.Dispatch) {
			d.Param(`String[1]`)
			d.Param(`Any`)
			d.Function(func(c eval.Context, args []eval.PValue) eval.PValue {
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
