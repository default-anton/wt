package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestFuzzyMatching(t *testing.T) {
	tests := []struct {
		name        string
		items       []Item
		query       string
		wantLen     int
		wantMatches []string // expected matches (in any order)
	}{
		{
			name: "matches containing query",
			items: []Item{
				{Label: "feature/auth", Value: "auth"},
				{Label: "main", Value: "main"},
				{Label: "auth-feature", Value: "af"},
			},
			query:       "auth",
			wantLen:     2,
			wantMatches: []string{"feature/auth", "auth-feature"},
		},
		{
			name: "case insensitive matching",
			items: []Item{
				{Label: "FeatureBranch", Value: "fb"},
				{Label: "MAIN", Value: "main"},
			},
			query:       "featurebranch",
			wantLen:     1,
			wantMatches: []string{"FeatureBranch"},
		},
		{
			name: "no matches returns empty",
			items: []Item{
				{Label: "main", Value: "main"},
				{Label: "develop", Value: "dev"},
			},
			query:   "xyz",
			wantLen: 0,
		},
		{
			name: "empty query returns all",
			items: []Item{
				{Label: "a", Value: "a"},
				{Label: "b", Value: "b"},
			},
			query:   "",
			wantLen: 2,
		},
		{
			name: "subsequence matching",
			items: []Item{
				{Label: "feature-branch", Value: "fb"},
				{Label: "main", Value: "m"},
			},
			query:       "fb",
			wantLen:     1,
			wantMatches: []string{"feature-branch"},
		},
		{
			name: "multiple matches with scores",
			items: []Item{
				{Label: "very-long-feature-branch", Value: "long"},
				{Label: "feat", Value: "short"},
			},
			query:       "feat",
			wantLen:     2,
			wantMatches: []string{"feat", "very-long-feature-branch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newSelectorModel(tt.items, false)
			m.textInput.SetValue(tt.query)
			m.filterItems()

			if len(m.filtered) != tt.wantLen {
				t.Errorf("got %d results, want %d", len(m.filtered), tt.wantLen)
				return
			}

			if len(tt.wantMatches) > 0 {
				gotLabels := make(map[string]bool)
				for _, s := range m.filtered {
					gotLabels[s.item.Label] = true
				}
				for _, want := range tt.wantMatches {
					if !gotLabels[want] {
						t.Errorf("expected match %q not found in results", want)
					}
				}
			}
		})
	}
}

func TestMatchPositions(t *testing.T) {
	m := newSelectorModel([]Item{{Label: "feature", Value: "f"}}, false)
	m.textInput.SetValue("ft")
	m.filterItems()

	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 match, got %d", len(m.filtered))
	}

	// Should have match positions
	positions := m.filtered[0].positions
	if len(positions) == 0 {
		t.Error("expected positions to be populated")
	}
}

func TestOriginalIndexPreserved(t *testing.T) {
	items := []Item{
		{Label: "first", Value: "1"},
		{Label: "second", Value: "2"},
		{Label: "third", Value: "3"},
	}

	m := newSelectorModel(items, true)
	m.textInput.SetValue("ir") // matches "first" and "third"
	m.filterItems()

	// Verify original indices are preserved
	for _, scored := range m.filtered {
		expectedIdx := -1
		for i, item := range items {
			if item.Value == scored.item.Value {
				expectedIdx = i
				break
			}
		}
		if scored.origIndex != expectedIdx {
			t.Errorf("origIndex = %d, want %d for %q",
				scored.origIndex, expectedIdx, scored.item.Label)
		}
	}

	// Verify we have matches
	if len(m.filtered) == 0 {
		t.Error("expected at least one match")
	}
}

func TestRenderHighlightedLabel(t *testing.T) {
	baseStyle := lipgloss.NewStyle()
	matchStyle := lipgloss.NewStyle()

	tests := []struct {
		name      string
		label     string
		positions []int
		wantLen   int // rendered string should contain original label
	}{
		{
			name:      "no positions returns styled label",
			label:     "test",
			positions: nil,
			wantLen:   4, // at minimum, original characters
		},
		{
			name:      "empty positions returns styled label",
			label:     "test",
			positions: []int{},
			wantLen:   4,
		},
		{
			name:      "with positions highlights chars",
			label:     "feature",
			positions: []int{0, 3},
			wantLen:   7, // at minimum, original characters
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderHighlightedLabel(tt.label, tt.positions, baseStyle, matchStyle)
			if len(result) < tt.wantLen {
				t.Errorf("result length = %d, want at least %d", len(result), tt.wantLen)
			}
		})
	}
}
