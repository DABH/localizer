// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

//go:build !darwin && !windows

package locale

// platformLanguages has no OS-wide setting to consult beyond the POSIX environment on this platform.
func platformLanguages() []string { return nil }
