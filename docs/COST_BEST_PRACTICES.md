# Cost Best Practices

Guidance for the cost patterns FinOps-Guard flags. These are recommendations, not
auto-applied refactors — you decide what fits your code.

## Loop-bound API calls (LLM / OpenAI / Anthropic)

An API call inside a loop multiplies cost by the iteration count and often trips
rate limits.

- **Batch** inputs into a single request where the API supports it.
- **Cache** responses keyed by input so repeats are free.
- **Summarize/aggregate** multiple items in one call instead of one call per item.
- Add **rate limiting** and **per-run cost monitoring** so a runaway loop is caught early.

## Athena / warehouse queries in a loop

Bytes scanned drive the bill.

- **Partition** and **compress** data to cut bytes scanned.
- Add a `LIMIT` / `WHERE` filter before scanning full tables.
- **Cache** query results for repeated reads.

## DynamoDB per-item calls in a loop

Per-item request units add up fast at scale.

- Use `BatchWriteItem` / `BatchGetItem` instead of per-item calls.
- Use **provisioned capacity** for predictable workloads.
- **Cache** hot reads to avoid repeated request units.

## General

- Move expensive calls **outside** loops whenever the result doesn't depend on the iteration.
- Treat cost as a first-class signal in review — that's what the gauge and the
  spend-ledger (`git log --notes=refs/notes/finops`) are for.
