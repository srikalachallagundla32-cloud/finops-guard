package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// SVGBurnMeter holds data for SVG generation
type SVGBurnMeter struct {
	TotalRisk  float64
	Threshold  float64
	IssueCount int
}

// GenerateBurnSVG creates a gauge-style SVG meter (like a dashboard gauge)
func GenerateBurnSVG(meter SVGBurnMeter, outputPath string) error {
	ratio := meter.TotalRisk / meter.Threshold
	if ratio > 1.0 {
		ratio = 1.0
	}

	// Gauge colors: green → yellow → red → black
	gaugeColor := "#5DCAA5" // mint (safe)
	gaugeLabel := "SAFE"
	if ratio > 0.9 {
		gaugeColor = "#1a0a00" // black (inferno)
		gaugeLabel = "INFERNO"
	} else if ratio > 0.75 {
		gaugeColor = "#F09595" // red (on fire)
		gaugeLabel = "ON FIRE"
	} else if ratio > 0.6 {
		gaugeColor = "#ED93B1" // pink (sweating)
		gaugeLabel = "SWEATING"
	} else if ratio > 0.25 {
		gaugeColor = "#FAC775" // butter (warning)
		gaugeLabel = "CAUTION"
	}

	// Calculate needle angle (0-180 degrees across the gauge arc)
	needleAngle := -90 + (ratio * 180)

	// SVG gauge meter (asymmetric, designed to feel intentional)
	svg := fmt.Sprintf(`<svg viewBox="0 0 320 200" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <style>
      @keyframes needleShake {
        0%% { transform: rotate(%.2fdeg); }
        50%% { transform: rotate(%.2fdeg); }
        100%% { transform: rotate(%.2fdeg); }
      }
      @keyframes pulseGauge {
        0%%, 100%% { opacity: 0.9; }
        50%% { opacity: 1.0; }
      }
      .gauge-bg { fill: #0d0713; }
      .gauge-text { font-family: 'Courier New', monospace; font-size: 11px; fill: #AFA9EC; }
      .gauge-value { font-family: 'Courier New', monospace; font-size: 18px; font-weight: bold; fill: %s; }
      .gauge-label { font-family: 'Courier New', monospace; font-size: 13px; font-weight: bold; fill: %s; letter-spacing: 1px; }
      .needle { animation: needleShake 0.15s infinite; transform-origin: 160px 140px; }
      .arc { animation: pulseGauge 1.5s ease-in-out infinite; }
    </style>
  </defs>

  <!-- Background -->
  <rect width="320" height="200" class="gauge-bg" />

  <!-- Gauge Arc (semicircle) -->
  <defs>
    <linearGradient id="gaugeGrad" x1="50" y1="140" x2="270" y2="140">
      <stop offset="0%%" style="stop-color:#5DCAA5;stop-opacity:0.3" />
      <stop offset="50%%" style="stop-color:#FAC775;stop-opacity:0.6" />
      <stop offset="100%%" style="stop-color:#F09595;stop-opacity:1" />
    </linearGradient>
  </defs>

  <!-- Arc background (full gauge outline) -->
  <path d="M 50 140 A 110 110 0 0 1 270 140" stroke="#2a1a35" stroke-width="8" fill="none" />

  <!-- Active arc (shows current cost) -->
  <path d="M 50 140 A 110 110 0 0 1 %(arc_end).0f %(arc_y).0f" stroke="%s" stroke-width="8" fill="none" class="arc" stroke-linecap="round" />

  <!-- Needle -->
  <g class="needle">
    <line x1="160" y1="140" x2="160" y2="40" stroke="%s" stroke-width="2" />
    <circle cx="160" cy="140" r="3" fill="%s" />
  </g>

  <!-- Tick marks (0%%, 50%%, 100%%) -->
  <line x1="50" y1="140" x2="50" y2="150" stroke="#AFA9EC" stroke-width="1" />
  <text x="45" y="165" class="gauge-text" text-anchor="end">0%%</text>

  <line x1="160" y1="30" x2="160" y2="20" stroke="#AFA9EC" stroke-width="1" />
  <text x="160" y="15" class="gauge-text" text-anchor="middle">50%%</text>

  <line x1="270" y1="140" x2="270" y2="150" stroke="#AFA9EC" stroke-width="1" />
  <text x="275" y="165" class="gauge-text" text-anchor="start">100%%</text>

  <!-- Cost values (left side, asymmetric) -->
  <text x="20" y="35" class="gauge-value">$%.2f</text>
  <text x="20" y="52" class="gauge-text">/ $%.2f</text>

  <!-- Status label (right side, asymmetric) -->
  <text x="300" y="85" class="gauge-label" text-anchor="end">%s</text>
  <text x="300" y="105" class="gauge-text" text-anchor="end">%%%.0f used</text>

  <!-- Issues count (bottom, left) -->
  <text x="20" y="190" class="gauge-text">%d issues</text>
</svg>`,
		needleAngle, needleAngle, needleAngle,
		gaugeColor, gaugeColor,
		50+220*ratio, 140-110*ratio, // arc end point (semicircle)
		gaugeColor,
		gaugeColor,
		gaugeColor,
		meter.TotalRisk,
		meter.Threshold,
		gaugeLabel,
		ratio*100,
		meter.IssueCount,
	)

	if err := os.WriteFile(outputPath, []byte(svg), 0644); err != nil {
		return fmt.Errorf("failed to write SVG: %w", err)
	}

	return nil
}

// GeneratePRComment creates a Markdown PR comment with receipt (terse, left-aligned, no filler)
func GeneratePRComment(meter SVGBurnMeter, issues []Issue, svgPath string) string {
	var buf strings.Builder

	// Decision first (top, bold)
	ratio := meter.TotalRisk / meter.Threshold
	decision := "✅ SAFE TO MERGE"
	if ratio > 0.9 {
		decision = "🛑 BUDGET EXCEEDED"
	} else if ratio > 0.75 {
		decision = "⚠️  BUDGET ALERT"
	} else if ratio > 0.6 {
		decision = "⚡ APPROACHING LIMIT"
	}
	buf.WriteString(decision + "\n\n")

	// SVG reference
	buf.WriteString(fmt.Sprintf("![Burn Meter](%s)\n\n", svgPath))

	// Receipt (no decoration, just facts)
	buf.WriteString("```\n")

	if len(issues) == 0 {
		buf.WriteString("no issues\n")
	} else {
		// Sort by cost (highest first)
		sort.Slice(issues, func(i, j int) bool {
			return issues[i].EstCostRisk > issues[j].EstCostRisk
		})

		// Show top 5 findings
		limit := len(issues)
		if limit > 5 {
			limit = 5
		}

		for _, issue := range issues[:limit] {
			buf.WriteString(fmt.Sprintf("%s:%d  +$%.2f/run\n", issue.FilePath, issue.LineNumber, issue.EstCostRisk))
			buf.WriteString(fmt.Sprintf("  %s\n", issue.RuleName))
		}

		if len(issues) > 5 {
			buf.WriteString(fmt.Sprintf("\n+%d more issues\n", len(issues)-5))
		}
	}

	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("total: $%.2f / $%.2f budget\n", meter.TotalRisk, meter.Threshold))
	buf.WriteString("```\n")

	return buf.String()
}

// Issue is a minimal issue representation for PR comments
type Issue struct {
	FilePath    string
	LineNumber  int
	RuleName    string
	EstCostRisk float64
}
