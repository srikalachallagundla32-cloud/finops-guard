package analyzer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/srikalachallagundla32-cloud/finops-guard/pkg/analyzer"
)

func TestScanFile_LoopDetection(t *testing.T) {
	rules := analyzer.GetDefaultRules()
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		filename      string
		content       string
		expectedCount int
		expectedRule  string
	}{
		{
			name:     "Python OpenAI call inside for loop (Violates Guardrail)",
			filename: "test_slop.py",
			content: `import openai

def process_items(items):
    for item in items:
        response = openai.chat.completions.create(model="gpt-4o", messages=[{"role": "user", "content": item}])
        print(response)
`,
			expectedCount: 1,
			expectedRule:  "FG-001",
		},
		{
			name:     "Python safe top-level call (No Violation)",
			filename: "test_safe.py",
			content: `import openai

def single_request():
    return openai.chat.completions.create(model="gpt-4o", messages=[{"role": "user", "content": "hello"}])
`,
			expectedCount: 0,
			expectedRule:  "",
		},
		{
			name:     "TypeScript AWS Athena query inside forEach (Violates Guardrail)",
			filename: "test_athena.ts",
			content: `import { AthenaClient } from "@aws-sdk/client-athena";

function queryLogs(ids: string[]) {
    ids.forEach(id => {
        athena.start_query_execution({ QueryString: "SELECT * FROM logs" });
    });
}
`,
			expectedCount: 1,
			expectedRule:  "FG-003",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			issues, err := analyzer.ScanFile(filePath, rules)
			if err != nil {
				t.Fatalf("unexpected error scanning file: %v", err)
			}

			if len(issues) != tt.expectedCount {
				t.Errorf("ScanFile() found %d issues; want %d", len(issues), tt.expectedCount)
			}

			if tt.expectedCount > 0 && len(issues) > 0 {
				if issues[0].ID != tt.expectedRule {
					t.Errorf("ScanFile() issue rule ID = %s; want %s", issues[0].ID, tt.expectedRule)
				}
			}
		})
	}
}

func TestPreviewFixAndApplyFix(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "loop.py")
	original := `import openai

def process_items(items):
    for item in items:
        response = openai.chat.completions.create(model="gpt-4o", messages=[{"role": "user", "content": item}])
        print(response)
`
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	issues, err := analyzer.ScanFile(filePath, analyzer.GetDefaultRules())
	if err != nil {
		t.Fatalf("unexpected error scanning file: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	issue := issues[0]

	before, after, err := analyzer.PreviewFix(issue)
	if err != nil {
		t.Fatalf("PreviewFix() unexpected error: %v", err)
	}
	if !strings.Contains(before, "openai.chat.completions.create") {
		t.Errorf("PreviewFix() before snippet missing flagged line: %q", before)
	}
	if strings.Contains(before, "FINOPS-GUARD") {
		t.Errorf("PreviewFix() before snippet should not contain the remediation comment: %q", before)
	}
	if !strings.Contains(after, "FINOPS-GUARD") {
		t.Errorf("PreviewFix() after snippet should contain the remediation comment: %q", after)
	}

	// PreviewFix must not touch the file on disk.
	unchanged, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to re-read file: %v", err)
	}
	if string(unchanged) != original {
		t.Errorf("PreviewFix() modified the file on disk, want it untouched")
	}

	if err := analyzer.ApplyFix(issue); err != nil {
		t.Fatalf("ApplyFix() unexpected error: %v", err)
	}

	fixed, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read fixed file: %v", err)
	}

	lines := strings.Split(string(fixed), "\n")
	if len(lines) != strings.Count(original, "\n")+1+1 {
		t.Fatalf("ApplyFix() expected exactly one inserted line, got %d lines: %q", len(lines), string(fixed))
	}
	if !strings.Contains(lines[issue.LineNumber-1], "FINOPS-GUARD") {
		t.Errorf("ApplyFix() expected the remediation comment at line %d, got %q", issue.LineNumber, lines[issue.LineNumber-1])
	}
	if !strings.Contains(lines[issue.LineNumber], "openai.chat.completions.create") {
		t.Errorf("ApplyFix() expected the original flagged line right after the comment, got %q", lines[issue.LineNumber])
	}
}

// TestScanFile_NewProviderRules covers the Phase 2 rules FG-005 (AWS Bedrock),
// FG-006 (GCP Vertex AI), and FG-007 (Pinecone / vector DB) across Python and
// TypeScript/JavaScript, plus a negative (non-loop) case.
func TestScanFile_NewProviderRules(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		code         string
		wantRuleID   string
		wantIssueCnt int
	}{
		{
			name: "Bedrock invoke_model inside Python for loop",
			file: "test.py",
			code: `
for item in dataset:
    response = bedrock.invoke_model(modelId='anthropic.claude-3', body=item)
`,
			wantRuleID:   "FG-005",
			wantIssueCnt: 1,
		},
		{
			name: "Bedrock Converse command inside TS forEach",
			file: "test.ts",
			code: `
ids.forEach((id) => {
  client.send(new ConverseCommand({ modelId: "anthropic.claude-3" }));
});
`,
			wantRuleID:   "FG-005",
			wantIssueCnt: 1,
		},
		{
			name: "Vertex AI generate_content inside Python for loop",
			file: "test.py",
			code: `
for prompt in prompts:
    result = model.generate_content(prompt)
`,
			wantRuleID:   "FG-006",
			wantIssueCnt: 1,
		},
		{
			name: "Vertex AI generateContent inside TS forEach",
			file: "test.ts",
			code: `
prompts.forEach((p) => {
  const r = generativeModel.generateContent(p);
});
`,
			wantRuleID:   "FG-006",
			wantIssueCnt: 1,
		},
		{
			name: "Pinecone query inside Python for loop",
			file: "test.py",
			code: `
for vec in embeddings:
    res = index.query(vector=vec, top_k=5)
`,
			wantRuleID:   "FG-007",
			wantIssueCnt: 1,
		},
		{
			name: "Vector upsert inside TS forEach",
			file: "test.ts",
			code: `
vectors.forEach((v) => {
  index.upsert([v]);
});
`,
			wantRuleID:   "FG-007",
			wantIssueCnt: 1,
		},
		{
			name: "Bedrock call OUTSIDE a loop is not flagged",
			file: "test.py",
			code: `
def one_shot(item):
    return bedrock.invoke_model(modelId='anthropic.claude-3', body=item)
`,
			wantRuleID:   "",
			wantIssueCnt: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := analyzer.ScanCodeSnippet(tt.code, tt.file)
			if len(issues) != tt.wantIssueCnt {
				t.Fatalf("ScanCodeSnippet() found %d issues; want %d (%+v)", len(issues), tt.wantIssueCnt, issues)
			}
			if tt.wantIssueCnt > 0 && issues[0].ID != tt.wantRuleID {
				t.Errorf("ScanCodeSnippet() rule ID = %s; want %s", issues[0].ID, tt.wantRuleID)
			}
		})
	}
}

// TestScanFile_AISlopRules covers the RE2-compliant AI-slop rules FG-010
// (history re-tokenization), FG-011 (Promise.all fan-out), and FG-012
// (hardcoded secret in a loop) across Python and TypeScript.
func TestScanFile_AISlopRules(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		code         string
		wantRuleID   string
		wantIssueCnt int
	}{
		{
			name: "FG-010 history re-tokenization (Python)",
			file: "chat.py",
			code: `
for turn in conversation:
    messages.append(turn)
    resp = client.chat.completions.create(messages=messages)
`,
			wantRuleID:   "FG-010",
			wantIssueCnt: 1,
		},
		{
			name: "FG-010 history re-tokenization (TypeScript)",
			file: "chat.ts",
			code: `
for (const turn of conversation) {
  messages.push(turn);
  const r = await client.chat.completions.create({ messages: messages });
}
`,
			wantRuleID:   "FG-010",
			wantIssueCnt: 1,
		},
		{
			name: "FG-011 Promise.all fan-out inside loop (TypeScript)",
			file: "blast.ts",
			code: `
for (const batch of batches) {
  await Promise.all(batch.map((p) => callModel(p)));
}
`,
			wantRuleID:   "FG-011",
			wantIssueCnt: 1,
		},
		{
			name: "FG-012 hardcoded OpenAI key in loop (Python)",
			file: "leak.py",
			code: `
for user in users:
    client = OpenAI(api_key="sk-proj-ABCDEFGHIJKLMNOPQRSTUVWX1234")
`,
			wantRuleID:   "FG-012",
			wantIssueCnt: 1,
		},
		{
			name: "FG-012 hardcoded AWS key in loop (TypeScript)",
			file: "leak.ts",
			code: `
for (const region of regions) {
  const cfg = { accessKeyId: "AKIAIOSFODNN7EXAMPLE" };
}
`,
			wantRuleID:   "FG-012",
			wantIssueCnt: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := analyzer.ScanCodeSnippet(tt.code, tt.file)
			if len(issues) != tt.wantIssueCnt {
				t.Fatalf("ScanCodeSnippet() found %d issues; want %d (%+v)", len(issues), tt.wantIssueCnt, issues)
			}
			if tt.wantIssueCnt > 0 && issues[0].ID != tt.wantRuleID {
				t.Errorf("ScanCodeSnippet() rule ID = %s; want %s", issues[0].ID, tt.wantRuleID)
			}
		})
	}
}

// TestScanFile_PollLoopRule covers FG-009 (unthrottled poll loop): it must fire
// on an infinite loop that polls with no delay, and stay silent when the loop
// backs off or when the poll sits in a bounded (non-infinite) loop.
func TestScanFile_PollLoopRule(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		code         string
		wantRuleID   string
		wantIssueCnt int
	}{
		{
			name: "FG-009 unthrottled while True (Python)",
			file: "poll.py",
			code: `
while True:
    resp = requests.get("https://api/jobs/1")
    if resp.json()["state"] == "done":
        return resp
`,
			wantRuleID:   "FG-009",
			wantIssueCnt: 1,
		},
		{
			name: "FG-009 throttled loop does not fire (Python)",
			file: "poll_ok.py",
			code: `
while True:
    resp = requests.get("https://api/jobs/1")
    if resp.json()["state"] == "done":
        return resp
    time.sleep(5)
`,
			wantIssueCnt: 0,
		},
		{
			name: "FG-009 unthrottled while(true) (TypeScript)",
			file: "poll.ts",
			code: `
while (true) {
  const r = await fetch("https://api/jobs/1");
  if (done) break;
}
`,
			wantRuleID:   "FG-009",
			wantIssueCnt: 1,
		},
		{
			name: "FG-009 backoff via setTimeout does not fire (TypeScript)",
			file: "poll_ok.ts",
			code: `
while (true) {
  const r = await fetch("https://api/jobs/1");
  if (done) break;
  await new Promise((res) => setTimeout(res, 1000));
}
`,
			wantIssueCnt: 0,
		},
		{
			name: "FG-009 comment mentioning sleep still fires (regression)",
			file: "poll_comment.py",
			code: `
while True:
    r = requests.get("https://api/jobs/1")
    # TODO: add time.sleep here later
`,
			wantRuleID:   "FG-009",
			wantIssueCnt: 1,
		},
		{
			name: "FG-009 does not fire on a bounded for-loop poll",
			file: "bounded.py",
			code: `
for i in range(10):
    r = requests.get("https://api/jobs/1")
`,
			wantIssueCnt: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := analyzer.ScanCodeSnippet(tt.code, tt.file)
			if len(issues) != tt.wantIssueCnt {
				t.Fatalf("ScanCodeSnippet() found %d issues; want %d (%+v)", len(issues), tt.wantIssueCnt, issues)
			}
			if tt.wantIssueCnt > 0 && issues[0].ID != tt.wantRuleID {
				t.Errorf("ScanCodeSnippet() rule ID = %s; want %s", issues[0].ID, tt.wantRuleID)
			}
			if tt.wantIssueCnt > 0 && issues[0].TargetAPI != "poll" {
				t.Errorf("ScanCodeSnippet() TargetAPI = %s; want poll", issues[0].TargetAPI)
			}
		})
	}
}

// TestMerge covers the M0 AST-pass seam: two issue sets combine into a stable,
// de-duplicated slice keyed by (ID, FilePath, LineNumber).
func TestMerge(t *testing.T) {
	a := []analyzer.Issue{
		{ID: "FG-001", FilePath: "x.py", LineNumber: 3},
		{ID: "FG-009", FilePath: "x.py", LineNumber: 8},
	}
	b := []analyzer.Issue{
		{ID: "FG-009", FilePath: "x.py", LineNumber: 8}, // duplicate of a[1]
		{ID: "FG-008", FilePath: "x.py", LineNumber: 12},
	}
	got := analyzer.Merge(a, b)
	if len(got) != 3 {
		t.Fatalf("Merge() = %d issues; want 3 (%+v)", len(got), got)
	}
	// order preserved, first occurrence wins
	wantOrder := []string{"FG-001", "FG-009", "FG-008"}
	for i, id := range wantOrder {
		if got[i].ID != id {
			t.Errorf("Merge()[%d].ID = %s; want %s", i, got[i].ID, id)
		}
	}
}
