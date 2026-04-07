package tui

import (
	"strings"
	"testing"

	"github.com/taxilian/tpg/internal/model"
)

func TestScrollText(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		scrollOffset int
		maxVisible   int
		wantLines    int
		wantFirst    string
	}{
		{
			name:         "no scroll",
			text:         "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
			scrollOffset: 0,
			maxVisible:   3,
			wantLines:    5,
			wantFirst:    "Line 1",
		},
		{
			name:         "scroll down",
			text:         "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
			scrollOffset: 2,
			maxVisible:   3,
			wantLines:    5,
			wantFirst:    "Line 3",
		},
		{
			name:         "scroll past end clamps to last page",
			text:         "Line 1\nLine 2\nLine 3",
			scrollOffset: 10,
			maxVisible:   3,
			wantLines:    3,
			wantFirst:    "Line 1",
		},
		{
			name:         "scroll negative",
			text:         "Line 1\nLine 2\nLine 3",
			scrollOffset: -5,
			maxVisible:   2,
			wantLines:    3,
			wantFirst:    "Line 1",
		},
		{
			name:         "single line",
			text:         "Only line",
			scrollOffset: 0,
			maxVisible:   5,
			wantLines:    1,
			wantFirst:    "Only line",
		},
		{
			name:         "empty text",
			text:         "",
			scrollOffset: 0,
			maxVisible:   5,
			wantLines:    1,
			wantFirst:    "",
		},
		{
			name:         "real description",
			text:         "## Objective\nImplement feature\n\n## Context\nDetails here\n\n## Tasks\n- Task 1\n- Task 2\n- Task 3",
			scrollOffset: 0,
			maxVisible:   4,
			wantLines:    10,
			wantFirst:    "## Objective",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visible, totalLines := scrollText(tt.text, tt.scrollOffset, tt.maxVisible)

			if totalLines != tt.wantLines {
				t.Errorf("scrollText() totalLines = %d, want %d", totalLines, tt.wantLines)
			}

			if tt.wantFirst != "" {
				lines := strings.Split(visible, "\n")
				if len(lines) == 0 {
					t.Errorf("scrollText() returned no lines, want at least 1")
					return
				}
				if lines[0] != tt.wantFirst {
					t.Errorf("scrollText() first line = %q, want %q", lines[0], tt.wantFirst)
				}
			}

			visibleLines := len(strings.Split(visible, "\n"))
			if visible != "" && visibleLines > tt.maxVisible {
				t.Errorf("scrollText() returned %d visible lines, max allowed is %d", visibleLines, tt.maxVisible)
			}
		})
	}
}

func TestDetailViewScrollPreservedOnRefreshForSameItem(t *testing.T) {
	longDesc := "Intro line\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nConclusion"
	item := model.Item{
		ID:          "ts-test1",
		Title:       "Test Item",
		Type:        model.ItemTypeTask,
		Status:      model.StatusOpen,
		Description: longDesc,
	}
	items := []model.Item{item}

	statuses := map[model.Status]bool{
		model.StatusOpen: true,
	}

	m := Model{
		items:          items,
		filtered:       items,
		cursor:         0,
		height:         20,
		width:          80,
		viewMode:       ViewDetail,
		detailID:       "ts-test1",
		descScroll:     2,
		filterStatuses: statuses,
	}
	m.applyFilters()

	_ = m.View()

	m2, _ := m.Update(detailMsg{itemID: "ts-test1"})
	t2 := m2.(Model)

	view2 := t2.View()

	if t2.descScroll != 2 {
		t.Errorf("BUG REPRODUCED: descScroll was reset to %d, want 2", t2.descScroll)
	}
	if !strings.Contains(view2, "Intro line") {
		t.Errorf("BUG REPRODUCED: view2 does not contain 'Intro line' at all\nview2:\n%s", view2)
	}
	if !strings.Contains(view2, "Line 3") {
		t.Errorf("BUG REPRODUCED: view2 does not show 'Line 3'\nview2:\n%s", view2)
	}
}

func TestDetailViewScrollResetWhenSwitchingItems(t *testing.T) {
	longDesc := "Intro line\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nConclusion"
	item1 := model.Item{
		ID:          "ts-test1",
		Title:       "Test Item 1",
		Type:        model.ItemTypeTask,
		Status:      model.StatusOpen,
		Description: longDesc,
	}
	item2 := model.Item{
		ID:          "ts-test2",
		Title:       "Test Item 2",
		Type:        model.ItemTypeTask,
		Status:      model.StatusOpen,
		Description: "Different description content here",
	}
	items := []model.Item{item1, item2}

	statuses := map[model.Status]bool{
		model.StatusOpen: true,
	}

	m := Model{
		items:          items,
		filtered:       items,
		cursor:         0,
		height:         20,
		width:          80,
		viewMode:       ViewDetail,
		detailID:       "ts-test1",
		descScroll:     2,
		filterStatuses: statuses,
	}
	m.applyFilters()

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
	if t2.descScroll != 0 {
		t.Errorf("descScroll = %d, want 0 (scroll should reset when switching to different item)", t2.descScroll)
	}
}
