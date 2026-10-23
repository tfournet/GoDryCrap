// Package usererr marks errors the user can fix, such as a bad flag, a
// missing or stale coverage profile, or a directory that is not a git
// repository, and carries a hint that says how. Any other error is treated
// as a bug in godrycrap.
package usererr

import "errors"

// Error is an error caused by the input, with a hint for fixing it.
type Error struct {
	Err  error
	Hint string
}

func (e *Error) Error() string { return e.Err.Error() }

func (e *Error) Unwrap() error { return e.Err }

// Wrap marks err as the user's to fix, with hint. A nil err stays nil.
func Wrap(err error, hint string) error {
	if err == nil {
		return nil
	}
	return &Error{Err: err, Hint: hint}
}

// HintOf returns the hint of the outermost user error in err's chain, and
// whether there is one.
func HintOf(err error) (string, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Hint, true
	}
	return "", false
}
