//go:build unix

package state

import (
	"io"
	"os"
	"syscall"
)

func requireSupportedPlatform(string) error {
	return nil
}

func readFileNoFollow(path string) ([]byte, error) {
	file, err := os.OpenFile(
		path,
		os.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func removeFileEntry(path string) error {
	return syscall.Unlink(path)
}
