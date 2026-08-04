package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/your-username/finops-guard/pkg/analyzer"
	"github.com/your-username/finops-guard/pkg/config"
	"github.com/your-username/finops-guard/pkg/costengine"
	"github.com/your-username/finops-guard/pkg/ui"
)

const guardConfigPath = ".finops-guard.yml"

func main() {
	pricingPath := flag.String("pricing", "pricing.json", "Path to the pricing catalog JSON file")
	scanPath := flag.String("scan", "", "Path to a Python/TypeScript file to scan for loop-bound API calls")
	themeName := flag.String("theme", "tactical", "Cockpit HUD theme: "+strings.Join(ui.ThemeNames(), ", "))
	flag.Parse()

	theme, ok := ui.ByName(*themeName)
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

	if err := ui.RunFinalTUI(issues, totalRisk, costFailThreshold, theme); err != nil {
		fmt.Printf("❌ [Error] Running interactive TUI: %v\n", err)
		os.Exit(1)
	}

	if len(issues) > 0 {
		os.Exit(1)
	}
}
