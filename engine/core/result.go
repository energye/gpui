package core

import (
	"errors"
	"strconv"
	"strings"
)

// Code classifies a game-side failure so callers can tell a missing file
// from corrupt data from an exhausted GPU budget.
type Code int

const (
	CodeUnknown Code = iota
	CodeNotFound
	CodeBadData
	CodeOutOfMemory
	CodeUnsupported
	CodeInvalidArg
	CodeVersionMismatch
)

// String returns the stable log name of c.
func (c Code) String() string {
	switch c {
	case CodeNotFound:
		return "not-found"
	case CodeBadData:
		return "bad-data"
	case CodeOutOfMemory:
		return "out-of-memory"
	case CodeUnsupported:
		return "unsupported"
	case CodeInvalidArg:
		return "invalid-arg"
	case CodeVersionMismatch:
		return "version-mismatch"
	default:
		return "unknown"
	}
}

// Error is the unified game-side error: what kind, which operation, which
// asset, and the wrapped cause when there is one.
type Error struct {
	Code Code
	Op   string
	ID   string
	Err  error
}

// Error renders `op "id": code: cause` (empty parts skipped).
func (e *Error) Error() string {
	var b strings.Builder
	if e.Op != "" {
		b.WriteString(e.Op)
	} else {
		b.WriteString("core")
	}
	if e.ID != "" {
		b.WriteString(" ")
		b.WriteString(strconv.Quote(e.ID))
	}
	b.WriteString(": ")
	b.WriteString(e.Code.String())
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

// Unwrap returns the wrapped cause, if any.
func (e *Error) Unwrap() error { return e.Err }

func wrapErr(code Code, op, id string, cause []error) *Error {
	e := &Error{Code: code, Op: op, ID: id}
	for _, c := range cause {
		if c != nil {
			e.Err = c
			break
		}
	}
	return e
}

// New builds a Code error without a cause.
func New(code Code, op, id string) *Error { return wrapErr(code, op, id, nil) }

// NotFound reports a missing file, asset, or handle.
func NotFound(op, id string, cause ...error) *Error {
	return wrapErr(CodeNotFound, op, id, cause)
}

// BadData reports corrupt or unparsable data.
func BadData(op, id string, cause ...error) *Error {
	return wrapErr(CodeBadData, op, id, cause)
}

// OutOfMemory reports an exhausted memory or VRAM budget.
func OutOfMemory(op, id string, cause ...error) *Error {
	return wrapErr(CodeOutOfMemory, op, id, cause)
}

// Unsupported reports a format or path that is reserved but not built yet
// (unfrozen Spine/TMX subsets, P5 float pipeline, ...).
func Unsupported(op, id string, cause ...error) *Error {
	return wrapErr(CodeUnsupported, op, id, cause)
}

// InvalidArg reports an empty, zero, or otherwise illegal argument.
func InvalidArg(op, id string, cause ...error) *Error {
	return wrapErr(CodeInvalidArg, op, id, cause)
}

// VersionMismatch reports data whose major version cannot load here.
func VersionMismatch(op, id string, cause ...error) *Error {
	return wrapErr(CodeVersionMismatch, op, id, cause)
}

// CodeOf classifies err: the Code of a *Error (even when wrapped), or
// CodeUnknown for nil and foreign errors.
func CodeOf(err error) Code {
	if err == nil {
		return CodeUnknown
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeUnknown
}
