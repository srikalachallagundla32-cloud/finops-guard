#!/usr/bin/env bash
# spend-note.sh — Attach a cost delta note to the current commit
# Usage: ./scripts/spend-note.sh <cost-delta> [status]
# Example: ./scripts/spend-note.sh "-135.00" "bounty claimed"

set -euo pipefail

COST_DELTA="${1:-0}"
STATUS="${2:-}"

# Format the note
if [ -n "$STATUS" ]; then
  NOTE="💰 \$${COST_DELTA}/mo (${STATUS})"
else
  NOTE="💰 \$${COST_DELTA}/mo"
fi

# Attach note to current commit using finops ref
git notes --ref=finops add -f -m "$NOTE" HEAD

echo "✅ Attached spend note to $(git rev-parse --short HEAD): $NOTE"
echo ""
echo "View spend history with:"
echo "  git log --notes=finops --oneline"
