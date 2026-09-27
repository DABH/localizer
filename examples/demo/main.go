// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Command taskctl is a tiny Cobra CLI that demonstrates Localizer.
//
//	go run ./examples/demo --help                     # English
//	LANG=ja_JP.UTF-8 go run ./examples/demo --help    # Japanese
//	LOCALIZER_LANG=qps go run ./examples/demo list    # pseudo-localization
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/DABH/localizer"
	"github.com/DABH/localizer/examples/demo/locales"
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
	root := newRootCmd()
	localizer.Localize(root, locales.FS) // the one line
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "taskctl",
		Short: "Manage your tasks from the terminal.",
		Long:  "taskctl keeps a small list of tasks.\nUse it to add, list and complete tasks.",
	}
	root.PersistentFlags().Bool("verbose", false, "Print more details.")
	root.AddCommand(newAddCmd(), newListCmd(), newDoneCmd())
	return root
}

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a task.",
		Args:  cobra.ExactArgs(1),
		Example: "Add a high-priority task.\n\n" +
			"  $ taskctl add \"Write docs\" --priority high",
		RunE: func(cmd *cobra.Command, args []string) error {
			priority, _ := cmd.Flags().GetString("priority")
			switch priority {
			case "low", "normal", "high":
			default:
				return fmt.Errorf("invalid priority %q: must be low, normal or high", priority)
			}
			fmt.Fprintf(cmd.OutOrStdout(), localizer.T("Added task %d: %q\n"), len(tasks)+1, args[0])
			return nil
		},
	}
	cmd.Flags().String("priority", "normal", "Priority of the task: `level` is low, normal or high.")
	cmd.Flags().String("due", "", "Due date in YYYY-MM-DD format.")
	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			all, _ := cmd.Flags().GetBool("all")
			format, _ := cmd.Flags().GetString("output")
			var shown []task
			for _, t := range tasks {
				if all || !t.Done {
					shown = append(shown, t)
				}
			}
			if format == "json" { // serialized output is never translated
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(shown)
			}
			if len(shown) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), localizer.T("No tasks found."))
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", localizer.T("ID"), localizer.T("Title"), localizer.T("Priority"), localizer.T("Done"))
			for _, t := range shown {
				fmt.Fprintf(w, "%d\t%s\t%s\t%t\n", t.ID, t.Title, t.Priority, t.Done)
			}
			return w.Flush()
		},
	}
	cmd.Flags().Bool("all", false, "Include completed tasks.")
	cmd.Flags().StringP("output", "o", "human", "Output format: human or json.")
	return cmd
}

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark a task as done.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("task ID must be a number: %w", err)
			}
			for _, t := range tasks {
				if t.ID == id {
					fmt.Fprintf(cmd.OutOrStdout(), localizer.T("Completed task %d.\n"), id)
					return nil
				}
			}
			return fmt.Errorf("task %d not found", id)
		},
	}
}
