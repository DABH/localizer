# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""taskctl: the Typer version of Localizer's demo CLI.

Run it in another language to see every string change while the data stays as it is:

    LANG=ja_JP.UTF-8 python -m taskctl --help
    LANG=de_DE.UTF-8 python -m taskctl add --help
    LOCALIZER_LANG=qps python -m taskctl done 9
"""

import json

import typer

import localizer

app = typer.Typer(
    help="taskctl keeps a small list of tasks.\n\nUse it to add, list and complete tasks.",
    rich_markup_mode="markdown",
    add_completion=False,
)

TASKS = [
    {"id": 1, "title": "Write docs", "priority": "high", "done": False},
    {"id": 2, "title": "Ship v1", "priority": "normal", "done": True},
]


@app.callback()
def main(verbose: bool = typer.Option(False, "--verbose", help="Print more details.")):
    """Manage your tasks from the terminal."""


@app.command()
def add(
    title: str = typer.Argument(..., help="Title of the task."),
    priority: str = typer.Option("normal", metavar="level", help="Priority of the task: `level` is low, normal or high."),
    due: str = typer.Option(None, help="Due date in YYYY-MM-DD format."),
):
    """Add a task.

    Add a high-priority task:

        $ taskctl add "Write docs" --priority high
    """
    n = len(TASKS) + 1
    typer.echo(localizer.tf("Added task {n}: {title!r}", n=n, title=title))


@app.command("list")
def list_(
    all_: bool = typer.Option(False, "--all", help="Include completed tasks."),
    output: str = typer.Option("human", "-o", "--output", help="Output format: human or json."),
):
    """List tasks."""
    tasks = TASKS if all_ else [t for t in TASKS if not t["done"]]
    if output == "json":
        print(json.dumps(tasks))  # serialized output is never translated
        return
    for t in tasks:
        typer.echo(f"{t['id']:>3}  {t['title']}")


@app.command()
def done(id: int = typer.Argument(..., help="ID of the task.")):
    """Mark a task as done."""
    for t in TASKS:
        if t["id"] == id:
            t["done"] = True
            return
    # An f-string message: the catalog holds its template ("task {id} not found") and the runtime matches
    # the formatted text when Typer displays the error (inside its own "Invalid value: ..." wording).
    raise typer.BadParameter(f"task {id} not found")


def run() -> None:
    localizer.localize(app, "taskctl.locales")  # the whole integration
    app()


if __name__ == "__main__":
    run()
