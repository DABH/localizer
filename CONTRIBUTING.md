# Contributing to Localizer

Thank you for helping! For anything larger than a small fix, please open an issue first so we can agree on
the approach. Report security problems privately, as described in [SECURITY.md](SECURITY.md).

## Contributor Assignment Agreement

Localizer is developed by Snizyx Software LLC. Before your first pull request can be merged, you need to sign
the [Contributor Assignment Agreement](CLA.md). It assigns the copyright in your contributions to Snizyx
Software LLC and gives you back a license to use them however you like.

A bot checks every pull request. To sign, post this comment on your pull request:

> I have read the Localizer Contributor Assignment Agreement and I hereby sign it.

You only sign once. Everyone who authored a commit in the pull request needs to sign, and commits must be
authored with an email address that is linked to your GitHub account.

## Development

The Go library needs Go 1.22 or later; the Python runtime (`python/`) needs Python 3.10 or later and [uv](https://docs.astral.sh/uv/) for its tests; the documentation site needs Node.js 24.

```sh
make            # gofmt check, go vet and tests
make race       # tests with the race detector
make fuzz       # 30 seconds of each fuzzer
make site-dev   # serve the documentation site locally
```

## Pull requests

- Keep each pull request focused, and add tests for changes in behavior.
- Run `make` before pushing.
- Start new source files with the same copyright header as the existing ones.
