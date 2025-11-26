package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	normalStyle     = lipgloss.NewStyle()
	cursorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	dimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type Item struct {
	Label string
	Value string
}

type selectorModel struct {
	items       []Item
	filtered    []Item
	cursor      int
	selected    string
	textInput   textinput.Model
	quitting    bool
	multiSelect bool
	checked     map[int]bool
	cancelled   bool
}

func newSelectorModel(items []Item, multiSelect bool) selectorModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()

	return selectorModel{
		items:       items,
		filtered:    items,
		textInput:   ti,
		multiSelect: multiSelect,
		checked:     make(map[int]bool),
	}
}

func (m selectorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if len(m.filtered) > 0 {
				if m.multiSelect {
					// Return checked items
				} else {
					m.selected = m.filtered[m.cursor].Value
				}
			}
			m.quitting = true
			return m, tea.Quit
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "tab", " ":
			if m.multiSelect && len(m.filtered) > 0 {
				idx := m.findOriginalIndex(m.filtered[m.cursor])
				m.checked[idx] = !m.checked[idx]
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
				}
			}
		default:
			m.textInput, cmd = m.textInput.Update(msg)
			m.filterItems()
			return m, cmd
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *selectorModel) filterItems() {
	query := strings.ToLower(m.textInput.Value())
	if query == "" {
		m.filtered = m.items
		m.cursor = 0
		return
	}

	var filtered []Item
	for _, item := range m.items {
		if fuzzyMatch(strings.ToLower(item.Label), query) {
			filtered = append(filtered, item)
		}
	}
	m.filtered = filtered
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m selectorModel) findOriginalIndex(item Item) int {
	for i, it := range m.items {
		if it.Value == item.Value {
			return i
		}
	}
	return -1
}

func fuzzyMatch(s, query string) bool {
	qi := 0
	for _, c := range s {
		if qi < len(query) && byte(c) == query[qi] {
			qi++
		}
	}
	return qi == len(query)
}

func (m selectorModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	for i, item := range m.filtered {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
		}

		check := ""
		if m.multiSelect {
			originalIdx := m.findOriginalIndex(item)
			if m.checked[originalIdx] {
				check = selectedStyle.Render("[x] ")
			} else {
				check = dimStyle.Render("[ ] ")
			}
		}

		label := item.Label
		if i == m.cursor {
			label = selectedStyle.Render(label)
		} else {
			label = normalStyle.Render(label)
		}

		b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, check, label))
	}

	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("  No matches"))
	}

	if m.multiSelect {
		b.WriteString(dimStyle.Render("\n\nTAB/SPACE to select, ENTER to confirm, ESC to cancel"))
	} else {
		b.WriteString(dimStyle.Render("\n\nENTER to select, ESC to cancel"))
	}

	return b.String()
}

// Select shows a single-select fuzzy finder and returns the selected item's value.
func Select(items []Item) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no items to select")
	}

	// Open /dev/tty directly to ensure TUI works even when stdout is captured
	// (e.g., in shell command substitution like result=$(wt cd --print-path))
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return "", fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	defer tty.Close()

	m := newSelectorModel(items, false)
	p := tea.NewProgram(m, tea.WithInput(tty), tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	result := finalModel.(selectorModel)
	if result.cancelled {
		return "", nil
	}
	return result.selected, nil
}

// MultiSelect shows a multi-select fuzzy finder and returns the selected items' values.
func MultiSelect(items []Item) ([]string, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select")
	}

	// Open /dev/tty directly to ensure TUI works even when stdout is captured
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	defer tty.Close()

	m := newSelectorModel(items, true)
	p := tea.NewProgram(m, tea.WithInput(tty), tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	result := finalModel.(selectorModel)
	if result.cancelled {
		return nil, nil
	}

	var selected []string
	for i, item := range result.items {
		if result.checked[i] {
			selected = append(selected, item.Value)
		}
	}
	return selected, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
