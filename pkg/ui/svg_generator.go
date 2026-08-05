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
		bestPractices := fmt.Sprintf("%s/blob/%s/docs/COST_BEST_PRACTICES.md", repoURL, ref)
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

	cPurple := "#a371f7"

	// Status-driven mission message.
	missionMsg := "You're in the green zone. Keep shipping! 🚀"
	missionSub := fmt.Sprintf("%d issue(s) detected with low cost impact.", len(issues))
	if len(issues) == 0 {
		missionMsg = "All clear — nothing to burn. 🚀"
		missionSub = "No loop-bound API calls detected."
	} else if ratio > 0.9 {
		missionMsg = "Budget breached — hold before merge."
		missionSub = fmt.Sprintf("%d issue(s) with high cost impact.", len(issues))
	} else if ratio > 0.6 {
		missionMsg = "Costs climbing — review before merge."
		missionSub = fmt.Sprintf("%d issue(s) with notable cost impact.", len(issues))
	} else if ratio > 0.25 {
		missionMsg = "Watch the burn — minor cost added."
		missionSub = fmt.Sprintf("%d issue(s) with moderate cost impact.", len(issues))
	}
	analysis := "< 2 sec"
	if analysisSeconds >= 1 {
		analysis = fmt.Sprintf("%.0f sec", analysisSeconds)
	}
	panelFill := "#0d1526"

	var s strings.Builder
	s.WriteString(`<svg width="1500" height="1180" viewBox="0 0 1500 1180" xmlns="http://www.w3.org/2000/svg">`)
	s.WriteString(fmt.Sprintf(`<defs>`+
		`<radialGradient id="space" cx="50%%" cy="34%%" r="85%%"><stop offset="0" stop-color="#13223e"/><stop offset="0.55" stop-color="#0a1020"/><stop offset="1" stop-color="#05070d"/></radialGradient>`+
		`<linearGradient id="gg" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="%s"/><stop offset="0.5" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient>`+
		`<radialGradient id="portal" cx="50%%" cy="50%%" r="50%%"><stop offset="0" stop-color="%s" stop-opacity="0.85"/><stop offset="1" stop-color="%s" stop-opacity="0"/></radialGradient>`+
		`<radialGradient id="orb" cx="40%%" cy="35%%" r="65%%"><stop offset="0" stop-color="#ff9a9a"/><stop offset="0.5" stop-color="#f85149"/><stop offset="1" stop-color="#4c0d10"/></radialGradient>`+
		`<filter id="glow" x="-60%%" y="-60%%" width="220%%" height="220%%"><feGaussianBlur stdDeviation="4" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter>`+
		`</defs>`, cGreen, cYellow, cRed, cGreen, cGreen))
	s.WriteString(`<rect width="1500" height="1180" rx="16" fill="url(#space)"/>`)
	for i := 0; i < 70; i++ {
		fi := float64(i)
		stx := math.Mod(fi*137.508*7.31+fi*fi*3.7, 1500)
		sty := math.Mod(fi*91.7+fi*fi*13.3, 1180)
		str := 0.4 + math.Mod(fi*0.37, 1.4)
		sto := 0.15 + math.Mod(fi*0.11, 0.55)
		s.WriteString(fmt.Sprintf(`<circle cx="%.0f" cy="%.0f" r="%.1f" fill="#fff" opacity="%.2f"/>`, stx, sty, str, sto))
	}
	s.WriteString(fmt.Sprintf(`<rect x="5" y="5" width="1490" height="1170" rx="14" fill="none" stroke="%s" stroke-width="1.5"/>`, cBorder))
	s.WriteString(txt(40, 62, 30, cText, "start", "bold", "FinOps analysis complete for this Pull Request 🚀"))

	// ---- Mission Control panel ----
	s.WriteString(rrect(30, 95, 740, 430, 16, panelFill, "#233149", 1))
	s.WriteString(fmt.Sprintf(`<circle cx="58" cy="138" r="5" fill="%s"/>`, cGreen))
	s.WriteString(txt(72, 143, 13, cGreen, "start", "bold", "MISSION CONTROL"))
	s.WriteString(rrect(636, 122, 112, 30, 15, "#0b1a13", statusColor, 1))
	s.WriteString(txt(692, 142, 13, statusColor, "middle", "bold", statusWord))
	s.WriteString(txt(56, 182, 24, cText, "start", "bold", missionMsg))
	s.WriteString(txt(56, 210, 15, cMuted, "start", "", missionSub))
	// cost box
	s.WriteString(rrect(56, 236, 210, 90, 10, "#0b1a13", "#1c3a2a", 1))
	s.WriteString(txt(78, 278, 30, cGreen, "start", "bold", fmt.Sprintf("$%.2f", meter.TotalRisk)))
	s.WriteString(txt(78, 298, 11, cMuted, "start", "", "EST. MONTHLY IMPACT"))
	s.WriteString(txt(78, 316, 13, cMuted, "start", "", fmt.Sprintf("/ $%.2f budget", meter.Threshold)))
	// gauge with rocket marker
	cx, cy, R := 500.0, 360.0, 135.0
	fillRatio := ratio
	if fillRatio < 0.03 {
		fillRatio = 0.03
	}
	theta := math.Pi * fillRatio
	mx := cx - R*math.Cos(theta)
	my := cy - R*math.Sin(theta)
	s.WriteString(fmt.Sprintf(`<ellipse cx="%.0f" cy="%.0f" rx="72" ry="18" fill="url(#portal)"/>`, cx, cy+6))
	s.WriteString(fmt.Sprintf(`<path d="M %.1f %.1f A %.1f %.1f 0 0 1 %.1f %.1f" fill="none" stroke="url(#gg)" stroke-width="4" stroke-linecap="round" filter="url(#glow)"/>`, cx-R, cy, R, R, cx+R, cy))
	s.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="7" fill="%s" filter="url(#glow)"/>`, cx-R, cy, cGreen))
	s.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="7" fill="%s" filter="url(#glow)"/>`, cx+R, cy, cRed))
	s.WriteString(txt(cx, cy-R-10, 14, cMuted, "middle", "", "50%"))
	s.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" font-size="30" text-anchor="middle">🚀</text>`, mx, my+8))
	s.WriteString(txt(cx-R, cy+28, 14, cMuted, "middle", "", "0%"))
	s.WriteString(txt(cx-R, cy+46, 12, cMuted, "middle", "", "used"))
	s.WriteString(txt(cx+R, cy+28, 14, cMuted, "middle", "", "100%"))
	s.WriteString(txt(cx+R, cy+46, 12, cMuted, "middle", "", "budget"))
	// SAFE TO MERGE pill
	s.WriteString(rrect(cx-118, cy+74, 236, 44, 22, "#0b1a13", decisionColor, 1.5))
	s.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="10" fill="none" stroke="%s" stroke-width="2"/>`, cx-78, cy+96, decisionColor))
	s.WriteString(fmt.Sprintf(`<path d="M %.1f %.1f l4 4 l7 -8" fill="none" stroke="%s" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>`, cx-83, cy+96, decisionColor))
	s.WriteString(txt(cx+8, cy+102, 18, decisionColor, "middle", "bold", decision))

	// ---- FinOps Impact panel ----
	px, pw := 790.0, 440.0
	s.WriteString(rrect(px, 95, pw, 300, 16, panelFill, "#233149", 1))
	s.WriteString(txt(px+28, 140, 15, cText, "start", "bold", "📊 FINOPS IMPACT"))
	rightX := px + pw - 28
	row := func(y float64, label, value, vc string) {
		s.WriteString(txt(px+28, y, 15, cMuted, "start", "", label))
		s.WriteString(txt(rightX, y, 15, vc, "end", "bold", value))
	}
	row(185, "Estimated Monthly Cost Impact", "$"+humanMoney(monthlyImpact), statusColor)
	row(223, "Resources Affected", fmt.Sprintf("%d", len(issues)), cText)
	row(261, "Potential Savings", "$"+humanMoney(monthlyImpact)+" / month", cGreen)
	row(299, "Analysis Time", analysis, cText)
	s.WriteString(rrect(px+22, 322, pw-44, 56, 10, "#0b1220", "#233149", 1))
	s.WriteString(txt(px+38, 346, 13, cMuted, "start", "", "Potential cost increase from loop-bound API calls."))
	s.WriteString(txt(px+38, 366, 13, cMuted, "start", "", "Docs & fix suggestion below ↓"))

	// ---- Robot mascot ----
	s.WriteString(rrect(1250, 110, 220, 96, 12, panelFill, "#233149", 1))
	s.WriteString(txt(1268, 140, 13, cText, "start", "", "Nice work! Just a small"))
	s.WriteString(txt(1268, 160, 13, cText, "start", "", "tweak to make it even"))
	s.WriteString(txt(1268, 180, 13, cText, "start", "", "more efficient."))
	s.WriteString(fmt.Sprintf(`<path d="M1330 206 l0 22 l20 -22 z" fill="%s"/>`, panelFill))
	rx, ry := 1315.0, 250.0
	s.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="%s" stroke-width="2"/>`, rx+45, ry, rx+45, ry-16, cGreen))
	s.WriteString(fmt.Sprintf(`<circle cx="%.0f" cy="%.0f" r="5" fill="%s" filter="url(#glow)"/>`, rx+45, ry-20, cGreen))
	s.WriteString(rrect(rx, ry, 90, 74, 18, "#0e2430", cGreen, 2))
	s.WriteString(fmt.Sprintf(`<circle cx="%.0f" cy="%.0f" r="9" fill="%s" filter="url(#glow)"/>`, rx+30, ry+34, cGreen))
	s.WriteString(fmt.Sprintf(`<circle cx="%.0f" cy="%.0f" r="9" fill="%s" filter="url(#glow)"/>`, rx+60, ry+34, cGreen))
	s.WriteString(fmt.Sprintf(`<rect x="%.0f" y="%.0f" width="26" height="4" rx="2" fill="%s"/>`, rx+32, ry+54, cGreen))
	s.WriteString(rrect(rx+14, ry+80, 62, 46, 10, "#0e2430", cGreen, 2))
	s.WriteString(fmt.Sprintf(`<path d="M %.0f %.0f l20 0 l0 12 q0 13 -10 18 q-10 -5 -10 -18 z" fill="#0b1a13" stroke="%s" stroke-width="1.5"/>`, rx+35, ry+90, cGreen))
	s.WriteString(txt(rx+45, ry+108, 13, cGreen, "middle", "bold", "$"))

	// ---- Issue panel + Why it matters + Recommended Actions + Journey ----
	if len(issues) > 0 {
		top := issues[0]
		expl := explanationFor(top.TargetAPI)
		sev := strings.ToUpper(top.Severity)
		if sev == "" {
			sev = "HIGH"
		}

		// Issue panel
		s.WriteString(rrect(30, 545, 1440, 300, 16, "#160f14", "#5c2b2b", 1))
		s.WriteString(fmt.Sprintf(`<rect x="30" y="545" width="6" height="300" rx="3" fill="%s"/>`, cRed))
		s.WriteString(fmt.Sprintf(`<circle cx="64" cy="588" r="9" fill="none" stroke="%s" stroke-width="2"/>`, cRed))
		s.WriteString(fmt.Sprintf(`<circle cx="64" cy="588" r="3" fill="%s"/>`, cRed))
		s.WriteString(txt(84, 593, 14, cRed, "start", "bold", fmt.Sprintf("%d ISSUE FOUND", len(issues))))

		// glowing orb (left)
		s.WriteString(`<circle cx="150" cy="715" r="70" fill="url(#orb)" filter="url(#glow)"/>`)
		s.WriteString(`<circle cx="150" cy="715" r="88" fill="none" stroke="#f85149" stroke-opacity="0.25" stroke-width="1"/>`)
		s.WriteString(`<circle cx="150" cy="715" r="104" fill="none" stroke="#f85149" stroke-opacity="0.14" stroke-width="1"/>`)

		// middle: the finding
		s.WriteString(txt(310, 636, 22, cRed, "start", "bold", "🎯 WANTED: "+top.RuleName))
		s.WriteString(rrect(310, 656, 60, 26, 13, "#2d1618", cRed, 1))
		s.WriteString(txt(340, 674, 12, cRed, "middle", "bold", sev))
		s.WriteString(txt(384, 674, 14, cMuted, "start", "", fmt.Sprintf("in %s", top.FilePath)))
		s.WriteString(txt(310, 712, 14, cText, "start", "", fmt.Sprintf("🚨 %d issue(s) found:", len(issues))))
		s.WriteString(rrect(310, 726, 600, 96, 8, "#0b0d12", "#5c2b2b", 1))
		s.WriteString(mono(330, 756, 14, cRed, fmt.Sprintf("[%s] %s:%d (%s)", top.ID, top.FilePath, top.LineNumber, top.TargetAPI)))
		snip := top.CodeSnippet
		if snip == "" {
			snip = "response = openai.chat.completions.create("
		}
		if i := strings.Index(snip, "= "); i >= 0 {
			s.WriteString(mono(330, 786, 14, cText, snip[:i+2]))
			s.WriteString(mono(330+float64(len(snip[:i+2]))*8.4, 786, 14, cOrange, snip[i+2:]))
		} else {
			s.WriteString(mono(330, 786, 14, cOrange, snip))
		}
		s.WriteString(txt(330, 810, 12, cMuted, "start", "", expl))

		// right: why it matters
		s.WriteString(txt(970, 636, 13, cYellow, "start", "bold", "✦ WHY IT MATTERS"))
		whys := []string{"Repeated API calls = higher cost", "Risk of hitting rate limits", "Harder to scale and monitor"}
		for i, w := range whys {
			wy := 682.0 + float64(i)*38.0
			s.WriteString(fmt.Sprintf(`<circle cx="980" cy="%.0f" r="4" fill="%s"/>`, wy-4, cRed))
			s.WriteString(txt(996, wy, 14, cText, "start", "", w))
		}

		// ---- Recommended Actions ----
		s.WriteString(rrect(30, 865, 1440, 160, 16, "#0d1a12", "#1f5132", 1))
		s.WriteString(txt(56, 905, 14, cGreen, "start", "bold", "⚡ RECOMMENDED ACTIONS"))
		acts := [][3]string{
			{"Batch API Requests", "Group multiple inputs and", "send in a single API call."},
			{"Add Rate Limiting", "Implement rate limiting and", "cost monitoring per run."},
			{"Use Caching", "Cache responses to avoid", "redundant calls."},
		}
		icons := []string{"📦", "🛡️", "💾"}
		for i, a := range acts {
			ax := 70.0 + float64(i)*470.0
			s.WriteString(rrect(ax, 935, 50, 50, 12, "#0e2a1a", cGreen, 1.5))
			s.WriteString(fmt.Sprintf(`<text x="%.0f" y="968" font-size="24" text-anchor="middle">%s</text>`, ax+25, icons[i]))
			s.WriteString(txt(ax+66, 953, 16, cText, "start", "bold", a[0]))
			s.WriteString(txt(ax+66, 974, 12, cMuted, "start", "", a[1]))
			s.WriteString(txt(ax+66, 992, 12, cMuted, "start", "", a[2]))
		}

		// ---- PR Cost Journey ----
		s.WriteString(rrect(30, 1045, 1440, 105, 16, "#14122a", "#2c2650", 1))
		s.WriteString(txt(56, 1082, 13, cPurple, "start", "bold", "⟳ PR COST JOURNEY"))
		steps := [][2]string{{"PR Opened", "just now"}, {"Code Scanned", "< 1 sec"}, {"FinOps Analysis", analysis}, {"Results Ready", "< 1 sec"}}
		for i, st := range steps {
			sx := 280.0 + float64(i)*200.0
			s.WriteString(fmt.Sprintf(`<circle cx="%.0f" cy="1092" r="9" fill="none" stroke="%s" stroke-width="2"/>`, sx, cPurple))
			s.WriteString(fmt.Sprintf(`<circle cx="%.0f" cy="1092" r="3.5" fill="%s"/>`, sx, cGreen))
			if i < len(steps)-1 {
				s.WriteString(fmt.Sprintf(`<line x1="%.0f" y1="1092" x2="%.0f" y2="1092" stroke="#3a3566" stroke-width="1.5" stroke-dasharray="4 5"/>`, sx+14, sx+186))
			}
			s.WriteString(txt(sx+18, 1088, 14, cText, "start", "", st[0]))
			s.WriteString(txt(sx+18, 1108, 12, cMuted, "start", "", st[1]))
		}
		s.WriteString(rrect(1130, 1066, 320, 64, 12, "#1a1640", cPurple, 1))
		s.WriteString(txt(1150, 1094, 13, cText, "start", "", "FinOps-Guard helps you build cost-aware."))
		s.WriteString(txt(1150, 1114, 13, cText, "start", "", "Safe today, scalable tomorrow. 💚"))
	}

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
