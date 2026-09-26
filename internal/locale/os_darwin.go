package locale

import (
	"os"
	"path/filepath"
)

// platformLanguages reads AppleLanguages from the user's global preferences. The file is read directly
// (no cgo, no `defaults` exec); cfprefsd may hold a newer value in memory for a short while after the user
// changes the setting, which is acceptable for picking a UI language.
func platformLanguages() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, "Library", "Preferences", ".GlobalPreferences.plist"))
	if err != nil || len(data) > 8<<20 {
		return nil
	}
	langs, err := appleLanguages(data)
	if err != nil {
		return nil
	}
	return langs
}
