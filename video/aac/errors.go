package aac

import "errors"

// Sentinel errors. Messages stay in plain English so callers can match
// with errors.Is and show their own localized text on top.
var (
	ErrBadASC         = errors.New("aac: bad AudioSpecificConfig")
	ErrUnsupportedAOT = errors.New("aac: unsupported audio object type")
	ErrBadADTS        = errors.New("aac: bad ADTS header")
	ErrTruncated      = errors.New("aac: truncated packet")
	ErrUnsupported    = errors.New("aac: unsupported feature for this stage")
)
