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

// white is a fixed neutral text/background-contrast color, independent of theme.
var white = lipgloss.Color("#FFFFFF")

var cfoQuotes = []string{
	`"If this loop hits production, our cloud bill will surpass our revenue."`,
	`"My cloud bill alert notification just gave me a heart attack."`,
	`"Is this an AI agent or a money incineration engine?"`,
	`"We're going to have to downgrade the office coffee to fund this loop."`,
}

type FinalTUIModel struct {
	theme         Theme
	issues        []analyzer.Issue
	cursor        int
	totalRisk     float64
	threshold     float64
	frame         int
	blink         bool
	selectedQuote string
}

func NewFinalTUIModel(issues []analyzer.Issue, totalRisk float64, threshold float64, theme Theme) FinalTUIModel {
	rand.Seed(time.Now().UnixNano())
	quote := cfoQuotes[rand.Intn(len(cfoQuotes))]
	return FinalTUIModel{
		theme:         theme,
		issues:        issues,
		cursor:        0,
		totalRisk:     totalRisk,
		threshold:     threshold,
		frame:         0,
		blink:         true,
		selectedQuote: quote,
	}
}

func (m FinalTUIModel) titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(white).Background(m.theme.Primary).Padding(0, 1)
}

func (m FinalTUIModel) gridBox() lipgloss.Style {
	return lipgloss.NewStyle().Border(m.theme.Border).BorderForeground(m.theme.BorderColor).Padding(0, 1)
}

func (m FinalTUIModel) hazardBox() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(m.theme.Accent).Padding(0, 1)
}

func (m FinalTUIModel) statusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(white).Background(m.theme.Primary).Padding(0, 1)
}

func (m FinalTUIModel) alertStatusStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(white).Background(m.theme.Accent).Bold(true).Padding(0, 1)
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
	doc.WriteString(m.titleStyle().Render("🛡️  FINOPS-GUARD :: COCKPIT HUD ["+strings.ToUpper(m.theme.Name)+"]") + "\n\n")

	// 2. Money Burn ASCII Flame Graphic Header
	doc.WriteString(m.renderFlameHeader() + "\n\n")

	if len(m.issues) == 0 {
		doc.WriteString(lipgloss.NewStyle().Foreground(m.theme.Primary).Bold(true).Render("✨ AAA REPO HEALTH! Zero loop cost exposure detected. Wall Street level efficiency!\n\nPress 'q' to exit."))
		return doc.String()
	}

	// 3. Main 2-Column Split: Findings List (Left) vs Code Inspector (Right)
	leftCol := m.renderFindingsList(28)
	rightCol := m.renderCodeInspector(48)
	mainSplit := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
	doc.WriteString(mainSplit + "\n\n")

	// 4. Virtual CFO Persona Quote Box
	quoteContent := fmt.Sprintf("💼 VIRTUAL CFO SAYS:\n%s", lipgloss.NewStyle().Foreground(m.theme.Secondary).Italic(true).Render(m.selectedQuote))
	doc.WriteString(m.gridBox().Render(quoteContent) + "\n\n")

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
			flameStr.WriteString(lipgloss.NewStyle().Foreground(m.theme.Accent).Render(char))
		} else {
			flameStr.WriteString(lipgloss.NewStyle().Foreground(m.theme.Muted).Render("░"))
		}
	}

	statusMsg := "SAFE"
	statusColor := m.theme.Primary
	if m.totalRisk > m.threshold {
		statusMsg = "🚨 THRESHOLD EXCEEDED"
		statusColor = m.theme.Accent
	}

	return m.gridBox().Render(fmt.Sprintf(
		"BURN RATE: [%s] $%.2f / $%.2f (%.0f%%)\nSTATUS:    %s",
		flameStr.String(), m.totalRisk, m.threshold, (m.totalRisk/m.threshold)*100,
		lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(statusMsg),
	))
}

func (m FinalTUIModel) renderFindingsList(width int) string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(m.theme.Primary).Render("FINDINGS (↑/↓)") + "\n\n")

	for i, issue := range m.issues {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(white)

		if m.cursor == i {
			cursor = "❯ "
			if m.blink {
				cursor = "█ "
			}
			style = lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
		}

		s.WriteString(fmt.Sprintf("%s%s %s\n   └─ $%.2f/run\n",
			cursor,
			style.Render(issue.ID),
			issue.RuleName,
			issue.EstCostRisk,
		))
	}

	return m.gridBox().Width(width).Render(s.String())
}

func (m FinalTUIModel) renderCodeInspector(width int) string {
	if len(m.issues) == 0 {
		return m.gridBox().Width(width).Render("No issues selected.")
	}

	selected := m.issues[m.cursor]
	snippet := getCodeScopeSnippet(selected.FilePath, selected.LineNumber, m.theme)

	content := fmt.Sprintf(
		"🔍 INSPECTOR: %s [%s]\n📍 %s:%d\n💸 Est. Waste: $%.2f/run\n\nCODE SCOPE VISUALIZER:\n%s\n💡 REMEDIATION:\nBatch request parameters outside loop scope.",
		selected.RuleName, selected.ID,
		selected.FilePath, selected.LineNumber,
		selected.EstCostRisk,
		snippet,
	)

	return m.hazardBox().Width(width).Render(content)
}

func getCodeScopeSnippet(filePath string, targetLine int, theme Theme) string {
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
			lineNumStyle := lipgloss.NewStyle().Foreground(theme.Muted)

			if currentLine == targetLine {
				pointer = "⚡"
				lineNumStyle = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
				lineText = lipgloss.NewStyle().Bold(true).Render(lineText) + " ◄── [MONEY LEAK]"
			}

			snippet.WriteString(fmt.Sprintf("%s %s │ %s\n", pointer, lineNumStyle.Render(fmt.Sprintf("%3d", currentLine)), lineText))
		}
	}
	return snippet.String()
}

func (m FinalTUIModel) renderStatusBar() string {
	statusTag := m.alertStatusStyle().Render("STATUS: BREACHED")
	if m.totalRisk <= m.threshold {
		statusTag = m.statusBarStyle().Render("STATUS: SAFE")
	}

	info := m.statusBarStyle().Render(fmt.Sprintf("FINOPS-GUARD v0.1.0 │ FRAME: %04d │ TrueColor 24FPS │ [q] Quit", m.frame))
	return lipgloss.JoinHorizontal(lipgloss.Top, statusTag, info)
}

func RunFinalTUI(issues []analyzer.Issue, totalRisk float64, threshold float64, theme Theme) error {
	p := tea.NewProgram(NewFinalTUIModel(issues, totalRisk, threshold, theme))
	_, err := p.Run()
	return err
}
