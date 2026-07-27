package state_test

import (
	"encoding/json"
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

type storedExampleRecord struct {
	state.RecordHeader
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestWriteJSONAtomicWritesVersionedRecord(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "nested", "state.json")
	expected := exampleRecord{Name: "worktree", Count: 2}

	if err := root.WriteJSON(path, expected, state.JSONOptions{Kind: "worktree"}); err != nil {
		t.Fatalf("WriteJSONAtomic returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written record: %v", err)
	}
	var stored storedExampleRecord
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal stored record: %v", err)
	}
	expectedStored := storedExampleRecord{
		RecordHeader: state.RecordHeader{
			SchemaVersion: state.SchemaVersion,
			Kind:          "worktree",
		},
		Name:  "worktree",
		Count: 2,
	}
	if diff := cmp.Diff(expectedStored, stored); diff != "" {
		t.Errorf("mismatch stored record (-expected, +actual):\n%s", diff)
	}
}

func TestReadJSONValidatesRecordHeader(t *testing.T) {
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
			body:       `{"schema_version": 1, "kind": "worktree"`,
			pathSuffix: "malformed.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "missing header",
			body:       `{"name": "worktree"}`,
			pathSuffix: "missing-header.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "negative schema version",
			body:       `{"schema_version": -1, "kind": "worktree", "name": "worktree"}`,
			pathSuffix: "negative-version.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "blank kind",
			body:       `{"schema_version": 1, "kind": "   ", "name": "worktree"}`,
			pathSuffix: "blank-kind.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "kind with surrounding whitespace",
			body:       `{"schema_version": 1, "kind": " worktree ", "name": "worktree"}`,
			pathSuffix: "noncanonical-kind.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "malformed body",
			body:       `{"schema_version": 1, "kind": "worktree", "name": "worktree", "count": "invalid"}`,
			pathSuffix: "malformed-body.json",
			errorCode:  state.CodeMalformed,
		},
		{
			name:       "unsupported version",
			body:       `{"schema_version": 2, "kind": "worktree", "name": "worktree"}`,
			pathSuffix: "unsupported-version.json",
			errorCode:  state.CodeUnsupportedVersion,
		},
		{
			name:       "kind mismatch",
			body:       `{"schema_version": 1, "kind": "turn", "name": "worktree"}`,
			pathSuffix: "kind-mismatch.json",
			errorCode:  state.CodeKindMismatch,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := openStateRoot(t, t.TempDir())
			path := filepath.Join(root.Path(), test.pathSuffix)
			if test.body != "" {
				if err := os.WriteFile(path, []byte(test.body), 0o600); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
			}

			var record exampleRecord
			err := root.ReadJSON(path, &record, state.JSONOptions{Kind: "worktree"})
			if err == nil {
				t.Fatalf("ReadJSON error = nil, expected %s", test.errorCode)
			}
			if !state.IsCode(err, test.errorCode) {
				t.Errorf("error code = %s, expected %s", state.CodeOf(err), test.errorCode)
			}
		})
	}
}

func TestReadJSONReturnsRecordWithoutHeaderFields(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "state.json")
	expected := exampleRecord{Name: "turn", Count: 7}
	if err := root.WriteJSON(path, expected, state.JSONOptions{Kind: "turn"}); err != nil {
		t.Fatalf("WriteJSONAtomic returned error: %v", err)
	}

	var actual exampleRecord
	err := root.ReadJSON(path, &actual, state.JSONOptions{Kind: "turn"})
	if err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch read record (-expected, +actual):\n%s", diff)
	}
}

func TestJSONOperationsRejectInvalidInputs(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "state.json")
	var typedNil *exampleRecord
	tests := []struct {
		name      string
		operation func() error
		code      state.ErrorCode
	}{
		{
			name: "read requires target",
			operation: func() error {
				return root.ReadJSON(
					path,
					nil,
					state.JSONOptions{Kind: "worktree"},
				)
			},
			code: state.CodeInvalid,
		},
		{
			name: "read rejects non-pointer target",
			operation: func() error {
				return root.ReadJSON(
					path,
					exampleRecord{},
					state.JSONOptions{Kind: "worktree"},
				)
			},
			code: state.CodeInvalid,
		},
		{
			name: "read rejects typed nil target",
			operation: func() error {
				return root.ReadJSON(
					path,
					typedNil,
					state.JSONOptions{Kind: "worktree"},
				)
			},
			code: state.CodeInvalid,
		},
		{
			name: "write requires kind",
			operation: func() error {
				return root.WriteJSON(path, exampleRecord{}, state.JSONOptions{})
			},
			code: state.CodeInvalid,
		},
		{
			name: "write requires positive explicit version",
			operation: func() error {
				return root.WriteJSON(
					path,
					exampleRecord{},
					state.JSONOptions{
						Kind:          "worktree",
						SchemaVersion: -1,
					},
				)
			},
			code: state.CodeInvalid,
		},
		{
			name: "write requires object",
			operation: func() error {
				return root.WriteJSON(
					path,
					nil,
					state.JSONOptions{Kind: "worktree"},
				)
			},
			code: state.CodeMalformed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.operation()
			if err == nil {
				t.Fatalf("operation error = nil, expected %s", test.code)
			}
			if diff := cmp.Diff(test.code, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestWriteJSONNormalizesRecordKind(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "state.json")
	if err := root.WriteJSON(
		path,
		exampleRecord{Name: "worktree"},
		state.JSONOptions{Kind: " worktree "},
	); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	var stored storedExampleRecord
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written record: %v", err)
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal written record: %v", err)
	}
	if diff := cmp.Diff("worktree", stored.Kind); diff != "" {
		t.Errorf("mismatch stored kind (-expected, +actual):\n%s", diff)
	}
}

func TestWriteJSONAtomicRejectsNonObjectRecordsWithoutReplacingExistingFile(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "state.json")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	err := root.WriteJSON(path, []string{"not", "an", "object"}, state.JSONOptions{Kind: "worktree"})
	if err == nil {
		t.Fatalf("WriteJSONAtomic error = nil, expected %s", state.CodeMalformed)
	}
	if !state.IsCode(err, state.CodeMalformed) {
		t.Errorf("error code = %s, expected %s", state.CodeOf(err), state.CodeMalformed)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	if diff := cmp.Diff("old\n", string(data)); diff != "" {
		t.Errorf("mismatch existing content (-expected, +actual):\n%s", diff)
	}
}

func TestWriteFileAtomicReplacesFileAndLeavesStaleTempsAlone(t *testing.T) {
	dir := t.TempDir()
	root := openStateRoot(t, dir)
	dir = root.Path()
	path := filepath.Join(dir, "state.json")
	staleTemp := filepath.Join(dir, ".openrig-state-stale")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	if err := os.WriteFile(staleTemp, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale temp: %v", err)
	}

	if err := root.WriteFile(path, []byte("new"), state.FileOptions{}); err != nil {
		t.Fatalf("WriteFileAtomic returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced file: %v", err)
	}
	if diff := cmp.Diff("new", string(data)); diff != "" {
		t.Errorf("mismatch replaced content (-expected, +actual):\n%s", diff)
	}
	stale, err := os.ReadFile(staleTemp)
	if err != nil {
		t.Fatalf("read stale temp: %v", err)
	}
	if diff := cmp.Diff("stale", string(stale)); diff != "" {
		t.Errorf("mismatch stale temp content (-expected, +actual):\n%s", diff)
	}
}

func TestWriteFileAtomicUsesRestrictivePermissions(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "nested", "state.json")

	if err := root.WriteFile(path, []byte("state"), state.FileOptions{}); err != nil {
		t.Fatalf("WriteFileAtomic returned error: %v", err)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if actual, expected := fileInfo.Mode().Perm(), os.FileMode(0o600); actual != expected {
		t.Errorf("file mode = %s, expected %s", actual, expected)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat parent dir: %v", err)
	}
	if actual, expected := dirInfo.Mode().Perm(), os.FileMode(0o700); actual != expected {
		t.Errorf("dir mode = %s, expected %s", actual, expected)
	}
}

func TestWriteFileAtomicCreatesMissingRootAndNestedDirectories(t *testing.T) {
	parent := t.TempDir()
	root := openStateRoot(t, filepath.Join(parent, "state"))
	path := filepath.Join(
		root.Path(),
		"worktrees",
		"wt_01",
		"state.json",
	)

	if err := root.WriteFile(path, []byte("state"), state.FileOptions{}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	for _, dir := range []string{
		root.Path(),
		filepath.Join(root.Path(), "worktrees"),
		filepath.Join(root.Path(), "worktrees", "wt_01"),
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("stat created directory %q: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("created path %q is not a directory", dir)
		}
		if diff := cmp.Diff(os.FileMode(0o700), info.Mode().Perm()); diff != "" {
			t.Errorf(
				"mismatch directory mode for %q (-expected, +actual):\n%s",
				dir,
				diff,
			)
		}
	}
	if data, err := os.ReadFile(path); err != nil {
		t.Errorf("read published state file: %v", err)
	} else if diff := cmp.Diff("state", string(data)); diff != "" {
		t.Errorf("mismatch published state (-expected, +actual):\n%s", diff)
	}
}

func TestWriteFileAtomicDoesNotChmodExistingParentDirectory(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	dir := filepath.Join(root.Path(), "existing")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create existing dir: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod existing dir: %v", err)
	}

	if err := root.WriteFile(filepath.Join(dir, "state.json"), []byte("state"), state.FileOptions{}); err != nil {
		t.Fatalf("WriteFileAtomic returned error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat existing dir: %v", err)
	}
	if actual, expected := info.Mode().Perm(), os.FileMode(0o755); actual != expected {
		t.Errorf("dir mode = %s, expected %s", actual, expected)
	}
}

func TestWriteFileRejectsNonDirectoryAncestor(t *testing.T) {
	root := t.TempDir()
	stateRoot := openStateRoot(t, root)
	blocker := filepath.Join(stateRoot.Path(), "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	err := stateRoot.WriteFile(filepath.Join(blocker, "state.json"), []byte("state"), state.FileOptions{})
	if err == nil {
		t.Fatalf("WriteFile error = nil, expected %s", state.CodeInvalid)
	}
	if !state.IsCode(err, state.CodeInvalid) {
		t.Errorf("error code = %s, expected %s", state.CodeOf(err), state.CodeInvalid)
	}
}

func TestWriteJSONAtomicFailedPublicationLeavesNoFinalRecordOrTemporaryFile(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "state.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create final-path blocker: %v", err)
	}

	err := root.WriteJSON(path, exampleRecord{Name: "blocked"}, state.JSONOptions{Kind: "worktree"})
	if err == nil {
		t.Fatal("WriteJSON error = nil, expected publication failure")
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stat final path: %v", statErr)
	}
	if !info.IsDir() {
		t.Errorf("final path became a file after failed publication")
	}
	entries, readErr := os.ReadDir(root.Path())
	if readErr != nil {
		t.Fatalf("read state root: %v", readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".openrig-state-") {
			t.Errorf("failed publication left temporary file %q", entry.Name())
		}
	}
}

func TestWriteJSONAtomicConcurrentPublicationNeverExposesMalformedRecord(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "state.json")
	opts := state.JSONOptions{Kind: "worktree"}
	if err := root.WriteJSON(path, exampleRecord{Name: "initial"}, opts); err != nil {
		t.Fatalf("write initial record: %v", err)
	}

	const workers = 4
	const iterations = 100
	start := make(chan struct{})
	errors := make(chan error, workers*2)
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			for iteration := range iterations {
				record := exampleRecord{Name: fmt.Sprintf("writer-%d", worker), Count: iteration}
				if err := root.WriteJSON(path, record, opts); err != nil {
					errors <- fmt.Errorf("write record: %w", err)
					return
				}
			}
		}()
	}
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			for range iterations * 2 {
				var record exampleRecord
				if err := root.ReadJSON(path, &record, opts); err != nil {
					errors <- fmt.Errorf("read record: %w", err)
					return
				}
			}
		}()
	}
	close(start)
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestWriteJSONAtomicConcurrentFirstPublicationCreatesNestedDirectory(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "worktrees", "wt_01", "state.json")
	opts := state.JSONOptions{Kind: "worktree"}

	const workers = 8
	start := make(chan struct{})
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			record := exampleRecord{
				Name:  fmt.Sprintf("writer-%d", worker),
				Count: worker,
			}
			if err := root.WriteJSON(path, record, opts); err != nil {
				errors <- err
			}
		}()
	}
	close(start)
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Errorf("concurrent first publication: %v", err)
	}

	var record exampleRecord
	if err := root.ReadJSON(path, &record, opts); err != nil {
		t.Fatalf("read final record: %v", err)
	}
}

func TestRemoveFile(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "state.json")
	if err := root.WriteFile(path, []byte("state"), state.FileOptions{}); err != nil {
		t.Fatalf("WriteFileAtomic returned error: %v", err)
	}

	if err := root.Remove(path); err != nil {
		t.Fatalf("RemoveFile returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("stat removed file error = %v, expected not exist", err)
	}
	if err := root.Remove(path); err != nil {
		t.Errorf("RemoveFile missing file returned error: %v", err)
	}
}

func TestRootFileOperationsRejectRootPath(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	tests := []struct {
		name      string
		operation func() error
	}{
		{
			name: "read",
			operation: func() error {
				var record exampleRecord
				return root.ReadJSON(
					"",
					&record,
					state.JSONOptions{Kind: "worktree"},
				)
			},
		},
		{
			name: "write json",
			operation: func() error {
				return root.WriteJSON(
					root.Path(),
					exampleRecord{},
					state.JSONOptions{Kind: "worktree"},
				)
			},
		},
		{
			name: "write file",
			operation: func() error {
				return root.WriteFile("", []byte("state"), state.FileOptions{})
			},
		},
		{
			name: "remove",
			operation: func() error {
				return root.Remove(root.Path())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.operation()
			if err == nil {
				t.Fatal("operation error = nil, expected error")
			}
			if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
	if _, err := os.Stat(root.Path()); err != nil {
		t.Errorf("state root was modified: %v", err)
	}
}

func TestRootFileOperationsRejectFinalSymlinkAliases(t *testing.T) {
	tests := []struct {
		name      string
		operation func(*state.Root, string) error
	}{
		{
			name: "read json",
			operation: func(root *state.Root, path string) error {
				var record exampleRecord
				return root.ReadJSON(
					path,
					&record,
					state.JSONOptions{Kind: "operation"},
				)
			},
		},
		{
			name: "write json",
			operation: func(root *state.Root, path string) error {
				return root.WriteJSON(
					path,
					exampleRecord{Name: "operation"},
					state.JSONOptions{Kind: "operation"},
				)
			},
		},
		{
			name: "write file",
			operation: func(root *state.Root, path string) error {
				return root.WriteFile(
					path,
					[]byte("operation"),
					state.FileOptions{},
				)
			},
		},
		{
			name: "remove",
			operation: func(root *state.Root, path string) error {
				return root.Remove(path)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := openStateRoot(t, t.TempDir())
			target := filepath.Join(root.Path(), "runtime.json")
			expected := []byte("runtime state\n")
			if err := os.WriteFile(target, expected, 0o600); err != nil {
				t.Fatalf("write target: %v", err)
			}
			alias := filepath.Join(root.Path(), "operation.json")
			if err := os.Symlink("runtime.json", alias); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}

			err := test.operation(root, alias)
			if err == nil {
				t.Fatal("operation error = nil, expected error")
			}
			if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
			actual, readErr := os.ReadFile(target)
			if readErr != nil {
				t.Fatalf("read target after rejected operation: %v", readErr)
			}
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("mismatch target content (-expected, +actual):\n%s", diff)
			}
			info, lstatErr := os.Lstat(alias)
			if lstatErr != nil {
				t.Fatalf("lstat alias after rejected operation: %v", lstatErr)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("alias mode = %s, expected symbolic link", info.Mode())
			}
		})
	}
}

func TestRootFileOperationsRejectDirectories(t *testing.T) {
	tests := []struct {
		name      string
		operation func(*state.Root, string) error
	}{
		{
			name: "read json",
			operation: func(root *state.Root, path string) error {
				var record exampleRecord
				return root.ReadJSON(
					path,
					&record,
					state.JSONOptions{Kind: "worktree"},
				)
			},
		},
		{
			name: "write json",
			operation: func(root *state.Root, path string) error {
				return root.WriteJSON(
					path,
					exampleRecord{},
					state.JSONOptions{Kind: "worktree"},
				)
			},
		},
		{
			name: "write file",
			operation: func(root *state.Root, path string) error {
				return root.WriteFile(path, nil, state.FileOptions{})
			},
		},
		{
			name: "remove",
			operation: func(root *state.Root, path string) error {
				return root.Remove(path)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := openStateRoot(t, t.TempDir())
			path := filepath.Join(root.Path(), "resource")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatalf("create directory fixture: %v", err)
			}

			err := test.operation(root, path)
			if err == nil {
				t.Fatal("operation error = nil, expected error")
			}
			if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
			info, statErr := os.Stat(path)
			if statErr != nil {
				t.Fatalf("stat directory after rejected operation: %v", statErr)
			}
			if !info.IsDir() {
				t.Errorf("fixture is no longer a directory")
			}
		})
	}
}

func TestRootSizeAndGCReport(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	if err := root.WriteFile(
		"second/state.bin",
		[]byte("four"),
		state.FileOptions{},
	); err != nil {
		t.Fatalf("write second state file: %v", err)
	}
	if err := root.WriteFile(
		"first.bin",
		[]byte("one"),
		state.FileOptions{},
	); err != nil {
		t.Fatalf("write first state file: %v", err)
	}

	size, err := root.Size("")
	if err != nil {
		t.Fatalf("Size returned error: %v", err)
	}
	if diff := cmp.Diff(int64(7), size); diff != "" {
		t.Errorf("mismatch state size (-expected, +actual):\n%s", diff)
	}
	missingSize, err := root.Size("missing")
	if err != nil {
		t.Fatalf("Size missing path returned error: %v", err)
	}
	if diff := cmp.Diff(int64(0), missingSize); diff != "" {
		t.Errorf("mismatch missing size (-expected, +actual):\n%s", diff)
	}

	actual := state.GCReport{
		DryRun:      true,
		BeforeBytes: size,
		AfterBytes:  0,
		Warnings:    []string{"z warning", "a warning"},
	}
	actual.Add(state.GCItem{
		Kind:  "turn",
		ID:    "turn_01",
		Path:  "turns/turn_01",
		Bytes: 4,
	})
	actual.Add(state.GCItem{
		Kind:  "revision",
		ID:    "rev_01",
		Path:  "revisions/rev_01",
		Bytes: 3,
	})
	actual.Normalize()
	expected := state.GCReport{
		DryRun:         true,
		BeforeBytes:    7,
		AfterBytes:     0,
		ReclaimedBytes: 7,
		Items: []state.GCItem{
			{
				Kind:  "revision",
				ID:    "rev_01",
				Path:  "revisions/rev_01",
				Bytes: 3,
			},
			{
				Kind:  "turn",
				ID:    "turn_01",
				Path:  "turns/turn_01",
				Bytes: 4,
			},
		},
		Warnings: []string{"a warning", "z warning"},
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch GC report (-expected, +actual):\n%s", diff)
	}
}

func TestOperationID(t *testing.T) {
	operationID := state.NewOperationID()
	if !state.ValidOperationID(operationID) {
		t.Errorf("operation id %q is invalid", operationID)
	}
	if state.ValidOperationID("op_deadbeef") {
		t.Errorf("old operation id accepted")
	}
}

func TestStateErrorPreservesTypeAndCause(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "missing.json")

	var record exampleRecord
	err := root.ReadJSON(path, &record, state.JSONOptions{Kind: "worktree"})
	if err == nil {
		t.Fatalf("ReadJSON error = nil, expected %s", state.CodeNotFound)
	}
	var stateErr *state.Error
	if !errors.As(err, &stateErr) {
		t.Fatalf("error type = %T, expected *state.Error", err)
	}
	expected := state.CodeNotFound
	if diff := cmp.Diff(expected, stateErr.Code); diff != "" {
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

func TestRootRejectsLexicalEscape(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	err := root.WriteFile(filepath.Join(root.Path(), "..", "escaped"), []byte("no"), state.FileOptions{})
	if !state.IsCode(err, state.CodeInvalid) {
		t.Errorf("error code = %s, expected %s", state.CodeOf(err), state.CodeInvalid)
	}
}

func TestRootRejectsSymlinkEscapes(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	outside := t.TempDir()
	link := filepath.Join(root.Path(), "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	tests := []struct {
		name string
		path string
	}{
		{
			name: "existing symlink",
			path: link,
		},
		{
			name: "missing descendant",
			path: filepath.Join(link, "new", "state.json"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := root.WriteFile(test.path, []byte("no"), state.FileOptions{})
			if err == nil {
				t.Fatal("WriteFile error = nil, expected error")
			}
			if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestRootResolvesContainedSymlink(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	physicalDir := filepath.Join(root.Path(), "physical")
	if err := os.Mkdir(physicalDir, 0o700); err != nil {
		t.Fatalf("create physical directory: %v", err)
	}
	link := filepath.Join(root.Path(), "link")
	if err := os.Symlink(physicalDir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	actual, err := root.Resolve(filepath.Join(link, "state.json"))
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	expected := filepath.Join(physicalDir, "state.json")
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch resolved path (-expected, +actual):\n%s", diff)
	}
	if err := root.WriteFile(
		filepath.Join(link, "state.json"),
		[]byte("state"),
		state.FileOptions{},
	); err != nil {
		t.Fatalf("WriteFile through contained parent symlink returned error: %v", err)
	}
	data, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("read state through physical parent: %v", err)
	}
	if diff := cmp.Diff("state", string(data)); diff != "" {
		t.Errorf("mismatch state through contained symlink (-expected, +actual):\n%s", diff)
	}
}

func TestOpenUsesXDGStateHome(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	root, err := state.Open("")
	if state.IsCode(err, state.CodeUnsupportedPlatform) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	physicalStateHome, err := filepath.EvalSymlinks(stateHome)
	if err != nil {
		t.Fatalf("resolve state home symlinks: %v", err)
	}
	expected := filepath.Join(physicalStateHome, "openrig")
	if diff := cmp.Diff(expected, root.Path()); diff != "" {
		t.Errorf("mismatch default state root (-expected, +actual):\n%s", diff)
	}
}

func TestOpenValidatesStateRootType(t *testing.T) {
	parent := t.TempDir()
	physicalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatalf("resolve parent symlinks: %v", err)
	}
	directory := filepath.Join(parent, "state")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("create directory root: %v", err)
	}
	regularFile := filepath.Join(parent, "state-file")
	if err := os.WriteFile(regularFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create file root: %v", err)
	}
	missing := filepath.Join(parent, "missing")

	tests := []struct {
		name          string
		path          string
		expectedCode  state.ErrorCode
		expectedPath  string
		requiresLinks bool
	}{
		{
			name:         "missing path",
			path:         missing,
			expectedPath: filepath.Join(physicalParent, "missing"),
		},
		{
			name:         "existing directory",
			path:         directory,
			expectedPath: filepath.Join(physicalParent, "state"),
		},
		{
			name:         "regular file",
			path:         regularFile,
			expectedCode: state.CodeInvalid,
		},
		{
			name:          "symlink to directory",
			path:          filepath.Join(parent, "directory-link"),
			expectedPath:  filepath.Join(physicalParent, "state"),
			requiresLinks: true,
		},
		{
			name:          "symlink to regular file",
			path:          filepath.Join(parent, "file-link"),
			expectedCode:  state.CodeInvalid,
			requiresLinks: true,
		},
	}
	if err := os.Symlink(directory, filepath.Join(parent, "directory-link")); err != nil {
		for index := range tests {
			if tests[index].requiresLinks {
				tests[index].path = ""
			}
		}
	}
	if err := os.Symlink(regularFile, filepath.Join(parent, "file-link")); err != nil {
		for index := range tests {
			if tests[index].requiresLinks {
				tests[index].path = ""
			}
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.requiresLinks && test.path == "" {
				t.Skip("symlinks unavailable")
			}
			actual, err := state.Open(test.path)
			if state.IsCode(err, state.CodeUnsupportedPlatform) {
				t.Skip(err)
			}
			if test.expectedCode != "" {
				if err == nil {
					t.Fatalf("Open error = nil, expected %s", test.expectedCode)
				}
				if diff := cmp.Diff(test.expectedCode, state.CodeOf(err)); diff != "" {
					t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
				}
				var stateErr *state.Error
				if !errors.As(err, &stateErr) {
					t.Errorf("error type = %T, expected *state.Error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Open returned error: %v", err)
			}
			if diff := cmp.Diff(test.expectedPath, actual.Path()); diff != "" {
				t.Errorf("mismatch root path (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func openStateRoot(t *testing.T, path string) *state.Root {
	t.Helper()
	root, err := state.Open(path)
	if state.IsCode(err, state.CodeUnsupportedPlatform) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	return root
}
