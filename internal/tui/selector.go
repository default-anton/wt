package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"

	"github.com/default-anton/wt/internal/styles"
)

type Item struct {
	Label string
	Value string
}

// scoredItem holds an item with its fuzzy match score and positions.
type scoredItem struct {
	item      Item
	score     int
	positions []int // indices of matched characters in Label
	origIndex int   // original index in items slice (for multi-select)
}

type selectorModel struct {
	items       []Item
	filtered    []scoredItem
	cursor      int
	selected    string
	textInput   textinput.Model
	quitting    bool
	multiSelect bool
	checked     map[int]bool
	cancelled   bool
	slab        *util.Slab
}

func newSelectorModel(items []Item, multiSelect bool) selectorModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()

	// Convert initial items to scoredItems with no match positions
	filtered := make([]scoredItem, len(items))
	for i, item := range items {
		filtered[i] = scoredItem{
			item:      item,
			score:     0,
			positions: nil,
			origIndex: i,
		}
	}

	return selectorModel{
		items:       items,
		filtered:    filtered,
		textInput:   ti,
		multiSelect: multiSelect,
		checked:     make(map[int]bool),
		slab:        util.MakeSlab(100, 2048),
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
					m.selected = m.filtered[m.cursor].item.Value
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
				idx := m.filtered[m.cursor].origIndex
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
	query := m.textInput.Value()

	// Empty query: show all items in original order with no highlights
	if query == "" {
		m.filtered = make([]scoredItem, len(m.items))
		for i, item := range m.items {
			m.filtered[i] = scoredItem{
				item:      item,
				score:     0,
				positions: nil,
				origIndex: i,
			}
		}
		m.cursor = 0
		return
	}

	// Convert query to lowercase runes for case-insensitive matching
	patternRunes := []rune(strings.ToLower(query))

	var scored []scoredItem

	for i, item := range m.items {
		// Convert item label to util.Chars
		chars := util.ToChars([]byte(item.Label))

		// Call FuzzyMatchV2:
		// - caseSensitive: false (case-insensitive matching)
		// - normalize: true (normalize unicode)
		// - forward: true (match left-to-right)
		// - withPos: true (we need positions for highlighting)
		result, positions := algo.FuzzyMatchV2(
			false,        // caseSensitive
			true,         // normalize
			true,         // forward
			&chars,       // input text
			patternRunes, // pattern (already lowercase)
			true,         // withPos (need positions for highlighting)
			m.slab,       // reusable memory slab
		)

		// Score > 0 means we have a match
		if result.Score > 0 {
			var posSlice []int
			if positions != nil {
				posSlice = make([]int, len(*positions))
				copy(posSlice, *positions)
			}

			scored = append(scored, scoredItem{
				item:      item,
				score:     result.Score,
				positions: posSlice,
				origIndex: i,
			})
		}
	}

	// Sort by score descending (best matches first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	m.filtered = scored

	// Reset cursor, ensure it's within bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// renderHighlightedLabel renders a label with matched characters highlighted.
// positions contains the indices of matched characters.
// baseStyle is applied to non-matched characters.
func renderHighlightedLabel(label string, positions []int, baseStyle, matchStyle lipgloss.Style) string {
	if len(positions) == 0 {
		return baseStyle.Render(label)
	}

	// Create a set for O(1) position lookup
	posSet := make(map[int]bool, len(positions))
	for _, p := range positions {
		posSet[p] = true
	}

	var result strings.Builder
	for i, r := range label {
		char := string(r)
		if posSet[i] {
			result.WriteString(matchStyle.Render(char))
		} else {
			result.WriteString(baseStyle.Render(char))
		}
	}

	return result.String()
}

func (m selectorModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	for i, scored := range m.filtered {
		cursor := "  "
		if i == m.cursor {
			cursor = styles.CursorStyle.Render("> ")
		}

		check := ""
		if m.multiSelect {
			if m.checked[scored.origIndex] {
				check = styles.BranchStyle.Render("[x] ")
			} else {
				check = styles.DimStyle.Render("[ ] ")
			}
		}

		// Render label with match highlighting
		var label string
		if i == m.cursor {
			// Selected row: use BranchStyle as base, MatchStyle for matches
			label = renderHighlightedLabel(
				scored.item.Label,
				scored.positions,
				styles.BranchStyle,
				styles.MatchStyle,
			)
		} else {
			// Unselected row: use NormalStyle as base, MatchStyle for matches
			label = renderHighlightedLabel(
				scored.item.Label,
				scored.positions,
				styles.NormalStyle,
				styles.MatchStyle,
			)
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
