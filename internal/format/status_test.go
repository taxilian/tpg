package format

import (
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func TestIsStale(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		item      model.Item
		wantStale bool
	}{
		{
			name: "in_progress updated 6 minutes ago is stale",
			item: model.Item{
				Status:    model.StatusInProgress,
				UpdatedAt: now.Add(-6 * time.Minute),
			},
			wantStale: true,
		},
		{
			name: "in_progress updated 4 minutes ago is not stale",
			item: model.Item{
				Status:    model.StatusInProgress,
				UpdatedAt: now.Add(-4 * time.Minute),
			},
			wantStale: false,
		},
		{
			name: "in_progress updated exactly 5 minutes ago is not stale",
			item: model.Item{
				Status:    model.StatusInProgress,
				UpdatedAt: now.Add(-5 * time.Minute),
			},
			wantStale: false,
		},
		{
			name: "in_progress updated 5 minutes 1 second ago is stale",
			item: model.Item{
				Status:    model.StatusInProgress,
				UpdatedAt: now.Add(-5*time.Minute - 1*time.Second),
			},
			wantStale: true,
		},
		{
			name: "open status is never stale",
			item: model.Item{
				Status:    model.StatusOpen,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStale: false,
		},
		{
			name: "done status is never stale",
			item: model.Item{
				Status:    model.StatusDone,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStale: false,
		},
		{
			name: "blocked status is never stale",
			item: model.Item{
				Status:    model.StatusBlocked,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStale: false,
		},
		{
			name: "canceled status is never stale",
			item: model.Item{
				Status:    model.StatusCanceled,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStale: false,
		},
		{
			name: "recently updated in_progress is not stale",
			item: model.Item{
				Status:    model.StatusInProgress,
				UpdatedAt: now,
			},
			wantStale: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsStale(tt.item, now)
			if got != tt.wantStale {
				t.Errorf("IsStale() = %v, want %v", got, tt.wantStale)
			}
		})
	}
}

func TestStatusDisplay(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		item       model.Item
		wantStatus string
	}{
		{
			name: "stale in_progress shows stale",
			item: model.Item{
				Status:    model.StatusInProgress,
				UpdatedAt: now.Add(-10 * time.Minute),
			},
			wantStatus: "stale",
		},
		{
			name: "active in_progress shows in_progress",
			item: model.Item{
				Status:    model.StatusInProgress,
				UpdatedAt: now.Add(-1 * time.Minute),
			},
			wantStatus: "in_progress",
		},
		{
			name: "open shows open",
			item: model.Item{
				Status:    model.StatusOpen,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStatus: "open",
		},
		{
			name: "done shows done",
			item: model.Item{
				Status:    model.StatusDone,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStatus: "done",
		},
		{
			name: "blocked shows blocked",
			item: model.Item{
				Status:    model.StatusBlocked,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStatus: "blocked",
		},
		{
			name: "canceled shows canceled",
			item: model.Item{
				Status:    model.StatusCanceled,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantStatus: "canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatusDisplay(tt.item, now)
			if got != tt.wantStatus {
				t.Errorf("StatusDisplay() = %v, want %v", got, tt.wantStatus)
			}
		})
	}
}

func TestStaleThreshold(t *testing.T) {
	// Verify the threshold is exactly 5 minutes
	if StaleThreshold != 5*time.Minute {
		t.Errorf("StaleThreshold = %v, want %v", StaleThreshold, 5*time.Minute)
	}
}
