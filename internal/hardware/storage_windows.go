//go:build windows

package hardware

// probeStorage is not implemented on windows in the MVP; values stay unknown.
func probeStorage(path string) Storage {
	return Storage{Path: path}
}
