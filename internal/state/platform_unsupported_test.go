//go:build !darwin && !linux && !windows

package state_test

import (
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/state"
)

func TestOpenRejectsUnsupportedPlatform(t *testing.T) {
	root, err := state.Open(t.TempDir())
	if root != nil {
		_ = root.Close()
		t.Error("Open result is non-nil on unsupported platform")
	}
	if diff := cmp.Diff(
		state.CodeUnsupportedPlatform,
		state.CodeOf(err),
	); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	if err == nil || err.Error() == "" {
		t.Errorf("Open error = %v, expected unsupported %s error", err, runtime.GOOS)
	}
}
