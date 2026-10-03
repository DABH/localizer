// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Command taskctl is a tiny kong CLI that demonstrates Localizer.
//
//	go run ./examples/kong-demo --help                     # English
//	LANG=ja_JP.UTF-8 go run ./examples/kong-demo --help    # Japanese
//	LOCALIZER_LANG=qps go run ./examples/kong-demo list    # pseudo-localization
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/alecthomas/kong"

	"github.com/DABH/localizer"
	"github.com/DABH/localizer/examples/kong-demo/locales"
	"github.com/DABH/localizer/kongx"
)

type task struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Done     bool   `json:"done"`
}

// Sample data stands in for a server response: it is never translated.
var tasks = []task{
	{1, "Write the quarterly report", "high", false},
	{2, "Renew TLS certificates", "normal", true},
}

// CLI is the kong grammar: struct tags describe the commands, flags and arguments.
type CLI struct {
	Verbose bool `help:"Print more details."`

	Add  AddCmd  `cmd:"" help:"Add a task."`
	List ListCmd `cmd:"" help:"List tasks."`
	Done DoneCmd `cmd:"" help:"Mark a task as done."`
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("taskctl"),
		kong.Description("taskctl keeps a small list of tasks.\nUse it to add, list and complete tasks."),
		kongx.Localize(locales.FS), // the one line, last
	)
	ctx.FatalIfErrorf(ctx.Run())
}

// AddCmd adds a task.
type AddCmd struct {
	Title    string `arg:"" help:"Title of the task."`
	Priority string `help:"Priority of the task: low, normal or high." default:"normal" enum:"low,normal,high"`
	Due      string `help:"Due date in YYYY-MM-DD format."`
}

// Run adds the task.
func (c *AddCmd) Run() error {
	fmt.Printf(localizer.T("Added task %d: %q\n"), len(tasks)+1, c.Title)
	return nil
}

// ListCmd lists tasks.
type ListCmd struct {
	All    bool   `help:"Include completed tasks."`
	Output string `short:"o" help:"Output format: human or json." default:"human" enum:"human,json"`
}

// Run prints the tasks.
func (c *ListCmd) Run() error {
	var shown []task
	for _, t := range tasks {
		if c.All || !t.Done {
			shown = append(shown, t)
		}
	}
	if c.Output == "json" { // serialized output is never translated
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(shown)
	}
	if len(shown) == 0 {
		fmt.Println(localizer.T("No tasks found."))
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", localizer.T("ID"), localizer.T("Title"), localizer.T("Priority"), localizer.T("Done"))
	for _, t := range shown {
		fmt.Fprintf(w, "%d\t%s\t%s\t%t\n", t.ID, t.Title, t.Priority, t.Done)
	}
	return w.Flush()
}

// DoneCmd marks a task as done.
type DoneCmd struct {
	ID int `arg:"" help:"ID of the task."`
}

// Run marks the task as done. The error it returns is displayed by kong, through the translated error
// writer.
func (c *DoneCmd) Run() error {
	for _, t := range tasks {
		if t.ID == c.ID {
			fmt.Printf(localizer.T("Completed task %d.\n"), c.ID)
			return nil
		}
	}
	return fmt.Errorf("task %d not found", c.ID)
}
