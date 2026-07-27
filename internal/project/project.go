// Package project resolves public repository selectors and workspace paths.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/norunners/openrig/internal/result"
)

// Definition describes one configured repository and its optional aliases.
// Root must be an absolute directory path.
type Definition struct {
	Root    string
	Aliases []string
}

// Source supplies the current configured repository definitions.
type Source interface {
	Projects() map[string]Definition
}

// SourceFunc adapts a function into a project definition source.
type SourceFunc func() map[string]Definition

func (f SourceFunc) Projects() map[string]Definition {
	if f == nil {
		return nil
	}
	return f()
}

type ResolverOptions struct {
	Source        Source
	ProcessCWD    string
	AllowRelative bool
}

// Resolver converts one public repo selector into a canonical physical
// directory. It reads Source on each call so a later immutable configuration
// snapshot can be swapped without rebuilding the resolver.
type Resolver struct {
	source        Source
	processCWD    string
	allowRelative bool
}

// PathResolution describes a path physically contained by a workspace.
type PathResolution struct {
	WorkspaceRoot string
	Absolute      string
	Relative      string
}

func NewResolver(options ResolverOptions) (*Resolver, error) {
	resolver := &Resolver{
		source:        options.Source,
		allowRelative: options.AllowRelative,
	}
	if options.AllowRelative {
		if strings.TrimSpace(options.ProcessCWD) == "" {
			return nil, result.NewError(
				result.CodeInvalidArgument,
				"process cwd is required when relative repository selectors are allowed",
			).WithField("process_cwd")
		}
		cwd, err := canonicalDir(options.ProcessCWD)
		if err != nil {
			return nil, result.Wrap(
				result.CodeInvalidArgument,
				"resolve process cwd",
				err,
			).WithPath(options.ProcessCWD)
		}
		resolver.processCWD = cwd
	}
	if _, err := resolver.projects(); err != nil {
		return nil, err
	}
	return resolver, nil
}

// ResolveRepo resolves a configured name, alias, absolute path, or
// transport-permitted relative path. Configured names and aliases take
// precedence over relative path interpretation.
func (r *Resolver) ResolveRepo(repo string) (string, error) {
	if r == nil {
		return "", result.NewError(
			result.CodeInternal,
			"project resolver is not initialized",
		)
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", result.NewError(
			result.CodeInvalidArgument,
			"repo is required",
		).WithField("repo")
	}

	projects, err := r.projects()
	if err != nil {
		return "", err
	}
	if root, ok := projects[repo]; ok {
		return root, nil
	}
	if !filepath.IsAbs(repo) {
		if filepath.VolumeName(repo) != "" {
			return "", result.NewError(
				result.CodeInvalidArgument,
				"repo must not be a volume-relative path",
			).WithField("repo").WithPath(repo)
		}
		if !r.allowRelative {
			return "", result.NewError(
				result.CodeInvalidArgument,
				"repo must be a configured name, alias, or absolute path",
			).WithField("repo").WithPath(repo)
		}
		repo = filepath.Join(r.processCWD, repo)
	}

	resolved, err := canonicalDir(repo)
	if err != nil {
		return "", result.Wrap(
			result.CodeNotFound,
			"resolve repository",
			err,
		).WithField("repo").WithPath(repo)
	}
	return resolved, nil
}

// ResolvePath resolves path beneath workspaceRoot, following existing
// symlinks and the nearest existing ancestor for paths that do not yet exist.
func (r *Resolver) ResolvePath(workspaceRoot, path string) (*PathResolution, error) {
	root, err := r.ResolveRepo(workspaceRoot)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	target := path
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return nil, result.Wrap(
			result.CodeInvalidArgument,
			"resolve workspace path",
			err,
		).WithPath(path)
	}
	physical, err := physicalPath(target)
	if err != nil {
		return nil, result.Wrap(
			result.CodeForbidden,
			"resolve physical workspace path",
			err,
		).WithPath(path)
	}
	relative, err := filepath.Rel(root, physical)
	if err != nil || escapesRoot(relative) {
		return nil, result.NewError(
			result.CodeForbidden,
			"path escapes the scoped workspace root",
		).WithPath(path)
	}
	return &PathResolution{
		WorkspaceRoot: root,
		Absolute:      physical,
		Relative:      filepath.ToSlash(relative),
	}, nil
}

func (r *Resolver) projects() (map[string]string, error) {
	projects := map[string]string{}
	if r.source == nil {
		return projects, nil
	}
	for name, definition := range r.source.Projects() {
		rootValue := strings.TrimSpace(definition.Root)
		if !filepath.IsAbs(rootValue) {
			return nil, result.NewError(
				result.CodeInvalidArgument,
				"configured repository root must be absolute",
			).WithField("projects").WithPath(definition.Root)
		}
		root, err := canonicalDir(rootValue)
		if err != nil {
			return nil, result.Wrap(
				result.CodeInvalidArgument,
				"resolve configured repository root",
				err,
			).WithPath(definition.Root)
		}
		if err := addProjectName(projects, name, root); err != nil {
			return nil, err
		}
		for _, alias := range definition.Aliases {
			if err := addProjectName(projects, alias, root); err != nil {
				return nil, err
			}
		}
	}
	return projects, nil
}

func addProjectName(projects map[string]string, name, root string) error {
	name = strings.TrimSpace(name)
	if name == "" ||
		name == "." ||
		name == ".." ||
		filepath.IsAbs(name) ||
		strings.ContainsAny(name, `/\`) {
		return result.NewError(
			result.CodeInvalidArgument,
			"configured repository name or alias must be one path-free name",
		).WithField("projects").WithPath(name)
	}
	if existing, ok := projects[name]; ok && existing != root {
		return result.NewError(
			result.CodeInvalidArgument,
			"configured repository name or alias resolves to multiple roots",
		).WithField("projects").WithPath(name)
	}
	projects[name] = root
	return nil
}

func canonicalDir(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make path absolute: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("path is not a directory")
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve directory symlinks: %w", err)
	}
	return filepath.Clean(physical), nil
}

func physicalPath(path string) (string, error) {
	path = filepath.Clean(path)
	current := path
	for {
		_, err := os.Lstat(current)
		if err == nil {
			physical, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", fmt.Errorf("resolve symlinks for %q: %w", current, err)
			}
			relative, err := filepath.Rel(current, path)
			if err != nil {
				return "", fmt.Errorf("resolve path relative to %q: %w", current, err)
			}
			return filepath.Clean(filepath.Join(physical, relative)), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect path %q: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", os.ErrNotExist
		}
		current = parent
	}
}

func escapesRoot(relative string) bool {
	return relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
