package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/taxilian/tpg/internal/model"
)

func newDetailViewFixture() Model {
	longDescLines := []string{
		"Intro line",
		"Line 2",
		"Line 3",
		"Line 4",
		"Line 5",
		"Line 6",
		"Line 7",
		"Line 8",
		"Line 9",
		"Line 10",
		"Line 11",
		"Line 12",
		"Line 13",
		"Line 14",
		"Line 15",
		"Line 16",
		"Conclusion",
	}
	item1 := model.Item{
		ID:          "ts-test1",
		Title:       "Test Item 1",
		Type:        model.ItemTypeTask,
		Status:      model.StatusOpen,
		Description: strings.Join(longDescLines, "\n"),
	}
	item2 := model.Item{
		ID:          "ts-test2",
		Title:       "Test Item 2",
		Type:        model.ItemTypeTask,
		Status:      model.StatusOpen,
		Description: "Different description content here",
	}
	items := []model.Item{item1, item2}

	m := Model{
		items:    items,
		filtered: items,
		cursor:   0,
		width:    80,
		height:   16,
		viewMode: ViewDetail,
		detailID: "ts-test1",
		filterStatuses: map[model.Status]bool{
			model.StatusOpen: true,
		},
		help:           newHelpModel(),
		detailViewport: newViewportModel(),
	}
	m.applyFilters()
	m.syncDetailViewport()
	return m
}

func TestDetailViewScrollPreservedOnRefreshForSameItem(t *testing.T) {
	m := newDetailViewFixture()
	m.detailViewport.SetYOffset(2)

	_ = m.View()

	m2, _ := m.Update(detailMsg{itemID: "ts-test1"})
	t2 := m2.(Model)

	view2 := t2.View()

	if t2.detailViewport.YOffset != 2 {
		t.Errorf("BUG REPRODUCED: detail viewport offset was reset to %d, want 2", t2.detailViewport.YOffset)
	}
	if !strings.Contains(view2, "Intro line") {
		t.Errorf("BUG REPRODUCED: view2 does not contain 'Intro line' at all\nview2:\n%s", view2)
	}
	if !strings.Contains(view2, "Line 3") {
		t.Errorf("BUG REPRODUCED: view2 does not show 'Line 3'\nview2:\n%s", view2)
	}
}

func TestDetailViewScrollResetWhenSwitchingItems(t *testing.T) {
	m := newDetailViewFixture()
	m.detailViewport.SetYOffset(2)

	_ = m.View()

	m.cursor = 1
	m2, _ := m.Update(detailMsg{itemID: "ts-test2"})
	t2 := m2.(Model)
	view2 := t2.View()

	if !strings.Contains(view2, "Test Item 2") {
		t.Errorf("view2 should show item 2 title, got: %s", view2)
	}
	if !strings.Contains(view2, "Different description") {
		t.Errorf("view2 should show item 2 description, got: %s", view2)
	}
	if t2.detailViewport.YOffset != 0 {
		t.Errorf("detail viewport offset = %d, want 0 when switching to different item", t2.detailViewport.YOffset)
	}
}

func TestDetailViewportMovementAndBounds(t *testing.T) {
	m := newDetailViewFixture()
	maxOffset := m.detailViewport.TotalLineCount() - m.detailViewport.VisibleLineCount()
	if maxOffset <= 0 {
		t.Fatalf("fixture should produce scrollable content, got maxOffset=%d", maxOffset)
	}

	updated, _ := m.handleDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if m.detailViewport.YOffset != 1 {
		t.Fatalf("YOffset after down = %d, want 1", m.detailViewport.YOffset)
	}

	updated, _ = m.handleDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)
	if m.detailViewport.YOffset != 0 {
		t.Fatalf("YOffset after up = %d, want 0", m.detailViewport.YOffset)
	}

	updated, _ = m.handleDetailKey(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(Model)
	if m.detailViewport.YOffset <= 0 {
		t.Fatalf("YOffset after page down = %d, want > 0", m.detailViewport.YOffset)
	}
	if m.detailViewport.YOffset > maxOffset {
		t.Fatalf("YOffset after page down = %d, want <= %d", m.detailViewport.YOffset, maxOffset)
	}

	updated, _ = m.handleDetailKey(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(Model)
	if m.detailViewport.YOffset != maxOffset {
		t.Fatalf("YOffset after end = %d, want %d", m.detailViewport.YOffset, maxOffset)
	}

	updated, _ = m.handleDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if m.detailViewport.YOffset != maxOffset {
		t.Fatalf("YOffset after extra down at bottom = %d, want %d", m.detailViewport.YOffset, maxOffset)
	}

	updated, _ = m.handleDetailKey(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(Model)
	if m.detailViewport.YOffset >= maxOffset {
		t.Fatalf("YOffset after page up = %d, want < %d", m.detailViewport.YOffset, maxOffset)
	}

	updated, _ = m.handleDetailKey(tea.KeyMsg{Type: tea.KeyHome})
	m = updated.(Model)
	if m.detailViewport.YOffset != 0 {
		t.Fatalf("YOffset after home = %d, want 0", m.detailViewport.YOffset)
	}

	updated, _ = m.handleDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)
	if m.detailViewport.YOffset != 0 {
		t.Fatalf("YOffset after extra up at top = %d, want 0", m.detailViewport.YOffset)
	}
}
