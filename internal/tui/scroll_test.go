package tui

import (
	"strings"
	"testing"

	"github.com/taxilian/tpg/internal/model"
)

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
		filterStatuses: statuses,
		detailViewport: newViewportModel(),
	}
	m.applyFilters()
	m.syncDetailViewport()
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
		filterStatuses: statuses,
		detailViewport: newViewportModel(),
	}
	m.applyFilters()
	m.syncDetailViewport()
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
