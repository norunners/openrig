//go:build darwin || linux || windows

package state

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReadRecordDataEnforcesMaximumSize(t *testing.T) {
	tests := []struct {
		name         string
		reportedSize int64
		actualSize   int64
		expectedCode ErrorCode
	}{
		{
			name:         "exact maximum",
			reportedSize: maxJSONRecordBytes,
			actualSize:   maxJSONRecordBytes,
		},
		{
			name:         "reported size one byte over",
			reportedSize: maxJSONRecordBytes + 1,
			actualSize:   maxJSONRecordBytes + 1,
			expectedCode: CodeMalformed,
		},
		{
			name:         "grew one byte after stat",
			reportedSize: maxJSONRecordBytes,
			actualSize:   maxJSONRecordBytes + 1,
			expectedCode: CodeMalformed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := bytes.NewReader(make([]byte, test.actualSize))
			actual, err := readRecordData(
				reader,
				test.reportedSize,
				"state.json",
			)
			if diff := cmp.Diff(test.expectedCode, CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
			if test.expectedCode != "" {
				return
			}
			if diff := cmp.Diff(int(test.actualSize), len(actual)); diff != "" {
				t.Errorf("mismatch read size (-expected, +actual):\n%s", diff)
			}
		})
	}
}
