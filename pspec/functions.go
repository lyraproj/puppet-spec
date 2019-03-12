package pspec

import (
	"bytes"
	"strings"

	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/pcore/serialization"
	"github.com/lyraproj/pcore/types"
)

func init() {

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

	px.NewGoFunction(`json_to_data`,
		func(d px.Dispatch) {
			d.Param(`String`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				fc := px.NewCollector()
				serialization.JsonToData(``, strings.NewReader(args[0].String()), fc)
				return fc.Value()
			})
		})
}
