package db

import (
	"encoding/json"
	"fmt"
)

func marshalTemplateVars(vars map[string]string) (string, error) {
	if len(vars) == 0 {
		return "", nil
	}
	data, err := json.Marshal(vars)
	if err != nil {
		return "", fmt.Errorf("failed to encode template variables: %w", err)
	}
	return string(data), nil
}

func unmarshalTemplateVars(raw string) (map[string]string, error) {
	if raw == "" {
		return nil, nil
	}
	vars := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &vars); err != nil {
		return nil, fmt.Errorf("failed to decode template variables: %w", err)
	}
	return vars, nil
}
