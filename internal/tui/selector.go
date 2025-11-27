package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/default-anton/wt/internal/styles"
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
		case "tab":
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
			cursor = styles.CursorStyle.Render("> ")
		}

		check := ""
		if m.multiSelect {
			originalIdx := m.findOriginalIndex(item)
			if m.checked[originalIdx] {
				check = styles.BranchStyle.Render("[x] ")
			} else {
				check = styles.DimStyle.Render("[ ] ")
			}
		}

		label := item.Label
		if i == m.cursor {
			label = styles.BranchStyle.Render(label)
		} else {
			label = styles.NormalStyle.Render(label)
		}

		b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, check, label))
	}

	if len(m.filtered) == 0 {
		b.WriteString(styles.DimStyle.Render("  No matches"))
	}

	if m.multiSelect {
		b.WriteString(styles.DimStyle.Render("\n\nTAB to select, ENTER to confirm, ESC to cancel"))
	} else {
		b.WriteString(styles.DimStyle.Render("\n\nENTER to select, ESC to cancel"))
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
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	defer tty.Close()

	m := newSelectorModel(items, false)
	p := tea.NewProgram(
		m,
		tea.WithInput(tty),
		tea.WithOutput(tty),
	)
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
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	defer tty.Close()

	m := newSelectorModel(items, true)
	p := tea.NewProgram(
		m,
		tea.WithInput(tty),
		tea.WithOutput(tty),
	)
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

// confirmModel is a simple yes/no confirmation prompt.
type confirmModel struct {
	message  string
	selected bool
	quitting bool
	result   bool
}

func newConfirmModel(message string) confirmModel {
	return confirmModel{
		message:  message,
		selected: false,
	}
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			m.result = false
			return m, tea.Quit
		case "enter":
			m.quitting = true
			m.result = m.selected
			return m, tea.Quit
		case "left", "h":
			m.selected = true
		case "right", "l":
			m.selected = false
		case "y", "Y":
			m.quitting = true
			m.result = true
			return m, tea.Quit
		case "n", "N":
			m.quitting = true
			m.result = false
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.message)
	b.WriteString(" ")

	yes := "Yes"
	no := "No"

	if m.selected {
		yes = styles.BranchStyle.Render("[Yes]")
		no = styles.DimStyle.Render(" No ")
	} else {
		yes = styles.DimStyle.Render(" Yes ")
		no = styles.BranchStyle.Render("[No]")
	}

	b.WriteString(yes)
	b.WriteString(" / ")
	b.WriteString(no)
	b.WriteString(styles.DimStyle.Render("  (y/n, ←/→ to select, enter to confirm)"))

	return b.String()
}

// Confirm shows a yes/no confirmation prompt and returns true if the user selects Yes.
func Confirm(message string) (bool, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false, fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	defer tty.Close()

	m := newConfirmModel(message)
	p := tea.NewProgram(
		m,
		tea.WithInput(tty),
		tea.WithOutput(tty),
	)
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	result := finalModel.(confirmModel)
	return result.result, nil
}
