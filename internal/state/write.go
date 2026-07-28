package state

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const defaultFileMode = 0o600

// FileOptions controls atomic file publication.
type FileOptions struct {
	FileMode os.FileMode
	DirMode  os.FileMode
}

// WriteFile publishes data atomically beneath the opened state root.
func (r *Root) WriteFile(name string, data []byte, opts FileOptions) error {
	name, path, err := r.resourceName(name)
	if err != nil {
		return err
	}
	return r.writeFileAtomic(name, path, data, opts)
}

func (r *Root) writeFileAtomic(
	name string,
	path string,
	data []byte,
	opts FileOptions,
) error {
	return r.writeFileAtomicWithSync(name, path, data, opts, syncDirectory)
}

func (r *Root) writeFileAtomicWithSync(
	name string,
	path string,
	data []byte,
	opts FileOptions,
	syncDir func(*os.Root, string) error,
) error {
	opts = opts.withDefaults()
	if err := requireDurableMutations(path); err != nil {
		return err
	}
	if err := ensureParentDirectories(
		r.dir,
		name,
		opts.DirMode,
		syncDir,
	); err != nil {
		code := CodeIO
		if errors.Is(err, errPathComponentNotDirectory) {
			code = CodeInvalid
		}
		return stateError(code, path, "create parent directory", err)
	}

	parent := filepath.Dir(name)
	parentRoot, err := r.dir.OpenRoot(parent)
	if err != nil {
		return stateError(CodeInvalid, path, "open parent directory", err)
	}
	defer parentRoot.Close()

	finalName := filepath.Base(name)
	if err := validatePublicationTarget(parentRoot, finalName, path); err != nil {
		return err
	}

	tmp, tmpName, err := createTemporaryFile(parentRoot, opts.FileMode)
	if err != nil {
		return stateError(CodeIO, path, "create temporary file", err)
	}
	published := false
	defer func() {
		if !published {
			_ = parentRoot.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(opts.FileMode); err != nil {
		_ = tmp.Close()
		return stateError(CodeIO, path, "chmod temporary file", err)
	}
	written, err := tmp.Write(data)
	if err != nil {
		_ = tmp.Close()
		return stateError(CodeIO, path, "write temporary file", err)
	}
	if written != len(data) {
		_ = tmp.Close()
		return stateError(CodeIO, path, "write temporary file", io.ErrShortWrite)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return stateError(CodeIO, path, "sync temporary file", err)
	}
	if err := tmp.Close(); err != nil {
		return stateError(CodeIO, path, "close temporary file", err)
	}
	if err := parentRoot.Rename(tmpName, finalName); err != nil {
		return stateError(CodeIO, path, "replace file", err)
	}
	published = true
	if err := syncDir(parentRoot, "."); err != nil {
		return stateError(
			CodeDurability,
			path,
			"file published but parent directory sync failed",
			err,
		)
	}
	return nil
}

func validatePublicationTarget(root *os.Root, name, path string) error {
	info, err := root.Lstat(name)
	switch {
	case err == nil && !info.Mode().IsRegular():
		return stateError(
			CodeInvalid,
			path,
			"state file must not replace an alias, directory, or special entry",
			nil,
		)
	case err == nil:
		return nil
	case os.IsNotExist(err):
		return nil
	default:
		return stateError(CodeIO, path, "inspect state file", err)
	}
}

func createTemporaryFile(root *os.Root, mode os.FileMode) (*os.File, string, error) {
	for range 100 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := ".openrig-state-" + hex.EncodeToString(random[:])
		file, err := root.OpenFile(
			name,
			os.O_CREATE|os.O_EXCL|os.O_RDWR,
			mode,
		)
		if os.IsExist(err) {
			continue
		}
		return file, name, err
	}
	return nil, "", os.ErrExist
}

func (opts FileOptions) withDefaults() FileOptions {
	if opts.FileMode == 0 {
		opts.FileMode = defaultFileMode
	}
	if opts.DirMode == 0 {
		opts.DirMode = defaultDirMode
	}
	return opts
}
