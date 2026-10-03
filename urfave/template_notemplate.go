// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

//go:build urfave_cli_no_template

package urfave

// Built with urfave/cli's urfave_cli_no_template tag, help is rendered without text/template and the
// template variables are constants: the section headings stay in English, and everything else is
// translated as usual.

type templates struct{}

func saveTemplates() templates { return templates{} }

func (templates) restore() {}

func localizeTemplates() {}

func translateTemplate(tpl string) string { return tpl }
