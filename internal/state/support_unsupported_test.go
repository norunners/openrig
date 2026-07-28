//go:build !unix

package state_test

import (
	"errors"
	"testing"

	"github.com/norunners/openrig/internal/state"
)

func TestOpenRejectsUnsupportedPlatform(t *testing.T) {
	_, err := state.Open(t.TempDir())
	if err == nil {
		t.Fatal("Open error = nil, expected unsupported platform")
	}
	if !state.IsCode(err, state.CodeUnsupportedPlatform) {
		t.Errorf(
			"error code = %s, expected %s",
			state.CodeOf(err),
			state.CodeUnsupportedPlatform,
		)
	}
	var stateErr *state.Error
	if !errors.As(err, &stateErr) {
		t.Errorf("error type = %T, expected *state.Error", err)
	}
}
