package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/result"
)

func TestResolveRepo(t *testing.T) {
	configuredRoot := t.TempDir()
	processCWD := t.TempDir()
	relativeRoot := filepath.Join(processCWD, "relative")
	if err := os.Mkdir(relativeRoot, 0o755); err != nil {
		t.Fatalf("create relative repository: %v", err)
	}
	if err := os.Mkdir(filepath.Join(processCWD, "openrig"), 0o755); err != nil {
		t.Fatalf("create colliding relative repository: %v", err)
	}
	source := SourceFunc(func() map[string]Definition {
		return map[string]Definition{
			"openrig": {
				Root:    configuredRoot,
				Aliases: []string{"rig"},
			},
		}
	})
	relativeResolver, err := NewResolver(ResolverOptions{
		Source:        source,
		ProcessCWD:    processCWD,
		AllowRelative: true,
	})
	if err != nil {
		t.Fatalf("NewResolver(relative) returned error: %v", err)
	}
	strictResolver, err := NewResolver(ResolverOptions{Source: source})
	if err != nil {
		t.Fatalf("NewResolver(strict) returned error: %v", err)
	}
	configuredRoot, err = filepath.EvalSymlinks(configuredRoot)
	if err != nil {
		t.Fatalf("resolve configured root symlinks: %v", err)
	}
	relativeRoot, err = filepath.EvalSymlinks(relativeRoot)
	if err != nil {
		t.Fatalf("resolve relative root symlinks: %v", err)
	}

	tests := []struct {
		name         string
		resolver     *Resolver
		repo         string
		expected     string
		expectedCode result.Code
	}{
		{
			name:     "configured name",
			resolver: strictResolver,
			repo:     "openrig",
			expected: configuredRoot,
		},
		{
			name:     "configured alias",
			resolver: strictResolver,
			repo:     "rig",
			expected: configuredRoot,
		},
		{
			name:     "configured name precedes relative path",
			resolver: relativeResolver,
			repo:     "openrig",
			expected: configuredRoot,
		},
		{
			name:     "absolute path",
			resolver: strictResolver,
			repo:     configuredRoot,
			expected: configuredRoot,
		},
		{
			name:     "relative path when allowed",
			resolver: relativeResolver,
			repo:     "relative",
			expected: relativeRoot,
		},
		{
			name:         "empty selector",
			resolver:     relativeResolver,
			expectedCode: result.CodeInvalidArgument,
		},
		{
			name:         "relative path when forbidden",
			resolver:     strictResolver,
			repo:         "relative",
			expectedCode: result.CodeInvalidArgument,
		},
		{
			name:         "missing absolute path",
			resolver:     strictResolver,
			repo:         filepath.Join(configuredRoot, "missing"),
			expectedCode: result.CodeNotFound,
		},
		{
			name:         "nil resolver",
			expectedCode: result.CodeInternal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := test.resolver.ResolveRepo(test.repo)
			if test.expectedCode != "" {
				if err == nil {
					t.Fatal("ResolveRepo error = nil, expected error")
				}
				if diff := cmp.Diff(test.expectedCode, result.ErrorOf(err).Code); diff != "" {
					t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveRepo returned error: %v", err)
			}
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch resolved repository (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestResolverReadsCurrentDefinitions(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	definitions := map[string]Definition{
		"openrig": {
			Root: first,
		},
	}
	resolver, err := NewResolver(ResolverOptions{
		Source: SourceFunc(func() map[string]Definition {
			return definitions
		}),
	})
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}
	definitions = map[string]Definition{
		"openrig": {
			Root: second,
		},
	}

	actual, err := resolver.ResolveRepo("openrig")
	if err != nil {
		t.Fatalf("ResolveRepo returned error: %v", err)
	}
	expected, err := filepath.EvalSymlinks(second)
	if err != nil {
		t.Fatalf("resolve expected symlinks: %v", err)
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch resolved repository (-expected, +actual):\n%s", diff)
	}
}

func TestResolverRejectsRelativeRootFromUpdatedDefinitions(t *testing.T) {
	root := t.TempDir()
	definitions := map[string]Definition{
		"openrig": {
			Root: root,
		},
	}
	resolver, err := NewResolver(ResolverOptions{
		Source: SourceFunc(func() map[string]Definition {
			return definitions
		}),
	})
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}
	definitions = map[string]Definition{
		"openrig": {
			Root: "repo",
		},
	}

	_, err = resolver.ResolveRepo("openrig")
	if err == nil {
		t.Fatal("ResolveRepo error = nil, expected error")
	}
	if diff := cmp.Diff(
		result.CodeInvalidArgument,
		result.ErrorOf(err).Code,
	); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
}

func TestNewResolverRejectsInvalidConfiguration(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	tests := []struct {
		name         string
		options      ResolverOptions
		expectedCode result.Code
	}{
		{
			name: "relative policy without process cwd",
			options: ResolverOptions{
				AllowRelative: true,
			},
			expectedCode: result.CodeInvalidArgument,
		},
		{
			name: "empty configured name",
			options: ResolverOptions{
				Source: SourceFunc(func() map[string]Definition {
					return map[string]Definition{
						"": {
							Root: first,
						},
					}
				}),
			},
			expectedCode: result.CodeInvalidArgument,
		},
		{
			name: "relative configured root",
			options: ResolverOptions{
				Source: SourceFunc(func() map[string]Definition {
					return map[string]Definition{
						"openrig": {
							Root: "repo",
						},
					}
				}),
			},
			expectedCode: result.CodeInvalidArgument,
		},
		{
			name: "path shaped alias",
			options: ResolverOptions{
				Source: SourceFunc(func() map[string]Definition {
					return map[string]Definition{
						"openrig": {
							Root:    first,
							Aliases: []string{"team/openrig"},
						},
					}
				}),
			},
			expectedCode: result.CodeInvalidArgument,
		},
		{
			name: "alias collision",
			options: ResolverOptions{
				Source: SourceFunc(func() map[string]Definition {
					return map[string]Definition{
						"first": {
							Root:    first,
							Aliases: []string{"shared"},
						},
						"second": {
							Root:    second,
							Aliases: []string{"shared"},
						},
					}
				}),
			},
			expectedCode: result.CodeInvalidArgument,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewResolver(test.options)
			if err == nil {
				t.Fatal("NewResolver error = nil, expected error")
			}
			if diff := cmp.Diff(test.expectedCode, result.ErrorOf(err).Code); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatalf("create inside directory: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(inside, "existing.txt"),
		[]byte("content\n"),
		0o600,
	); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	resolver, err := NewResolver(ResolverOptions{})
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve root symlinks: %v", err)
	}

	tests := []struct {
		name         string
		path         string
		expectedPath string
		expectedCode result.Code
	}{
		{
			name:         "existing file",
			path:         "inside/existing.txt",
			expectedPath: "inside/existing.txt",
		},
		{
			name:         "not yet created descendant",
			path:         "inside/new/deep.txt",
			expectedPath: "inside/new/deep.txt",
		},
		{
			name:         "empty path selects root",
			expectedPath: ".",
		},
		{
			name:         "lexical traversal",
			path:         "../outside.txt",
			expectedCode: result.CodeForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := resolver.ResolvePath(root, test.path)
			if test.expectedCode != "" {
				if err == nil {
					t.Fatal("ResolvePath error = nil, expected error")
				}
				if diff := cmp.Diff(test.expectedCode, result.ErrorOf(err).Code); diff != "" {
					t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolvePath returned error: %v", err)
			}
			expected := &PathResolution{
				WorkspaceRoot: canonicalRoot,
				Absolute: filepath.Join(
					canonicalRoot,
					filepath.FromSlash(test.expectedPath),
				),
				Relative: test.expectedPath,
			}
			if test.expectedPath == "." {
				expected.Absolute = canonicalRoot
			}
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("mismatch path resolution (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestResolvePathRejectsSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	resolver, err := NewResolver(ResolverOptions{})
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "existing symlink escape",
			path: "escape",
		},
		{
			name: "not yet created symlink escape",
			path: "escape/new/deep.txt",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := resolver.ResolvePath(root, test.path)
			if err == nil {
				t.Fatal("ResolvePath error = nil, expected error")
			}
			if diff := cmp.Diff(
				result.CodeForbidden,
				result.ErrorOf(err).Code,
			); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}
