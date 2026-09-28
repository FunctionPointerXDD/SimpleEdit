//go:build !windows

package detect

// addDLLDir는 Windows 밖에서는 할 일이 없다.
func addDLLDir(string) {}
