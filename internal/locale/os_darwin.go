// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package locale

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// maxPlistSize bounds what platformLanguages reads. .GlobalPreferences.plist is a few kilobytes; anything
// near this limit is not the file we want.
const maxPlistSize = 8 << 20

// platformLanguages reads AppleLanguages from the user's global preferences. The file is read directly
// (no cgo, no `defaults` exec); cfprefsd may hold a newer value in memory for a short while after the user
// changes the setting, which is acceptable for picking a UI language.
func platformLanguages() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return plistLanguages(filepath.Join(home, "Library", "Preferences", ".GlobalPreferences.plist"))
}

// plistLanguages returns the AppleLanguages array of the binary plist at path, or nil when anything is off:
// the file is missing or unreadable, is not a regular file (a FIFO or a device could block forever), is
// larger than maxPlistSize, or is malformed. This runs at every CLI startup, so it must neither block nor
// fail; the worst outcome is English.
func plistLanguages(path string) []string {
	// O_NONBLOCK makes opening a FIFO return at once instead of waiting for a writer; it does not affect
	// reads from a regular file. The mode is checked on the open descriptor, so nothing can be swapped in
	// between the check and the read.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxPlistSize {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, maxPlistSize+1))
	if err != nil || len(data) > maxPlistSize {
		return nil
	}
	langs, err := appleLanguages(data)
	if err != nil {
		return nil
	}
	return langs
}
