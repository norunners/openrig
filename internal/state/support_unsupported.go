//go:build !unix

package state

import (
	"errors"
	"runtime"
)

var errUnsupportedPlatform = errors.New(
	"local state requires a Unix operating system",
)

func requireSupportedPlatform(path string) error {
	return stateError(
		CodeUnsupportedPlatform,
		path,
		"local state is unsupported on "+runtime.GOOS,
		errUnsupportedPlatform,
	)
}
