//go:build !unix

package state

import (
	"errors"
	"runtime"
)

var errUnsupportedPlatform = errors.New(
	"durable local state requires a Unix operating system",
)

func requireSupportedPlatform(path string) error {
	return stateError(
		CodeUnsupportedPlatform,
		path,
		"durable local state is unsupported on "+runtime.GOOS,
		errUnsupportedPlatform,
	)
}

func readFileNoFollow(string) ([]byte, error) {
	return nil, errUnsupportedPlatform
}

func removeFileEntry(string) error {
	return errUnsupportedPlatform
}
