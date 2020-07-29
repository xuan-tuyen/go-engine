// +build !windows

package common

// volumeNameLen returns length of the leading volume name on Windows.
// It returns 0 elsewhere.
func volumeNameLen(path string) int {
	return 0
}

func evalSymlinks(path string) (string, error) {
	return walkSymlinks(path)
}
