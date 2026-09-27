---
title: Security model
description: What the runtime and the hosted service can and can’t do.
---

Localizer is built so that adopting it adds as little risk as possible for CLI maintainers and their users.

## The shipped CLI

- **No network, no telemetry, no credentials.** Translations are compiled into the binary with `go:embed`,
  or shipped as data files inside the Python package. Nothing is downloaded at runtime, and nothing about
  the user is reported.
- **Catalogs are untrusted data.** Before a translation is used, the runtime checks that it keeps exactly
  the source’s placeholders (Go verbs, including flags, width and precision, and template actions; Python
  `str.format` fields, printf verbs and Rich markup tags; backquoted spans) and adds no control characters
  or terminal escape sequences. Anything else falls back to English. A malicious catalog entry can’t inject
  escape sequences, change how arguments are rendered, or allocate huge padding.
- **Only known strings change.** Output that doesn’t match a catalog entry, such as server responses, IDs
  and JSON, passes through untouched. Reverse matching refuses captures that look like prose.
- **Python hooks fail open.** The runtime works on copies of your command objects and never modifies them;
  any internal error leaves the CLI in English rather than breaking it.

## Your repository

- **No secrets in the repository.** Identity comes from the GitHub App installation or, with the Action,
  from a short-lived GitHub OIDC token. There is no API key to leak.
- **Open source only.** The service refuses private repositories at installation and on every job.
- **Least privilege.** Each App job mints an installation token scoped to one repository, with
  `contents:write`, `pull_requests:write` and `metadata:read`. Tokens expire within an hour and are never
  stored. With the Action, Localizer has no write access at all.
- **Pull requests only.** Localizer never pushes to your default branch. It updates its own
  `localizer-translations` branch and a single rolling pull request that you review. It writes only the
  catalogs, plus, in the onboarding pull request, `.localizer.yml`, the locales package file
  (`locales/embed.go` or `locales/__init__.py`), the one-line integration and, for Python, the dependency in
  `pyproject.toml`. An allowlist in the service enforces this.
- **Human edits win.** Existing catalog entries are never retranslated, so your corrections stay.

## The service

- **Your code is never executed.** Source is fetched as a tarball and unpacked in memory. Only `*.go`,
  `go.mod`, `*.py`, `pyproject.toml`, `setup.py`, `setup.cfg`, the configuration and catalogs are kept;
  links, absolute paths and `..` are rejected; and size and file-count limits apply. Go is parsed with
  `go/parser`, Python with the standard library’s `ast` in a separate function with no network access.
  Nothing is built, imported or run, and no source is kept after a job.
- **Abuse resistance.** The service translates only strings it extracted itself from a public
  repository’s default branch, so a forged trigger can’t make it translate arbitrary text. Per-account
  monthly quotas, a translation memory (nothing is paid for twice) and a cap on concurrent jobs bound cost.
- **Prompt injection.** Strings are sent to the model as JSON data with a fixed instruction set. Every
  answer is schema-checked and validated (placeholders, backquotes, markup, URLs, flags, `<args>`, glossary
  terms, quoting, paragraph structure, length) before it can reach a pull request, and the runtime checks
  it again.
- **Secrets.** The GitHub App private key and webhook secret live in AWS Secrets Manager, encrypted with a
  customer-managed KMS key. Webhooks are authenticated with HMAC-SHA256 in constant time and de-duplicated
  by delivery ID. Action requests are authenticated by verifying GitHub’s OIDC token signature and claims.
- **Data.** DynamoDB, encrypted with KMS and backed up with point-in-time recovery, holds installation
  records, job history (kept for 90 days), usage counters, and the translation memory (public source
  strings and their translations). Results for the Action are kept encrypted for at most 7 days and are
  handed out through links that expire after 10 minutes. Logs are kept for 30 days.

## Compliance

Localizer doesn’t have a SOC 2 report yet. The design keeps the scope small: public data only, no end-user
data, no customer secrets, no retained source, infrastructure as code, least-privilege IAM, encryption at
rest and in transit, and audit logging. The subprocessors are AWS (including Claude, through Amazon Bedrock
or Claude Platform on AWS) and GitHub.

## Reporting a vulnerability

Please report vulnerabilities privately through
[GitHub’s security advisories](https://github.com/DABH/localizer/security/advisories/new), not in public
issues.
