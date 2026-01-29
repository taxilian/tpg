package db

import (
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func createItem(t *testing.T, db *DB, id, title, project string, status model.Status) {
	t.Helper()
	now := time.Now()
	item := &model.Item{
		ID:        id,
		Project:   project,
		Type:      model.ItemTypeTask,
		Title:     title,
		Status:    status,
		Priority:  2,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item %s: %v", id, err)
	}
}

func TestMergeItems_Basic(t *testing.T) {
	db := setupTestDB(t)

	createItem(t, db, "ts-src", "Source Task", "test", model.StatusOpen)
	createItem(t, db, "ts-tgt", "Target Task", "test", model.StatusOpen)

	if err := db.MergeItems("ts-src", "ts-tgt"); err != nil {
		t.Fatalf("MergeItems failed: %v", err)
	}

	// Source should be gone
	_, err := db.GetItem("ts-src")
	if err == nil {
		t.Error("source item should be deleted after merge")
	}

	// Target should still exist
	tgt, err := db.GetItem("ts-tgt")
	if err != nil {
		t.Fatalf("target item should exist: %v", err)
	}
	if tgt.Title != "Target Task" {
		t.Errorf("target title = %q, want %q", tgt.Title, "Target Task")
	}
}

func TestMergeItems_TransfersDeps(t *testing.T) {
	db := setupTestDB(t)

	createItem(t, db, "ts-a", "Blocker A", "test", model.StatusOpen)
	createItem(t, db, "ts-src", "Source", "test", model.StatusOpen)
	createItem(t, db, "ts-tgt", "Target", "test", model.StatusOpen)
	createItem(t, db, "ts-b", "Blocked B", "test", model.StatusOpen)

	// src depends on A
	db.AddDep("ts-src", "ts-a")
	// B depends on src
	db.AddDep("ts-b", "ts-src")

	if err := db.MergeItems("ts-src", "ts-tgt"); err != nil {
		t.Fatalf("MergeItems failed: %v", err)
	}

	// Target should now depend on A
	tgtDeps, _ := db.GetDeps("ts-tgt")
	found := false
	for _, d := range tgtDeps {
		if d == "ts-a" {
			found = true
		}
	}
	if !found {
		t.Errorf("target should depend on ts-a, got deps: %v", tgtDeps)
	}

	// B should now depend on target
	bDeps, _ := db.GetDeps("ts-b")
	found = false
	for _, d := range bDeps {
		if d == "ts-tgt" {
			found = true
		}
	}
	if !found {
		t.Errorf("ts-b should depend on ts-tgt, got deps: %v", bDeps)
	}

	// No deps should reference source
	srcDeps, _ := db.getDepsRaw("ts-src")
	srcRev, _ := db.getReverseDepsRaw("ts-src")
	if len(srcDeps) > 0 || len(srcRev) > 0 {
		t.Errorf("no deps should reference deleted source, got deps=%v rev=%v", srcDeps, srcRev)
	}
}

func TestMergeItems_TransfersLogs(t *testing.T) {
	db := setupTestDB(t)

	createItem(t, db, "ts-src", "Source", "test", model.StatusOpen)
	createItem(t, db, "ts-tgt", "Target", "test", model.StatusOpen)

	db.AddLog("ts-src", "Source log entry")
	db.AddLog("ts-tgt", "Target log entry")

	if err := db.MergeItems("ts-src", "ts-tgt"); err != nil {
		t.Fatalf("MergeItems failed: %v", err)
	}

	logs, _ := db.GetLogs("ts-tgt")
	// Should have: target's original + source's transferred + merge note
	if len(logs) < 3 {
		t.Errorf("expected at least 3 log entries, got %d", len(logs))
	}

	// Check merge note exists
	hasMergeNote := false
	for _, log := range logs {
		if len(log.Message) > 6 && log.Message[:6] == "Merged" {
			hasMergeNote = true
		}
	}
	if !hasMergeNote {
		t.Error("expected a merge note in logs")
	}
}

func TestMergeItems_AppendsDescription(t *testing.T) {
	db := setupTestDB(t)

	createItem(t, db, "ts-src", "Source", "test", model.StatusOpen)
	createItem(t, db, "ts-tgt", "Target", "test", model.StatusOpen)

	db.SetDescription("ts-src", "Source description")
	db.SetDescription("ts-tgt", "Target description")

	if err := db.MergeItems("ts-src", "ts-tgt"); err != nil {
		t.Fatalf("MergeItems failed: %v", err)
	}

	tgt, _ := db.GetItem("ts-tgt")
	if tgt.Description == "Target description" {
		t.Error("target description should include source content")
	}
	if len(tgt.Description) <= len("Target description") {
		t.Errorf("target description too short: %q", tgt.Description)
	}
}

func TestMergeItems_RejectsDirectCycle(t *testing.T) {
	db := setupTestDB(t)

	createItem(t, db, "ts-src", "Source", "test", model.StatusOpen)
	createItem(t, db, "ts-tgt", "Target", "test", model.StatusOpen)

	// Source depends on target — merging would mean target depends on itself
	db.AddDep("ts-src", "ts-tgt")

	err := db.MergeItems("ts-src", "ts-tgt")
	if err == nil {
		t.Error("expected cycle error when source depends on target")
	}
}

func TestMergeItems_RejectsReverseCycle(t *testing.T) {
	db := setupTestDB(t)

	createItem(t, db, "ts-src", "Source", "test", model.StatusOpen)
	createItem(t, db, "ts-tgt", "Target", "test", model.StatusOpen)

	// Target depends on source — merging would mean target depends on itself
	db.AddDep("ts-tgt", "ts-src")

	err := db.MergeItems("ts-src", "ts-tgt")
	if err == nil {
		t.Error("expected cycle error when target depends on source")
	}
}

func TestMergeItems_RejectsTransitiveCycle(t *testing.T) {
	db := setupTestDB(t)

	createItem(t, db, "ts-a", "A", "test", model.StatusOpen)
	createItem(t, db, "ts-src", "Source", "test", model.StatusOpen)
	createItem(t, db, "ts-tgt", "Target", "test", model.StatusOpen)

	// Chain: target → A → ... and source depends on target (via A)
	// src depends on A, A depends on target
	db.AddDep("ts-src", "ts-a")
	db.AddDep("ts-a", "ts-tgt")

	// Merging src into tgt would make tgt depend on A, which depends on tgt = cycle
	err := db.MergeItems("ts-src", "ts-tgt")
	if err == nil {
		t.Error("expected cycle error for transitive dependency")
	}
}

func TestMergeItems_RejectsSelfMerge(t *testing.T) {
	db := setupTestDB(t)

	createItem(t, db, "ts-a", "A", "test", model.StatusOpen)

	err := db.MergeItems("ts-a", "ts-a")
	if err == nil {
		t.Error("expected error when merging item into itself")
	}
}

func TestMergeItems_NoDuplicateDeps(t *testing.T) {
	db := setupTestDB(t)

	createItem(t, db, "ts-a", "Blocker", "test", model.StatusOpen)
	createItem(t, db, "ts-src", "Source", "test", model.StatusOpen)
	createItem(t, db, "ts-tgt", "Target", "test", model.StatusOpen)

	// Both source and target depend on A
	db.AddDep("ts-src", "ts-a")
	db.AddDep("ts-tgt", "ts-a")

	if err := db.MergeItems("ts-src", "ts-tgt"); err != nil {
		t.Fatalf("MergeItems failed: %v", err)
	}

	// Target should have exactly one dep on A (not duplicated)
	deps, _ := db.GetDeps("ts-tgt")
	count := 0
	for _, d := range deps {
		if d == "ts-a" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 dep on ts-a, got %d", count)
	}
}
