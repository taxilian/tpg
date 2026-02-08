package tui

import (
	"strings"
	"testing"
)

func TestScrollText(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		scrollOffset int
		maxVisible   int
		wantLines    int    // expected total lines
		wantFirst    string // expected first line of visible output
		wantEmpty    bool   // expect empty string
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
			name:         "scroll past end",
			text:         "Line 1\nLine 2\nLine 3",
			scrollOffset: 10,
			maxVisible:   3,
			wantLines:    3,
			wantEmpty:    true,
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

			if tt.wantEmpty {
				if visible != "" {
					t.Errorf("scrollText() visible = %q, want empty string", visible)
				}
				return
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

			// Check visible lines don't exceed maxVisible
			visibleLines := len(strings.Split(visible, "\n"))
			if visible != "" && visibleLines > tt.maxVisible {
				t.Errorf("scrollText() returned %d visible lines, max allowed is %d", visibleLines, tt.maxVisible)
			}
		})
	}
}

func TestDetailDescriptionVisibleHeight(t *testing.T) {
	tests := []struct {
		name   string
		height int
		want   int
	}{
		{
			name:   "large terminal",
			height: 50,
			want:   25,
		},
		{
			name:   "medium terminal",
			height: 30,
			want:   10, // 30-25=5, but minimum is 10
		},
		{
			name:   "small terminal",
			height: 25,
			want:   10, // minimum
		},
		{
			name:   "very small terminal",
			height: 10,
			want:   10, // minimum
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{height: tt.height}
			got := m.detailDescriptionVisibleHeight()
			if got != tt.want {
				t.Errorf("detailDescriptionVisibleHeight() = %d, want %d", got, tt.want)
			}
		})
	}
}
