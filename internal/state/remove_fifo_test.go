//go:build darwin || linux

package state

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRemoveRejectsSpecialFile(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "record.fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("create FIFO: %v", err)
	}

	err := root.Remove("record.fifo")
	if !IsCode(err, CodeInvalid) {
		t.Errorf("error code = %s, expected %s", CodeOf(err), CodeInvalid)
	}
	if _, statErr := os.Lstat(path); statErr != nil {
		t.Errorf("special file was removed: %v", statErr)
	}
}
