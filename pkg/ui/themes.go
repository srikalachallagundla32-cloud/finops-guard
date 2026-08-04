package ui

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette and border style applied to CLI output.
type Theme struct {
	Name        string
	Primary     lipgloss.Color
	Secondary   lipgloss.Color
	Accent      lipgloss.Color
	Muted       lipgloss.Color
	Border      lipgloss.Border
	BorderColor lipgloss.Color
}

// Tactical: hazard tape / lime green, thick borders.
var Tactical = Theme{
	Name:        "tactical",
	Primary:     lipgloss.Color("#39FF14"),
	Secondary:   lipgloss.Color("#B5B5B5"),
	Accent:      lipgloss.Color("#FFD400"),
	Muted:       lipgloss.Color("#4B5320"),
	Border:      lipgloss.ThickBorder(),
	BorderColor: lipgloss.Color("#FFD400"),
}

// Retro: amber CRT terminal, heavy double-line borders.
var Retro = Theme{
	Name:        "retro",
	Primary:     lipgloss.Color("#FFB000"),
	Secondary:   lipgloss.Color("#FF8C00"),
	Accent:      lipgloss.Color("#FFCC66"),
	Muted:       lipgloss.Color("#805800"),
	Border:      lipgloss.DoubleBorder(),
	BorderColor: lipgloss.Color("#FFB000"),
}

// Vector: neon pink/cyan ASCII schematic, blueprint-style corners.
var Vector = Theme{
	Name:      "vector",
	Primary:   lipgloss.Color("#FF2E9A"),
	Secondary: lipgloss.Color("#00FFF0"),
	Accent:    lipgloss.Color("#00FFF0"),
	Muted:     lipgloss.Color("#7A2E63"),
	Border: lipgloss.Border{
		Top:         "-",
		Bottom:      "-",
		Left:        "|",
		Right:       "|",
		TopLeft:     "+",
		TopRight:    "+",
		BottomLeft:  "+",
		BottomRight: "+",
	},
	BorderColor: lipgloss.Color("#00FFF0"),
}

var themes = map[string]Theme{
	Tactical.Name: Tactical,
	Retro.Name:    Retro,
	Vector.Name:   Vector,
}

// ThemeNames returns the valid --theme flag values, in a stable order.
func ThemeNames() []string {
	return []string{Tactical.Name, Retro.Name, Vector.Name}
}

// ByName looks up a theme by its flag value.
func ByName(name string) (Theme, bool) {
	t, ok := themes[name]
	return t, ok
}
