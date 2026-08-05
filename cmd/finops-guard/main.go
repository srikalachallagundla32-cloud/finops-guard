package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/your-username/finops-guard/internal/tui"
	"github.com/your-username/finops-guard/pkg/analyzer"
	"github.com/your-username/finops-guard/pkg/config"
	"github.com/your-username/finops-guard/pkg/costengine"
	"github.com/your-username/finops-guard/pkg/ui"
)

const guardConfigPath = ".finops-guard.yml"

func main() {
	pricingPath := flag.String("pricing", "pricing.json", "Path to the pricing catalog JSON file")
	scanPath := flag.String("scan", "", "Path to a Python/TypeScript file to scan for loop-bound API calls")
	themeName := flag.String("theme", "tactical", "UI theme for legacy --no-tui mode: "+strings.Join(ui.ThemeNames(), ", "))
	noTUI := flag.Bool("no-tui", false, "Disable animated TUI, print static output instead")
	outputSVG := flag.String("output-svg", "", "Write animated SVG burn meter to file (e.g., burn.svg)")
	generatePRComment := flag.Bool("generate-pr-comment", false, "Generate Markdown PR comment with receipt + SVG reference (requires --output-svg)")
	svgURL := flag.String("svg-url", "", "Absolute URL for the SVG image in the PR comment (falls back to ./<output-svg> when empty)")
	findingsJSON := flag.String("findings-json", "", "Write findings (with committable suggestion text) to a JSON file for the CI suggestion step")
	emitNote := flag.Bool("note", false, "Print a one-line spend-ledger note for git notes (no other output), then exit")
	repoURL := flag.String("repo-url", "", "Repository URL (e.g. https://github.com/owner/repo) for clickable links in the PR comment")
	commitSHA := flag.String("commit-sha", "", "Commit SHA used to build the 'View flagged line' link")
	flag.Parse()

	// In --note mode, stdout must be a single clean ledger line: route all
	// human banners to stderr.
	banner := os.Stdout
	if *emitNote {
		banner = os.Stderr
	}

	_, ok := ui.ByName(*themeName)
	if !ok {
		fmt.Printf("❌ [Error] Unknown theme %q. Valid options: %s\n", *themeName, strings.Join(ui.ThemeNames(), ", "))
		os.Exit(1)
	}

	fmt.Fprintln(banner, "🛡️  FinOps-Guard CLI v0.1.0 — Static Cost Analysis Engine")
	fmt.Fprintln(banner, "---------------------------------------------------------")

	catalog, err := costengine.LoadPricingCatalog(*pricingPath)
	if err != nil {
		fmt.Printf("❌ [Error] Loading pricing catalog: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(banner, "✅ Loaded Pricing Catalog (Currency: %s, Region: %s)\n", catalog.Currency, catalog.RegionDefault)

	iterations := 1000
	estimatedCost := catalog.CalculateLLMLoopCost("gpt-4o", 1000, 500, iterations)

	fmt.Fprintf(banner, "\n[Simulation] 1 loop detected executing 'gpt-4o' over %d iterations:\n", iterations)
	fmt.Fprintf(banner, "💰 Projected Cost Risk: $%.2f USD\n", estimatedCost)

	if *scanPath == "" {
		return
	}

	scanStart := time.Now()
	issues, err := analyzer.ScanFile(*scanPath, analyzer.GetDefaultRules())
	if err != nil {
		fmt.Printf("❌ [Error] Scanning file: %v\n", err)
		os.Exit(1)
	}
	analysisSeconds := time.Since(scanStart).Seconds()

	var totalRisk float64
	for i := range issues {
		issues[i].EstCostRisk = catalog.EstimateCallCost(issues[i].TargetAPI)
		totalRisk += issues[i].EstCostRisk
	}

	costFailThreshold := config.FailOnCostThreshold(guardConfigPath)

	// --note: emit a single-line spend-ledger entry for `git notes` and exit.
	// Format: finops: +$<perRun>/run · ~$<monthly>/mo · <n> issue(s)
	if *emitNote {
		monthly := totalRisk * 1000 * 30
		fmt.Printf("finops: +$%.2f/run · ~$%.0f/mo · %d issue(s)\n", totalRisk, monthly, len(issues))
		return
	}

	// Generate SVG if requested
	if *outputSVG != "" {
		meter := ui.SVGBurnMeter{
			TotalRisk:  totalRisk,
			Threshold:  costFailThreshold,
			IssueCount: len(issues),
		}

		// Convert issues to the richer ui.Issue used by the card + comment.
		commentIssues := make([]ui.Issue, len(issues))
		for i, issue := range issues {
			commentIssues[i] = ui.Issue{
				ID:          issue.ID,
				FilePath:    issue.FilePath,
				LineNumber:  issue.LineNumber,
				RuleName:    issue.RuleName,
				EstCostRisk: issue.EstCostRisk,
				TargetAPI:   issue.TargetAPI,
				Severity:    issue.Severity,
				CodeSnippet: issue.CodeSnippet,
			}
		}

		if err := ui.GenerateCardSVG(meter, commentIssues, analysisSeconds, *outputSVG); err != nil {
			fmt.Fprintf(os.Stderr, "❌ [Error] Generating SVG: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "✅ Generated FinOps card: %s\n", *outputSVG)

		// Generate PR comment if requested
		if *generatePRComment {
			imgSrc := "./" + *outputSVG
			if *svgURL != "" {
				imgSrc = *svgURL
			}
			prComment := ui.GeneratePRComment(meter, commentIssues, imgSrc, *repoURL, *commitSHA)
			fmt.Println(prComment)
		}

		// Emit findings + committable suggestion text for the CI suggestion step.
		if *findingsJSON != "" {
			type finding struct {
				ID         string  `json:"id"`
				File       string  `json:"file"`
				Line       int     `json:"line"`
				TargetAPI  string  `json:"target_api"`
				CostPerRun float64 `json:"cost_per_run"`
				Suggestion string  `json:"suggestion"`
			}
			out := make([]finding, 0, len(issues))
			for _, iss := range issues {
				repl, ferr := analyzer.SuggestionBlock(iss)
				if ferr != nil {
					continue // line out of range / unreadable — skip rather than emit a broken suggestion
				}
				out = append(out, finding{iss.ID, iss.FilePath, iss.LineNumber, iss.TargetAPI, iss.EstCostRisk, repl})
			}
			data, _ := json.MarshalIndent(out, "", "  ")
			if werr := os.WriteFile(*findingsJSON, data, 0644); werr != nil {
				fmt.Fprintf(os.Stderr, "❌ [Error] Writing findings JSON: %v\n", werr)
			}
		}

		// When generating a PR comment, stdout is the comment payload only —
		// do not fall through to the legacy scan banner.
		if *generatePRComment {
			return
		}
	}

	if *noTUI {
		// Legacy static output
		fmt.Printf("\n🔍 Scanning %s for loop-bound API calls...\n", *scanPath)
		if len(issues) == 0 {
			fmt.Println("✅ No loop-bound API risks detected.")
		} else {
			fmt.Printf("🚨 %d issue(s) found:\n", len(issues))
			for _, issue := range issues {
				fmt.Printf("  [%s] %s:%d (%s, severity=%s)\n      %s\n",
					issue.ID, issue.FilePath, issue.LineNumber, issue.TargetAPI, issue.Severity, issue.CodeSnippet)
			}
		}
		if len(issues) > 0 {
			os.Exit(1)
		}
		return
	}

	// Animated Cost Reactor TUI (default)
	if err := tui.Run(issues, totalRisk, costFailThreshold); err != nil {
		fmt.Printf("❌ [Error] Running Cost Reactor: %v\n", err)
		os.Exit(1)
	}

	if len(issues) > 0 {
		os.Exit(1)
	}
}
