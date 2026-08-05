// Package tui provides the animated "Cost Reactor" Bubble Tea dashboard
// for FinOps-Guard, with a Doom-fire-based burn gauge and dollar-rain particles.
package tui

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

// --- Color & Palette ---

type rgb struct{ r, g, b uint8 }

func (c rgb) hex() string { return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b) }

// Candy palette: neon pink, magenta, coral, butter, lavender, mint
var candyRamp = []rgb{
	{0x0d, 0x07, 0x13}, // bg: deep plum
	{0x4b, 0x15, 0x28}, // dark magenta
	{0x99, 0x35, 0x56}, // magenta
	{0xd4, 0x53, 0x7e}, // magenta-pink
	{0xf0, 0x99, 0x7b}, // coral
	{0xfa, 0xc7, 0x75}, // butter
	{0xfa, 0xee, 0xda}, // off-white
}

var (
	colorBg        = lipgloss.Color("#0d0713")
	colorPink      = lipgloss.Color("#ED93B1")
	colorMagenta   = lipgloss.Color("#D4537E")
	colorCoral     = lipgloss.Color("#F0997B")
	colorButter    = lipgloss.Color("#FAC775")
	colorLavender  = lipgloss.Color("#AFA9EC")
	colorMint      = lipgloss.Color("#5DCAA5")
	colorRed       = lipgloss.Color("#F09595")
)

func heatColor(t float64) rgb {
	if t <= 0 {
		return candyRamp[0]
	}
	if t >= 1 {
		return candyRamp[len(candyRamp)-1]
	}
	f := t * float64(len(candyRamp)-1)
	i := int(f)
	frac := f - float64(i)
	a, b := candyRamp[i], candyRamp[i+1]
	lerp := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*frac) }
	return rgb{lerp(a.r, b.r), lerp(a.g, b.g), lerp(a.b, b.b)}
}

// --- Fire Simulation ---

type fire struct {
	w, h      int
	heat      []float64
	intensity float64
	rng       *rand.Rand
}

func newFire(w, h int) *fire {
	return &fire{
		w:    w,
		h:    h,
		heat: make([]float64, w*h),
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
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
			bot := candyRamp[0]
			if y+1 < f.h {
				bot = heatColor(f.heat[(y+1)*f.w+x])
			}
			st := lipgloss.NewStyle().
				Foreground(lipgloss.Color(top.hex())).
				Background(lipgloss.Color(bot.hex()))
			ch := "▀"
			if r, ok := overlay[[2]int{x, y / 2}]; ok {
				st = lipgloss.NewStyle().
					Foreground(colorButter).
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

// --- Particles & Status ---

type particle struct {
	x     int
	y     float64
	speed float64
}

type Status int

const (
	StatusSafe Status = iota
	StatusSweating
	StatusOnFire
	StatusInferno
)

func (s Status) String() string {
	switch s {
	case StatusSafe:
		return "SAFE"
	case StatusSweating:
		return "SWEATING"
	case StatusOnFire:
		return "ON FIRE"
	case StatusInferno:
		return "INFERNO"
	default:
		return "?"
	}
}

func (s Status) Color() lipgloss.Color {
	switch s {
	case StatusSafe:
		return colorMint
	case StatusSweating:
		return colorButter
	case StatusOnFire:
		return colorRed
	case StatusInferno:
		return colorPink
	default:
		return colorLavender
	}
}

func statusFromRisk(totalRisk, threshold float64) Status {
	if threshold == 0 {
		return StatusSafe
	}
	ratio := totalRisk / threshold
	if ratio < 0.25 {
		return StatusSafe
	} else if ratio < 0.6 {
		return StatusSweating
	} else if ratio < 1.0 {
		return StatusOnFire
	}
	return StatusInferno
}

var quotes = map[Status][]string{
	StatusSafe: {
		`"Finally, an engineering team that respects a budget."`,
		`"This is the kind of efficiency that gets you a bonus."`,
		`"Cloud bills go down, morale goes up."`,
	},
	StatusSweating: {
		`"Uh... is anyone else seeing these numbers?"`,
		`"Let's just... not look at the bill this month."`,
		`"This is probably fine, right?"`,
	},
	StatusOnFire: {
		`"THIS IS FINE." [sips coffee as everything burns]`,
		`"I've made some questionable decisions."`,
		`"Why is the smoke detector going off?"`,
	},
	StatusInferno: {
		`"If this loop hits production, our cloud bill will surpass our revenue."`,
		`"My cloud bill alert just gave me a heart attack."`,
		`"We're downgrading office coffee to fund this loop."`,
	},
}

// --- Bubble Tea Model ---

type TickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second/24, func(t time.Time) tea.Msg { return TickMsg(t) })
}

type CostReactor struct {
	issues        []analyzer.Issue
	cursor        int
	totalRisk     float64
	threshold     float64
	width         int
	height        int
	fire          *fire
	fireRows      int
	particles     []particle
	frameCount    int
	bootCountdown int
	shakeFrame    int
	selectedQuote string
	lastQuoteTime int
	rng           *rand.Rand
}

func NewCostReactor(issues []analyzer.Issue, totalRisk, threshold float64) *CostReactor {
	rand.Seed(time.Now().UnixNano())
	width, fireRows := 120, 10
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	status := statusFromRisk(totalRisk, threshold)
	var quote string
	if pool, ok := quotes[status]; ok && len(pool) > 0 {
		quote = pool[rng.Intn(len(pool))]
	}
	return &CostReactor{
		issues:        issues,
		cursor:        0,
		totalRisk:     totalRisk,
		threshold:     threshold,
		width:         width,
		height:        40,
		fire:          newFire(width, fireRows*2),
		fireRows:      fireRows,
		bootCountdown: 48, // ~2s at 24fps
		selectedQuote: quote,
		rng:           rng,
	}
}

func (m *CostReactor) Init() tea.Cmd {
	return tick()
}

func (m *CostReactor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 60 && msg.Height > 20 {
			m.width = msg.Width
			m.height = msg.Height
			m.fire = newFire(m.width, m.fireRows*2)
		}

	case TickMsg:
		m.frameCount++
		m.lastQuoteTime++

		if m.shakeFrame > 0 {
			m.shakeFrame--
		}

		if m.bootCountdown > 0 {
			m.bootCountdown--
		} else {
			intensity := m.totalRisk / m.threshold
			if intensity > 1.0 {
				intensity = 1.0
			}
			m.fire.SetIntensity(intensity)
			m.fire.Step()

			// Spawn particles
			spawnRate := int(math.Max(1, m.totalRisk/10))
			for i := 0; i < spawnRate && len(m.particles) < 60; i++ {
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

			// Rotate quote every ~8 seconds
			if m.lastQuoteTime > 192 { // 192 frames * 24fps ≈ 8s
				status := statusFromRisk(m.totalRisk, m.threshold)
				if pool, ok := quotes[status]; ok && len(pool) > 0 {
					m.selectedQuote = pool[m.rng.Intn(len(pool))]
				}
				m.lastQuoteTime = 0
			}
		}
		return m, tick()

	case tea.KeyMsg:
		if m.bootCountdown > 0 {
			m.bootCountdown = 0
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
		}
	}
	return m, nil
}

func (m *CostReactor) View() string {
	if m.bootCountdown > 0 {
		return m.renderBootSequence()
	}

	status := statusFromRisk(m.totalRisk, m.threshold)

	// Particle overlay
	overlay := make(map[[2]int]rune, len(m.particles))
	for _, p := range m.particles {
		overlay[[2]int{p.x, int(p.y)}] = '$'
	}

	var doc strings.Builder

	// Header bar
	headerLeft := lipgloss.NewStyle().Foreground(colorPink).Bold(true).Render("🛡️ FINOPS-GUARD")
	headerRight := lipgloss.NewStyle().Foreground(status.Color()).Bold(true).Render(fmt.Sprintf("STATUS: %s", status))
	headerVersion := lipgloss.NewStyle().Foreground(colorLavender).Render("v0.1.0")
	headerBar := fmt.Sprintf("%s %s %s", headerLeft, headerVersion, headerRight)
	doc.WriteString(headerBar + "\n\n")

	// Code inspector (simplified for now)
	if len(m.issues) > 0 {
		selected := m.issues[m.cursor]
		inspector := fmt.Sprintf("📍 %s:%d — $%.4f/run  [%s]", selected.FilePath, selected.LineNumber, selected.EstCostRisk, selected.ID)
		doc.WriteString(lipgloss.NewStyle().Foreground(colorCoral).Render(inspector) + "\n\n")
	}

	// Findings list
	doc.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorPink).Render("🔥 DETECTED LEAKS") + "\n")
	if len(m.issues) == 0 {
		doc.WriteString(lipgloss.NewStyle().Foreground(colorMint).Render("None.\n\n"))
	} else {
		for i, issue := range m.issues {
			marker := "  "
			if i == m.cursor {
				marker = "▌ " // Left rule for selected
			}
			doc.WriteString(fmt.Sprintf("%s[%s] $%.4f/run — %s\n", marker, issue.ID, issue.EstCostRisk, issue.RuleName))
		}
		doc.WriteString("\n")
	}

	// Fire canvas
	doc.WriteString(m.fire.RenderHalfBlocks(overlay))
	doc.WriteString("\n\n")

	// CFO quote bar
	cfoStyle := lipgloss.NewStyle().Italic(true).Foreground(status.Color())
	doc.WriteString(lipgloss.NewStyle().Foreground(colorLavender).Render("💼 CFO: "))
	doc.WriteString(cfoStyle.Render(m.selectedQuote) + "\n")

	// Status bar
	doc.WriteString("\n")
	burnPill := lipgloss.NewStyle().Bold(true).Foreground(colorBg).Background(status.Color()).Padding(0, 1).Render(fmt.Sprintf("BURN: $%.2f / $%.2f", m.totalRisk, m.threshold))
	frameStr := fmt.Sprintf("FRAME: %04d │ [↑/↓] navigate  [q] quit", m.frameCount)
	doc.WriteString(burnPill + " " + lipgloss.NewStyle().Foreground(colorLavender).Render(frameStr))

	return doc.String()
}

func (m *CostReactor) renderBootSequence() string {
	progress := float64(48-m.bootCountdown) / 48
	scanline := int(float64(m.height-8) * progress)

	var doc strings.Builder
	doc.WriteString("\n")
	doc.WriteString(lipgloss.NewStyle().Foreground(colorPink).Bold(true).Render("  ███████████████████████████\n"))
	doc.WriteString(lipgloss.NewStyle().Foreground(colorMagenta).Render("  🛡️  F I N O P S - G U A R D\n"))
	doc.WriteString(lipgloss.NewStyle().Foreground(colorPink).Bold(true).Render("  ███████████████████████████\n\n"))

	doc.WriteString("  INITIALIZING COST REACTOR...\n\n")

	for i := 0; i < 6; i++ {
		if i < scanline {
			doc.WriteString(lipgloss.NewStyle().Foreground(colorMint).Render("  ✓ SYSTEMS ONLINE\n"))
		} else {
			doc.WriteString("  • SYSTEMS ONLINE\n")
		}
	}

	return doc.String()
}

// Run starts the Cost Reactor TUI.
func Run(issues []analyzer.Issue, totalRisk, threshold float64) error {
	p := tea.NewProgram(NewCostReactor(issues, totalRisk, threshold), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
