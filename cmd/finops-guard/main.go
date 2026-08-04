package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/your-username/finops-guard/pkg/analyzer"
	"github.com/your-username/finops-guard/pkg/costengine"
)

func main() {
	pricingPath := flag.String("pricing", "pricing.json", "Path to the pricing catalog JSON file")
	scanPath := flag.String("scan", "", "Path to a Python/TypeScript file to scan for loop-bound API calls")
	flag.Parse()

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

	fmt.Printf("\n🔍 Scanning %s for loop-bound API calls...\n", *scanPath)
	issues, err := analyzer.ScanFile(*scanPath, analyzer.GetDefaultRules())
	if err != nil {
		fmt.Printf("❌ [Error] Scanning file: %v\n", err)
		os.Exit(1)
	}

	if len(issues) == 0 {
		fmt.Println("✅ No loop-bound API risks detected.")
		return
	}

	fmt.Printf("🚨 %d issue(s) found:\n", len(issues))
	for _, issue := range issues {
		fmt.Printf("  [%s] %s:%d (%s, severity=%s)\n      %s\n",
			issue.ID, issue.FilePath, issue.LineNumber, issue.TargetAPI, issue.Severity, issue.CodeSnippet)
	}
	os.Exit(1)
}
