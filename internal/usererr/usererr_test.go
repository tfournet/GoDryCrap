package usererr

import (
	"errors"
	"fmt"
	"testing"
)

func TestWrapAndHint(t *testing.T) {
	if Wrap(nil, "h") != nil {
		t.Error("Wrap(nil) is not nil")
	}
	base := errors.New("no such file")
	err := fmt.Errorf("loading: %w", Wrap(base, "check the path"))
	if hint, ok := HintOf(err); !ok || hint != "check the path" {
		t.Errorf("HintOf = %q, %v", hint, ok)
	}
	if !errors.Is(err, base) || err.Error() != "loading: no such file" {
		t.Errorf("err = %v, want it to wrap %v", err, base)
	}
	if _, ok := HintOf(base); ok {
		t.Error("a plain error has a hint")
	}
}
