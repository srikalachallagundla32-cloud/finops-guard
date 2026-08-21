## What this changes

<!-- One or two sentences. Link the issue: Closes #___ -->

## Type
- [ ] New / changed detection rule (FG-###)
- [ ] TUI / card / PR-comment output
- [ ] Cost model
- [ ] Docs
- [ ] Build / release / CI

## Checklist
- [ ] `go build ./...` and `go test ./...` pass
- [ ] New rules ship with **positive and negative** fixtures (a near-miss that must *not* fire)
- [ ] `[]Issue` stays the contract — no downstream consumer (card, comment, TUI, ledger) needed changes
- [ ] README detections table / CLI reference updated if user-facing

## Notes for the reviewer
<!-- Precision/recall tradeoffs, follow-ups, anything deliberately deferred. -->
