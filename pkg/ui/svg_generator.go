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

// GeneratePRComment renders the PR comment: the full FinOps-Guard card as a
// single embedded image (the hero), plus a compact, accessible text fallback
// and a pointer to the committable fix suggestion. GitHub strips HTML/CSS
// layout, so all the rich visual structure lives in the card image; the
// markdown here stays minimal and copy-friendly.
func GeneratePRComment(meter SVGBurnMeter, issues []Issue, svgPath, repoURL, commitSHA string) string {
	var buf strings.Builder

	buf.WriteString(CommentMarker + "\n")
	buf.WriteString("**FinOps analysis complete for this Pull Request** 🚀\n\n")

	sort.Slice(issues, func(i, j int) bool { return issues[i].EstCostRisk > issues[j].EstCostRisk })

	// Build the file link up front so the whole card can point at the flagged code.
	ref := commitSHA
	if ref == "" {
		ref = "HEAD"
	}
	var fileLink string
	if repoURL != "" && len(issues) > 0 {
		fileLink = fmt.Sprintf("%s/blob/%s/%s#L%d", repoURL, ref, strings.TrimPrefix(issues[0].FilePath, "./"), issues[0].LineNumber)
	}

	img := fmt.Sprintf("<img src=\"%s\" width=\"860\" alt=\"FinOps-Guard cost analysis card\" />", svgPath)
	if fileLink != "" {
		// Make the whole card clickable → opens the flagged file (the intuitive
		// action; painted buttons inside the image are never clickable).
		buf.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a>\n\n", fileLink, img))
	} else {
		buf.WriteString(img + "\n\n")
	}

	if len(issues) == 0 {
		buf.WriteString("_No loop-bound API calls detected — safe to merge._\n")
		return buf.String()
	}

	top := issues[0]

	// Real, clickable links row (these work — unlike the painted ↗ in the image).
	if repoURL != "" {
		bestPractices := repoURL + "/blob/main/docs/COST_BEST_PRACTICES.md"
		buf.WriteString(fmt.Sprintf("**📄 [View flagged line](%s)** · 📚 [Cost best practices](%s) · 📖 [Repo](%s)\n\n",
			fileLink, bestPractices, repoURL))
	}

	// Accessible / copy-friendly text fallback (the image isn't selectable).
	buf.WriteString("<details><summary>Text summary (accessibility)</summary>\n\n")
	buf.WriteString("| Location | Per run | At scale¹ | Pattern |\n")
	buf.WriteString("|---|--:|--:|---|\n")
	for _, issue := range issues {
		monthly := issue.EstCostRisk * 1000 * 30 // 1,000 iterations × 30 runs/mo
		buf.WriteString(fmt.Sprintf("| `%s:%d` | +$%.2f | ~$%s/mo | %s |\n",
			issue.FilePath, issue.LineNumber, issue.EstCostRisk, humanMoney(monthly), issue.RuleName))
	}
	buf.WriteString("\n<sub>¹ if this call runs ~1,000 iterations, 30 times/month.</sub>\n")
	buf.WriteString("</details>\n\n")

	buf.WriteString(fmt.Sprintf("🔧 A one-click **Commit suggestion** to flag `%s:%d` is attached as a review comment on that line (visible on open PRs).\n",
		top.FilePath, top.LineNumber))

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

// Issue is a minimal issue representation for PR comments and the card SVG.
type Issue struct {
	ID          string
	FilePath    string
	LineNumber  int
	RuleName    string
	EstCostRisk float64
	TargetAPI   string
	Severity    string
	CodeSnippet string
}

// recommendationsFor returns honest, generic best-practice bullets for a rule.
// These are guidance, never auto-applied code.
func recommendationsFor(targetAPI string) []string {
	switch targetAPI {
	case "openai", "anthropic":
		return []string{
			"Batch requests or cache responses instead of calling per-iteration",
			"Add rate limiting and per-run cost monitoring",
			"Summarize multiple items in a single API call where possible",
		}
	case "athena":
		return []string{
			"Partition and compress data to cut bytes scanned",
			"Add a LIMIT / WHERE filter before scanning full tables",
			"Cache query results for repeated reads",
		}
	case "dynamodb":
		return []string{
			"Batch writes with BatchWriteItem instead of per-item calls",
			"Use provisioned capacity for predictable workloads",
			"Cache hot reads to avoid repeated request units",
		}
	default:
		return []string{
			"Move the call outside the loop",
			"Batch or cache repeated work",
			"Add cost monitoring on this path",
		}
	}
}

func explanationFor(targetAPI string) string {
	switch targetAPI {
	case "openai", "anthropic":
		return "API call inside a loop can lead to unexpected costs and rate limit issues."
	case "athena":
		return "Per-iteration query scans data repeatedly — bytes scanned drive the bill."
	case "dynamodb":
		return "Per-item request units in a loop add up fast at scale."
	default:
		return "Repeated calls in a loop can multiply cost quickly."
	}
}

// Card palette (GitHub-dark aligned).
const (
	cBg     = "#0d1117"
	cPanel  = "#161b22"
	cBorder = "#30363d"
	cText   = "#e6edf3"
	cMuted  = "#8b949e"
	cGreen  = "#3fb950"
	cRed    = "#f85149"
	cOrange = "#ff7b72"
	cYellow = "#d29922"
)

// GenerateCardSVG renders the full FinOps-Guard result card as a single SVG so
// it can be embedded as one image in a GitHub PR comment (GitHub strips HTML/CSS
// layout, so the whole card must be an image). Everything is data-driven.
func GenerateCardSVG(meter SVGBurnMeter, issues []Issue, analysisSeconds float64, outputPath string) error {
	ratio := meter.TotalRisk / meter.Threshold
	if ratio > 1 {
		ratio = 1
	}

	statusWord, statusColor := "SAFE", cGreen
	decision, decisionColor := "SAFE TO MERGE", cGreen
	if ratio > 0.9 {
		statusWord, statusColor = "OVER", cRed
		decision, decisionColor = "BUDGET EXCEEDED", cRed
	} else if ratio > 0.6 {
		statusWord, statusColor = "AT RISK", cRed
		decision, decisionColor = "REVIEW COST", cYellow
	} else if ratio > 0.25 {
		statusWord, statusColor = "CAUTION", cYellow
		decision, decisionColor = "REVIEW COST", cYellow
	}

	sort.Slice(issues, func(i, j int) bool { return issues[i].EstCostRisk > issues[j].EstCostRisk })

	var monthlyImpact float64
	for _, is := range issues {
		monthlyImpact += is.EstCostRisk * 1000 * 30
	}

	// Small text helpers (content is an arg, so literal '%' is safe).
	txt := func(x, y float64, size int, fill, anchor, weight, content string) string {
		w := ""
		if weight != "" {
			w = ` font-weight="` + weight + `"`
		}
		return fmt.Sprintf(`<text x="%.1f" y="%.1f" font-family="sans-serif" font-size="%d" fill="%s" text-anchor="%s"%s>%s</text>`,
			x, y, size, fill, anchor, w, esc(content))
	}
	mono := func(x, y float64, size int, fill, content string) string {
		return fmt.Sprintf(`<text x="%.1f" y="%.1f" font-family="monospace" font-size="%d" fill="%s">%s</text>`,
			x, y, size, fill, esc(content))
	}
	rrect := func(x, y, w, h, r float64, fill, stroke string, sw float64) string {
		return fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.1f" fill="%s" stroke="%s" stroke-width="%.1f"/>`,
			x, y, w, h, r, fill, stroke, sw)
	}

	var s strings.Builder
	s.WriteString(`<svg width="1200" height="1120" viewBox="0 0 1200 1120" xmlns="http://www.w3.org/2000/svg">`)
	s.WriteString(fmt.Sprintf(`<rect width="1200" height="1120" rx="14" fill="%s"/>`, cBg))
	s.WriteString(fmt.Sprintf(`<rect x="4" y="4" width="1192" height="1112" rx="12" fill="none" stroke="%s" stroke-width="1.5"/>`, cBorder))

	// ---- Gauge gradient ----
	s.WriteString(fmt.Sprintf(`<defs><linearGradient id="gg" x1="200" y1="380" x2="540" y2="380">`+
		`<stop offset="0" stop-color="%s"/><stop offset="0.5" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient></defs>`,
		cGreen, cYellow, cRed))

	// ---- SAFE TO MERGE badge (top-left) ----
	s.WriteString(fmt.Sprintf(`<circle cx="60" cy="62" r="13" fill="none" stroke="%s" stroke-width="2.5"/>`, decisionColor))
	s.WriteString(fmt.Sprintf(`<path d="M53 62 l5 5 l9 -11" fill="none" stroke="%s" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/>`, decisionColor))
	s.WriteString(txt(84, 70, 22, decisionColor, "start", "bold", decision))

	// ---- Gauge (real meter: dim track + value-proportional fill + knob) ----
	cx, cy, R := 370.0, 380.0, 170.0
	// A tiny floor so the fill is always visible even at ~0 cost.
	fillRatio := ratio
	if fillRatio < 0.02 {
		fillRatio = 0.02
	}
	theta := math.Pi * fillRatio
	dotX := cx - R*math.Cos(theta)
	dotY := cy - R*math.Sin(theta)
	// Dim full-scale track.
	s.WriteString(fmt.Sprintf(`<path d="M %.1f %.1f A %.1f %.1f 0 0 1 %.1f %.1f" fill="none" stroke="#21262d" stroke-width="16" stroke-linecap="round"/>`,
		cx-R, cy, R, R, cx+R, cy))
	// Colored fill from 0%% up to the current value (large-arc flag set past 50%%).
	largeArc := 0
	if fillRatio > 0.5 {
		largeArc = 1
	}
	s.WriteString(fmt.Sprintf(`<path d="M %.1f %.1f A %.1f %.1f 0 %d 1 %.1f %.1f" fill="none" stroke="%s" stroke-width="16" stroke-linecap="round"/>`,
		cx-R, cy, R, R, largeArc, dotX, dotY, statusColor))
	// Prominent value knob at the fill head.
	s.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="13" fill="%s" stroke="%s" stroke-width="4"/>`, dotX, dotY, statusColor, cBg))
	// center cost
	s.WriteString(txt(cx, cy-18, 46, statusColor, "middle", "bold", fmt.Sprintf("$%.2f", meter.TotalRisk)))
	s.WriteString(txt(cx, cy+14, 22, cMuted, "middle", "", fmt.Sprintf("/ $%.2f", meter.Threshold)))
	// right-of-arc status
	s.WriteString(txt(cx+R+40, cy-22, 26, statusColor, "start", "bold", statusWord))
	s.WriteString(txt(cx+R+40, cy+6, 18, cMuted, "start", "", fmt.Sprintf("%.0f%%", ratio*100)))
	s.WriteString(txt(cx+R+40, cy+28, 16, cMuted, "start", "", "used"))
	// ticks
	s.WriteString(txt(cx-R-6, cy+34, 15, cMuted, "middle", "", "0%"))
	s.WriteString(txt(cx, cy-R-16, 15, cMuted, "middle", "", "50%"))
	s.WriteString(txt(cx+R+6, cy+34, 15, cMuted, "middle", "", "100%"))

	// ---- issue count pill (under gauge) ----
	if len(issues) > 0 {
		label := fmt.Sprintf("%d Issue Found • High Impact", len(issues))
		if len(issues) > 1 {
			label = fmt.Sprintf("%d Issues Found • High Impact", len(issues))
		}
		pw := 40.0 + float64(len(label))*8.2
		s.WriteString(rrect(cx-pw/2, 448, pw, 40, 20, "#2d1618", "#f85149", 1))
		s.WriteString(txt(cx, 473, 16, cRed, "middle", "bold", "⚠  "+label))
	} else {
		s.WriteString(rrect(cx-120, 448, 240, 40, 20, "#132a1a", cGreen, 1))
		s.WriteString(txt(cx, 473, 16, cGreen, "middle", "bold", "✓  No blocking issues"))
	}

	// ---- PR FinOps Impact panel (top-right) ----
	px, pw := 760.0, 410.0
	s.WriteString(rrect(px, 45, pw, 430, 12, cPanel, cBorder, 1))
	s.WriteString(txt(px+26, 90, 18, cText, "start", "bold", "PR FinOps Impact"))
	rightX := px + pw - 26
	row := func(y float64, label, value, valColor string) {
		s.WriteString(txt(px+26, y, 15, cMuted, "start", "", label))
		s.WriteString(txt(rightX, y, 15, valColor, "end", "bold", value))
	}
	row(140, "Estimated Monthly Cost Impact", "$"+humanMoney(monthlyImpact), cRed)
	row(185, "Resources Affected", fmt.Sprintf("%d", len(issues)), cText)
	row(230, "Potential Savings", "$"+humanMoney(monthlyImpact)+" / mo", cGreen)
	analysis := "<1 sec"
	if analysisSeconds >= 1 {
		analysis = fmt.Sprintf("%.0f sec", analysisSeconds)
	}
	row(275, "Analysis Time", analysis, cText)
	s.WriteString(fmt.Sprintf(`<line x1="%.1f" y1="308" x2="%.1f" y2="308" stroke="%s" stroke-width="1"/>`, px+26, rightX, cBorder))
	s.WriteString(txt(px+26, 345, 14, cMuted, "start", "", "This PR introduces a potential cost increase due to"))
	s.WriteString(txt(px+26, 366, 14, cMuted, "start", "", "loop-bound API calls."))
	// Actionable links live in the comment markdown below the card (a painted
	// link inside an image can never be clicked).
	s.WriteString(txt(px+26, 430, 13, cMuted, "start", "", "Links & fix suggestion below ↓"))

	// ---- Issue panel + Recommended Action (only when there are findings) ----
	if len(issues) > 0 {
		top := issues[0]
		expl := explanationFor(top.TargetAPI)
		sev := strings.ToUpper(top.Severity)
		if sev == "" {
			sev = "HIGH"
		}

		// Issue panel
		s.WriteString(rrect(30, 500, 1140, 300, 12, cPanel, cBorder, 1))
		s.WriteString(fmt.Sprintf(`<rect x="30" y="500" width="6" height="300" rx="3" fill="%s"/>`, cRed))
		s.WriteString(fmt.Sprintf(`<circle cx="70" cy="540" r="14" fill="%s"/>`, cRed))
		s.WriteString(txt(70, 545, 15, "#ffffff", "middle", "bold", "1"))
		s.WriteString(txt(98, 547, 22, cText, "start", "bold", "🎯 WANTED: "+top.RuleName))
		s.WriteString(rrect(1058, 524, 62, 30, 15, "#2d1618", cRed, 1))
		s.WriteString(txt(1089, 544, 13, cRed, "middle", "bold", sev))

		s.WriteString(txt(70, 592, 15, cMuted, "start", "", fmt.Sprintf("🔍 Scanning %s for loop-bound API calls…", top.FilePath)))
		s.WriteString(txt(70, 620, 15, cText, "start", "", fmt.Sprintf("🚨 %d issue(s) found:", len(issues))))

		// code box
		s.WriteString(rrect(70, 640, 1060, 130, 8, cBg, "#5c2b2b", 1))
		s.WriteString(mono(92, 674, 15, cRed, fmt.Sprintf("[%s] %s:%d (%s)", top.ID, top.FilePath, top.LineNumber, top.TargetAPI)))
		// snippet with light syntax split
		snip := top.CodeSnippet
		if snip == "" {
			snip = "response = openai.chat.completions.create("
		}
		if i := strings.Index(snip, "= "); i >= 0 {
			s.WriteString(mono(92, 712, 15, cText, snip[:i+2]))
			s.WriteString(mono(92+float64(len(snip[:i+2]))*9.0, 712, 15, cOrange, snip[i+2:]))
		} else {
			s.WriteString(mono(92, 712, 15, cOrange, snip))
		}
		s.WriteString(txt(92, 748, 14, cMuted, "start", "", expl))

		// Recommended Action panel
		s.WriteString(rrect(30, 830, 1140, 200, 12, "#0f1a12", "#238636", 1))
		s.WriteString(txt(70, 878, 18, cGreen, "start", "bold", "💡 Recommended Action"))
		recs := recommendationsFor(top.TargetAPI)
		for i, r := range recs {
			if i > 2 {
				break
			}
			y := 918.0 + float64(i)*36.0
			s.WriteString(fmt.Sprintf(`<circle cx="82" cy="%.1f" r="3" fill="%s"/>`, y-5, cGreen))
			s.WriteString(txt(98, y, 15, cText, "start", "", r))
		}
	}

	// ---- Footer ----
	s.WriteString(txt(600, 1085, 15, cMuted, "middle", "", "FinOps-Guard helps you build cost-aware. Safe today, scalable tomorrow. 💚"))

	s.WriteString(`</svg>`)

	if err := os.WriteFile(outputPath, []byte(s.String()), 0644); err != nil {
		return fmt.Errorf("failed to write card SVG: %w", err)
	}
	return nil
}

// esc escapes XML-special characters in text content.
func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
