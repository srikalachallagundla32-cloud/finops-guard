<div align="center">

# 🛡️ FinOps-Guard

### Catch what your coding agent wrote — before it bills you

**AI coding agents write code that _runs_. They never feel the invoice.** FinOps-Guard is a static-analysis CLI that catches the loop-bound, fan-out, and re-tokenizing API calls — the **"AI slop"** an agent happily ships — that quietly turn a $5 script into a $5,000 cloud bill. It scans the diff **before merge** across **7 providers** (OpenAI, Anthropic, Bedrock, Vertex AI, Athena, DynamoDB, Pinecone), projects the dollar risk, and reports it right where you review: **the pull request**.

[![CI](https://github.com/srikalachallagundla32-cloud/finops-guard/actions/workflows/finops-guard.yml/badge.svg)](https://github.com/srikalachallagundla32-cloud/finops-guard/actions/workflows/finops-guard.yml)
![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-green)
![PRs welcome](https://img.shields.io/badge/PRs-welcome-ff69b4)

<br/>

<img src="https://raw.githubusercontent.com/srikalachallagundla32-cloud/finops-guard/main/assets/burn.svg" width="820" alt="FinOps-Guard cost analysis card — the animated report posted on every pull request" />

<sub><b>Every PR gets this.</b> An animated cost card with a live burn gauge, the flagged leak, a projected monthly impact, and a one-click <i>Commit suggestion</i> to fix it.</sub>

</div>

---

## ▶ See it live in your terminal

The default `finops-guard` run is an animated **Cost Reactor** TUI — a boot sequence, a Doom-fire cost canvas whose height tracks your projected spend, dollar-rain particles, and navigable findings. Press **`d`** for a side-by-side refactor diff; try **`--theme=matrix`** or **`--theme=cosmos`** for alternate animated canvases.

<img src="assets/demo.png" width="900" alt="FinOps-Guard Cost Reactor TUI — a Doom-fire cost canvas, dollar-rain particles, a live budget-burn gauge, and navigable findings" />

> **Want the animated version?** The recording spec lives in [`demo.tape`](demo.tape). Generate a GIF with `brew install vhs && make demo-gif`.

---

## 🚀 Quickstart

### Install

```bash
# Go (recommended)
go install github.com/srikalachallagundla32-cloud/finops-guard/cmd/finops-guard@latest

# Homebrew (via tap)
brew install srikalachallagundla32-cloud/tap/finops-guard

# From source
git clone https://github.com/srikalachallagundla32-cloud/finops-guard.git
cd finops-guard && make build     # → bin/finops-guard
```

### Scan a file

```bash
# Animated TUI (default)
finops-guard --pricing=pricing.json --scan=examples/ingest_pipeline.py

# Plain output for CI / logs (exit 1 when leaks are found)
finops-guard --pricing=pricing.json --scan=examples/ingest_pipeline.py --no-tui
```

```text
🚨 1 issue(s) found:
  [FG-001] examples/ingest_pipeline.py:9 (openai, severity=HIGH)
      response = openai.chat.completions.create(
```

---

## ✨ Features

<table>
<tr>
<td width="50%" valign="top">

### 🔍 Static Loop Detector
Scans Python & TypeScript for **7 providers** (OpenAI, Anthropic, Bedrock, Vertex AI, Athena, DynamoDB, Pinecone) plus **AI-slop patterns** — all nested in `for`/`while` loops, the shape that multiplies cost by the iteration count. Zero runtime, zero API keys.

</td>
<td width="50%" valign="top">

### 🔥 Doom-fire Cost TUI
A full-screen "Cost Reactor" dashboard: a Doom-fire burn gauge sized to `projected / budget`, dollar-rain particles, escalating status (SAFE → SWEATING → ON FIRE → INFERNO), and a rotating Virtual CFO quote.

</td>
</tr>
<tr>
<td width="50%" valign="top">

### 📊 PR Cost Card
On every pull request the GitHub Action posts a single animated SVG card — burn gauge, the flagged leak, projected monthly impact, and a committable **Commit suggestion** on the exact line.

</td>
<td width="50%" valign="top">

### 🧾 Git-Notes Ledger
Every merge is stamped with a cost delta via `git notes`, so `git log --notes=refs/notes/finops` becomes an offline spend ledger — no dashboard required.

</td>
</tr>
</table>

---

## 🎯 Detections

**The agent-slop rules (`FG-010`–`FG-012`) are the point.** Every linter catches a call in a loop. Nobody catches the shape an autonomous coding agent actually writes: a full chat history re-tokenized every turn, an unthrottled `Promise.all` fanning out a thousand calls at once, a secret hardcoded into a hot path. That's the code that ships green, passes tests, and lands the bill three weeks later.

| Rule | Catches | Severity |
|------|---------|:--------:|
| **`FG-010`** | **Full chat-history re-tokenization in a loop** — the classic agent context blow-up | **HIGH** |
| **`FG-011`** | **Unthrottled `Promise.all` fan-out** — agent parallelizes without a concurrency cap | **HIGH** |
| **`FG-012`** | **Hardcoded API key / secret in a loop** — agent inlines a credential into a hot path | **CRITICAL** |
| `FG-001` / `FG-002` | OpenAI / Anthropic call in a loop | HIGH |
| `FG-003` | Athena query in a loop | CRITICAL |
| `FG-004` | DynamoDB writes in a loop | HIGH |
| `FG-005` | AWS Bedrock call in a loop | CRITICAL |
| `FG-006` | GCP Vertex AI call in a loop | HIGH |
| `FG-007` | Pinecone / vector-DB query in a loop | HIGH |

<sub>`FG-008`/`FG-009` (recursive-agent + unthrottled-poll detection) need a real parser and are on the AST-pass roadmap — the next slice of the agent-slop wedge.</sub>

---

## ⚙️ Configuration

Drop a `.finops-guard.yml` in your repo root:

```yaml
version: "1.0"

settings:
  currency: USD
  fail_on_cost_threshold: 50.00   # exit 1 if projected waste exceeds this ($)

rules:
  - id: FG-001
    name: LLM-API-In-Loop
    severity: HIGH
    target_apis:
      - openai.chat.completions
      - anthropic.messages
```

Pricing lives in [`pricing.json`](pricing.json) (OpenAI, Anthropic, Amazon Athena, DynamoDB) — edit it to match your negotiated rates.

---

## 🤖 GitHub Action

FinOps-Guard ships a ready workflow at [`.github/workflows/finops-guard.yml`](.github/workflows/finops-guard.yml). On each PR it:

1. Scans the changed code and generates the cost card
2. Posts (and updates in place) a single PR comment
3. Attaches a committable fix suggestion on the flagged line
4. Publishes a narrative check run (`✅ the CFO sleeps soundly` … `❌ the CFO has fainted`)
5. Records the spend-ledger note on pushes to `main`

```yaml
# minimal usage
- uses: actions/checkout@v4
- uses: actions/setup-go@v4
  with: { go-version: '1.22' }
- run: make build
- run: ./bin/finops-guard --pricing=pricing.json --scan=<file> --no-tui
```

---

## 🧭 CLI reference

| Flag | Description |
|------|-------------|
| `--pricing` | Path to the pricing catalog JSON (default `pricing.json`) |
| `--scan` | File to scan for loop-bound API calls |
| `--no-tui` | Static output instead of the animated TUI (CI-friendly; exit 1 on findings) |
| `--output-svg` | Write the cost card SVG to a file |
| `--generate-pr-comment` | Emit the Markdown PR comment (needs `--output-svg`) |
| `--findings-json` | Write findings + committable suggestion text for the CI suggestion step |
| `--note` | Print a one-line spend-ledger note for `git notes`, then exit |
| `--audit` | Print the git-notes historical spend ledger and exit |
| `--theme` | TUI canvas: `tactical` (default), `matrix`, or `cosmos` |
| `--version` | Print the version and exit |

---

## 🛠️ Development

```bash
make build     # compile → bin/finops-guard
make run       # scan the sample file (plain output)
make demo      # launch the animated TUI on the sample file
make test      # go test ./...
make demo-gif  # record demo.tape → assets/demo.gif (needs vhs)
make ledger    # record + show the git-notes spend ledger
```

---

## 📄 License

MIT — see [LICENSE](LICENSE).
