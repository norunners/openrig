package state

import (
	"os"
	"path/filepath"
)

// Remove durably removes a regular state file without following aliases.
func (r *Root) Remove(name string) error {
	name, path, err := r.resourceName(name)
	if err != nil {
		return err
	}
	return r.removeFile(name, path, syncDirectory)
}

func (r *Root) removeFile(
	name string,
	path string,
	syncParent func(*os.Root, string) error,
) error {
	if err := requireDurableMutations(path); err != nil {
		return err
	}
	parentRoot, err := r.dir.OpenRoot(filepath.Dir(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return stateError(CodeIO, path, "open parent directory", err)
	}
	defer parentRoot.Close()

	finalName := filepath.Base(name)
	info, err := parentRoot.Lstat(finalName)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return stateError(CodeIO, path, "inspect state file", err)
	case !info.Mode().IsRegular():
		return stateError(
			CodeInvalid,
			path,
			"state file must be a regular file, not an alias or special entry",
			nil,
		)
	}

	if err := parentRoot.Remove(finalName); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return stateError(CodeIO, path, "remove file", err)
	}
	if err := syncParent(parentRoot, "."); err != nil {
		return stateError(
			CodeDurability,
			path,
			"file removed but parent directory sync failed",
			err,
		)
	}
	return nil
}
