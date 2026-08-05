package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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
	flag.Parse()

	_, ok := ui.ByName(*themeName)
	if !ok {
		fmt.Printf("❌ [Error] Unknown theme %q. Valid options: %s\n", *themeName, strings.Join(ui.ThemeNames(), ", "))
		os.Exit(1)
	}

	fmt.Println("🛡️  FinOps-Guard CLI v0.1.0 — Static Cost Analysis Engine")
	fmt.Println("---------------------------------------------------------")

	catalog, err := costengine.LoadPricingCatalog(*pricingPath)
	if err != nil {
		fmt.Printf("❌ [Error] Loading pricing catalog: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Loaded Pricing Catalog (Currency: %s, Region: %s)\n", catalog.Currency, catalog.RegionDefault)

	iterations := 1000
	estimatedCost := catalog.CalculateLLMLoopCost("gpt-4o", 1000, 500, iterations)

	fmt.Printf("\n[Simulation] 1 loop detected executing 'gpt-4o' over %d iterations:\n", iterations)
	fmt.Printf("💰 Projected Cost Risk: $%.2f USD\n", estimatedCost)

	if *scanPath == "" {
		return
	}

	issues, err := analyzer.ScanFile(*scanPath, analyzer.GetDefaultRules())
	if err != nil {
		fmt.Printf("❌ [Error] Scanning file: %v\n", err)
		os.Exit(1)
	}

	var totalRisk float64
	for i := range issues {
		issues[i].EstCostRisk = catalog.EstimateCallCost(issues[i].TargetAPI)
		totalRisk += issues[i].EstCostRisk
	}

	costFailThreshold := config.FailOnCostThreshold(guardConfigPath)

	// Generate SVG if requested
	if *outputSVG != "" {
		meter := ui.SVGBurnMeter{
			TotalRisk:  totalRisk,
			Threshold:  costFailThreshold,
			IssueCount: len(issues),
		}
		if err := ui.GenerateBurnSVG(meter, *outputSVG); err != nil {
			fmt.Printf("❌ [Error] Generating SVG: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Generated SVG burn meter: %s\n", *outputSVG)

		// Generate PR comment if requested
		if *generatePRComment {
			// Convert issues to ui.Issue for comment generation
			commentIssues := make([]ui.Issue, len(issues))
			for i, issue := range issues {
				commentIssues[i] = ui.Issue{
					FilePath:    issue.FilePath,
					LineNumber:  issue.LineNumber,
					RuleName:    issue.RuleName,
					EstCostRisk: issue.EstCostRisk,
				}
			}
			prComment := ui.GeneratePRComment(meter, commentIssues, "./"+*outputSVG)
			fmt.Println("\n" + prComment)
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
