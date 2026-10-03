// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Command taskctl is a tiny urfave/cli (v3) CLI that demonstrates Localizer.
//
//	go run ./examples/urfave-demo --help                     # English
//	LANG=ja_JP.UTF-8 go run ./examples/urfave-demo --help    # Japanese
//	LOCALIZER_LANG=qps go run ./examples/urfave-demo list    # pseudo-localization
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/DABH/localizer"
	"github.com/DABH/localizer/examples/urfave-demo/locales"
	"github.com/DABH/localizer/urfave"
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

func main() {
	cmd := newRootCmd()
	urfave.Localize(cmd, locales.FS) // the one line
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, localizer.Error(err))
		os.Exit(1)
	}
}

func newRootCmd() *cli.Command {
	return &cli.Command{
		Name:                  "taskctl",
		Usage:                 "Manage your tasks from the terminal.",
		Description:           "taskctl keeps a small list of tasks.\nUse it to add, list and complete tasks.",
		Version:               "0.1.0",
		Suggest:               true,
		EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Usage: "Print more details."},
		},
		Commands: []*cli.Command{newAddCmd(), newListCmd(), newDoneCmd()},
	}
}

func newAddCmd() *cli.Command {
	return &cli.Command{
		Name:        "add",
		Usage:       "Add a task.",
		Description: "Add a high-priority task.\n\n  $ taskctl add \"Write docs\" --priority high",
		ArgsUsage:   "<title>",
		Category:    "Changing tasks",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "priority", Value: "normal", Usage: "Priority of the task: `level` is low, normal or high.", Category: "Task details"},
			&cli.StringFlag{Name: "due", Usage: "Due date in YYYY-MM-DD format.", Category: "Task details"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			title := cmd.Args().First()
			if title == "" || cmd.Args().Len() != 1 {
				return cli.ShowSubcommandHelp(cmd)
			}
			switch priority := cmd.String("priority"); priority {
			case "low", "normal", "high":
			default:
				return localizer.Errorf("invalid priority %q: must be low, normal or high", priority)
			}
			fmt.Fprintf(cmd.Root().Writer, localizer.T("Added task %d: %q\n"), len(tasks)+1, title)
			return nil
		},
	}
}

func newListCmd() *cli.Command {
	return &cli.Command{
		Name:     "list",
		Usage:    "List tasks.",
		Category: "Viewing tasks",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "all", Usage: "Include completed tasks."},
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Value: "human", Usage: "Output format: human or json."},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			var shown []task
			for _, t := range tasks {
				if cmd.Bool("all") || !t.Done {
					shown = append(shown, t)
				}
			}
			out := cmd.Root().Writer
			if cmd.String("output") == "json" { // serialized output is never translated
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(shown)
			}
			if len(shown) == 0 {
				fmt.Fprintln(out, localizer.T("No tasks found."))
				return nil
			}
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", localizer.T("ID"), localizer.T("Title"), localizer.T("Priority"), localizer.T("Done"))
			for _, t := range shown {
				fmt.Fprintf(w, "%d\t%s\t%s\t%t\n", t.ID, t.Title, t.Priority, t.Done)
			}
			return w.Flush()
		},
	}
}

func newDoneCmd() *cli.Command {
	return &cli.Command{
		Name:      "done",
		Usage:     "Mark a task as done.",
		ArgsUsage: "<id>",
		Category:  "Changing tasks",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return cli.ShowSubcommandHelp(cmd)
			}
			id, err := strconv.Atoi(cmd.Args().First())
			if err != nil {
				return localizer.Errorf("task ID must be a number: %w", err)
			}
			for _, t := range tasks {
				if t.ID == id {
					fmt.Fprintf(cmd.Root().Writer, localizer.T("Completed task %d.\n"), id)
					return nil
				}
			}
			return localizer.Errorf("task %d not found", id)
		},
	}
}
