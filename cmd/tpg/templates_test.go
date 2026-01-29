package main

import (
	"testing"
)

func TestParseTemplateVarsYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "simple strings",
			yaml: `feature_name: authentication
scope: API endpoint`,
			want: map[string]string{
				"feature_name": "authentication",
				"scope":        "API endpoint",
			},
		},
		{
			name: "multiline string",
			yaml: `feature_name: auth
requirements: |
  Line 1
  Line 2`,
			want: map[string]string{
				"feature_name": "auth",
				"requirements": "Line 1\nLine 2",
			},
		},
		{
			name: "empty value",
			yaml: `feature_name: auth
constraints:`,
			want: map[string]string{
				"feature_name": "auth",
				"constraints":  "",
			},
		},
		{
			name:    "invalid yaml",
			yaml:    `feature_name: [unclosed`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs, err := parseTemplateVarsYAML([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTemplateVarsYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Parse back to map for comparison
			got, err := parseTemplateVars(pairs)
			if err != nil {
				t.Fatalf("parseTemplateVars() failed: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Errorf("got %d vars, want %d", len(got), len(tt.want))
			}

			for key, wantValue := range tt.want {
				gotValue, ok := got[key]
				if !ok {
					t.Errorf("missing key %q", key)
					continue
				}
				if gotValue != wantValue {
					t.Errorf("key %q: got %q, want %q", key, gotValue, wantValue)
				}
			}
		})
	}
}
