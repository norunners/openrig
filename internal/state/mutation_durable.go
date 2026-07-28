//go:build darwin || linux

package state

import "os"

func requireDurableMutations(string) error {
	return nil
}

func syncDirectory(root *os.Root, name string) error {
	dir, err := root.Open(name)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
