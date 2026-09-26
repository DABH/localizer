#!/usr/bin/env bash
# Localizer GitHub Action: ask the Localizer service to sync this commit's translations (authenticated with
# the workflow's short-lived GitHub OIDC token — no stored secrets), then apply the returned catalogs and
# open or update one pull request with the repository's own token.
set -euo pipefail

if [[ -z "${ACTIONS_ID_TOKEN_REQUEST_URL:-}" ]]; then
  echo "::error::Localizer needs an OIDC token: add 'permissions: id-token: write' to the workflow."
  exit 1
fi
if [[ ! -d .git ]]; then
  echo "::error::Run actions/checkout before Localizer."
  exit 1
fi

oidc_token() {
  curl -sSf -H "Authorization: bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" \
    "$ACTIONS_ID_TOKEN_REQUEST_URL&audience=$LOCALIZER_AUDIENCE" | jq -r .value
}

api() { # method path — a fresh token per call (they expire after a few minutes); never printed
  curl -sS -X "$1" -H "Authorization: Bearer $(oidc_token)" -H "Accept: application/json" "$LOCALIZER_API$2"
}

resp="$(api POST /sync)"
job="$(jq -r '.job // empty' <<<"$resp")"
if [[ -z "$job" ]]; then
  echo "::error::Localizer refused the request: $(jq -r '.error // .' <<<"$resp")"
  exit 1
fi
echo "Localizer job $job queued for ${GITHUB_REPOSITORY}@${GITHUB_SHA:0:7}"

deadline=$(( $(date +%s) + LOCALIZER_TIMEOUT_MINUTES * 60 ))
while :; do
  st="$(api GET "/jobs/$job")"
  status="$(jq -r '.status // "unknown"' <<<"$st")"
  case "$status" in
    done) break ;;
    up-to-date)
      echo "Translations are up to date."
      echo "status=up-to-date" >>"$GITHUB_OUTPUT"
      exit 0 ;;
    queued|running) ;;
    *)
      echo "::error::Localizer job $status: $(jq -r '.detail // .error // .' <<<"$st")"
      exit 1 ;;
  esac
  if (( $(date +%s) > deadline )); then
    echo "::error::Timed out waiting for translations (job $job is still $status)."
    exit 1
  fi
  sleep 15
done

result="$(mktemp)"
curl -sSf "$(jq -r .result_url <<<"$st")" -o "$result"

# Write the files. The service only ever returns catalogs (and, on a first run, the one-line integration),
# but never trust paths blindly.
mapfile -t paths < <(jq -r '.files[].path' "$result")
for p in "${paths[@]}"; do
  case "$p" in
    /*|*..*|.git/*|.github/*) echo "::error::Refusing to write $p"; exit 1 ;;
  esac
done
jq -r '.files[] | @base64' "$result" | while read -r f; do
  p="$(base64 -d <<<"$f" | jq -r .path)"
  mkdir -p "$(dirname "$p")"
  base64 -d <<<"$f" | jq -j .content >"$p"
done

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
base="${GITHUB_REF_NAME}"
git switch -q -C "$LOCALIZER_BRANCH"
git add -- "${paths[@]}"
if git diff --cached --quiet; then
  echo "Nothing to commit."
  echo "status=up-to-date" >>"$GITHUB_OUTPUT"
  exit 0
fi
git commit -q -m "$(jq -r .title "$result")"
git push -q --force origin "$LOCALIZER_BRANCH"

body="$(mktemp)"
jq -r .body "$result" >"$body"
title="$(jq -r .title "$result")"
number="$(gh pr list --head "$LOCALIZER_BRANCH" --state open --json number --jq '.[0].number // empty')"
if [[ -n "$number" ]]; then
  gh pr edit "$number" --title "$title" --body-file "$body" >/dev/null
  url="$(gh pr view "$number" --json url --jq .url)"
  echo "status=pr-updated" >>"$GITHUB_OUTPUT"
else
  url="$(gh pr create --head "$LOCALIZER_BRANCH" --base "$base" --title "$title" --body-file "$body")"
  echo "status=pr-opened" >>"$GITHUB_OUTPUT"
fi
echo "pull-request=$url" >>"$GITHUB_OUTPUT"
echo "Translations pull request: $url"
