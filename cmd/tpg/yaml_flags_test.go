package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestSetFlagFromYAML_String(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    string
		wantErr bool
	}{
		{"valid string", "hello", "hello", false},
		{"empty string", "", "", false},
		{"multiline string", "line1\nline2", "line1\nline2", false},
		{"int value", 42, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			var strVal string
			fs.StringVar(&strVal, "test-flag", "", "test")
			flag := fs.Lookup("test-flag")

			err := setFlagFromYAML(flag, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if strVal != tt.want {
				t.Errorf("got %q, want %q", strVal, tt.want)
			}
		})
	}
}

func TestSetFlagFromYAML_Int(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    int
		wantErr bool
	}{
		{"int value", 42, 42, false},
		{"int64 value", int64(100), 100, false},
		{"float64 value", float64(25), 25, false},
		{"zero", 0, 0, false},
		{"negative", -5, -5, false},
		{"string value", "42", 0, true},
		{"bool value", true, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			var intVal int
			fs.IntVar(&intVal, "test-flag", 0, "test")
			flag := fs.Lookup("test-flag")

			err := setFlagFromYAML(flag, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if intVal != tt.want {
				t.Errorf("got %d, want %d", intVal, tt.want)
			}
		})
	}
}

func TestSetFlagFromYAML_Bool(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    bool
		wantErr bool
	}{
		{"true", true, true, false},
		{"false", false, false, false},
		{"string value", "true", false, true},
		{"int value", 1, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			var boolVal bool
			fs.BoolVar(&boolVal, "test-flag", false, "test")
			flag := fs.Lookup("test-flag")

			err := setFlagFromYAML(flag, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if boolVal != tt.want {
				t.Errorf("got %v, want %v", boolVal, tt.want)
			}
		})
	}
}

func TestSetFlagFromYAML_StringArray(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    []string
		wantErr bool
	}{
		{
			"single string",
			"label1",
			[]string{"label1"},
			false,
		},
		{
			"string slice",
			[]string{"a", "b", "c"},
			[]string{"a", "b", "c"},
			false,
		},
		{
			"interface slice",
			[]interface{}{"x", "y", "z"},
			[]string{"x", "y", "z"},
			false,
		},
		{
			"int in slice",
			[]interface{}{"a", 42},
			nil,
			true,
		},
		{
			"int value",
			42,
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			var strArray []string
			fs.StringArrayVar(&strArray, "test-flag", nil, "test")
			flag := fs.Lookup("test-flag")

			err := setFlagFromYAML(flag, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(strArray) != len(tt.want) {
				t.Errorf("got %v, want %v", strArray, tt.want)
				return
			}
			for i, got := range strArray {
				if got != tt.want[i] {
					t.Errorf("element %d: got %q, want %q", i, got, tt.want[i])
				}
			}
		})
	}
}

func TestSetFlagFromYAML_Nil(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	var strVal string
	fs.StringVar(&strVal, "test-flag", "default", "test")
	flag := fs.Lookup("test-flag")

	// nil values should be skipped (no error, no change)
	err := setFlagFromYAML(flag, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if strVal != "default" {
		t.Errorf("nil value should not change flag, got %q", strVal)
	}
}

func TestFindStdinMarkerFlag(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*cobra.Command)
		wantFlag string
	}{
		{
			name: "no stdin marker",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().String("desc", "", "description")
				cmd.Flags().String("context", "", "context")
			},
			wantFlag: "",
		},
		{
			name: "stdin marker on desc",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().String("desc", "", "description")
				cmd.Flags().String("context", "", "context")
				_ = cmd.Flags().Set("desc", "-")
			},
			wantFlag: "desc",
		},
		{
			name: "stdin marker on context",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().String("desc", "", "description")
				cmd.Flags().String("context", "", "context")
				_ = cmd.Flags().Set("context", "-")
			},
			wantFlag: "context",
		},
		{
			name: "non-dash value",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().String("desc", "", "description")
				_ = cmd.Flags().Set("desc", "hello")
			},
			wantFlag: "",
		},
		{
			name: "dash in default but not changed",
			setup: func(cmd *cobra.Command) {
				// Flag has default of - but wasn't explicitly set
				cmd.Flags().String("desc", "-", "description")
			},
			wantFlag: "",
		},
		{
			name: "int flag with value",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().String("desc", "", "description")
				cmd.Flags().Int("priority", 0, "priority")
				_ = cmd.Flags().Set("priority", "1")
			},
			wantFlag: "",
		},
		{
			name: "bool flag",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().String("desc", "", "description")
				cmd.Flags().Bool("verbose", false, "verbose")
				_ = cmd.Flags().Set("verbose", "true")
			},
			wantFlag: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			tt.setup(cmd)

			got := findStdinMarkerFlag(cmd)
			if got != tt.wantFlag {
				t.Errorf("findStdinMarkerFlag() = %q, want %q", got, tt.wantFlag)
			}
		})
	}
}

func TestApplyYAMLFlagsFromData_MultipleFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var strVal string
	var intVal int
	var boolVal bool
	var strArray []string
	cmd.Flags().StringVar(&strVal, "desc", "", "description")
	cmd.Flags().IntVar(&intVal, "priority", 0, "priority")
	cmd.Flags().BoolVar(&boolVal, "epic", false, "is epic")
	cmd.Flags().StringArrayVar(&strArray, "label", nil, "labels")

	yamlData := map[string]interface{}{
		"desc":     "My description",
		"priority": 1,
		"epic":     true,
		"label":    []interface{}{"bug", "urgent"},
	}

	err := applyYAMLFlagsFromData(cmd, yamlData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strVal != "My description" {
		t.Errorf("desc: got %q, want %q", strVal, "My description")
	}
	if intVal != 1 {
		t.Errorf("priority: got %d, want %d", intVal, 1)
	}
	if !boolVal {
		t.Errorf("epic: got %v, want true", boolVal)
	}
	if len(strArray) != 2 || strArray[0] != "bug" || strArray[1] != "urgent" {
		t.Errorf("label: got %v, want [bug urgent]", strArray)
	}
}

func TestApplyYAMLFlagsFromData_MarksChanged(t *testing.T) {
	// This test verifies that flags set via YAML are marked as Changed
	// so cmd.Flags().Changed("flag-name") returns true.
	// This is critical for commands like "epic edit" that check Changed().
	cmd := &cobra.Command{Use: "test"}
	var strVal string
	var intVal int
	var boolVal bool
	var strArray []string
	cmd.Flags().StringVar(&strVal, "context", "", "context")
	cmd.Flags().IntVar(&intVal, "priority", 0, "priority")
	cmd.Flags().BoolVar(&boolVal, "epic", false, "is epic")
	cmd.Flags().StringArrayVar(&strArray, "label", nil, "labels")

	// Before applying YAML, flags should not be changed
	if cmd.Flags().Changed("context") {
		t.Error("context should not be changed before YAML")
	}
	if cmd.Flags().Changed("priority") {
		t.Error("priority should not be changed before YAML")
	}
	if cmd.Flags().Changed("epic") {
		t.Error("epic should not be changed before YAML")
	}
	if cmd.Flags().Changed("label") {
		t.Error("label should not be changed before YAML")
	}

	yamlData := map[string]interface{}{
		"context":  "new context",
		"priority": 1,
		"epic":     true,
		"label":    []interface{}{"bug"},
	}

	err := applyYAMLFlagsFromData(cmd, yamlData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After applying YAML, flags should be marked as changed
	if !cmd.Flags().Changed("context") {
		t.Error("context should be marked as changed after YAML")
	}
	if !cmd.Flags().Changed("priority") {
		t.Error("priority should be marked as changed after YAML")
	}
	if !cmd.Flags().Changed("epic") {
		t.Error("epic should be marked as changed after YAML")
	}
	if !cmd.Flags().Changed("label") {
		t.Error("label should be marked as changed after YAML")
	}
}

func TestApplyYAMLFlagsFromData_UnknownKey(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("desc", "", "description")

	yamlData := map[string]interface{}{
		"unknown_flag": "value",
	}

	err := applyYAMLFlagsFromData(cmd, yamlData)
	if err == nil {
		t.Error("expected error for unknown flag, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "unknown flag from YAML") {
		t.Errorf("expected 'unknown flag' error, got: %v", err)
	}
}

func TestApplyYAMLFlagsFromData_TypeMismatch(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*cobra.Command)
		yamlData  map[string]interface{}
		wantErrKw string
	}{
		{
			name: "string for int",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().Int("priority", 0, "priority")
			},
			yamlData:  map[string]interface{}{"priority": "not-an-int"},
			wantErrKw: "expected int",
		},
		{
			name: "string for bool",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().Bool("epic", false, "is epic")
			},
			yamlData:  map[string]interface{}{"epic": "yes"},
			wantErrKw: "expected bool",
		},
		{
			name: "int for string",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().String("desc", "", "description")
			},
			yamlData:  map[string]interface{}{"desc": 123},
			wantErrKw: "expected string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			tt.setup(cmd)

			err := applyYAMLFlagsFromData(cmd, tt.yamlData)
			if err == nil {
				t.Error("expected error for type mismatch, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrKw) {
				t.Errorf("expected error containing %q, got: %v", tt.wantErrKw, err)
			}
		})
	}
}

func TestApplyYAMLFlagsFromData_UnderscoreConversion(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var val string
	cmd.Flags().StringVar(&val, "some-flag", "", "a flag with hyphens")

	yamlData := map[string]interface{}{
		"some_flag": "value from yaml",
	}

	err := applyYAMLFlagsFromData(cmd, yamlData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val != "value from yaml" {
		t.Errorf("got %q, want %q", val, "value from yaml")
	}
}

func TestApplyYAMLFlagsFromData_EmptyData(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var val string
	cmd.Flags().StringVar(&val, "desc", "default", "description")

	// Empty map should not change anything
	err := applyYAMLFlagsFromData(cmd, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val != "default" {
		t.Errorf("empty data should not change flag, got %q", val)
	}
}

func TestFromYAML_ConflictWithStdinMarker(t *testing.T) {
	// Test the conflict detection logic: when --from-yaml is used
	// and a flag has '-' as its value, we should detect the conflict
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("desc", "", "description")
	_ = cmd.Flags().Set("desc", "-") // Set desc to stdin marker

	// findStdinMarkerFlag should detect this
	flagName := findStdinMarkerFlag(cmd)
	if flagName != "desc" {
		t.Errorf("expected to find 'desc' as stdin marker flag, got %q", flagName)
	}

	// This is how the PersistentPreRunE would reject the combination:
	// if flagFromYAML && flagName != "" → error
	// We're testing that the detection works correctly
}
