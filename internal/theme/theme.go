// Package theme holds the colors the terminal views share: the Bioluminescent direction of the
// hive page, in light and dark.
package theme

import "github.com/charmbracelet/lipgloss"

var (
	Green = lipgloss.AdaptiveColor{Light: "#12a37a", Dark: "#54f0b0"}
	Amber = lipgloss.AdaptiveColor{Light: "#c98a12", Dark: "#ffc65c"}
	Rose  = lipgloss.AdaptiveColor{Light: "#d64570", Dark: "#ff6f91"}
	Slate = lipgloss.AdaptiveColor{Light: "#8aa3a8", Dark: "#6d8a92"}
	Blue  = lipgloss.AdaptiveColor{Light: "#2f7fd1", Dark: "#6ab8ff"}
	Mist  = lipgloss.AdaptiveColor{Light: "#b6cccf", Dark: "#34525a"}

	HubFill = lipgloss.AdaptiveColor{Light: "#0b3036", Dark: "#0b2b33"}
	HubInk  = lipgloss.AdaptiveColor{Light: "#8ff5e8", Dark: "#5ef2e0"}
	Muted   = lipgloss.AdaptiveColor{Light: "#4d7a80", Dark: "#6fa5ad"}
	Rule    = lipgloss.AdaptiveColor{Light: "#b6cccf", Dark: "#2a4a52"}
)
