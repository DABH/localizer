# Localizer GitHub Action

Keeps your Go or Python CLI's translation catalogs in sync without giving Localizer write access to your repository.
The workflow authenticates with its short-lived GitHub OIDC token, so there are no secrets to store.
Localizer fetches the public source at that commit and translates new strings, and the workflow opens one
rolling pull request with its own `GITHUB_TOKEN`.

```yaml
name: Localizer
on:
  push:
    branches: [main] # your default branch
  workflow_dispatch:

permissions:
  contents: write
  pull-requests: write
  id-token: write

concurrency:
  group: localizer # one run at a time: a second push waits
  cancel-in-progress: false

jobs:
  translations:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: DABH/localizer/action@v0.5.2
```

Also enable **Settings → Actions → General → Allow GitHub Actions to create and approve pull requests**.

Inputs, outputs and notes: see the [GitHub Action guide](https://locale.dev/guides/github-action/).
The hosted service needs a subscription for the repository's GitHub account ([pricing](https://locale.dev/pricing/)) and serves public repositories only.
