//go:build !darwin && !linux

package state

import (
	"errors"
	"os"
	"runtime"
)

var errUnsupportedMutation = errors.New(
	"durable state mutation requires macOS or Linux",
)

func requireDurableMutations(path string) error {
	return stateError(
		CodeUnsupportedPlatform,
		path,
		"durable state mutation is unsupported on "+runtime.GOOS,
		errUnsupportedMutation,
	)
}

func syncDirectory(*os.Root, string) error {
	return errUnsupportedMutation
}
