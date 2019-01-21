package eval

import (
	"bytes"
	"github.com/lyraproj/puppet-evaluator/impl"
	"strings"
	"testing"

	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/puppet-evaluator/eval"
	"github.com/lyraproj/puppet-evaluator/serialization"
	"github.com/lyraproj/puppet-evaluator/types"
	"github.com/lyraproj/puppet-spec/pspec"
)

func TestPSpecs(t *testing.T) {
	pspec.RunPspecTests(t, `testdata`, func() eval.DefiningLoader {
		eval.NewGoFunction(`load_plan`,
			func(d eval.Dispatch) {
				d.Param(`String`)
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					planName := args[0].String()
					if plan, ok := eval.Load(c, eval.NewTypedName(eval.NsPlan, planName)); ok {
						return eval.Wrap(nil, plan)
					}
					panic(eval.Error(eval.EVAL_UNKNOWN_PLAN, issue.H{`name`: planName}))
				})
			})

		eval.NewGoFunction(`load_task`,
			func(d eval.Dispatch) {
				d.Param(`String`)
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					taskName := args[0].String()
					if task, ok := eval.Load(c, eval.NewTypedName(eval.NsTask, taskName)); ok {
						return task.(eval.Value)
					}
					panic(eval.Error(eval.EVAL_UNKNOWN_TASK, issue.H{`name`: taskName}))
				})
			})

		eval.NewGoFunction(`to_symbol`,
			func(d eval.Dispatch) {
				d.Param(`String`)
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					return types.WrapRuntime(serialization.Symbol(args[0].String()))
				})
			})

		eval.NewGoFunction(`to_data`,
			func(d eval.Dispatch) {
				d.Param(`Any`)
				d.OptionalParam(
					`Struct[
  Optional['local_reference'] => Boolean,
  Optional['symbol_as_string'] => Boolean,
  Optional['rich_data'] => Boolean,
  Optional['message_prefix'] => String
]`)
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					options := eval.EMPTY_MAP
					if len(args) > 1 {
						options = args[1].(eval.OrderedMap)
					}
					s := serialization.NewSerializer(c, options)
					cl := serialization.NewCollector()
					s.Convert(args[0], cl)
					return cl.Value()
				})
			})

		eval.NewGoFunction(`from_data`,
			func(d eval.Dispatch) {
				d.Param(`Data`)
				d.OptionalParam(
					`Struct[
					Optional['allow_unresolved'] => Boolean
				]`)
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					options := eval.EMPTY_MAP
					if len(args) > 1 {
						options = args[1].(eval.OrderedMap)
					}
					s := serialization.NewSerializer(c, eval.Wrap(c, map[string]bool{`rich_data`: false, `local_reference`: false}).(eval.OrderedMap))
					d := serialization.NewDeserializer(c, options)
					s.Convert(args[0], d)
					return d.Value()
				})
			})

		eval.NewGoFunction(`data_to_json`,
			func(d eval.Dispatch) {
				d.Param(`Data`)
				d.OptionalParam(
					`Struct[
					Optional['prefix'] => String,
					Optional['indent'] => String
				]`)
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					options := eval.EMPTY_MAP
					if len(args) > 1 {
						options = args[1].(eval.OrderedMap)
					}
					out := bytes.NewBufferString(``)
					js := serialization.NewJsonStreamer(out)
					serialization.NewSerializer(c, options).Convert(args[0], js)
					return types.WrapString(out.String())
				})
			})

		eval.NewGoFunction(`all_types`,
			func(d eval.Dispatch) {
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					allTypes := make([]eval.Value, 0, 50)
					impl.EachCoreType(func(t eval.Type) { allTypes = append(allTypes, t) })
					return types.WrapValues(allTypes)
				})
			})

		eval.NewGoFunction(`abstract_types`,
			func(d eval.Dispatch) {
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					return types.WrapValues([]eval.Value{
						types.DefaultAnyType(),
						types.DefaultAnnotationType(),
						types.DefaultCallableType(),
						types.DefaultCollectionType(),
						types.DefaultEnumType(),
						types.DefaultDataType(),
						types.DefaultDefaultType(),
						types.DefaultInitType(),
						types.DefaultIterableType(),
						types.DefaultIteratorType(),
						types.DefaultLikeType(),
						types.DefaultNotUndefType(),
						types.DefaultObjectType(),
						types.DefaultOptionalType(),
						types.DefaultRuntimeType(),
						types.DefaultPatternType(),
						types.DefaultRichDataType(),
						types.DefaultScalarDataType(),
						types.DefaultScalarType(),
						types.DefaultTypeReferenceType(),
						types.DefaultTypeAliasType(),
						types.DefaultTypeSetType(),
						types.DefaultVariantType(),
						types.DefaultUndefType(),
					})
				})
			})

		eval.NewGoFunction(`internal_types`,
			func(d eval.Dispatch) {
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					return types.WrapValues([]eval.Value{
						types.DefaultTypeReferenceType(),
						types.DefaultTypeAliasType(),
					})
				})
			})

		eval.NewGoFunction(`scalar_types`,
			func(d eval.Dispatch) {
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					return types.WrapValues([]eval.Value{
						types.DefaultScalarDataType(),
						types.DefaultScalarType(),
						types.DefaultStringType(),
						types.DefaultNumericType(),
						types.DefaultIntegerType(),
						types.DefaultFloatType(),
						types.DefaultBooleanType(),
						types.DefaultRegexpType(),
						types.DefaultPatternType(),
						types.DefaultEnumType(),
						types.DefaultSemVerType(),
						types.DefaultTimespanType(),
						types.DefaultTimestampType(),
					})
				})
			})

		eval.NewGoFunction(`json_to_data`,
			func(d eval.Dispatch) {
				d.Param(`String`)
				d.Function(func(c eval.Context, args []eval.Value) eval.Value {
					fc := serialization.NewCollector()
					serialization.JsonToData(``, strings.NewReader(args[0].String()), fc)
					return fc.Value()
				})
			})

		return eval.StaticLoader().(eval.DefiningLoader)
	})
}
