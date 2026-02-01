package db

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ConfigField represents a single config field with its path, value, and metadata.
type ConfigField struct {
	Path        string // e.g., "prefixes.task", "warnings.short_description"
	Key         string // e.g., "task", "short_description"
	Value       any
	Type        string // "string", "int", "bool", "map"
	Description string // from json tag or field name
	Default     string // default value description
}

// GetConfigFields returns all config fields using reflection.
func GetConfigFields(config *Config) []ConfigField {
	var fields []ConfigField
	extractFields(reflect.ValueOf(config).Elem(), "", &fields)
	return fields
}

// extractFields recursively extracts fields from a struct.
func extractFields(v reflect.Value, prefix string, fields *[]ConfigField) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// Get JSON tag for the key name
		jsonTag := field.Tag.Get("json")
		key := strings.Split(jsonTag, ",")[0]
		if key == "" || key == "-" {
			continue
		}

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		// Handle different types
		switch value.Kind() {
		case reflect.Struct:
			extractFields(value, path, fields)
		case reflect.Ptr:
			if !value.IsNil() {
				*fields = append(*fields, ConfigField{
					Path:  path,
					Key:   key,
					Value: value.Elem().Interface(),
					Type:  value.Elem().Kind().String(),
				})
			} else {
				// For nil pointers, determine the underlying type
				elemType := field.Type.Elem()
				*fields = append(*fields, ConfigField{
					Path:  path,
					Key:   key,
					Value: nil,
					Type:  elemType.Kind().String(),
				})
			}
		case reflect.Map:
			*fields = append(*fields, ConfigField{
				Path:  path,
				Key:   key,
				Value: value.Interface(),
				Type:  "map",
			})
		default:
			*fields = append(*fields, ConfigField{
				Path:  path,
				Key:   key,
				Value: value.Interface(),
				Type:  value.Kind().String(),
			})
		}
	}
}

// SetConfigField sets a config field by path.
func SetConfigField(config *Config, path string, value string) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return fmt.Errorf("invalid path: %s", path)
	}

	v := reflect.ValueOf(config).Elem()
	return setFieldByPath(v, parts, value)
}

// setFieldByPath recursively navigates to and sets a field by path parts.
func setFieldByPath(v reflect.Value, parts []string, value string) error {
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Get JSON tag for the key name
		jsonTag := field.Tag.Get("json")
		key := strings.Split(jsonTag, ",")[0]
		if key == "" || key == "-" {
			continue
		}

		if key != parts[0] {
			continue
		}

		// Found the field
		if len(parts) == 1 {
			// This is the target field, set it
			return setFieldValue(fieldValue, field.Type, value)
		}

		// Need to go deeper
		if fieldValue.Kind() == reflect.Struct {
			return setFieldByPath(fieldValue, parts[1:], value)
		}

		return fmt.Errorf("cannot navigate into non-struct field: %s", parts[0])
	}

	return fmt.Errorf("field not found: %s", parts[0])
}

// setFieldValue sets a field's value from a string.
func setFieldValue(fieldValue reflect.Value, fieldType reflect.Type, value string) error {
	if !fieldValue.CanSet() {
		return fmt.Errorf("cannot set field")
	}

	switch fieldValue.Kind() {
	case reflect.String:
		fieldValue.SetString(value)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: %s", value)
		}
		fieldValue.SetInt(intVal)
		return nil

	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %s (use true/false)", value)
		}
		fieldValue.SetBool(boolVal)
		return nil

	case reflect.Ptr:
		// Handle pointer types (like *bool)
		elemType := fieldType.Elem()
		newVal := reflect.New(elemType)
		if err := setFieldValue(newVal.Elem(), elemType, value); err != nil {
			return err
		}
		fieldValue.Set(newVal)
		return nil

	case reflect.Map:
		return fmt.Errorf("cannot set map values directly; use 'key=value' format or edit config.json")

	default:
		return fmt.Errorf("unsupported field type: %s", fieldValue.Kind())
	}
}

// GetConfigField gets a config field value by path.
func GetConfigField(config *Config, path string) (any, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid path: %s", path)
	}

	v := reflect.ValueOf(config).Elem()
	return getFieldByPath(v, parts)
}

// getFieldByPath recursively navigates to and gets a field by path parts.
func getFieldByPath(v reflect.Value, parts []string) (any, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Get JSON tag for the key name
		jsonTag := field.Tag.Get("json")
		key := strings.Split(jsonTag, ",")[0]
		if key == "" || key == "-" {
			continue
		}

		if key != parts[0] {
			continue
		}

		// Found the field
		if len(parts) == 1 {
			// This is the target field, return its value
			if fieldValue.Kind() == reflect.Ptr {
				if fieldValue.IsNil() {
					return nil, nil
				}
				return fieldValue.Elem().Interface(), nil
			}
			return fieldValue.Interface(), nil
		}

		// Need to go deeper
		if fieldValue.Kind() == reflect.Struct {
			return getFieldByPath(fieldValue, parts[1:])
		}

		return nil, fmt.Errorf("cannot navigate into non-struct field: %s", parts[0])
	}

	return nil, fmt.Errorf("field not found: %s", parts[0])
}

// FormatConfigValue formats a config value for display.
func FormatConfigValue(value any) string {
	if value == nil {
		return "<not set>"
	}

	switch v := value.(type) {
	case map[string]string:
		if len(v) == 0 {
			return "{}"
		}
		var parts []string
		for k, val := range v {
			parts = append(parts, fmt.Sprintf("%s=%s", k, val))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case string:
		if v == "" {
			return `""`
		}
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
