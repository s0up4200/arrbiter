//go:build windows

package hardlink

import "fmt"

// HasHardlinks checks if a file has multiple hardlinks
// Windows implementation - returns error as not supported
func HasHardlinks(path string) (bool, error) {
	return false, fmt.Errorf("hardlink detection not supported on Windows")
}

// GetHardlinkCount returns the number of hardlinks for a file
// Windows implementation - returns error as not supported
func GetHardlinkCount(path string) (uint32, error) {
	return 0, fmt.Errorf("hardlink detection not supported on Windows")
}

// AreHardlinked checks if two files are hardlinks to the same data
// Windows implementation - returns error as not supported
func AreHardlinked(file1, file2 string) (bool, error) {
	return false, fmt.Errorf("hardlink detection not supported on Windows")
}
