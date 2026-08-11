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
