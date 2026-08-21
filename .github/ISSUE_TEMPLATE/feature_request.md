---
name: Feature request / new rule
about: Propose a new detection (FG-###) or a capability
title: "[feat] "
labels: enhancement
---

**The cost leak you want caught**
<!-- What shape of code bills you? Paste an example. Agent-written slop especially welcome. -->

```python
# example of the pattern
```

**Why regex/line-scan can or can't express it**
<!-- Does it need a call graph, cross-function analysis, or "absence across a body"?
     If so it likely belongs to the AST pass — see docs/design/ast-pass.md. -->

**Proposed severity + rough per-call cost**
