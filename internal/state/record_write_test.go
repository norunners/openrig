//go:build darwin || linux

package state_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/state"
)

type storedExampleRecord struct {
	state.RecordHeader
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestWriteJSONPublishesVersionedRecord(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	name := "state.json"
	expected := exampleRecord{Name: "worktree", Count: 2}

	if err := root.WriteJSON(
		name,
		expected,
		state.JSONOptions{Kind: "worktree"},
	); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root.Path(), name))
	if err != nil {
		t.Fatalf("read written record: %v", err)
	}
	var actual storedExampleRecord
	if err := json.Unmarshal(data, &actual); err != nil {
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
	if diff := cmp.Diff(expectedStored, actual); diff != "" {
		t.Errorf("mismatch stored record (-expected, +actual):\n%s", diff)
	}
}

func TestWriteJSONReplacesCallerHeader(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	name := "state.json"
	value := map[string]any{
		"schema_version": 99,
		"kind":           "caller-kind",
		"name":           "worktree",
	}

	if err := root.WriteJSON(
		name,
		value,
		state.JSONOptions{
			Kind:          "worktree",
			SchemaVersion: state.SchemaVersion,
		},
	); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	var actual storedExampleRecord
	data, err := os.ReadFile(filepath.Join(root.Path(), name))
	if err != nil {
		t.Fatalf("read written record: %v", err)
	}
	if err := json.Unmarshal(data, &actual); err != nil {
		t.Fatalf("unmarshal written record: %v", err)
	}
	expected := state.RecordHeader{
		SchemaVersion: state.SchemaVersion,
		Kind:          "worktree",
	}
	if diff := cmp.Diff(expected, actual.RecordHeader); diff != "" {
		t.Errorf("mismatch stored header (-expected, +actual):\n%s", diff)
	}
}

func TestWriteJSONNormalizesRecordKind(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	name := "state.json"
	if err := root.WriteJSON(
		name,
		exampleRecord{Name: "worktree"},
		state.JSONOptions{Kind: " worktree "},
	); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	var actual storedExampleRecord
	data, err := os.ReadFile(filepath.Join(root.Path(), name))
	if err != nil {
		t.Fatalf("read written record: %v", err)
	}
	if err := json.Unmarshal(data, &actual); err != nil {
		t.Fatalf("unmarshal written record: %v", err)
	}
	if diff := cmp.Diff("worktree", actual.Kind); diff != "" {
		t.Errorf("mismatch stored kind (-expected, +actual):\n%s", diff)
	}
}

func TestWriteJSONRejectsNonObjectWithoutReplacingExistingFile(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	name := "state.json"
	path := filepath.Join(root.Path(), name)
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	err := root.WriteJSON(
		name,
		[]string{"not", "an", "object"},
		state.JSONOptions{Kind: "worktree"},
	)
	if diff := cmp.Diff(state.CodeMalformed, state.CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read existing file: %v", readErr)
	}
	if diff := cmp.Diff("old\n", string(data)); diff != "" {
		t.Errorf("mismatch existing content (-expected, +actual):\n%s", diff)
	}
}

func TestWriteJSONRejectsOversizedRecordWithoutReplacingExistingFile(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	name := "state.json"
	path := filepath.Join(root.Path(), name)
	expected := []byte("old\n")
	if err := os.WriteFile(path, expected, 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	const maximumRecordBytes = 16 << 20
	value := exampleRecord{
		Name: strings.Repeat("x", maximumRecordBytes-24),
	}
	rawValue, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal input fixture: %v", err)
	}
	if len(rawValue) > maximumRecordBytes {
		t.Fatalf(
			"input fixture size = %d, expected at most %d",
			len(rawValue),
			maximumRecordBytes,
		)
	}

	err = root.WriteJSON(
		name,
		value,
		state.JSONOptions{Kind: "worktree"},
	)
	if diff := cmp.Diff(state.CodeMalformed, state.CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	actual, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read existing file: %v", readErr)
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch existing content (-expected, +actual):\n%s", diff)
	}
	entries, readDirErr := os.ReadDir(root.Path())
	if readDirErr != nil {
		t.Fatalf("read state root: %v", readDirErr)
	}
	expectedEntries := []string{name}
	actualEntries := make([]string, 0, len(entries))
	for _, entry := range entries {
		actualEntries = append(actualEntries, entry.Name())
	}
	if diff := cmp.Diff(expectedEntries, actualEntries); diff != "" {
		t.Errorf("mismatch state entries (-expected, +actual):\n%s", diff)
	}
}

func TestWriteJSONValidatesOptions(t *testing.T) {
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
			err := root.WriteJSON("state.json", exampleRecord{}, test.options)
			if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestWriteJSONConcurrentPublicationNeverExposesMalformedRecord(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	name := "state.json"
	opts := state.JSONOptions{Kind: "worktree"}
	if err := root.WriteJSON(
		name,
		exampleRecord{Name: "initial"},
		opts,
	); err != nil {
		t.Fatalf("write initial record: %v", err)
	}

	const workers = 4
	const iterations = 100
	start := make(chan struct{})
	failures := make(chan error, workers*2)
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			for iteration := range iterations {
				record := exampleRecord{
					Name:  fmt.Sprintf("writer-%d", worker),
					Count: iteration,
				}
				if err := root.WriteJSON(name, record, opts); err != nil {
					failures <- fmt.Errorf("write record: %w", err)
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
				if err := root.ReadJSON(name, &record, opts); err != nil {
					failures <- fmt.Errorf("read record: %w", err)
					return
				}
			}
		}()
	}
	close(start)
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}
