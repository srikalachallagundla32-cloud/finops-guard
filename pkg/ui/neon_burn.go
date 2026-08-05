package ui

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/your-username/finops-guard/pkg/analyzer"
)

// ---------- color and fire engine (from costfire.go) ----------

type rgb struct{ r, g, b uint8 }

func (c rgb) hex() string { return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b) }

// Neon candy ramp: deep plum -> magenta -> pink -> coral -> butter -> white
var neonRamp = []rgb{
	{0x0d, 0x07, 0x13}, // deep plum
	{0x4b, 0x15, 0x28}, // plum
	{0x99, 0x35, 0x56}, // magenta
	{0xd4, 0x53, 0x7e}, // pink
	{0xf0, 0x99, 0x7b}, // coral
	{0xfa, 0xc7, 0x75}, // butter
	{0xfa, 0xee, 0xda}, // off-white
}

func heatColor(t float64) rgb {
	if t <= 0 {
		return neonRamp[0]
	}
	if t >= 1 {
		return neonRamp[len(neonRamp)-1]
	}
	f := t * float64(len(neonRamp)-1)
	i := int(f)
	frac := f - float64(i)
	a, b := neonRamp[i], neonRamp[i+1]
	lerp := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*frac) }
	return rgb{lerp(a.r, b.r), lerp(a.g, b.g), lerp(a.b, b.b)}
}

type fire struct {
	w, h      int
	heat      []float64
	intensity float64
	rng       *rand.Rand
}

func newFire(w, h int) *fire {
	return &fire{w: w, h: h, heat: make([]float64, w*h), rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (f *fire) SetIntensity(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	f.intensity = v
}

func (f *fire) Step() {
	base := (f.h - 1) * f.w
	for x := 0; x < f.w; x++ {
		if f.rng.Float64() < 0.85*f.intensity {
			f.heat[base+x] = 0.6 + 0.4*f.rng.Float64()
		} else {
			f.heat[base+x] = 0
		}
	}
	const decay = 3.07
	for y := 0; y < f.h-1; y++ {
		row := y * f.w
		below := (y + 1) * f.w
		for x := 0; x < f.w; x++ {
			l := (x - 1 + f.w) % f.w
			r := (x + 1) % f.w
			f.heat[row+x] = (f.heat[below+x] + f.heat[below+l] + f.heat[below+r]) / decay
		}
	}
}

func (f *fire) RenderHalfBlocks(overlay map[[2]int]rune) string {
	var sb strings.Builder
	sb.Grow(f.w * f.h * 8)
	for y := 0; y < f.h; y += 2 {
		for x := 0; x < f.w; x++ {
			top := heatColor(f.heat[y*f.w+x])
			bot := neonRamp[0]
			if y+1 < f.h {
				bot = heatColor(f.heat[(y+1)*f.w+x])
			}
			st := lipgloss.NewStyle().
				Foreground(lipgloss.Color(top.hex())).
				Background(lipgloss.Color(bot.hex()))
			ch := "▀"
			if r, ok := overlay[[2]int{x, y / 2}]; ok {
				st = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FAC775")).
					Background(lipgloss.Color(bot.hex())).
					Bold(true)
				ch = string(r)
			}
			sb.WriteString(st.Render(ch))
		}
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

// ---------- particles and TUI model ----------

type particle struct {
	x     int
	y     float64
	speed float64
}

type BurnStatus int

const (
	BurnSafe BurnStatus = iota
	BurnSweating
	BurnOnFire
	BurnInferno
)

func (bs BurnStatus) String() string {
	switch bs {
	case BurnSafe:
		return "SAFE"
	case BurnSweating:
		return "SWEATING"
	case BurnOnFire:
		return "ON FIRE"
	case BurnInferno:
		return "INFERNO"
	default:
		return "UNKNOWN"
	}
}

func (bs BurnStatus) Color() lipgloss.Color {
	switch bs {
	case BurnSafe:
		return green
	case BurnSweating:
		return lipgloss.Color("#FFAA00")
	case BurnOnFire:
		return pink
	case BurnInferno:
		return lipgloss.Color("#FF00FF")
	default:
		return white
	}
}

func statusFromRisk(totalRisk, threshold float64) BurnStatus {
	ratio := totalRisk / threshold
	if ratio < 0.25 {
		return BurnSafe
	} else if ratio < 0.6 {
		return BurnSweating
	} else if ratio < 1.0 {
		return BurnOnFire
	}
	return BurnInferno
}

var (
	sweatingQuotes = []string{
		`"Uh... is anyone else seeing these numbers?"`,
		`"There's probably an explanation for this."`,
		`"Let's just... not look at the bill this month."`,
	}

	onFireQuotes = []string{
		`"THIS IS FINE." [sips coffee as everything burns]`,
		`"I've made some questionable decisions."`,
		`"Maybe we should switch to a different cloud provider?"`,
	}
)

func pickNeonQuote(totalRisk, threshold float64) string {
	status := statusFromRisk(totalRisk, threshold)
	var pool []string
	switch status {
	case BurnSafe:
		pool = approvingQuotes
	case BurnSweating:
		pool = sweatingQuotes
	case BurnOnFire:
		pool = onFireQuotes
	case BurnInferno:
		pool = criticalQuotes
	default:
		pool = approvingQuotes
	}
	return pool[rand.Intn(len(pool))]
}

type NeonBurnModel struct {
	issues         []analyzer.Issue
	cursor         int
	totalRisk      float64
	threshold      float64
	width          int
	height         int
	fire           *fire
	fireRows       int
	particles      []particle
	frameCount     int
	bootCountdown  int
	selectedQuote  string
	statusMsg      string

	showFixModal   bool
	fixBefore      string
	fixAfter       string
	fixErr         string

	showPromptModal bool
	promptText     string

	rng *rand.Rand
}

func NewNeonBurnModel(issues []analyzer.Issue, totalRisk float64, threshold float64) NeonBurnModel {
	rand.Seed(time.Now().UnixNano())
	width, fireRows := 100, 14
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return NeonBurnModel{
		issues:        issues,
		cursor:        0,
		totalRisk:     totalRisk,
		threshold:     threshold,
		width:         width,
		height:        32,
		fire:          newFire(width, fireRows*2), // pixel height = 2x cell rows
		fireRows:      fireRows,
		bootCountdown: 48,
		selectedQuote: pickNeonQuote(totalRisk, threshold),
		rng:           rng,
	}
}

func (m NeonBurnModel) Init() tea.Cmd {
	return doTick()
}

func (m NeonBurnModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 40 && msg.Height > 20 {
			m.width = msg.Width
			m.height = msg.Height
			m.fire = newFire(m.width, m.fireRows*2)
		}

	case TickMsg:
		m.frameCount++
		if m.bootCountdown > 0 {
			m.bootCountdown--
		} else {
			intensity := m.totalRisk / m.threshold
			if intensity > 1.0 {
				intensity = 1.0
			}
			m.fire.SetIntensity(intensity)
			m.fire.Step()

			// Spawn particles proportional to cost
			spawnRate := int(math.Max(1, m.totalRisk/10))
			for i := 0; i < spawnRate && len(m.particles) < 80; i++ {
				m.particles = append(m.particles, particle{
					x:     m.rng.Intn(m.width),
					y:     0,
					speed: 0.3 + 0.4*m.rng.Float64(),
				})
			}
			// Update particles
			alive := m.particles[:0]
			for _, p := range m.particles {
				p.y += p.speed
				if int(p.y) < m.fireRows-2 {
					alive = append(alive, p)
				}
			}
			m.particles = alive
		}
		return m, doTick()

	case tea.KeyMsg:
		if m.bootCountdown > 0 {
			m.bootCountdown = 0
			return m, nil
		}

		if m.showFixModal {
			switch msg.String() {
			case "y":
				if len(m.issues) > 0 && m.fixErr == "" {
					issue := m.issues[m.cursor]
					if err := analyzer.ApplyFix(issue); err != nil {
						m.statusMsg = "❌ Fix failed: " + err.Error()
					} else {
						m.statusMsg = fmt.Sprintf("✅ Fixed %s:%d — fire cooling...", issue.FilePath, issue.LineNumber)
						m.particles = make([]particle, 0)
					}
				}
				m.showFixModal = false
			case "n", "esc":
				m.statusMsg = "Fix cancelled."
				m.showFixModal = false
			}
			return m, nil
		}

		if m.showPromptModal {
			switch msg.String() {
			case "p", "esc", "enter":
				m.showPromptModal = false
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.issues)-1 {
				m.cursor++
			}
		case "f":
			if len(m.issues) > 0 {
				before, after, err := analyzer.PreviewFix(m.issues[m.cursor])
				if err != nil {
					m.fixErr = err.Error()
				} else {
					m.fixErr = ""
					m.fixBefore = before
					m.fixAfter = after
				}
				m.showFixModal = true
			}
		case "p":
			if len(m.issues) > 0 {
				issue := m.issues[m.cursor]
				prompt := buildClaudeCodePrompt(issue)
				m.promptText = prompt
				if err := copyToClipboard(prompt); err != nil {
					m.statusMsg = "📋 Prompt ready (clipboard unavailable)"
				} else {
					m.statusMsg = "📋 Prompt copied to clipboard."
				}
				m.showPromptModal = true
			}
		}
	}
	return m, nil
}

func (m NeonBurnModel) renderBootSequence() string {
	var s strings.Builder
	progress := float64(48-m.bootCountdown) / 48

	scanline := int(float64(m.height-4) * progress)

	s.WriteString(lipgloss.NewStyle().Foreground(pink).Bold(true).Render("🛡️  F I N O P S - G U A R D") + "\n")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("████████████████████████████") + "\n\n")
	s.WriteString("INITIALIZING NEON BURN ENGINE...\n\n")

	for i := 0; i < 5; i++ {
		if i < scanline {
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("✓ SHIELD SYSTEMS ONLINE\n"))
		} else {
			s.WriteString("• SHIELD SYSTEMS ONLINE\n")
		}
	}

	return s.String()
}

func (m NeonBurnModel) View() string {
	if m.bootCountdown > 0 {
		return m.renderBootSequence()
	}

	if m.showFixModal {
		return m.renderFixModal()
	}
	if m.showPromptModal {
		return m.renderPromptModal()
	}

	status := statusFromRisk(m.totalRisk, m.threshold)
	statusColor := status.Color()

	// Build particle overlay
	overlay := make(map[[2]int]rune, len(m.particles))
	for _, p := range m.particles {
		overlay[[2]int{p.x, int(p.y)}] = '$'
	}

	var doc strings.Builder
	doc.WriteString(m.fire.RenderHalfBlocks(overlay))
	doc.WriteString("\n")

	// Findings
	doc.WriteString(lipgloss.NewStyle().Bold(true).Foreground(pink).Render("🎯 LEAKS DETECTED") + "\n")
	if len(m.issues) == 0 {
		doc.WriteString(lipgloss.NewStyle().Foreground(green).Render("None.\n\n"))
	} else {
		for i, issue := range m.issues {
			marker := "  "
			if i == m.cursor {
				marker = "❯ "
			}
			doc.WriteString(fmt.Sprintf("%s[%s] $%.4f/run  %s\n", marker, issue.ID, issue.EstCostRisk, issue.RuleName))
		}
		doc.WriteString("\n")
	}

	// CFO Status
	doc.WriteString(lipgloss.NewStyle().Bold(true).Foreground(statusColor).Render(fmt.Sprintf("💼 CFO STATUS: %s\n", status)) + "\n")
	doc.WriteString(lipgloss.NewStyle().Italic(true).Foreground(white).Render(m.selectedQuote) + "\n\n")

	// Footer
	statusPill := lipgloss.NewStyle().Bold(true).Foreground(white).Background(statusColor).Padding(0, 1).Render(fmt.Sprintf("BURN: $%.2f / $%.2f", m.totalRisk, m.threshold))
	footer := fmt.Sprintf(" │ FRAME: %04d │ [↑/↓] select  [f] fix  [p] prompt  [q] quit", m.frameCount)
	doc.WriteString(statusPill + lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#666666")).Render(footer))

	if m.statusMsg != "" {
		doc.WriteString("\n" + lipgloss.NewStyle().Foreground(white).Render(m.statusMsg))
	}

	return doc.String()
}

func (m NeonBurnModel) renderFixModal() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(pink).Render("⚠️  CONFIRM REFACTOR PREVIEW")

	if m.fixErr != "" {
		body := lipgloss.NewStyle().Foreground(pink).Render("Error loading preview: " + m.fixErr)
		return quoteBox.Render(lipgloss.JoinVertical(lipgloss.Left, title, "", body, "", "[esc] close"))
	}

	beforeBlock := lipgloss.NewStyle().Foreground(gray).Render("BEFORE:\n" + m.fixBefore)
	afterBlock := lipgloss.NewStyle().Foreground(green).Render("AFTER:\n" + m.fixAfter)
	note := lipgloss.NewStyle().Italic(true).Foreground(gray).Render(
		"This only inserts an advisory comment above the flagged line — it does not rewrite the loop logic.")
	footer := lipgloss.NewStyle().Bold(true).Render("[y] apply    [n / esc] cancel")

	width := m.width - 4
	if width < 40 {
		width = 40
	}
	return quoteBox.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, title, "", beforeBlock, "", afterBlock, "", note, "", footer))
}

func (m NeonBurnModel) renderPromptModal() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(white).Render("📋 CLAUDE CODE PROMPT")
	note := lipgloss.NewStyle().Foreground(green).Render(m.statusMsg)
	body := lipgloss.NewStyle().Foreground(white).Render(m.promptText)
	footer := lipgloss.NewStyle().Italic(true).Render("No files were modified. [esc / p] close")

	width := m.width - 4
	if width < 40 {
		width = 40
	}
	return quoteBox.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, title, note, "", body, "", footer))
}

func RunNeonBurnTUI(issues []analyzer.Issue, totalRisk float64, threshold float64) error {
	p := tea.NewProgram(NewNeonBurnModel(issues, totalRisk, threshold), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
