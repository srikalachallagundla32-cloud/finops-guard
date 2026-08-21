# Design: the AST pass — FG-008 & FG-009

> Status: **in progress** (M0 + M2 shipped) · Owner: @srikalachallagundla32-cloud · Tracking: #<!-- filled by issue -->

## Why

FinOps-Guard's wedge is *"catch what your coding agent wrote before it bills you."* The two most convincing agent-slop patterns are exactly the two rules that don't exist yet:

- **FG-008 — recursive-agent blow-up.** A function that calls *itself* (directly or through a tool loop) with no depth cap. Every LLM/tool call multiplies by the recursion fan-out. This is the canonical autonomous-agent failure.
- **FG-009 — unthrottled poll loop.** `while True: poll()` with no `sleep`, no backoff, no `break` — a hot loop hammering a paid endpoint.

Both are **out of reach for the current engine**, and that's not a tuning problem — it's structural.

### Why regex + the line scanner can't do it

The current engine (`pkg/analyzer/scan`) is a line-by-line loop-scope tracker: indentation for Python, brace-balance for TS/JS, regex-per-line inside an active loop scope. It has no notion of:

- **Function identity / binding** — FG-008 needs "is the call target the name of the enclosing function?" That requires a symbol bound to a scope, which a per-line regex cannot represent. (Go's RE2 also has no backreferences, so you can't even fake "same name here and there".)
- **Absence across a body** — FG-009 needs "this `while True` body contains *no* `sleep`/`break`/backoff." Proving a negative over a whole block is a tree/CFG question, not a line match.
- **Call graphs** — recursion through a helper (`run()` → `step()` → `run()`) needs edges between functions.

So FG-008/009 need a real parse tree. This doc scopes that.

## Non-goals

- Rewriting the existing regex rules. They stay. The AST pass is **additive**.
- A full control-flow/dataflow engine. We need a syntax tree with scopes and a shallow call graph — not SSA.
- New output surfaces. The card, PR comment, TUI, git-notes ledger, and suggestions all keep working **unchanged** (see Interface contract).

## Interface contract (the part that keeps everything else working)

`analyzer.Issue` is the universal currency of the whole tool — `costengine`, `pkg/ui` (card + comment), `internal/tui`, and `SuggestionBlock` all consume `[]Issue`. **The AST pass must emit `[]Issue` and nothing downstream changes.**

```go
// New, parallel to ScanFile. Same return type.
func ASTScanFile(filePath string) ([]Issue, error)

// main.go merges the two passes; order-stable, deduped by (ID, FilePath, LineNumber).
regexIssues, _ := analyzer.ScanFile(path, analyzer.GetDefaultRules())
astIssues,  _ := analyzer.ASTScanFile(path)
issues := analyzer.Merge(regexIssues, astIssues)
```

New `TargetAPI` values `"recursion"` and `"poll"` get cost cases in `costengine.EstimateCallCost` so `EstCostRisk` populates exactly like today. `Issue.CodeSnippet` = the offending line; `LineNumber` = the call site (FG-008) or the loop header (FG-009), so the existing SVG/comment/suggestion anchoring is unchanged.

**Fix suggestions:** FG-008 → advisory comment only (recursion caps are context-specific; no auto-rewrite — consistent with the current "never rewrite loop logic" rule). FG-009 → committable suggestion inserting a backoff/`sleep` scaffold, reusing the existing `SuggestionBlock` path.

## Parser decision

The hard constraint: FinOps-Guard ships as a **pure-Go, `CGO_ENABLED=0`, cross-compiled single binary** via GoReleaser → Homebrew. "Zero runtime deps" is a selling point. The parser choice is a tradeoff against that.

| Option | Langs | Single binary? | Cross-compile | Fidelity | Verdict |
|--------|-------|:---:|:---:|:---:|---|
| **Python `ast` sidecar** (`python3 -c`, emits JSON AST) | Python only | ✅ (Go stays pure) | ✅ | perfect (Python) | ⚠️ requires `python3` on the host at runtime — breaks zero-dep promise + hermetic CI |
| **tree-sitter, Go/CGO bindings** (`tree-sitter/go-tree-sitter` + py/ts grammars) | Py, TS, JS, Go | ✅ (grammars linked in) | ⚠️ needs CGO cross-toolchains per target | high | ✅ **recommended target** |
| **tree-sitter grammars on `wazero`** (WASM, pure Go) | Py, TS, JS | ✅ | ✅ (pure Go) | high | 🔬 promising, less trodden — fallback if we refuse CGO |
| Pure-Go native parsers | — | — | — | — | ❌ no mature Python/TS parser in Go |

**Recommendation: tree-sitter via CGO**, with GoReleaser switching FG's builds to per-platform native runners (matrix of `macos`/`linux` × `amd64`/`arm64`) instead of one cross-compile host. This is the standard cost of adding a C-backed parser and it's well-worn. Keep `CGO_ENABLED=0` for the current pure-Go paths behind a build tag so `go install` without a C toolchain still yields the regex-only engine (graceful degradation — see Rollout M0).

**If we want to keep pure-Go cross-compile:** `wazero` + precompiled grammar WASM is the escape hatch. Slower cold start, more novel, but no CGO. Decide at M1 with a spike.

## Detection algorithms

### FG-008 — recursive-agent blow-up (needs AST)

1. Build a per-file **function table**: name → node, params, body span.
2. Build a shallow **call graph**: for each function body, collect call expressions; resolve callee names against the function table (same file scope first).
3. Flag a function `f` when the call graph has a cycle reaching `f` **and** the recursive call is not guarded by a decrementing depth/budget parameter or an explicit base-case return before it. Heuristic guard detection: a param whose name matches `/(depth|budget|max_?(depth|calls|iters)|remaining|ttl)/i` that is compared (`<`, `<=`, `== 0`) on a path dominating the recursive call.
4. Extra signal (raises severity CRITICAL): the recursive path also contains an LLM/tool call (reuse the regex rule targets) — i.e. *recursion that spends money each level*.

`LineNumber` = the recursive call site. `TargetAPI = "recursion"`.

### FG-009 — unthrottled poll loop (AST; partial regex fallback possible)

1. Find `while True:` / `while (true)` / `for(;;)` loop nodes.
2. Within the loop body, require a **network/poll call** (LLM/HTTP/SDK targets — reuse regex targets + a small `requests|httpx|fetch|axios|.get(|.poll(` set).
3. Flag when the body contains **none** of: a `sleep`/`setTimeout`/`await asyncio.sleep`, a `break`/`return` reachable from the top, or a backoff construct. Proving this "none" over the block is the tree question regex can't answer.

`LineNumber` = loop header. `TargetAPI = "poll"`. **Shipped:** the scope-tracker version described here (infinite-loop scope + poll present + delay absent, comment-stripped) is live as of the FG-009 PR. An AST version would extend it to delays/exits that live in a called helper; the loop-local detector already covers the dominant in-line pattern.

## Cost model

Add to `costengine.EstimateCallCost`:

- `"recursion"` → `perCall × estimatedFanout` where `estimatedFanout` defaults to a conservative branching×depth constant (config-overridable), because static analysis can't know runtime depth. Documented as an *upper-bound risk*, matching how loop rules already project.
- `"poll"` → `perCall × callsPerMinute × 60` as an hourly burn projection (config-overridable).

Both surface in the same burn gauge / monthly-impact math already in the card and TUI.

## Rollout

- **M0 — seam (no behavior change).** ✅ **Shipped.** `ASTScanFile` (currently `nil, nil`) + `Merge` wired in `main.go`; regex-only default, no C toolchain needed. *(Landed pure-Go without the build tag, since no CGO code exists yet; the tag returns when the tree-sitter parser lands at M1.)*
- **M1 — parser spike.** CGO tree-sitter vs. wazero decision, one grammar (Python) loading and walked in a test. Pick the path.
- **M2 — FG-009.** ✅ **Shipped (scope-tracker version).** Detects an infinite loop that polls with no delay by proving the *absence* of any sleep/backoff across the loop body — comment-aware, `break`/`return` correctly treated as exits not delays. Cost case (`poll`) + positive/negative/regression fixtures included. The AST version can later supersede it for cross-function cases; the loop-local detector covers the dominant pattern.
- **M3 — FG-008.** Call graph + guard heuristic + CRITICAL-when-spending. Fixtures incl. the helper-indirection case.
- **M4 — TS/JS grammar**, GoReleaser per-platform build matrix, Homebrew cask update, docs. Flip AST on by default.

## Test plan

- Golden fixtures under `pkg/analyzer/testdata/ast/` — true-positive and near-miss (guarded recursion, `while True` *with* `sleep`, poll with `break`) for each rule; assert exact `Issue` fields.
- Fuzz the parser boundary (malformed/partial files) — must degrade to "no AST issues", never panic; regex pass still runs.
- Snapshot the merged `[]Issue` through the card + PR-comment generators to prove downstream is untouched.

## Risks

- **CGO vs. distribution.** The real cost. Mitigated by M0 graceful degradation (regex-only without a C toolchain) and the wazero escape hatch.
- **Heuristic guard detection (FG-008)** will have false positives/negatives at the edges. Ship conservative (favor precision over recall); tune from real fixtures.
- **Scope creep toward a full CFG.** Explicitly out of scope; if a rule "needs dataflow," it waits.
