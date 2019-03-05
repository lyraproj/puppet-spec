package pspec

import "github.com/lyraproj/issue/issue"

const (
	GetOfUnknownVariable = `PSPEC_GET_OF_UNKNOWN_VARIABLE`
	InvalidFileContent   = `PSPEC_INVALID_FILE_CONTENT`
	FormatNotString      = `PSPEC_FORMAT_NOT_STRING`
	ValueNotHash         = `PSPEC_VALUE_NOT_HASH`
	PnParseError         = `PSPEC_PN_PARSE_ERROR`
)

func init() {
	issue.Hard(FormatNotString, `Format 'format' is not a String`)
	issue.Hard(GetOfUnknownVariable, `Get of unknown variable named '%{name}'`)
	issue.Hard(InvalidFileContent, `Cannot create file content from a value of type %<value>T`)
	issue.Hard(ValueNotHash, `%{type} does not contain a Hash`)
	issue.Hard(PnParseError, `PN parse error: %{detail}`)
}
