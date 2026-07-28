//go:build darwin || linux

package state

import (
	"os"
	"syscall"
)

func openReadCandidate(root *os.Root, name string) (*os.File, error) {
	// os.Root owns containment and contained-symlink resolution. The preceding
	// Lstat rejects stable final aliases. Nonblocking open prevents a concurrent
	// FIFO replacement from hanging before descriptor validation.
	return root.OpenFile(
		name,
		os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW,
		0,
	)
}

func readCandidateMatches(os.FileInfo, os.FileInfo) bool {
	// A rooted, nonblocking Unix open followed by regular-file descriptor
	// validation is the snapshot boundary. Atomic replacement may legitimately
	// change the selected inode between Lstat and OpenFile.
	return true
}
