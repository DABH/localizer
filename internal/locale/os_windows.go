// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package locale

import "golang.org/x/sys/windows"

// platformLanguages returns the Windows display (UI) languages, most preferred first. This is the
// user's language choice, as opposed to the regional format that GetUserDefaultLocaleName reports.
func platformLanguages() []string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil {
		return nil
	}
	return langs
}
