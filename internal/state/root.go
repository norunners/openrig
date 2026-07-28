package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var errUnsupportedPlatform = errors.New(
	"state storage requires macOS, Linux, or Windows",
)

// Root is the filesystem authority for one OpenRig state tree.
//
// Path is retained only as a diagnostic locator. Filesystem operations must use
// dir so they remain attached to the opened directory across topology changes.
type Root struct {
	path string
	dir  *os.Root
}

// Open opens an existing state root.
func Open(value string) (*Root, error) {
	if !supportedPlatform {
		return nil, stateError(
			CodeUnsupportedPlatform,
			value,
			"state storage is unsupported on "+runtime.GOOS,
			errUnsupportedPlatform,
		)
	}

	path, err := statePath(value)
	if err != nil {
		return nil, stateError(CodeIO, value, "resolve state root", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, stateError(CodeIO, path, "inspect state root", err)
	}
	if !info.IsDir() {
		return nil, stateError(
			CodeInvalid,
			path,
			"state root must be a directory",
			nil,
		)
	}

	dir, err := os.OpenRoot(path)
	if err != nil {
		return nil, stateError(CodeIO, path, "open state root", err)
	}
	return &Root{
		path: path,
		dir:  dir,
	}, nil
}

// Path returns the absolute diagnostic locator supplied to os.OpenRoot.
//
// Callers must not use this path for state I/O.
func (r *Root) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// Close releases the state-root filesystem handle.
func (r *Root) Close() error {
	if r == nil || r.dir == nil {
		return nil
	}
	if err := r.dir.Close(); err != nil {
		return stateError(CodeIO, r.path, "close state root", err)
	}
	return nil
}

func (r *Root) resourceName(name string) (string, string, error) {
	if r == nil || r.dir == nil {
		return "", name, stateError(
			CodeInvalid,
			name,
			"state root is not initialized",
			nil,
		)
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", r.diagnosticPath(name), stateError(
			CodeInvalid,
			r.diagnosticPath(name),
			"state resource name contains a NUL byte",
			nil,
		)
	}
	if name == "" || !filepath.IsLocal(name) || filepath.Clean(name) == "." {
		return "", r.diagnosticPath(name), stateError(
			CodeInvalid,
			r.diagnosticPath(name),
			"state resource name must be relative to the state root",
			nil,
		)
	}
	name = filepath.Clean(name)
	return name, r.diagnosticPath(name), nil
}

func (r *Root) diagnosticPath(name string) string {
	if r == nil || r.path == "" {
		return name
	}
	if name == "" {
		return r.path
	}
	return filepath.Join(r.path, name)
}

func defaultRoot() (string, error) {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		return filepath.Join(stateHome, "openrig"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home for state root: %w", err)
	}
	return filepath.Join(home, ".local", "state", "openrig"), nil
}

func statePath(value string) (string, error) {
	if value == "" {
		var err error
		value, err = defaultRoot()
		if err != nil {
			return "", err
		}
	}
	if value == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home for state root: %w", err)
		}
		value = home
	} else if len(value) >= 2 &&
		value[0] == '~' &&
		os.IsPathSeparator(value[1]) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home for state root: %w", err)
		}
		value = filepath.Join(home, value[2:])
	}
	path, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve state root %q: %w", value, err)
	}
	return filepath.Clean(path), nil
}
