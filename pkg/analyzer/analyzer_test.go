package analyzer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/your-username/finops-guard/pkg/analyzer"
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
