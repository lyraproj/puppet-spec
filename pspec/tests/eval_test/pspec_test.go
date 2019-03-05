package eval

import (
	"bytes"
	"github.com/lyraproj/puppet-evaluator/pdsl"
	"strings"
	"testing"

	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/pcore/serialization"
	"github.com/lyraproj/pcore/types"
	"github.com/lyraproj/puppet-spec/pspec"
)

func TestPSpecs(t *testing.T) {
	pspec.RunPspecTests(t, `testdata`, func() px.DefiningLoader {
		px.NewGoFunction(`load_plan`,
			func(d px.Dispatch) {
				d.Param(`String`)
				d.Function(func(c px.Context, args []px.Value) px.Value {
					planName := args[0].String()
					if plan, ok := px.Load(c, px.NewTypedName(px.NsPlan, planName)); ok {
						return px.Wrap(nil, plan)
					}
					panic(px.Error(pdsl.UnknownPlan, issue.H{`name`: planName}))
				})
			})

		px.NewGoFunction(`load_task`,
			func(d px.Dispatch) {
				d.Param(`String`)
				d.Function(func(c px.Context, args []px.Value) px.Value {
					taskName := args[0].String()
					if task, ok := px.Load(c, px.NewTypedName(px.NsTask, taskName)); ok {
						return task.(px.Value)
					}
					panic(px.Error(pdsl.UnknownTask, issue.H{`name`: taskName}))
				})
			})

		px.NewGoFunction(`to_data`,
			func(d px.Dispatch) {
				d.Param(`Any`)
				d.OptionalParam(
					`Struct[
  Optional['local_reference'] => Boolean,
  Optional['symbol_as_string'] => Boolean,
  Optional['rich_data'] => Boolean,
  Optional['message_prefix'] => String
]`)
				d.Function(func(c px.Context, args []px.Value) px.Value {
					options := px.EmptyMap
					if len(args) > 1 {
						options = args[1].(px.OrderedMap)
					}
					s := serialization.NewSerializer(c, options)
					cl := px.NewCollector()
					s.Convert(args[0], cl)
					return cl.Value()
				})
			})

		px.NewGoFunction(`from_data`,
			func(d px.Dispatch) {
				d.Param(`Data`)
				d.OptionalParam(
					`Struct[
					Optional['allow_unresolved'] => Boolean
				]`)
				d.Function(func(c px.Context, args []px.Value) px.Value {
					options := px.EmptyMap
					if len(args) > 1 {
						options = args[1].(px.OrderedMap)
					}
					s := serialization.NewSerializer(c, px.Wrap(c, map[string]bool{`rich_data`: false, `local_reference`: false}).(px.OrderedMap))
					d := serialization.NewDeserializer(c, options)
					s.Convert(args[0], d)
					return d.Value()
				})
			})

		px.NewGoFunction(`data_to_json`,
			func(d px.Dispatch) {
				d.Param(`Data`)
				d.OptionalParam(
					`Struct[
					Optional['prefix'] => String,
					Optional['indent'] => String
				]`)
				d.Function(func(c px.Context, args []px.Value) px.Value {
					options := px.EmptyMap
					if len(args) > 1 {
						options = args[1].(px.OrderedMap)
					}
					out := bytes.NewBufferString(``)
					js := serialization.NewJsonStreamer(out)
					serialization.NewSerializer(c, options).Convert(args[0], js)
					return types.WrapString(out.String())
				})
			})

		px.NewGoFunction(`all_types`,
			func(d px.Dispatch) {
				d.Function(func(c px.Context, args []px.Value) px.Value {
					allTypes := make([]px.Value, 0, 50)
					types.EachCoreType(func(t px.Type) { allTypes = append(allTypes, t) })
					return types.WrapValues(allTypes)
				})
			})

		px.NewGoFunction(`abstract_types`,
			func(d px.Dispatch) {
				d.Function(func(c px.Context, args []px.Value) px.Value {
					return types.WrapValues([]px.Value{
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

		px.NewGoFunction(`internal_types`,
			func(d px.Dispatch) {
				d.Function(func(c px.Context, args []px.Value) px.Value {
					return types.WrapValues([]px.Value{
						types.DefaultTypeReferenceType(),
						types.DefaultTypeAliasType(),
					})
				})
			})

		px.NewGoFunction(`scalar_types`,
			func(d px.Dispatch) {
				d.Function(func(c px.Context, args []px.Value) px.Value {
					return types.WrapValues([]px.Value{
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

		px.NewGoFunction(`json_to_data`,
			func(d px.Dispatch) {
				d.Param(`String`)
				d.Function(func(c px.Context, args []px.Value) px.Value {
					fc := px.NewCollector()
					serialization.JsonToData(``, strings.NewReader(args[0].String()), fc)
					return fc.Value()
				})
			})

		return px.StaticLoader().(px.DefiningLoader)
	})
}
