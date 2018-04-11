package pspec

import "github.com/puppetlabs/go-issues/issue"

const (
	PSPEC_GET_OF_UNKNOWN_VARIABLE = `PSPEC_GET_OF_UNKNOWN_VARIABLE`
	PSPEC_INVALID_FILE_CONTENT    = `PSPEC_INVALID_FILE_CONTENT`
	PSPEC_FORMAT_NOT_STRING       = `PSPEC_FORMAT_NOT_STRING`
	PSPEC_VALUE_NOT_HASH          = `PSPEC_VALUE_NOT_HASH`
	PSPEC_PN_PARSE_ERROR          = `PSPEC_PN_PARSE_ERROR`
)

func init() {
	issue.Hard(PSPEC_FORMAT_NOT_STRING, `Format 'format' is not a String`)
	issue.Hard(PSPEC_GET_OF_UNKNOWN_VARIABLE, `Get of unknown variable named '%{name}'`)
	issue.Hard(PSPEC_INVALID_FILE_CONTENT, `Cannot create file content from a value of type %<value>T`)
	issue.Hard(PSPEC_VALUE_NOT_HASH, `%{type} does not contain a Hash`)
	issue.Hard(PSPEC_PN_PARSE_ERROR, `PN parse error: %{detail}`)
}
