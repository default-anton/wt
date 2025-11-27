package styles

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

func init() {
	// Force color support detection when running in command substitution
	// where stdout might be captured but /dev/tty is available
	if os.Getenv("CLICOLOR_FORCE") == "" {
		os.Setenv("CLICOLOR_FORCE", "1")
	}
}

var (
	// BranchStyle is used for highlighting branch names (purple/magenta)
	BranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))

	// DimStyle is used for dimmed text like paths and help text (gray)
	DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// CursorStyle is used for cursor indicator and badges (cyan)
	CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	// NormalStyle is the default style with no formatting
	NormalStyle = lipgloss.NewStyle()
)
