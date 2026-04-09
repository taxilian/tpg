package main

import (
	"strings"
	"testing"

	"github.com/taxilian/tpg/internal/model"
)

func TestEdit_VarsOnlySucceeds(t *testing.T) {
	database := setupCommandDB(t)
	item := createTestItem(t, database, "ts-var", "Task with vars", func(i *model.Item) {
		i.TemplateID = "test-template"
		i.TemplateVars = map[string]string{"foo": `"old"`}
		i.Status = model.StatusInProgress
	})

	flagEditVars = []string{`foo="new"`}
	flagEditVarsYAML = false
	flagEditTitle = ""
	flagEditDesc = ""
	flagEditPriority = 0
	flagEditParentSet = false
	flagEditAddLabels = nil
	flagEditRmLabels = nil
	flagEditStatus = ""
	flagForce = false
	flagDryRun = false
	t.Cleanup(func() {
		flagEditVars = nil
		flagEditVarsYAML = false
		flagEditTitle = ""
		flagEditDesc = ""
		flagEditPriority = 0
		flagEditParentSet = false
		flagEditAddLabels = nil
		flagEditRmLabels = nil
		flagEditStatus = ""
		flagForce = false
		flagDryRun = false
	})

	err := editCmd.RunE(editCmd, []string{item.ID})
	if err != nil {
		t.Fatalf("expected edit --var to succeed, got: %v", err)
	}

	updated, err := database.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}
	if updated.TemplateVars["foo"] != `"new"` {
		t.Errorf("expected foo=\"new\", got %q", updated.TemplateVars["foo"])
	}
}

func TestEdit_DescOnTemplateTaskFails(t *testing.T) {
	database := setupCommandDB(t)
	item := createTestItem(t, database, "ts-template", "Template task", func(i *model.Item) {
		i.TemplateID = "test-template"
		i.TemplateVars = map[string]string{"foo": `"bar"`}
	})

	flagEditDesc = "new description"
	flagEditVars = nil
	flagEditVarsYAML = false
	flagEditTitle = ""
	flagEditPriority = 0
	flagEditParentSet = false
	flagEditAddLabels = nil
	flagEditRmLabels = nil
	flagEditStatus = ""
	flagForce = false
	flagDryRun = false
	t.Cleanup(func() {
		flagEditVars = nil
		flagEditVarsYAML = false
		flagEditTitle = ""
		flagEditDesc = ""
		flagEditPriority = 0
		flagEditParentSet = false
		flagEditAddLabels = nil
		flagEditRmLabels = nil
		flagEditStatus = ""
		flagForce = false
		flagDryRun = false
	})

	err := editCmd.RunE(editCmd, []string{item.ID})
	if err == nil {
		t.Fatal("expected error when setting --desc on template task")
	}
	if !strings.Contains(err.Error(), "template") {
		t.Errorf("expected error about template, got: %v", err)
	}
}
