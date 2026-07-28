package state

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
)

const defaultDirMode = 0o700

var errPathComponentNotDirectory = errors.New("state path component is not a directory")

func openStateRoot(
	path string,
	mode os.FileMode,
	syncDir func(*os.Root, string) error,
) (*os.Root, error) {
	root, err := os.OpenRoot(path)
	if err == nil {
		return root, nil
	}
	if !os.IsNotExist(err) {
		info, statErr := os.Stat(path)
		if statErr == nil && !info.IsDir() {
			return nil, stateError(
				CodeInvalid,
				path,
				"state root must be a directory",
				nil,
			)
		}
		return nil, stateError(CodeIO, path, "open state root", err)
	}
	if err := requireDurableMutations(path); err != nil {
		return nil, err
	}
	return createMissingRoot(path, mode, syncDir)
}

func createMissingRoot(
	path string,
	mode os.FileMode,
	syncDir func(*os.Root, string) error,
) (*os.Root, error) {
	missing := make([]string, 0, 4)
	ancestor := path
	for {
		root, err := os.OpenRoot(ancestor)
		if err == nil {
			slices.Reverse(missing)
			created, createErr := createDirectories(
				root,
				missing,
				mode,
				syncDir,
			)
			if createErr != nil {
				_ = root.Close()
				code := CodeIO
				if errors.Is(createErr, errPathComponentNotDirectory) {
					code = CodeInvalid
				}
				return nil, stateError(
					code,
					path,
					"create state root",
					createErr,
				)
			}
			if created == nil {
				return root, nil
			}
			_ = root.Close()
			return created, nil
		}
		if !os.IsNotExist(err) {
			return nil, stateError(CodeIO, path, "open state root ancestor", err)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return nil, stateError(CodeIO, path, "locate state root ancestor", err)
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
	}
}

func ensureParentDirectories(
	root *os.Root,
	name string,
	mode os.FileMode,
	syncDir func(*os.Root, string) error,
) error {
	parent := filepath.Dir(name)
	if parent == "." {
		return nil
	}
	created, err := createDirectories(
		root,
		pathComponents(parent),
		mode,
		syncDir,
	)
	if created != nil {
		_ = created.Close()
	}
	return err
}

func createDirectories(
	root *os.Root,
	components []string,
	mode os.FileMode,
	syncDir func(*os.Root, string) error,
) (*os.Root, error) {
	parent := root
	var ownedParent *os.Root
	for _, component := range components {
		createdByCaller := false
		needsSync := false
		info, err := parent.Lstat(component)
		switch {
		case err == nil:
		case !os.IsNotExist(err):
			if ownedParent != nil {
				_ = ownedParent.Close()
			}
			return nil, err
		default:
			if err := parent.Mkdir(component, mode); err != nil {
				if !os.IsExist(err) {
					if ownedParent != nil {
						_ = ownedParent.Close()
					}
					return nil, err
				}
				// A concurrent creator made this entry after Lstat. Sync it
				// as though this call created it before publishing beneath it.
				needsSync = true
			} else {
				createdByCaller = true
				needsSync = true
			}
		}

		child, err := parent.OpenRoot(component)
		if err != nil {
			if ownedParent != nil {
				_ = ownedParent.Close()
			}
			if info != nil &&
				(!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
				return nil, errPathComponentNotDirectory
			}
			return nil, err
		}

		if createdByCaller {
			if err := chmodDirectory(child, ".", mode); err != nil {
				_ = child.Close()
				if ownedParent != nil {
					_ = ownedParent.Close()
				}
				return nil, err
			}
		}
		if needsSync {
			if err := syncDir(child, "."); err != nil {
				_ = child.Close()
				if ownedParent != nil {
					_ = ownedParent.Close()
				}
				return nil, err
			}
			if err := syncDir(parent, "."); err != nil {
				_ = child.Close()
				if ownedParent != nil {
					_ = ownedParent.Close()
				}
				return nil, err
			}
		}

		if ownedParent != nil {
			_ = ownedParent.Close()
		}
		ownedParent = child
		parent = child
	}
	return ownedParent, nil
}

func chmodDirectory(root *os.Root, name string, mode os.FileMode) error {
	dir, err := root.Open(name)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Chmod(mode)
}

func pathComponents(path string) []string {
	components := make([]string, 0, 4)
	for path != "." {
		components = append(components, filepath.Base(path))
		path = filepath.Dir(path)
	}
	slices.Reverse(components)
	return components
}
