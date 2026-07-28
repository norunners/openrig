package state

import "encoding/json"

// WriteJSON publishes a versioned JSON object atomically within an existing directory.
func (r *Root) WriteJSON(name string, value any, opts JSONOptions) error {
	name, path, err := r.resourceName(name)
	if err != nil {
		return err
	}
	if err := requireDurableMutations(path); err != nil {
		return err
	}
	opts = opts.withWriteDefaults()
	if err := validateJSONOptions(opts, path); err != nil {
		return err
	}

	data, err := versionedJSON(path, value, opts)
	if err != nil {
		return err
	}
	return r.writeFileAtomic(
		name,
		path,
		data,
		FileOptions{FileMode: opts.FileMode},
	)
}

func versionedJSON(path string, value any, opts JSONOptions) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, stateError(CodeMalformed, path, "marshal record", err)
	}
	if int64(len(data)) > maxJSONRecordBytes {
		return nil, recordSizeError(path)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, stateError(CodeMalformed, path, "record must be a JSON object", err)
	}
	if object == nil {
		return nil, stateError(
			CodeMalformed,
			path,
			"record must be a JSON object",
			nil,
		)
	}

	version, _ := json.Marshal(opts.SchemaVersion)
	kind, _ := json.Marshal(opts.Kind)
	object["schema_version"] = version
	object["kind"] = kind

	record, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return nil, stateError(CodeMalformed, path, "marshal versioned record", err)
	}
	record = append(record, '\n')
	if int64(len(record)) > maxJSONRecordBytes {
		return nil, recordSizeError(path)
	}
	return record, nil
}

func recordSizeError(path string) error {
	return stateError(
		CodeMalformed,
		path,
		"state record exceeds maximum size",
		nil,
	)
}
