package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

const (
	// SchemaVersion is the initial durable record schema version.
	SchemaVersion   = 1
	defaultFileMode = 0o600
	defaultDirMode  = 0o700
)

// RecordHeader is stored at the top level of every versioned JSON record.
type RecordHeader struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
}

// JSONOptions controls versioned JSON record reads and atomic writes.
type JSONOptions struct {
	Kind          string
	SchemaVersion int
	FileMode      os.FileMode
	DirMode       os.FileMode
}

// FileOptions controls atomic file writes.
type FileOptions struct {
	FileMode os.FileMode
	DirMode  os.FileMode
}

// ErrorCode classifies durable-state failures for recovery and diagnostics.
type ErrorCode string

const (
	CodeInvalid            ErrorCode = "INVALID"
	CodeNotFound           ErrorCode = "NOT_FOUND"
	CodeMalformed          ErrorCode = "MALFORMED"
	CodeUnsupportedVersion ErrorCode = "UNSUPPORTED_VERSION"
	CodeKindMismatch       ErrorCode = "KIND_MISMATCH"
	CodeIO                 ErrorCode = "IO"
	// CodeUnsupportedPlatform means the local durable-state backend is not
	// available on the current operating system.
	CodeUnsupportedPlatform ErrorCode = "UNSUPPORTED_PLATFORM"
	// CodeDurability means a mutation was published, but its directory entry
	// could not be synced. Callers must treat the resulting state as committed.
	CodeDurability ErrorCode = "DURABILITY_UNCERTAIN"
)

// Error is returned by durable-state primitives with a stable category.
type Error struct {
	Code    ErrorCode
	Path    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{string(e.Code)}
	if e.Path != "" {
		parts = append(parts, e.Path)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.Err != nil && e.Err.Error() != e.Message {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, ": ")
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// CodeOf returns the durable-state error category, defaulting to CodeIO for foreign errors.
func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var stateErr *Error
	if errors.As(err, &stateErr) {
		return stateErr.Code
	}
	return CodeIO
}

// IsCode reports whether err is a durable-state error with the requested category.
func IsCode(err error, code ErrorCode) bool {
	return CodeOf(err) == code
}

func readJSON(path string, target any, opts JSONOptions) error {
	opts = opts.withDefaults()
	if err := validateJSONOptions(opts, path); err != nil {
		return err
	}
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return stateError(
			CodeInvalid,
			path,
			"record target must be a non-nil pointer",
			nil,
		)
	}

	data, err := readFileNoFollow(path)
	if err != nil {
		if os.IsNotExist(err) {
			return stateError(CodeNotFound, path, "record not found", err)
		}
		return stateError(CodeIO, path, "read record", err)
	}

	var header RecordHeader
	if err := json.Unmarshal(data, &header); err != nil {
		return stateError(CodeMalformed, path, "parse record header", err)
	}
	if header.SchemaVersion < 1 ||
		strings.TrimSpace(header.Kind) == "" ||
		header.Kind != strings.TrimSpace(header.Kind) {
		return stateError(CodeMalformed, path, "record header is invalid", nil)
	}
	if header.SchemaVersion != opts.SchemaVersion {
		return stateError(CodeUnsupportedVersion, path, fmt.Sprintf("unsupported schema_version %d", header.SchemaVersion), nil)
	}
	if header.Kind != opts.Kind {
		return stateError(CodeKindMismatch, path, fmt.Sprintf("record kind %q does not match expected %q", header.Kind, opts.Kind), nil)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return stateError(CodeMalformed, path, "parse record", err)
	}
	return nil
}

func writeJSONAtomic(path string, value any, opts JSONOptions) error {
	opts = opts.withDefaults()
	if err := validateJSONOptions(opts, path); err != nil {
		return err
	}

	data, err := json.Marshal(value)
	if err != nil {
		return stateError(CodeMalformed, path, "marshal record", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return stateError(CodeMalformed, path, "record must be a JSON object", err)
	}
	if object == nil {
		return stateError(CodeMalformed, path, "record must be a JSON object", nil)
	}

	version, _ := json.Marshal(opts.SchemaVersion)
	kind, _ := json.Marshal(opts.Kind)
	object["schema_version"] = version
	object["kind"] = kind

	record, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return stateError(CodeMalformed, path, "marshal versioned record", err)
	}
	return writeFileAtomic(path, append(record, '\n'), FileOptions{
		FileMode: opts.FileMode,
		DirMode:  opts.DirMode,
	})
}

func writeFileAtomic(path string, data []byte, opts FileOptions) error {
	return writeFileAtomicWithDirectoryOps(path, data, opts, osDirectoryOps())
}

func writeFileAtomicWithDirectoryOps(
	path string,
	data []byte,
	opts FileOptions,
	dirOps directoryOps,
) error {
	opts = opts.withDefaults()
	if strings.TrimSpace(path) == "" {
		return stateError(CodeInvalid, path, "path is required", nil)
	}

	dir := filepath.Dir(path)
	if err := ensureParentDir(dir, opts.DirMode, dirOps); err != nil {
		return stateError(CodeIO, path, "create parent directory", err)
	}

	tmp, err := os.CreateTemp(dir, ".openrig-state-*")
	if err != nil {
		return stateError(CodeIO, path, "create temporary file", err)
	}
	tmpName := tmp.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(opts.FileMode); err != nil {
		_ = tmp.Close()
		return stateError(CodeIO, path, "chmod temporary file", err)
	}
	written, err := tmp.Write(data)
	if err != nil {
		_ = tmp.Close()
		return stateError(CodeIO, path, "write temporary file", err)
	}
	if written != len(data) {
		_ = tmp.Close()
		return stateError(CodeIO, path, "write temporary file", io.ErrShortWrite)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return stateError(CodeIO, path, "sync temporary file", err)
	}
	if err := tmp.Close(); err != nil {
		return stateError(CodeIO, path, "close temporary file", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return stateError(CodeIO, path, "replace file", err)
	}
	published = true
	if err := dirOps.syncDir(dir); err != nil {
		return stateError(CodeDurability, path, "file published but parent directory sync failed", err)
	}
	return nil
}

func removeFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return stateError(CodeInvalid, path, "path is required", nil)
	}
	if err := removeFileEntry(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return stateError(CodeIO, path, "remove file", err)
	}
	if err := syncDir(filepath.Dir(path)); err != nil {
		return stateError(CodeDurability, path, "file removed but parent directory sync failed", err)
	}
	return nil
}

func (opts JSONOptions) withDefaults() JSONOptions {
	opts.Kind = strings.TrimSpace(opts.Kind)
	if opts.SchemaVersion == 0 {
		opts.SchemaVersion = SchemaVersion
	}
	if opts.FileMode == 0 {
		opts.FileMode = defaultFileMode
	}
	if opts.DirMode == 0 {
		opts.DirMode = defaultDirMode
	}
	return opts
}

func (opts FileOptions) withDefaults() FileOptions {
	if opts.FileMode == 0 {
		opts.FileMode = defaultFileMode
	}
	if opts.DirMode == 0 {
		opts.DirMode = defaultDirMode
	}
	return opts
}

func validateJSONOptions(opts JSONOptions, path string) error {
	if strings.TrimSpace(path) == "" {
		return stateError(CodeInvalid, path, "path is required", nil)
	}
	if strings.TrimSpace(opts.Kind) == "" {
		return stateError(CodeInvalid, path, "kind is required", nil)
	}
	if opts.SchemaVersion < 1 {
		return stateError(CodeInvalid, path, "schema_version must be positive", nil)
	}
	return nil
}

type directoryOps struct {
	stat    func(string) (os.FileInfo, error)
	mkdir   func(string, os.FileMode) error
	chmod   func(string, os.FileMode) error
	syncDir func(string) error
}

func osDirectoryOps() directoryOps {
	return directoryOps{
		stat:    os.Stat,
		mkdir:   os.Mkdir,
		chmod:   os.Chmod,
		syncDir: syncDir,
	}
}

func ensureParentDir(path string, mode os.FileMode, ops directoryOps) error {
	path = filepath.Clean(path)
	missing := make([]string, 0, 4)
	current := path
	for {
		info, err := ops.stat(current)
		if err == nil {
			if !info.IsDir() {
				return &os.PathError{
					Op:   "mkdir",
					Path: current,
					Err:  errors.New("path is not a directory"),
				}
			}
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		missing = append(missing, current)
		parent := filepath.Dir(current)
		if parent == current {
			return err
		}
		current = parent
	}
	slices.Reverse(missing)

	for _, dir := range missing {
		created := false
		if err := ops.mkdir(dir, mode); err != nil {
			if !os.IsExist(err) {
				return err
			}
			info, statErr := ops.stat(dir)
			if statErr != nil {
				return statErr
			}
			if !info.IsDir() {
				return &os.PathError{
					Op:   "mkdir",
					Path: dir,
					Err:  errors.New("path is not a directory"),
				}
			}
		} else {
			created = true
		}
		if created {
			if err := ops.chmod(dir, mode); err != nil {
				return err
			}
		}
		if err := ops.syncDir(dir); err != nil {
			return err
		}
		if err := ops.syncDir(filepath.Dir(dir)); err != nil {
			return err
		}
	}
	return nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func stateError(code ErrorCode, path, message string, err error) *Error {
	return &Error{Code: code, Path: path, Message: message, Err: err}
}
