//go:build !windows

package hardlink

import (
	"fmt"
	"os"
	"syscall"
)

// HasHardlinks checks if a file has multiple hardlinks (Nlink > 1)
func HasHardlinks(path string) (bool, error) {
	count, err := GetHardlinkCount(path)
	if err != nil {
		return false, err
	}
	return count > 1, nil
}

// GetHardlinkCount returns the number of hardlinks for a file
func GetHardlinkCount(path string) (uint32, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("cannot convert to syscall.Stat_t for %s", path)
	}

	return uint32(stat.Nlink), nil
}

// AreHardlinked checks if two files are hardlinks to the same data
func AreHardlinked(file1, file2 string) (bool, error) {
	fi1, err := os.Lstat(file1)
	if err != nil {
		return false, fmt.Errorf("failed to stat file %s: %w", file1, err)
	}

	fi2, err := os.Lstat(file2)
	if err != nil {
		return false, fmt.Errorf("failed to stat file %s: %w", file2, err)
	}

	stat1, ok1 := fi1.Sys().(*syscall.Stat_t)
	stat2, ok2 := fi2.Sys().(*syscall.Stat_t)

	if !ok1 || !ok2 {
		return false, fmt.Errorf("cannot convert to syscall.Stat_t")
	}

	// Same device and inode means they're hardlinked
	return stat1.Dev == stat2.Dev && stat1.Ino == stat2.Ino, nil
}