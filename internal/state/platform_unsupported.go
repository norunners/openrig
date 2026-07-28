//go:build !darwin && !linux && !windows

package state

import "os"

const supportedPlatform = false

func openReadCandidate(*os.Root, string) (*os.File, error) {
	return nil, errUnsupportedPlatform
}

func readCandidateMatches(os.FileInfo, os.FileInfo) bool {
	return false
}
