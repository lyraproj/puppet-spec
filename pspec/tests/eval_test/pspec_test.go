package eval

import (
	"testing"

	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/pcore/types"
	"github.com/lyraproj/puppet-evaluator/pdsl"
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

		return px.StaticLoader().(px.DefiningLoader)
	})
}
