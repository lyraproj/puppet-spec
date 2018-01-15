package pspec

import . "github.com/puppetlabs/go-parser/issue"

const (
	PSPEC_GET_OF_UNKNOWN_VARIABLE   = `PSPEC_GET_OF_UNKNOWN_VARIABLE`
	PSPEC_INVALID_FILE_CONTENT      = `PSPEC_INVALID_FILE_CONTENT`
	PSPEC_VALUE_NOT_HASH            = `PSPEC_VALUE_NOT_HASH`
)

func init() {
	HardIssue(PSPEC_GET_OF_UNKNOWN_VARIABLE, `Get of unknown variable named '%{name}'`)
	HardIssue(PSPEC_INVALID_FILE_CONTENT, `Cannot create file content from a value of type %<value>T`)
	HardIssue(PSPEC_VALUE_NOT_HASH, `%{type} does not contain a Hash`)
}
