// Package locales holds taskctl's translation catalogs, maintained by Localizer.
package locales

import "embed"

// FS contains one "<language>.json" catalog per supported language.
//
//go:embed *.json
var FS embed.FS
