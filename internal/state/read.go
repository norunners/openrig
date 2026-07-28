package state

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// ReadJSON reads a versioned JSON record through the opened state root.
// Callers must ignore target when an error is returned because JSON decoding
// may have populated it partially before detecting a malformed body.
func (r *Root) ReadJSON(name string, target any, opts JSONOptions) error {
	name, path, err := r.resourceName(name)
	if err != nil {
		return err
	}
	opts = opts.withReadDefaults()
	if err := validateJSONOptions(opts, path); err != nil {
		return err
	}
	if err := validateReadTarget(path, target); err != nil {
		return err
	}

	data, err := r.readRegularFile(name, path)
	if err != nil {
		return err
	}
	if !utf8.Valid(data) {
		return stateError(
			CodeMalformed,
			path,
			"record is not valid UTF-8",
			nil,
		)
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
		return stateError(
			CodeUnsupportedVersion,
			path,
			fmt.Sprintf(
				"unsupported schema_version %d",
				header.SchemaVersion,
			),
			nil,
		)
	}
	if header.Kind != opts.Kind {
		return stateError(
			CodeKindMismatch,
			path,
			fmt.Sprintf(
				"record kind %q does not match expected %q",
				header.Kind,
				opts.Kind,
			),
			nil,
		)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return stateError(CodeMalformed, path, "parse record", err)
	}
	return nil
}

func (r *Root) readRegularFile(name, path string) ([]byte, error) {
	before, err := r.dir.Lstat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, stateError(CodeNotFound, path, "record not found", err)
		}
		return nil, stateError(CodeIO, path, "inspect record", err)
	}
	if !before.Mode().IsRegular() {
		return nil, stateError(
			CodeInvalid,
			path,
			"state record must be a regular file, not an alias or special entry",
			nil,
		)
	}

	file, err := openReadCandidate(r.dir, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, stateError(CodeNotFound, path, "record not found", err)
		}
		return nil, stateError(CodeIO, path, "open record", err)
	}
	defer file.Close()

	after, err := file.Stat()
	if err != nil {
		return nil, stateError(CodeIO, path, "inspect opened record", err)
	}
	if !after.Mode().IsRegular() {
		return nil, stateError(
			CodeInvalid,
			path,
			"opened state record must be a regular file",
			nil,
		)
	}
	if !readCandidateMatches(before, after) {
		return nil, stateError(
			CodeIO,
			path,
			"state record changed while it was opened",
			nil,
		)
	}

	data, err := readRecordData(file, after.Size(), path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readRecordData(reader io.Reader, size int64, path string) ([]byte, error) {
	if size > maxJSONRecordBytes {
		return nil, stateError(
			CodeMalformed,
			path,
			"state record exceeds maximum size",
			nil,
		)
	}

	data, err := io.ReadAll(io.LimitReader(reader, maxJSONRecordBytes+1))
	if err != nil {
		return nil, stateError(CodeIO, path, "read record", err)
	}
	if int64(len(data)) > maxJSONRecordBytes {
		return nil, stateError(
			CodeMalformed,
			path,
			"state record exceeds maximum size",
			nil,
		)
	}
	return data, nil
}
