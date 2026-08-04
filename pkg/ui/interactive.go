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

// Frame Animation Ticker
type TickMsg time.Time

func doTick() tea.Cmd {
	return tea.Tick(time.Millisecond*150, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Styling Palette matching SciPNET & Terminal Realism
var (
	purple = lipgloss.Color("#7D56F4")
	pink   = lipgloss.Color("#FF0055")
	yellow = lipgloss.Color("#FFCC00")
	green  = lipgloss.Color("#04B575")
	gray   = lipgloss.Color("#4A4A4A")
	white  = lipgloss.Color("#FFFFFF")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(white).Background(purple).Padding(0, 1)
	gridBox    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(purple).Padding(0, 1)
	hazardBox  = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(pink).Padding(0, 1)
	statusBar  = lipgloss.NewStyle().Foreground(white).Background(purple).Padding(0, 1)
	pinkStatus = lipgloss.NewStyle().Foreground(white).Background(pink).Bold(true).Padding(0, 1)
)

var cfoQuotes = []string{
	`"If this loop hits production, our cloud bill will surpass our revenue."`,
	`"My cloud bill alert notification just gave me a heart attack."`,
	`"Is this an AI agent or a money incineration engine?"`,
	`"We're going to have to downgrade the office coffee to fund this loop."`,
}

type FinalTUIModel struct {
	issues        []analyzer.Issue
	cursor        int
	totalRisk     float64
	threshold     float64
	frame         int
	blink         bool
	selectedQuote string
}

func NewFinalTUIModel(issues []analyzer.Issue, totalRisk float64, threshold float64) FinalTUIModel {
	rand.Seed(time.Now().UnixNano())
	quote := cfoQuotes[rand.Intn(len(cfoQuotes))]
	return FinalTUIModel{
		issues:        issues,
		cursor:        0,
		totalRisk:     totalRisk,
		threshold:     threshold,
		frame:         0,
		blink:         true,
		selectedQuote: quote,
	}
}

func (m FinalTUIModel) Init() tea.Cmd {
	return doTick()
}

func (m FinalTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		m.frame++
		m.blink = !m.blink
		return m, doTick()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				fmt.Print("\a") // Subtle terminal bell sound
			}
		case "down", "j":
			if m.cursor < len(m.issues)-1 {
				m.cursor++
				fmt.Print("\a")
			}
		}
	}
	return m, nil
}

func (m FinalTUIModel) View() string {
	var doc strings.Builder

	// 1. Header Banner
	doc.WriteString(titleStyle.Render("🛡️  FINOPS-GUARD :: TACTICAL COCKPIT HUD") + "\n\n")

	// 2. Money Burn ASCII Flame Graphic Header
	doc.WriteString(m.renderFlameHeader() + "\n\n")

	if len(m.issues) == 0 {
		doc.WriteString(lipgloss.NewStyle().Foreground(green).Bold(true).Render("✨ AAA REPO HEALTH! Zero loop cost exposure detected. Wall Street level efficiency!\n\nPress 'q' to exit."))
		return doc.String()
	}

	// 3. Main 2-Column Split: Findings List (Left) vs Code Inspector (Right)
	leftCol := m.renderFindingsList(28)
	rightCol := m.renderCodeInspector(48)
	mainSplit := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
	doc.WriteString(mainSplit + "\n\n")

	// 4. Virtual CFO Persona Quote Box
	quoteContent := fmt.Sprintf("💼 VIRTUAL CFO SAYS:\n%s", lipgloss.NewStyle().Foreground(yellow).Italic(true).Render(m.selectedQuote))
	doc.WriteString(gridBox.Render(quoteContent) + "\n\n")

	// 5. Termflix-Style Status Bar Footer
	doc.WriteString(m.renderStatusBar() + "\n")

	return doc.String()
}

func (m FinalTUIModel) renderFlameHeader() string {
	percent := (m.totalRisk / m.threshold) * 100
	if percent > 100 {
		percent = 100
	}

	barLen := 30
	filled := int((percent / 100) * float64(barLen))

	flameChars := []string{"▄", "█", "🔥", "▀"}
	var flameStr strings.Builder
	for i := 0; i < barLen; i++ {
		if i < filled {
			char := flameChars[(m.frame+i)%len(flameChars)]
			flameStr.WriteString(lipgloss.NewStyle().Foreground(pink).Render(char))
		} else {
			flameStr.WriteString(lipgloss.NewStyle().Foreground(gray).Render("░"))
		}
	}

	statusMsg := "SAFE"
	statusColor := green
	if m.totalRisk > m.threshold {
		statusMsg = "🚨 THRESHOLD EXCEEDED"
		statusColor = pink
	}

	return gridBox.Render(fmt.Sprintf(
		"BURN RATE: [%s] $%.2f / $%.2f (%.0f%%)\nSTATUS:    %s",
		flameStr.String(), m.totalRisk, m.threshold, (m.totalRisk/m.threshold)*100,
		lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(statusMsg),
	))
}

func (m FinalTUIModel) renderFindingsList(width int) string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(purple).Render("FINDINGS (↑/↓)") + "\n\n")

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

		s.WriteString(fmt.Sprintf("%s%s %s\n   └─ $%.2f/run\n",
			cursor,
			style.Render(issue.ID),
			issue.RuleName,
			issue.EstCostRisk,
		))
	}

	return gridBox.Width(width).Render(s.String())
}

func (m FinalTUIModel) renderCodeInspector(width int) string {
	if len(m.issues) == 0 {
		return gridBox.Width(width).Render("No issues selected.")
	}

	selected := m.issues[m.cursor]
	snippet := getCodeScopeSnippet(selected.FilePath, selected.LineNumber)

	content := fmt.Sprintf(
		"🔍 INSPECTOR: %s [%s]\n📍 %s:%d\n💸 Est. Waste: $%.2f/run\n\nCODE SCOPE VISUALIZER:\n%s\n💡 REMEDIATION:\nBatch request parameters outside loop scope.",
		selected.RuleName, selected.ID,
		selected.FilePath, selected.LineNumber,
		selected.EstCostRisk,
		snippet,
	)

	return hazardBox.Width(width).Render(content)
}

func getCodeScopeSnippet(filePath string, targetLine int) string {
	file, err := os.Open(filePath)
	if err != nil {
		return "   (Unable to load file context)"
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
			lineNumStyle := lipgloss.NewStyle().Foreground(gray)

			if currentLine == targetLine {
				pointer = "⚡"
				lineNumStyle = lipgloss.NewStyle().Foreground(pink).Bold(true)
				lineText = lipgloss.NewStyle().Bold(true).Render(lineText) + " ◄── [MONEY LEAK]"
			}

			snippet.WriteString(fmt.Sprintf("%s %s │ %s\n", pointer, lineNumStyle.Render(fmt.Sprintf("%3d", currentLine)), lineText))
		}
	}
	return snippet.String()
}

func (m FinalTUIModel) renderStatusBar() string {
	statusTag := pinkStatus.Render("STATUS: BREACHED")
	if m.totalRisk <= m.threshold {
		statusTag = statusBar.Render("STATUS: SAFE")
	}

	info := statusBar.Render(fmt.Sprintf("FINOPS-GUARD v0.1.0 │ FRAME: %04d │ TrueColor 24FPS │ [q] Quit", m.frame))
	return lipgloss.JoinHorizontal(lipgloss.Top, statusTag, info)
}

func RunFinalTUI(issues []analyzer.Issue, totalRisk float64, threshold float64) error {
	p := tea.NewProgram(NewFinalTUIModel(issues, totalRisk, threshold))
	_, err := p.Run()
	return err
}
