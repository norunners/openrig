//go:build darwin || linux || windows

package state_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/state"
)

type exampleRecord struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestReadJSONReturnsVersionedRecord(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	name := "state.json"
	writeRecordFixture(
		t,
		filepath.Join(root.Path(), name),
		`{"schema_version":1,"kind":"turn","name":"review","count":7}`,
	)

	var actual exampleRecord
	if err := root.ReadJSON(
		name,
		&actual,
		state.JSONOptions{Kind: "turn"},
	); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	expected := exampleRecord{Name: "review", Count: 7}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch read record (-expected, +actual):\n%s", diff)
	}
}

func TestReadJSONValidatesRecordHeaderAndBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		pathSuffix string
		errorCode  state.ErrorCode
	}{
		{
			name:       "missing file",
			pathSuffix: "missing.json",
			errorCode:  state.CodeNotFound,
		},
		{
			name:       "malformed json",
			body:       `{"schema_version":1,"kind":"worktree"`,
			pathSuffix: "malformed.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "missing header",
			body:       `{"name":"worktree"}`,
			pathSuffix: "missing-header.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "negative schema version",
			body:       `{"schema_version":-1,"kind":"worktree","name":"worktree"}`,
			pathSuffix: "negative-version.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "blank kind",
			body:       `{"schema_version":1,"kind":"   ","name":"worktree"}`,
			pathSuffix: "blank-kind.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "kind with surrounding whitespace",
			body:       `{"schema_version":1,"kind":" worktree ","name":"worktree"}`,
			pathSuffix: "noncanonical-kind.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "malformed body",
			body:       `{"schema_version":1,"kind":"worktree","count":"invalid"}`,
			pathSuffix: "malformed-body.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "unsupported version",
			body:       `{"schema_version":2,"kind":"worktree","name":"worktree"}`,
			pathSuffix: "unsupported-version.json",
			errorCode:  state.CodeUnsupportedVersion,
		},
		{
			name:       "kind mismatch",
			body:       `{"schema_version":1,"kind":"turn","name":"worktree"}`,
			pathSuffix: "kind-mismatch.json",
			errorCode:  state.CodeKindMismatch,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := openStateRoot(t, t.TempDir())
			if test.body != "" {
				writeRecordFixture(
					t,
					filepath.Join(root.Path(), test.pathSuffix),
					test.body,
				)
			}

			var record exampleRecord
			err := root.ReadJSON(
				test.pathSuffix,
				&record,
				state.JSONOptions{Kind: "worktree"},
			)
			if err == nil {
				t.Fatalf("ReadJSON error = nil, expected %s", test.errorCode)
			}
			if diff := cmp.Diff(test.errorCode, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestReadJSONAcceptsRecordAtMaximumSize(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	const maximumRecordBytes = 16 << 20
	prefix := `{"schema_version":1,"kind":"turn","name":"`
	suffix := `"}`
	body := prefix + strings.Repeat(
		"x",
		maximumRecordBytes-len(prefix)-len(suffix),
	) + suffix
	if diff := cmp.Diff(maximumRecordBytes, len(body)); diff != "" {
		t.Fatalf("mismatch fixture size (-expected, +actual):\n%s", diff)
	}
	writeRecordFixture(t, filepath.Join(root.Path(), "maximum.json"), body)

	var actual exampleRecord
	if err := root.ReadJSON(
		"maximum.json",
		&actual,
		state.JSONOptions{Kind: "turn"},
	); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	expectedNameLength := maximumRecordBytes - len(prefix) - len(suffix)
	if diff := cmp.Diff(expectedNameLength, len(actual.Name)); diff != "" {
		t.Errorf("mismatch decoded name length (-expected, +actual):\n%s", diff)
	}
}

func TestReadJSONRejectsSparseOversizedRecord(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "oversized.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create sparse record: %v", err)
	}
	const oversizedRecordBytes = 128 << 20
	if err := file.Truncate(oversizedRecordBytes); err != nil {
		_ = file.Close()
		t.Fatalf("truncate sparse record: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close sparse record: %v", err)
	}

	var actual exampleRecord
	err = root.ReadJSON(
		"oversized.json",
		&actual,
		state.JSONOptions{Kind: "turn"},
	)
	if diff := cmp.Diff(state.CodeMalformed, state.CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
}

func TestReadJSONRejectsInvalidUTF8(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{
			name: "body value",
			body: append(
				[]byte(`{"schema_version":1,"kind":"turn","name":"`),
				[]byte{0xff, '"', '}'}...,
			),
		},
		{
			name: "record kind",
			body: append(
				[]byte(`{"schema_version":1,"kind":"`),
				[]byte{0xff, '"', '}'}...,
			),
		},
		{
			name: "field name",
			body: append(
				[]byte(`{"schema_version":1,"kind":"turn","`),
				[]byte{0xff, '"', ':', '1', '}'}...,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := openStateRoot(t, t.TempDir())
			if err := os.WriteFile(
				filepath.Join(root.Path(), "invalid.json"),
				test.body,
				0o600,
			); err != nil {
				t.Fatalf("write invalid UTF-8 record: %v", err)
			}

			var actual exampleRecord
			err := root.ReadJSON(
				"invalid.json",
				&actual,
				state.JSONOptions{Kind: "turn"},
			)
			if diff := cmp.Diff(state.CodeMalformed, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestReadJSONValidatesTargetBeforeReading(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	var typedNil *exampleRecord
	tests := []struct {
		name   string
		target any
	}{
		{
			name: "nil target",
		},
		{
			name:   "non-pointer target",
			target: exampleRecord{},
		},
		{
			name:   "typed nil target",
			target: typedNil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := root.ReadJSON(
				"missing.json",
				test.target,
				state.JSONOptions{Kind: "worktree"},
			)
			if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestReadJSONValidatesOptions(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	tests := []struct {
		name    string
		options state.JSONOptions
	}{
		{
			name: "kind is required",
		},
		{
			name: "explicit version must be positive",
			options: state.JSONOptions{
				Kind:          "worktree",
				SchemaVersion: -1,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var target exampleRecord
			err := root.ReadJSON("missing.json", &target, test.options)
			if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestReadJSONRejectsFinalSymlinkAlias(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	target := filepath.Join(root.Path(), "runtime.json")
	expected := []byte(
		`{"schema_version":1,"kind":"runtime","name":"runtime"}`,
	)
	if err := os.WriteFile(target, expected, 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	alias := filepath.Join(root.Path(), "operation.json")
	if err := os.Symlink("runtime.json", alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	var record exampleRecord
	err := root.ReadJSON(
		"operation.json",
		&record,
		state.JSONOptions{Kind: "operation"},
	)
	if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	actual, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read target after rejected alias: %v", readErr)
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch target content (-expected, +actual):\n%s", diff)
	}
	info, lstatErr := os.Lstat(alias)
	if lstatErr != nil {
		t.Fatalf("lstat alias after rejected read: %v", lstatErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("alias mode = %s, expected symbolic link", info.Mode())
	}
}

func TestReadJSONRejectsDirectory(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "resource")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create directory fixture: %v", err)
	}

	var record exampleRecord
	err := root.ReadJSON(
		"resource",
		&record,
		state.JSONOptions{Kind: "worktree"},
	)
	if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
}

func TestReadJSONRejectsStateRoot(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	var record exampleRecord
	err := root.ReadJSON("", &record, state.JSONOptions{Kind: "worktree"})
	if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
}

func TestReadJSONRejectsInvalidResourceNames(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	tests := []struct {
		name     string
		resource string
	}{
		{
			name:     "absolute",
			resource: filepath.Join(root.Path(), "state.json"),
		},
		{
			name:     "parent traversal",
			resource: filepath.Join("..", "state.json"),
		},
		{
			name:     "embedded NUL",
			resource: "state\x00.json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var record exampleRecord
			err := root.ReadJSON(
				test.resource,
				&record,
				state.JSONOptions{Kind: "worktree"},
			)
			if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestReadJSONUsesContainedIntermediateSymlink(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	physicalDir := filepath.Join(root.Path(), "physical")
	if err := os.Mkdir(physicalDir, 0o700); err != nil {
		t.Fatalf("create physical directory: %v", err)
	}
	writeRecordFixture(
		t,
		filepath.Join(physicalDir, "state.json"),
		`{"schema_version":1,"kind":"turn","name":"review"}`,
	)
	link := filepath.Join(root.Path(), "link")
	if err := os.Symlink("physical", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	var actual exampleRecord
	if err := root.ReadJSON(
		filepath.Join("link", "state.json"),
		&actual,
		state.JSONOptions{Kind: "turn"},
	); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if diff := cmp.Diff("review", actual.Name); diff != "" {
		t.Errorf("mismatch record name (-expected, +actual):\n%s", diff)
	}
}

func TestReadJSONRejectsEscapingIntermediateSymlink(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	outside := t.TempDir()
	writeRecordFixture(
		t,
		filepath.Join(outside, "state.json"),
		`{"schema_version":1,"kind":"turn","name":"outside"}`,
	)
	link := filepath.Join(root.Path(), "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	var record exampleRecord
	err := root.ReadJSON(
		filepath.Join("escape", "state.json"),
		&record,
		state.JSONOptions{Kind: "turn"},
	)
	if err == nil {
		t.Fatal("ReadJSON error = nil, expected escaping symlink rejection")
	}
}

func TestStateErrorPreservesTypeAndCause(t *testing.T) {
	root := openStateRoot(t, t.TempDir())

	var record exampleRecord
	err := root.ReadJSON(
		"missing.json",
		&record,
		state.JSONOptions{Kind: "worktree"},
	)
	if err == nil {
		t.Fatalf("ReadJSON error = nil, expected %s", state.CodeNotFound)
	}
	var stateErr *state.Error
	if !errors.As(err, &stateErr) {
		t.Fatalf("error type = %T, expected *state.Error", err)
	}
	if diff := cmp.Diff(state.CodeNotFound, stateErr.Code); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error does not preserve os.ErrNotExist: %v", err)
	}
	if stateErr.Err == nil {
		t.Fatal("state error cause = nil, expected filesystem cause")
	}
	if !strings.Contains(err.Error(), stateErr.Err.Error()) {
		t.Errorf("error %q omits underlying cause", err)
	}
}

func TestReadJSONPreservesResourceNameWhitespace(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	name := " state.json "
	writeRecordFixture(
		t,
		filepath.Join(root.Path(), name),
		`{"schema_version":1,"kind":"turn","name":"review"}`,
	)

	var actual exampleRecord
	if err := root.ReadJSON(
		name,
		&actual,
		state.JSONOptions{Kind: "turn"},
	); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if diff := cmp.Diff("review", actual.Name); diff != "" {
		t.Errorf("mismatch record name (-expected, +actual):\n%s", diff)
	}
}

func TestReadJSONRemainsAttachedAfterRootPathReplacement(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "state")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create state root: %v", err)
	}
	writeRecordFixture(
		t,
		filepath.Join(path, "runtime.json"),
		`{"schema_version":1,"kind":"runtime","name":"opened-root"}`,
	)
	root := openStateRoot(t, path)

	moved := filepath.Join(parent, "state-moved")
	if err := os.Rename(path, moved); err != nil {
		t.Fatalf("move opened state root: %v", err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	writeRecordFixture(
		t,
		filepath.Join(outside, "runtime.json"),
		`{"schema_version":1,"kind":"runtime","name":"replacement-path"}`,
	)
	if err := os.Symlink("outside", path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	var actual exampleRecord
	if err := root.ReadJSON(
		"runtime.json",
		&actual,
		state.JSONOptions{Kind: "runtime"},
	); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if diff := cmp.Diff("opened-root", actual.Name); diff != "" {
		t.Errorf("mismatch record source (-expected, +actual):\n%s", diff)
	}
}

func TestReadJSONDuringAtomicReplacementNeverReturnsPartialRecord(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	name := "state.json"
	path := filepath.Join(root.Path(), name)
	writeRecordFixture(
		t,
		path,
		`{"schema_version":1,"kind":"turn","name":"value-a"}`,
	)

	const iterations = 500
	start := make(chan struct{})
	failures := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		for iteration := range iterations {
			value := "value-a"
			if iteration%2 == 1 {
				value = "value-b"
			}
			temporary := filepath.Join(root.Path(), "replacement.tmp")
			body := fmt.Sprintf(
				`{"schema_version":1,"kind":"turn","name":%q}`,
				value,
			)
			if err := os.WriteFile(temporary, []byte(body), 0o600); err != nil {
				failures <- fmt.Errorf("write replacement: %w", err)
				return
			}
			if err := os.Rename(temporary, path); err != nil {
				failures <- fmt.Errorf("publish replacement: %w", err)
				return
			}
		}
	}()
	go func() {
		defer wait.Done()
		<-start
		for range iterations * 2 {
			var actual exampleRecord
			err := root.ReadJSON(
				name,
				&actual,
				state.JSONOptions{Kind: "turn"},
			)
			if err != nil {
				if state.IsCode(err, state.CodeIO) {
					continue
				}
				failures <- fmt.Errorf("read replacement: %w", err)
				return
			}
			if actual.Name != "value-a" && actual.Name != "value-b" {
				failures <- fmt.Errorf(
					"read replacement name %q, expected a complete published value",
					actual.Name,
				)
				return
			}
		}
	}()

	close(start)
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}

func writeRecordFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write record fixture: %v", err)
	}
}
