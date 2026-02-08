package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteStatusValues(t *testing.T) {
	tests := []struct {
		name       string
		toComplete string
		want       []string
	}{
		{
			name:       "complete open",
			toComplete: "ope",
			want:       []string{"open"},
		},
		{
			name:       "complete done",
			toComplete: "don",
			want:       []string{"done"},
		},
		{
			name:       "empty returns all",
			toComplete: "",
			want:       []string{"open", "in_progress", "blocked", "done", "canceled"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := completeStatusValues(nil, nil, tt.toComplete)
			if len(got) != len(tt.want) {
				t.Errorf("completeStatusValues() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if i >= len(got) || got[i] != tt.want[i] {
					t.Errorf("completeStatusValues()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCompleteTypeValues(t *testing.T) {
	tests := []struct {
		name       string
		toComplete string
		want       []string
	}{
		{
			name:       "complete task",
			toComplete: "tas",
			want:       []string{"task"},
		},
		{
			name:       "complete epic",
			toComplete: "epi",
			want:       []string{"epic"},
		},
		{
			name:       "empty returns all",
			toComplete: "",
			want:       []string{"task", "epic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := completeTypeValues(nil, nil, tt.toComplete)
			if len(got) != len(tt.want) {
				t.Errorf("completeTypeValues() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if i >= len(got) || got[i] != tt.want[i] {
					t.Errorf("completeTypeValues()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCompleteClosedStatusValues(t *testing.T) {
	tests := []struct {
		name       string
		toComplete string
		want       []string
	}{
		{
			name:       "complete done",
			toComplete: "don",
			want:       []string{"done"},
		},
		{
			name:       "complete canceled",
			toComplete: "can",
			want:       []string{"canceled"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := completeClosedStatusValues(nil, nil, tt.toComplete)
			if len(got) != len(tt.want) {
				t.Errorf("completeClosedStatusValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCompletionFunctionsRegistered ensures completion functions are set up
func TestCompletionFunctionsRegistered(t *testing.T) {
	// These should not panic - just verify they exist and are callable
	_ = completeItemIDs
	_ = completeEpicIDs
	_ = completeTaskIDs
	_ = completeLabels
	_ = completeProjects
	_ = completeStatusValues
	_ = completeTypeValues
	_ = completeClosedStatusValues
	_ = completeTemplates

	// Verify addCompletionFunctions exists
	_ = addCompletionFunctions
}

// TestValidArgsFunction verifies that ValidArgsFunction is set on key commands
func TestValidArgsFunction(t *testing.T) {
	// Commands that should have item ID completion
	commandsWithItemCompletion := []*cobra.Command{
		showCmd,
		descCmd,
		appendCmd,
		editCmd,
		logCmd,
		doneCmd,
		cancelCmd,
		blockCmd,
		startCmd,
		deleteCmd,
		historyCmd,
		depCmd,
		impactCmd,
		replaceCmd,
	}

	for _, cmd := range commandsWithItemCompletion {
		if cmd.ValidArgsFunction == nil {
			t.Errorf("Command %s should have ValidArgsFunction set", cmd.Name())
		}
	}

	// Commands that should have epic ID completion
	commandsWithEpicCompletion := []*cobra.Command{
		epicEditCmd,
		epicListCmd,
		epicWorktreeCmd,
		epicFinishCmd,
		planCmd,
	}

	for _, cmd := range commandsWithEpicCompletion {
		if cmd.ValidArgsFunction == nil {
			t.Errorf("Command %s should have ValidArgsFunction set", cmd.Name())
		}
	}
}
