package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Root is the filesystem boundary for one OpenRig state tree.
type Root struct {
	path string
}

// Open resolves value and returns a canonical state root.
//
// The local state backend requires a Unix platform. Missing roots are accepted
// for later lazy creation.
func Open(value string) (*Root, error) {
	if err := requireSupportedPlatform(value); err != nil {
		return nil, err
	}
	path, err := resolveRoot(value)
	if err != nil {
		return nil, stateError(CodeIO, value, "resolve state root", err)
	}
	info, err := os.Stat(path)
	switch {
	case err == nil && !info.IsDir():
		return nil, stateError(
			CodeInvalid,
			path,
			"state root must be a directory",
			nil,
		)
	case err == nil:
	case os.IsNotExist(err):
	default:
		return nil, stateError(CodeIO, path, "inspect state root", err)
	}
	return &Root{path: path}, nil
}

// Path returns the canonical absolute root path.
func (r *Root) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// Resolve returns the physical path within the root, rejecting lexical and symlink escapes.
func (r *Root) Resolve(path string) (string, error) {
	if r == nil || strings.TrimSpace(r.path) == "" {
		return "", stateError(CodeInvalid, path, "state root is not initialized", nil)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return r.path, nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.path, path)
	}
	physical, err := resolveExistingPath(filepath.Clean(path))
	if err != nil {
		return "", stateError(CodeInvalid, path, "resolve state path symlinks", err)
	}
	rel, err := filepath.Rel(r.path, physical)
	if err != nil {
		return "", stateError(CodeInvalid, path, "resolve path within state root", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", stateError(CodeInvalid, path, "path escapes state root", nil)
	}
	return physical, nil
}

func defaultRoot() (string, error) {
	if stateHome := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); stateHome != "" {
		return filepath.Join(stateHome, "openrig"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home for state root: %w", err)
	}
	return filepath.Join(home, ".local", "state", "openrig"), nil
}

func resolveRoot(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		var err error
		value, err = defaultRoot()
		if err != nil {
			return "", err
		}
	}
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home for state root: %w", err)
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve state root %q: %w", value, err)
	}
	resolved, err := resolveExistingPath(filepath.Clean(abs))
	if err != nil {
		return "", fmt.Errorf("resolve state root symlinks %q: %w", abs, err)
	}
	return resolved, nil
}

// resolveExistingPath resolves symlinks through the nearest existing ancestor,
// then appends any missing descendants without requiring them to exist.
func resolveExistingPath(path string) (string, error) {
	path = filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path, nil
	}
	resolvedParent, err := resolveExistingPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(path)), nil
}
