package state

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// GCItem describes state selected by a concrete domain retention policy.
type GCItem struct {
	Kind  string `json:"kind"`
	ID    string `json:"id,omitempty"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// GCReport records the effect of a concrete domain garbage-collection pass.
type GCReport struct {
	DryRun         bool     `json:"dry_run"`
	BeforeBytes    int64    `json:"before_bytes"`
	AfterBytes     int64    `json:"after_bytes"`
	ReclaimedBytes int64    `json:"reclaimed_bytes"`
	Items          []GCItem `json:"items,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

func (r *GCReport) Add(item GCItem) {
	r.Items = append(r.Items, item)
	r.ReclaimedBytes += item.Bytes
}

func (r *GCReport) Normalize() {
	sort.Slice(r.Items, func(i, j int) bool {
		if r.Items[i].Kind == r.Items[j].Kind {
			return r.Items[i].Path < r.Items[j].Path
		}
		return r.Items[i].Kind < r.Items[j].Kind
	})
	sort.Strings(r.Warnings)
}

func pathSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return total, err
}
