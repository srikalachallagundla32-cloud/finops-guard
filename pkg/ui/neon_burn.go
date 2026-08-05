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

// Neon Palette: pink -> coral -> butter -> white (inspired by ORE terminal)
var neonPalette = []string{
	"#000000", "#0a0000", "#1a0000", "#2a0505", "#3a0a0a",
	"#4a1515", "#6a2020", "#8a3030", "#aa4545", "#ca5a5a",
	"#ea6f6f", "#ff8a8a", "#ffaa88", "#ffbb99", "#ffccaa",
	"#ffddbb", "#ffeecc", "#ffffdd", "#ffffff",
}

const neonFireHeight = 12
const maxRaindrops = 120

type Raindrop struct {
	x        float64
	y        float64
	vx       float64
	vy       float64
	lifetime int
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

type NeonBurnModel struct {
	issues       []analyzer.Issue
	cursor       int
	totalRisk    float64
	threshold    float64
	width        int
	height       int
	fireBuffer   []int
	raindrops    []*Raindrop
	frameCount   int
	bootCountdown int
	shakeCountdown int
	selectedQuote string
	statusMsg    string

	showFixModal   bool
	fixBefore     string
	fixAfter      string
	fixErr        string

	showPromptModal bool
	promptText     string

	leakX int
}

func NewNeonBurnModel(issues []analyzer.Issue, totalRisk float64, threshold float64) NeonBurnModel {
	rand.Seed(time.Now().UnixNano())
	width := 120
	height := 32
	return NeonBurnModel{
		issues:        issues,
		cursor:        0,
		totalRisk:     totalRisk,
		threshold:     threshold,
		width:         width,
		height:        height,
		fireBuffer:    make([]int, width*neonFireHeight),
		raindrops:     make([]*Raindrop, 0, maxRaindrops),
		bootCountdown: 48, // ~2 seconds at 24 fps
		selectedQuote: pickNeonQuote(totalRisk, threshold),
		leakX:         width / 3,
	}
}

func pickNeonQuote(totalRisk, threshold float64) string {
	status := statusFromRisk(totalRisk, threshold)
	switch status {
	case BurnSafe:
		return approvingQuotes[rand.Intn(len(approvingQuotes))]
	case BurnSweating:
		return `"Uh... is anyone else seeing these numbers?"` // Mid-tier panic
	case BurnOnFire:
		return `"THIS IS FINE." [sips coffee as everything burns]` // Classic
	case BurnInferno:
		return criticalQuotes[rand.Intn(len(criticalQuotes))]
	}
	return approvingQuotes[0]
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
			m.fireBuffer = make([]int, m.width*neonFireHeight)
			m.leakX = m.width / 3
		}

	case TickMsg:
		m.frameCount++
		if m.bootCountdown > 0 {
			m.bootCountdown--
		} else {
			m.updateFirePhysics()
			m.updateRain()
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
						m.raindrops = make([]*Raindrop, 0) // Clear rain
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

func (m *NeonBurnModel) updateFirePhysics() {
	if m.width == 0 {
		return
	}

	// Seed heat based on risk ratio and rain intensity
	maxHeat := len(neonPalette) - 1
	riskRatio := m.totalRisk / m.threshold
	if riskRatio > 1.0 {
		riskRatio = 1.0
	}
	maxHeat = int(float64(maxHeat) * (0.2 + 0.8*riskRatio))

	bottomRow := (neonFireHeight - 1) * m.width
	for x := 0; x < m.width; x++ {
		// Hotter at the leak site
		intensity := rand.Intn(maxHeat + 1)
		if x >= m.leakX-2 && x <= m.leakX+2 {
			intensity = maxHeat // Peak heat at leak location
		}
		m.fireBuffer[bottomRow+x] = intensity
	}

	// Propagate upwards with decay and drift
	for y := 1; y < neonFireHeight; y++ {
		for x := 0; x < m.width; x++ {
			srcIndex := y*m.width + x
			decay := rand.Intn(3) // Slightly more decay for more realistic burn
			dstX := (x + rand.Intn(3) - 1 + m.width) % m.width
			dstIndex := (y-1)*m.width + dstX

			val := m.fireBuffer[srcIndex] - decay
			if val < 0 {
				val = 0
			}
			m.fireBuffer[dstIndex] = val
		}
	}
}

func (m *NeonBurnModel) updateRain() {
	// Spawn raindrops proportional to cost
	spawnRate := int(math.Max(1, m.totalRisk/10))
	for i := 0; i < spawnRate && len(m.raindrops) < maxRaindrops; i++ {
		m.raindrops = append(m.raindrops, &Raindrop{
			x:        float64(m.leakX) + (rand.Float64()-0.5)*4,
			y:        0,
			vy:       2 + rand.Float64()*2,
			lifetime: 80,
		})
	}

	// Update falling raindrops
	newRain := make([]*Raindrop, 0, len(m.raindrops))
	for _, drop := range m.raindrops {
		drop.lifetime--
		drop.y += drop.vy
		if drop.lifetime > 0 && drop.y < float64(neonFireHeight) {
			newRain = append(newRain, drop)
		}
	}
	m.raindrops = newRain
}

func (m NeonBurnModel) renderNeonFire() string {
	var s strings.Builder

	for y := 0; y < neonFireHeight-1; y += 2 {
		for x := 0; x < m.width; x++ {
			topIdx := m.fireBuffer[y*m.width+x]
			botIdx := m.fireBuffer[(y+1)*m.width+x]

			// Draw raindrops on top if they're at this position
			for _, drop := range m.raindrops {
				dx := int(drop.x)
				dy := int(drop.y)
				if dx == x && dy/2 == y/2 {
					// Raindrop is here — override with money symbol
					alpha := 255 * (drop.lifetime / 80)
					rainColor := lipgloss.Color(fmt.Sprintf("#ff00ff%02x", int(alpha)))
					cell := lipgloss.NewStyle().
						Foreground(rainColor).
						Background(lipgloss.Color(neonPalette[botIdx])).
						Render("$")
					s.WriteString(cell)
					goto nextCell
				}
			}

			// Normal fire rendering
			{
				cell := lipgloss.NewStyle().
					Foreground(lipgloss.Color(neonPalette[botIdx])).
					Background(lipgloss.Color(neonPalette[topIdx])).
					Render("▀")
				s.WriteString(cell)
			}
		nextCell:
		}
		s.WriteString("\n")
	}

	return s.String()
}

func (m NeonBurnModel) renderBootSequence() string {
	var s strings.Builder
	progress := float64(48-m.bootCountdown) / 48

	// Scanline sweep
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

	var doc strings.Builder

	// Fire canvas (top)
	doc.WriteString(m.renderNeonFire())

	// Findings radar
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
