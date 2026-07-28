package state

import (
	"os"
	"reflect"
	"strings"
)

const (
	// SchemaVersion is the initial durable record schema version.
	SchemaVersion = 1

	// maxJSONRecordBytes bounds durable JSON records independently of callers.
	maxJSONRecordBytes int64 = 16 << 20
)

// RecordHeader is stored at the top level of every versioned JSON record.
type RecordHeader struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
}

// JSONOptions identifies the versioned JSON record expected by ReadJSON.
type JSONOptions struct {
	Kind          string
	SchemaVersion int
	FileMode      os.FileMode
}

func (opts JSONOptions) withReadDefaults() JSONOptions {
	opts.Kind = strings.TrimSpace(opts.Kind)
	if opts.SchemaVersion == 0 {
		opts.SchemaVersion = SchemaVersion
	}
	return opts
}

func (opts JSONOptions) withWriteDefaults() JSONOptions {
	opts = opts.withReadDefaults()
	if opts.FileMode == 0 {
		opts.FileMode = defaultFileMode
	}
	return opts
}

func validateJSONOptions(opts JSONOptions, path string) error {
	if path == "" {
		return stateError(CodeInvalid, path, "path is required", nil)
	}
	if strings.TrimSpace(opts.Kind) == "" {
		return stateError(CodeInvalid, path, "kind is required", nil)
	}
	if opts.SchemaVersion < 1 {
		return stateError(
			CodeInvalid,
			path,
			"schema_version must be positive",
			nil,
		)
	}
	return nil
}

func validateReadTarget(path string, target any) error {
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return stateError(
			CodeInvalid,
			path,
			"record target must be a non-nil pointer",
			nil,
		)
	}
	return nil
}
