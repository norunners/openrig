//go:build windows

package state

import "os"

func openReadCandidate(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY, 0)
}

func readCandidateMatches(before, opened os.FileInfo) bool {
	return os.SameFile(before, opened)
}
