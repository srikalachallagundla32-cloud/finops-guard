package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustContain(t *testing.T, haystack, needle, what string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %s (%q)", what, needle)
	}
}

func readAll(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestGenerateCardSVG_Structure asserts the live card SVG is structurally valid
// and swaps status text/colors between the safe and budget-breached states.
func TestGenerateCardSVG_Structure(t *testing.T) {
	dir := t.TempDir()

	// --- Safe state: nothing to burn ---
	safePath := filepath.Join(dir, "safe.svg")
	if err := GenerateCardSVG(SVGBurnMeter{TotalRisk: 0, Threshold: 50, IssueCount: 0}, nil, 0, safePath); err != nil {
		t.Fatalf("GenerateCardSVG(safe): %v", err)
	}
	safe := readAll(t, safePath)
	mustContain(t, safe, "<svg", "opening svg tag")
	mustContain(t, safe, "</svg>", "closing svg tag")
	mustContain(t, safe, "SAFE", "safe status word")
	mustContain(t, safe, "#3fb950", "green status color")

	// --- Breached state: cost far over budget, one critical finding ---
	breachPath := filepath.Join(dir, "breach.svg")
	issues := []Issue{{
		ID: "FG-005", FilePath: "x.py", LineNumber: 9,
		RuleName: "AWS Bedrock API Call in Loop", EstCostRisk: 0.015,
		TargetAPI: "bedrock", Severity: "CRITICAL",
		CodeSnippet: "response = bedrock.invoke_model(body=item)",
	}}
	if err := GenerateCardSVG(SVGBurnMeter{TotalRisk: 150, Threshold: 50, IssueCount: 1}, issues, 1, breachPath); err != nil {
		t.Fatalf("GenerateCardSVG(breach): %v", err)
	}
	breach := readAll(t, breachPath)
	mustContain(t, breach, "<svg", "opening svg tag")
	mustContain(t, breach, "</svg>", "closing svg tag")
	mustContain(t, breach, "BUDGET EXCEEDED", "breach decision text")
	mustContain(t, breach, "#f85149", "red status color")
	mustContain(t, breach, "WANTED", "issue panel present")
}
