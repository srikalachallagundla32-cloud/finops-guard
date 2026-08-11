package ui

import (
	"testing"
)

// TestGeneratePRComment_Structure asserts the live PR comment embeds the card
// image, links back to the flagged code, and points at the committable fix.
func TestGeneratePRComment_Structure(t *testing.T) {
	issues := []Issue{{
		ID: "FG-005", FilePath: "examples/x.py", LineNumber: 9,
		RuleName: "AWS Bedrock API Call in Loop", EstCostRisk: 0.015,
		TargetAPI: "bedrock", Severity: "CRITICAL",
	}}
	meter := SVGBurnMeter{TotalRisk: 0.015, Threshold: 50, IssueCount: 1}

	out := GeneratePRComment(meter, issues, "https://example.com/burn.svg",
		"https://github.com/o/r", "abc123")

	mustContain(t, out, CommentMarker, "hidden upsert marker")
	mustContain(t, out, `<img src="https://example.com/burn.svg"`, "embedded card image")
	mustContain(t, out, "examples/x.py", "findings reference the file")
	mustContain(t, out, "View flagged line", "real clickable link")
	mustContain(t, out, "Commit suggestion", "pointer to committable suggestion")

	// No-issues path renders the safe message and stops.
	clean := GeneratePRComment(SVGBurnMeter{TotalRisk: 0, Threshold: 50}, nil, "u", "", "")
	mustContain(t, clean, "No loop-bound API calls detected", "clean-state text")
}
