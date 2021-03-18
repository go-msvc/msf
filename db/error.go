package db

import "fmt"

type ErrorCode int

const (
	ERR_DUPLICATE_KEY ErrorCode = iota
	ERR_NOT_FOUND
	ERR_CREATE_TABLE
	ERR_INSERT_WRONG_TYPE
	ERR_INSERT_FAILED
	ERR_INSERT_NO_ID
	ERR_KEY_FIELD_UNKNOWN
	ERR_KEY_FIELD_TYPE
	ERR_QUERY_FAILED
	ERR_QUERY_ROW_PARSER
	ERR_QUERY_ONE_HAS_MORE
	ERR_NYI
)

var ErrorName = map[ErrorCode]string{
	ERR_DUPLICATE_KEY:      "DUPLICATE_KEY",
	ERR_NOT_FOUND:          "NOT_FOUND",
	ERR_CREATE_TABLE:       "CREATE_TABLE",
	ERR_INSERT_WRONG_TYPE:  "INSERT_WRONG_TYPE",
	ERR_INSERT_FAILED:      "INSERT_FAILED",
	ERR_INSERT_NO_ID:       "INSERT_NO_ID",
	ERR_KEY_FIELD_UNKNOWN:  "KEY_FIELD_UNKNOWN",
	ERR_KEY_FIELD_TYPE:     "KEY_FIELD_TYPE",
	ERR_QUERY_FAILED:       "QUERY_FAILED",
	ERR_QUERY_ROW_PARSER:   "QUERY_ROW_PARSER",
	ERR_QUERY_ONE_HAS_MORE: "ERR_QUERY_ONE_HAS_MORE",
	ERR_NYI:                "NYI", //not yet implemented
}

func NewError(code ErrorCode, err error) IError {
	return dbError{
		error: err,
		code:  code,
	}
}

func Errorf(code ErrorCode, format string, args ...interface{}) IError {
	return dbError{
		error: fmt.Errorf(format, args...),
		code:  code,
	}
}

func Wrapf(code ErrorCode, err error) IError {
	return dbError{
		error: fmt.Errorf("%s: %v", ErrorName[code], err),
		code:  code,
	}
}

//Error implements error interface
type IError interface {
	error
	Code() ErrorCode
}

type dbError struct {
	error
	code ErrorCode
}

func (e dbError) Code() ErrorCode {
	return e.code
}
