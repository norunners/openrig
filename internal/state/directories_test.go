//go:build darwin || linux

package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCreateDirectoriesDurablyLinksEachCreatedDirectory(t *testing.T) {
	parentPath := t.TempDir()
	parent := openInternalRoot(t, parentPath)
	events := make([]string, 0, 6)
	syncDir := func(root *os.Root, name string) error {
		path := filepath.Clean(filepath.Join(root.Name(), name))
		relative, err := filepath.Rel(parentPath, path)
		if err != nil {
			t.Fatalf("resolve relative sync path: %v", err)
		}
		events = append(events, relative)
		return syncDirectory(root, name)
	}

	created, err := createDirectories(
		parent.dir,
		[]string{"state", "worktrees", "wt_01"},
		0o700,
		syncDir,
	)
	if err != nil {
		t.Fatalf("createDirectories returned error: %v", err)
	}
	if err := created.Close(); err != nil {
		t.Fatalf("close created directory: %v", err)
	}

	expected := []string{
		"state",
		".",
		filepath.Join("state", "worktrees"),
		"state",
		filepath.Join("state", "worktrees", "wt_01"),
		filepath.Join("state", "worktrees"),
	}
	if diff := cmp.Diff(expected, events); diff != "" {
		t.Errorf(
			"mismatch directory sync order (-expected, +actual):\n%s",
			diff,
		)
	}
}

func TestOpenCreatesMissingRootAndWriteCreatesNestedDirectories(t *testing.T) {
	parent := t.TempDir()
	rootPath := filepath.Join(parent, " state ")
	root := openInternalRoot(t, rootPath)

	name := filepath.Join("worktrees", "wt_01", "state.json")
	if err := root.WriteFile(name, []byte("state"), FileOptions{}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	for _, dir := range []string{
		rootPath,
		filepath.Join(rootPath, "worktrees"),
		filepath.Join(rootPath, "worktrees", "wt_01"),
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
	actual, err := os.ReadFile(filepath.Join(rootPath, name))
	if err != nil {
		t.Fatalf("read published state file: %v", err)
	}
	if diff := cmp.Diff("state", string(actual)); diff != "" {
		t.Errorf("mismatch published state (-expected, +actual):\n%s", diff)
	}
	if _, err := os.Stat(filepath.Join(parent, "state")); !os.IsNotExist(err) {
		t.Errorf("trimmed state root stat error = %v, expected not exist", err)
	}
}

func TestWriteFileDoesNotChmodExistingParentDirectory(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	dir := filepath.Join(root.Path(), "existing")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("create existing dir: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod existing dir: %v", err)
	}

	if err := root.WriteFile(
		filepath.Join("existing", "state.json"),
		[]byte("state"),
		FileOptions{},
	); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat existing dir: %v", err)
	}
	if diff := cmp.Diff(os.FileMode(0o755), info.Mode().Perm()); diff != "" {
		t.Errorf("mismatch directory mode (-expected, +actual):\n%s", diff)
	}
}

func TestWriteFileRejectsNonDirectoryAncestor(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	if err := os.WriteFile(
		filepath.Join(root.Path(), "blocker"),
		[]byte("not a directory"),
		0o600,
	); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	err := root.WriteFile(
		filepath.Join("blocker", "state.json"),
		[]byte("state"),
		FileOptions{},
	)
	if diff := cmp.Diff(CodeInvalid, CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
}

func TestWriteFileAllowsContainedIntermediateDirectorySymlink(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	physical := filepath.Join(root.Path(), "physical")
	if err := os.Mkdir(physical, 0o700); err != nil {
		t.Fatalf("create physical directory: %v", err)
	}
	if err := os.Symlink("physical", filepath.Join(root.Path(), "alias")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	name := filepath.Join("alias", "worktrees", "state.json")
	if err := root.WriteFile(name, []byte("state"), FileOptions{}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	actual, err := os.ReadFile(
		filepath.Join(physical, "worktrees", "state.json"),
	)
	if err != nil {
		t.Fatalf("read state beneath contained symlink: %v", err)
	}
	if diff := cmp.Diff("state", string(actual)); diff != "" {
		t.Errorf("mismatch state content (-expected, +actual):\n%s", diff)
	}
}

func TestWriteJSONConcurrentFirstPublicationCreatesNestedDirectory(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	name := filepath.Join("worktrees", "wt_01", "state.json")
	opts := JSONOptions{Kind: "worktree"}

	const workers = 8
	start := make(chan struct{})
	failures := make(chan error, workers)
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			record := struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			}{
				Name:  fmt.Sprintf("writer-%d", worker),
				Count: worker,
			}
			if err := root.WriteJSON(name, record, opts); err != nil {
				failures <- err
			}
		}()
	}
	close(start)
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Errorf("concurrent first publication: %v", err)
	}

	var record map[string]any
	if err := root.ReadJSON(name, &record, opts); err != nil {
		t.Fatalf("read final record: %v", err)
	}
}

func TestWriteFileClassifiesDirectorySyncFailureBeforePublicationAsIO(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	syncFailure := errors.New("sync failed")
	name := filepath.Join("worktrees", "wt_01", "state.json")

	err := root.writeFileAtomicWithSync(
		name,
		filepath.Join(root.Path(), name),
		[]byte("state"),
		FileOptions{},
		func(*os.Root, string) error {
			return syncFailure
		},
	)
	if diff := cmp.Diff(CodeIO, CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	if !errors.Is(err, syncFailure) {
		t.Errorf("error does not preserve sync failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root.Path(), name)); !os.IsNotExist(err) {
		t.Errorf("final record stat error = %v, expected not exist", err)
	}
}

func TestOpenClassifiesDirectorySyncFailureBeforeReturningRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	syncFailure := errors.New("sync failed")

	root, err := openStateRoot(
		path,
		defaultDirMode,
		func(*os.Root, string) error {
			return syncFailure
		},
	)
	if root != nil {
		_ = root.Close()
		t.Error("openStateRoot result is non-nil after failed creation")
	}
	if diff := cmp.Diff(CodeIO, CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	if !errors.Is(err, syncFailure) {
		t.Errorf("error does not preserve sync failure: %v", err)
	}
}

func TestOpenConcurrentlyCreatesOneStateRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	const workers = 8
	start := make(chan struct{})
	failures := make(chan error, workers)
	var wait sync.WaitGroup

	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			root, err := Open(path)
			if err != nil {
				failures <- err
				return
			}
			if err := root.Close(); err != nil {
				failures <- err
			}
		}()
	}
	close(start)
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Errorf("concurrent Open: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat state root: %v", err)
	}
	if !info.IsDir() {
		t.Error("created state root is not a directory")
	}
}
