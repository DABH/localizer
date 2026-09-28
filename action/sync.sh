#!/usr/bin/env bash
# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

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

# Every request retries transient failures (a dropped connection, a 5xx) and gives up after a minute.
CURL=(curl -sS --retry 4 --retry-all-errors --retry-delay 5 --max-time 60)

oidc_token() { # a fresh token for one request (they expire after a few minutes); never printed
  local token
  token="$("${CURL[@]}" -f -H "Authorization: bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" \
    "$ACTIONS_ID_TOKEN_REQUEST_URL&audience=$LOCALIZER_AUDIENCE" | jq -r '.value // empty')" || return 1
  [[ -n "$token" ]] || return 1
  printf '%s' "$token"
}

api() { # method path — prints the response body; fails when the request could not be made at all
  local token
  token="$(oidc_token)" || return 1
  "${CURL[@]}" -X "$1" -H "Authorization: Bearer $token" -H "Accept: application/json" "$LOCALIZER_API$2"
}

detail() { # the message in a JSON error body, or else the body itself — on one line, so that it cannot
  # smuggle a workflow command into the log
  { jq -r '.detail // .error // .' <<<"$1" 2>/dev/null || printf '%s' "${1:0:300}"; } | tr -s '\000-\037\177' ' '
}

# Where the service may write. The result is data from the network, so every path has to be a plain
# relative path — no absolute paths, backslashes, colons (a Windows drive or stream) or control characters,
# and every component a real name: not empty, "." or "..", and not .git or .github in any letter case (the
# runner's disk may be case-insensitive, and Windows drops trailing dots and spaces from a name). And it has
# to be one of the files the service produces: a catalog (*.json), an integration source file (*.go, *.py),
# pyproject.toml, or .localizer.yml in the repository root. A rejected path is reported and nothing is written.
validate_path() {
  local p="$1" rest comp trimmed shown
  shown="$(printf '%q' "$p")"
  if [[ -z "$p" ]]; then
    echo "::error::Refusing to write an empty path"
    return 1
  fi
  if [[ "$p" == /* || "$p" == *\\* || "$p" == *:* || "$p" == *[[:cntrl:]]* ]]; then
    echo "::error::Refusing to write $shown: absolute, or with a backslash, colon or control character"
    return 1
  fi
  rest="$p"
  while :; do
    comp="${rest%%/*}"
    case "$comp" in
      '' | . | ..)
        echo "::error::Refusing to write $shown: empty, . or .. path component"
        return 1 ;;
    esac
    trimmed="$comp"
    while [[ "$trimmed" == *. || "$trimmed" == *' ' ]]; do trimmed="${trimmed%?}"; done
    case "$trimmed" in
      .[Gg][Ii][Tt] | .[Gg][Ii][Tt][Hh][Uu][Bb])
        echo "::error::Refusing to write $shown: inside .git or .github"
        return 1 ;;
    esac
    [[ "$rest" == */* ]] || break
    rest="${rest#*/}"
  done
  case "$p" in
    *.json | *.go | *.py | pyproject.toml | */pyproject.toml | .localizer.yml) return 0 ;;
  esac
  echo "::error::Refusing to write $shown: not a catalog, a source file, pyproject.toml or .localizer.yml"
  return 1
}

if ! resp="$(api POST /sync)"; then
  echo "::error::Could not reach Localizer at $LOCALIZER_API."
  exit 1
fi
job="$(jq -r '.job // empty' <<<"$resp" 2>/dev/null | tr -d '\000-\037\177')" || job=""
if [[ -z "$job" ]]; then
  echo "::error::Localizer refused the request: $(detail "$resp")"
  exit 1
fi
echo "Localizer job $job queued for ${GITHUB_REPOSITORY}@${GITHUB_SHA:0:7}"

# Poll until the job ends. An answer that is not JSON, or has no status (a proxy's error page, a 5xx), is
# a warning and another try, until the deadline.
deadline=$(( $(date +%s) + LOCALIZER_TIMEOUT_MINUTES * 60 ))
while :; do
  status=""
  if st="$(api GET "/jobs/$job")"; then
    status="$(jq -r '.status // empty' <<<"$st" 2>/dev/null | tr -d '\000-\037\177')" || status=""
  else
    st=""
  fi
  case "$status" in
    done) break ;;
    up-to-date)
      echo "Translations are up to date."
      echo "status=up-to-date" >>"$GITHUB_OUTPUT"
      exit 0 ;;
    queued|running) ;;
    "")
      # An answer with an error message and no status is the service refusing (403, 404): not transient.
      if [[ -n "$st" ]] && jq -e '.error | type == "string"' <<<"$st" >/dev/null 2>&1; then
        echo "::error::Localizer refused the request: $(detail "$st")"
        exit 1
      fi
      echo "::warning::No job status from Localizer, retrying${st:+: $(detail "$st")}" ;;
    *)
      echo "::error::Localizer job $status: $(detail "$st")"
      exit 1 ;;
  esac
  if (( $(date +%s) > deadline )); then
    echo "::error::Timed out waiting for translations (job $job is ${status:-not answering})."
    exit 1
  fi
  sleep 15
done

result_url="$(jq -r '.result_url // empty' <<<"$st")"
if [[ "$result_url" != https://* ]]; then
  echo "::error::Localizer job $job finished without a result."
  exit 1
fi
result="$(mktemp)"
if ! "${CURL[@]}" -f "$result_url" -o "$result"; then
  echo "::error::Could not download the result of Localizer job $job."
  exit 1
fi

# The result: {title, body, files: [{path, content}], ...}. Check its shape before trusting any of it (a
# NUL in a path would split it into two below, so it is rejected here).
if ! jq -e '(.title | type) == "string" and (.body | type) == "string" and (.files | type) == "array"
  and all(.files[]?; (.path | type) == "string" and (.content | type) == "string"
    and (.path | explode | index(0)) == null)' "$result" >/dev/null; then
  echo "::error::Localizer returned a malformed result."
  exit 1
fi

# Check every path first, so that a bad one means nothing gets written. Paths are read NUL-delimited: a
# newline in a path is a control character, not a record separator.
count="$(jq '.files | length' "$result")"
paths=()
while IFS= read -r -d '' p; do
  validate_path "$p" || exit 1
  paths+=("$p")
done < <(jq -j '.files[].path + "\u0000"' "$result")
if (( ${#paths[@]} != count )); then
  echo "::error::Localizer returned a malformed result."
  exit 1
fi
if (( count == 0 )); then
  echo "Nothing to write."
  echo "status=up-to-date" >>"$GITHUB_OUTPUT"
  exit 0
fi

# Write the validated paths, each with the content at the same index.
i=0
for p in "${paths[@]}"; do
  if [[ "$(jq -r ".files[$i].path" "$result")" != "$p" ]] || ! validate_path "$p"; then
    echo "::error::Localizer returned a malformed result."
    exit 1
  fi
  mkdir -p -- "$(dirname -- "$p")"
  jq -j ".files[$i].content" "$result" >"$p"
  i=$((i + 1))
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

# Push with the github-token input, whichever credentials actions/checkout kept: gh hands the token to git
# as a credential helper, and the push ignores the Authorization header checkout may have stored for its own
# token (which would otherwise take precedence). The token is never printed.
gh auth setup-git
git -c "http.${GITHUB_SERVER_URL:-https://github.com}/.extraheader=" push -q --force origin "$LOCALIZER_BRANCH"

body="$(mktemp)"
jq -r .body "$result" >"$body"
title="$(jq -r .title "$result")"
# Only a pull request from this repository's own branch: --head also matches a fork's branch of that name.
number="$(gh pr list --head "$LOCALIZER_BRANCH" --state open --json number,isCrossRepository \
  --jq '[.[] | select(.isCrossRepository | not)][0].number // empty')"
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
