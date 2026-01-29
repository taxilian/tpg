package db

import "github.com/taxilian/tpg/internal/model"

// GenerateItemID returns a new item ID using the configured prefixes.
func GenerateItemID(itemType model.ItemType) (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}
	prefix := config.Prefixes.Task
	if itemType == model.ItemTypeEpic {
		prefix = config.Prefixes.Epic
	}
	return model.GenerateIDWithPrefix(prefix), nil
}
