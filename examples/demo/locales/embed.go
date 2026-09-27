// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Package locales holds taskctl's translation catalogs, maintained by Localizer.
package locales

import "embed"

// FS contains one "<language>.json" catalog per supported language.
//
//go:embed *.json
var FS embed.FS
