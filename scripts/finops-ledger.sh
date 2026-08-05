#!/usr/bin/env bash
# finops-ledger.sh — annotate git history with per-commit cost deltas using
# git notes (refs/notes/finops). This makes `git log` itself a spend ledger,
# offline and dashboard-free.
#
# Usage:
#   scripts/finops-ledger.sh record <file> [commit]   # attach a note to a commit (default HEAD)
#   scripts/finops-ledger.sh log                       # show git log annotated with finops notes
#   scripts/finops-ledger.sh push                      # push notes to origin
#   scripts/finops-ledger.sh fetch                     # fetch notes from origin
#
# Notes are NOT fetched by default; run `fetch` (or configure
# `git config --add remote.origin.fetch '+refs/notes/*:refs/notes/*'`).

set -euo pipefail

REF="refs/notes/finops"
BIN="${FINOPS_BIN:-./bin/finops-guard}"
PRICING="${FINOPS_PRICING:-pricing.json}"

cmd="${1:-log}"

case "$cmd" in
  record)
    file="${2:?usage: finops-ledger.sh record <file> [commit]}"
    commit="${3:-HEAD}"
    note="$("$BIN" --pricing="$PRICING" --scan="$file" --note)"
    git notes --ref="$REF" add -f -m "$note" "$commit"
    echo "Recorded on $(git rev-parse --short "$commit"): $note"
    ;;
  log)
    git log --notes="$REF" --pretty=format:'%C(auto)%h%Creset %s%n    %C(yellow)%N%Creset' "${@:2}"
    ;;
  push)
    git push origin "$REF"
    ;;
  fetch)
    git fetch origin "$REF:$REF"
    ;;
  *)
    echo "unknown command: $cmd (use record|log|push|fetch)" >&2
    exit 2
    ;;
esac
