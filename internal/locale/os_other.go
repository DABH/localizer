//go:build !darwin && !windows

package locale

// platformLanguages has no OS-wide setting to consult beyond the POSIX environment on this platform.
func platformLanguages() []string { return nil }
