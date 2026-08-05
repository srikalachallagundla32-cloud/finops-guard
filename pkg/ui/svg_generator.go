package ui

import (
	"fmt"
	"math"
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

	// Gauge geometry: semicircle, center (cx,cy), radius R. 0% = left, 50% = top,
	// 100% = right. theta sweeps 0..π. All positions computed with real trig so
	// the render never depends on CSS transforms (GitHub strips those in comments).
	const cx, cy, radius, needleLen = 160.0, 140.0, 110.0, 96.0
	theta := math.Pi * ratio
	arcEndX := cx - radius*math.Cos(theta)
	arcEndY := cy - radius*math.Sin(theta)
	tipX := cx - needleLen*math.Cos(theta)
	tipY := cy - needleLen*math.Sin(theta)

	issuesText := fmt.Sprintf("%d issues", meter.IssueCount)
	if meter.IssueCount == 1 {
		issuesText = "1 issue"
	}

	svg := fmt.Sprintf(`<svg viewBox="0 0 320 200" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <style>
      @keyframes pulseGauge { 0%%,100%% { opacity: 0.85; } 50%% { opacity: 1; } }
      .gauge-text  { font-family: 'Courier New', monospace; font-size: 11px; fill: #AFA9EC; }
      .gauge-value { font-family: 'Courier New', monospace; font-size: 20px; font-weight: bold; fill: %s; }
      .gauge-label { font-family: 'Courier New', monospace; font-size: 13px; font-weight: bold; fill: %s; letter-spacing: 1px; }
      .arc         { animation: pulseGauge 1.6s ease-in-out infinite; }
    </style>
  </defs>

  <rect width="320" height="200" fill="#0d0713" />

  <!-- Track -->
  <path d="M 50 140 A 110 110 0 0 1 270 140" stroke="#2a1a35" stroke-width="9" fill="none" stroke-linecap="round" />

  <!-- Active arc (proportional to cost/budget) -->
  <path d="M 50 140 A 110 110 0 0 1 %.2f %.2f" stroke="%s" stroke-width="9" fill="none" class="arc" stroke-linecap="round" />

  <!-- Needle (static coords — no CSS transform) -->
  <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="2.5" stroke-linecap="round" />
  <circle cx="160" cy="140" r="5" fill="#0d0713" stroke="%s" stroke-width="2" />

  <!-- Tick labels -->
  <text x="46" y="162" class="gauge-text" text-anchor="end">0%%</text>
  <text x="160" y="18" class="gauge-text" text-anchor="middle">50%%</text>
  <text x="274" y="162" class="gauge-text" text-anchor="start">100%%</text>

  <!-- Cost (left) -->
  <text x="20" y="40" class="gauge-value">$%.2f</text>
  <text x="20" y="58" class="gauge-text">/ $%.2f</text>

  <!-- Status (right) -->
  <text x="300" y="90" class="gauge-label" text-anchor="end">%s</text>
  <text x="300" y="110" class="gauge-text" text-anchor="end">%.0f%% used</text>

  <!-- Issues (bottom-left) -->
  <text x="20" y="188" class="gauge-text">%s</text>
</svg>`,
		gaugeColor, gaugeColor,
		arcEndX, arcEndY, gaugeColor,
		tipX, tipY, cx, cy, gaugeColor, // needle line uses cx,cy as source; wait order below
		gaugeColor,
		meter.TotalRisk,
		meter.Threshold,
		gaugeLabel,
		ratio*100,
		issuesText,
	)

	if err := os.WriteFile(outputPath, []byte(svg), 0644); err != nil {
		return fmt.Errorf("failed to write SVG: %w", err)
	}

	return nil
}

// CommentMarker is a hidden HTML comment used to find-and-update the bot's
// single PR comment instead of posting a new one each run.
const CommentMarker = "<!-- finops-guard-comment -->"

// GeneratePRComment renders a clean, scannable PR comment: a factual headline,
// the gauge image, a findings table, and an at-scale projection. The playful
// "bounty" framing stays as one small accent — never as leaked-log clutter.
func GeneratePRComment(meter SVGBurnMeter, issues []Issue, svgPath string) string {
	var buf strings.Builder
	ratio := meter.TotalRisk / meter.Threshold
	pct := ratio * 100

	// Verdict word + factual headline (informative, not a glib stamp).
	verdict := "within budget"
	if ratio > 0.9 {
		verdict = "over budget"
	} else if ratio > 0.75 {
		verdict = "near budget limit"
	} else if ratio > 0.6 {
		verdict = "approaching budget"
	}

	buf.WriteString(CommentMarker + "\n")
	buf.WriteString("### 🛡️ finops-guard — cost check\n\n")
	buf.WriteString(fmt.Sprintf("**This change adds ~$%.2f/run** — %.0f%% of the $%.2f budget (%s).\n\n",
		meter.TotalRisk, pct, meter.Threshold, verdict))

	buf.WriteString(fmt.Sprintf("<img src=\"%s\" width=\"420\" alt=\"cost burn meter\" />\n\n", svgPath))

	if len(issues) == 0 {
		buf.WriteString("No loop-bound API calls detected. Nothing to worry about here. ✅\n")
		return buf.String()
	}

	// Sort by cost, highest first.
	sort.Slice(issues, func(i, j int) bool { return issues[i].EstCostRisk > issues[j].EstCostRisk })

	// Findings table — the clear, scannable core.
	buf.WriteString("| Location | Per run | At scale¹ | Pattern |\n")
	buf.WriteString("|---|--:|--:|---|\n")
	for _, issue := range issues {
		monthly := issue.EstCostRisk * 1000 * 30 // 1,000 iterations × 30 runs/mo
		buf.WriteString(fmt.Sprintf("| `%s:%d` | +$%.2f | ~$%s/mo | %s |\n",
			issue.FilePath, issue.LineNumber, issue.EstCostRisk, humanMoney(monthly), issue.RuleName))
	}
	buf.WriteString("\n<sub>¹ if this call runs ~1,000 iterations, 30 times/month.</sub>\n\n")

	// One tasteful accent + a pointer to the committable suggestion below.
	top := issues[0]
	topMonthly := top.EstCostRisk * 1000 * 30
	buf.WriteString(fmt.Sprintf("> 💸 Left unfixed, the top leak (`%s:%d`) bleeds about **$%s/mo** at scale.\n",
		top.FilePath, top.LineNumber, humanMoney(topMonthly)))
	buf.WriteString("> A one-click **Commit suggestion** to flag it is attached as a review comment on that line.\n")

	return buf.String()
}

// humanMoney formats a dollar amount with thousands separators and no cents
// once it's large enough that cents are noise (e.g. 225000 -> "225,000").
func humanMoney(v float64) string {
	n := int64(v + 0.5)
	s := fmt.Sprintf("%d", n)
	// Insert commas.
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

// Issue is a minimal issue representation for PR comments
type Issue struct {
	FilePath    string
	LineNumber  int
	RuleName    string
	EstCostRisk float64
}
