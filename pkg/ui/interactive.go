package ui

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/your-username/finops-guard/pkg/analyzer"
)

type TickMsg time.Time

func doTick() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Flame Palette (Black -> Red -> Orange -> Yellow -> White)
var firePalette = []string{
	"#000000", "#180000", "#300000", "#580000", "#800000",
	"#A00000", "#C80000", "#E82800", "#FF5000", "#FF7800",
	"#FFA000", "#FFC800", "#FFE000", "#FFFF40", "#FFFFFF",
}

// fireCanvasHeight is the raw fire-buffer row count; rendered as
// fireCanvasHeight/2 half-block lines (two buffer rows per glyph).
const fireCanvasHeight = 8

var (
	white = lipgloss.Color("#FFFFFF")
	pink  = lipgloss.Color("#FF0055")
	green = lipgloss.Color("#04B575")
	gray  = lipgloss.Color("#4A4A4A")

	radarBox     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(pink).Padding(0, 1)
	inspectorBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(gray).Padding(0, 1)
	quoteBox     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(white).Padding(0, 1)
)

var criticalQuotes = []string{
	`"If this loop hits production, our cloud bill will surpass our revenue."`,
	`"My cloud bill alert notification just gave me a heart attack."`,
	`"Is this an AI agent or a money incineration engine?"`,
	`"We're going to have to downgrade the office coffee to fund this loop."`,
}

var approvingQuotes = []string{
	`"Finally, an engineering team that respects a budget."`,
	`"This is the kind of efficiency that gets you a bonus, not a PIP."`,
	`"I could almost enjoy reading a cost report for once."`,
	`"Keep this up and we might actually hit our margin targets."`,
}

func pickCFOQuote(totalRisk, threshold float64) string {
	pool := approvingQuotes
	if totalRisk > threshold {
		pool = criticalQuotes
	}
	return pool[rand.Intn(len(pool))]
}

type InspoCanvasModel struct {
	issues        []analyzer.Issue
	cursor        int
	totalRisk     float64
	threshold     float64
	width         int
	fireBuffer    []int
	frameCount    int
	blink         bool
	selectedQuote string
}

func NewInspoCanvasModel(issues []analyzer.Issue, totalRisk float64, threshold float64) InspoCanvasModel {
	rand.Seed(time.Now().UnixNano())
	width := 80
	return InspoCanvasModel{
		issues:        issues,
		totalRisk:     totalRisk,
		threshold:     threshold,
		width:         width,
		fireBuffer:    make([]int, width*fireCanvasHeight),
		blink:         true,
		selectedQuote: pickCFOQuote(totalRisk, threshold),
	}
}

func (m InspoCanvasModel) Init() tea.Cmd {
	return doTick()
}

func (m InspoCanvasModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.width = msg.Width
			m.fireBuffer = make([]int, m.width*fireCanvasHeight)
		}

	case TickMsg:
		m.frameCount++
		m.blink = !m.blink
		m.updateFirePhysics()
		return m, doTick()

	case tea.KeyMsg:
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
		}
	}
	return m, nil
}

// Procedural Doom Fire Particle Physics Engine
func (m *InspoCanvasModel) updateFirePhysics() {
	if m.width == 0 {
		return
	}

	// 1. Seed heat source at the bottom row based on financial exposure
	maxHeat := len(firePalette) - 1
	if m.totalRisk == 0 {
		maxHeat = 3 // Subtle low ember glow
	}

	bottomRow := (fireCanvasHeight - 1) * m.width
	for x := 0; x < m.width; x++ {
		m.fireBuffer[bottomRow+x] = rand.Intn(maxHeat + 1)
	}

	// 2. Propagate fire particles upwards with decay and wind drift
	for y := 1; y < fireCanvasHeight; y++ {
		for x := 0; x < m.width; x++ {
			srcIndex := y*m.width + x
			decay := rand.Intn(2)
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

func (m InspoCanvasModel) renderFireCanvas() string {
	var s strings.Builder
	for y := 0; y < fireCanvasHeight-1; y += 2 {
		for x := 0; x < m.width; x++ {
			topColorIdx := m.fireBuffer[y*m.width+x]
			botColorIdx := m.fireBuffer[(y+1)*m.width+x]

			cell := lipgloss.NewStyle().
				Foreground(lipgloss.Color(firePalette[botColorIdx])).
				Background(lipgloss.Color(firePalette[topColorIdx])).
				Render("▄")

			s.WriteString(cell)
		}
		s.WriteString("\n")
	}
	return s.String()
}

func (m InspoCanvasModel) renderTargetRadar(width int) string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(pink).Render("🎯 TARGET RADAR") + "\n\n")

	if len(m.issues) == 0 {
		s.WriteString(lipgloss.NewStyle().Foreground(green).Render("No targets acquired."))
		return radarBox.Width(width).Render(s.String())
	}

	for i, issue := range m.issues {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(white)

		if m.cursor == i {
			cursor = "❯ "
			if m.blink {
				cursor = "█ "
			}
			style = lipgloss.NewStyle().Foreground(pink).Bold(true)
		}

		s.WriteString(fmt.Sprintf("%s%s %s\n   └─ $%.4f/run\n",
			cursor,
			style.Render(issue.ID),
			issue.RuleName,
			issue.EstCostRisk,
		))
	}

	return radarBox.Width(width).Render(s.String())
}

func (m InspoCanvasModel) renderCodeInspector(width int) string {
	if len(m.issues) == 0 {
		return inspectorBox.Width(width).Render("No issue selected.")
	}

	selected := m.issues[m.cursor]
	snippet := getCodeScopeSnippet(selected.FilePath, selected.LineNumber)

	content := fmt.Sprintf(
		"🔍 %s [%s]\n📍 %s:%d\n💸 $%.4f/run\n\n%s",
		selected.RuleName, selected.ID,
		selected.FilePath, selected.LineNumber,
		selected.EstCostRisk,
		snippet,
	)

	return inspectorBox.Width(width).Render(content)
}

func getCodeScopeSnippet(filePath string, targetLine int) string {
	file, err := os.Open(filePath)
	if err != nil {
		return "  (unable to load file context)"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentLine := 0
	var snippet strings.Builder

	for scanner.Scan() {
		currentLine++
		if currentLine >= targetLine-2 && currentLine <= targetLine+2 {
			lineText := scanner.Text()
			pointer := "  "
			numStyle := lipgloss.NewStyle().Foreground(gray)

			if currentLine == targetLine {
				pointer = "⚡"
				numStyle = lipgloss.NewStyle().Foreground(pink).Bold(true)
				lineText = lipgloss.NewStyle().Bold(true).Render(lineText) + " ◄── [MONEY LEAK]"
			}

			snippet.WriteString(fmt.Sprintf("%s %s │ %s\n", pointer, numStyle.Render(fmt.Sprintf("%3d", currentLine)), lineText))
		}
	}
	return snippet.String()
}

// renderCFOBanner keeps the Virtual CFO persona pinned at the bottom of the
// dashboard, switching tone based on whether totalRisk has breached threshold.
func (m InspoCanvasModel) renderCFOBanner() string {
	label := "💼 VIRTUAL CFO (satisfied):"
	color := green
	if m.totalRisk > m.threshold {
		label = "💼 VIRTUAL CFO (furious):"
		color = pink
	}

	content := fmt.Sprintf("%s\n%s",
		lipgloss.NewStyle().Bold(true).Foreground(color).Render(label),
		lipgloss.NewStyle().Italic(true).Render(m.selectedQuote),
	)

	width := m.width - 2
	if width < 20 {
		width = 20
	}
	return quoteBox.Width(width).Render(content)
}

func (m InspoCanvasModel) View() string {
	var doc strings.Builder

	// 1. Termflix particle canvas
	doc.WriteString(m.renderFireCanvas() + "\n")

	// 2. Target radar (left) + code inspector (right)
	leftWidth := 28
	rightWidth := m.width - leftWidth - 8
	if rightWidth < 20 {
		rightWidth = 20
	}
	radar := m.renderTargetRadar(leftWidth)
	inspector := m.renderCodeInspector(rightWidth)
	doc.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, radar, inspector) + "\n")

	// 3. Virtual CFO persona banner, pinned at the bottom
	doc.WriteString(m.renderCFOBanner() + "\n")

	statusLine := fmt.Sprintf(" BURN: $%.2f / $%.2f │ FRAME: %04d │ [↑/↓] select │ [q] quit ",
		m.totalRisk, m.threshold, m.frameCount)
	doc.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#888888")).Render(statusLine))

	return doc.String()
}

// RunInspoCanvasTUI launches the Termflix particle canvas dashboard with the
// target radar, code inspector, and Virtual CFO persona banner.
func RunInspoCanvasTUI(issues []analyzer.Issue, totalRisk float64, threshold float64) error {
	p := tea.NewProgram(NewInspoCanvasModel(issues, totalRisk, threshold), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
