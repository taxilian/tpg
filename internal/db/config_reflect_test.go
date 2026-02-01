package db

import (
	"testing"
)

func TestGetConfigFields(t *testing.T) {
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "ts",
			Epic: "ep",
		},
		DefaultProject: "myproject",
		IDLength:       3,
	}

	fields := GetConfigFields(config)

	// Check that we got the expected fields
	fieldMap := make(map[string]ConfigField)
	for _, f := range fields {
		fieldMap[f.Path] = f
	}

	// Check prefixes.task
	if f, ok := fieldMap["prefixes.task"]; !ok {
		t.Error("missing prefixes.task field")
	} else if f.Value != "ts" {
		t.Errorf("prefixes.task = %v, want ts", f.Value)
	}

	// Check prefixes.epic
	if f, ok := fieldMap["prefixes.epic"]; !ok {
		t.Error("missing prefixes.epic field")
	} else if f.Value != "ep" {
		t.Errorf("prefixes.epic = %v, want ep", f.Value)
	}

	// Check default_project
	if f, ok := fieldMap["default_project"]; !ok {
		t.Error("missing default_project field")
	} else if f.Value != "myproject" {
		t.Errorf("default_project = %v, want myproject", f.Value)
	}

	// Check id_length
	if f, ok := fieldMap["id_length"]; !ok {
		t.Error("missing id_length field")
	} else if f.Value != 3 {
		t.Errorf("id_length = %v, want 3", f.Value)
	}
}

func TestSetConfigField(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		value   string
		check   func(*Config) bool
		wantErr bool
	}{
		{
			name:  "set string field",
			path:  "prefixes.task",
			value: "tk",
			check: func(c *Config) bool { return c.Prefixes.Task == "tk" },
		},
		{
			name:  "set int field",
			path:  "id_length",
			value: "5",
			check: func(c *Config) bool { return c.IDLength == 5 },
		},
		{
			name:  "set nested int field",
			path:  "warnings.min_description_words",
			value: "25",
			check: func(c *Config) bool { return c.Warnings.MinDescriptionWords == 25 },
		},
		{
			name:  "set pointer bool field",
			path:  "warnings.short_description",
			value: "false",
			check: func(c *Config) bool {
				return c.Warnings.ShortDescription != nil && *c.Warnings.ShortDescription == false
			},
		},
		{
			name:    "invalid path",
			path:    "nonexistent.field",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "invalid int value",
			path:    "id_length",
			value:   "not-a-number",
			wantErr: true,
		},
		{
			name:    "invalid bool value",
			path:    "warnings.short_description",
			value:   "not-a-bool",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Prefixes: PrefixConfig{
					Task: "ts",
					Epic: "ep",
				},
			}

			err := SetConfigField(config, tt.path, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetConfigField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.check != nil && !tt.check(config) {
				t.Errorf("SetConfigField() did not set value correctly")
			}
		})
	}
}

func TestGetConfigField(t *testing.T) {
	boolVal := true
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "ts",
			Epic: "ep",
		},
		DefaultProject: "myproject",
		IDLength:       3,
		Warnings: WarningsConfig{
			ShortDescription:    &boolVal,
			MinDescriptionWords: 15,
		},
	}

	tests := []struct {
		name    string
		path    string
		want    any
		wantErr bool
	}{
		{
			name: "get string field",
			path: "prefixes.task",
			want: "ts",
		},
		{
			name: "get int field",
			path: "id_length",
			want: 3,
		},
		{
			name: "get nested int field",
			path: "warnings.min_description_words",
			want: 15,
		},
		{
			name: "get pointer bool field",
			path: "warnings.short_description",
			want: true,
		},
		{
			name:    "invalid path",
			path:    "nonexistent.field",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetConfigField(config, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetConfigField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("GetConfigField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatConfigValue(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "nil value",
			value: nil,
			want:  "<not set>",
		},
		{
			name:  "string value",
			value: "hello",
			want:  "hello",
		},
		{
			name:  "empty string",
			value: "",
			want:  `""`,
		},
		{
			name:  "bool true",
			value: true,
			want:  "true",
		},
		{
			name:  "bool false",
			value: false,
			want:  "false",
		},
		{
			name:  "int value",
			value: 42,
			want:  "42",
		},
		{
			name:  "empty map",
			value: map[string]string{},
			want:  "{}",
		},
		{
			name:  "map with values",
			value: map[string]string{"a": "1"},
			want:  "{a=1}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatConfigValue(tt.value)
			if got != tt.want {
				t.Errorf("FormatConfigValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
