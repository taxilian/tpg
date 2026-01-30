package db

import (
	"fmt"

	"github.com/taxilian/tpg/internal/model"
)

const maxIDRetries = 10

// GenerateItemID returns a new unique item ID using the configured prefixes and length.
// Retries with a new random hash on collision.
func (db *DB) GenerateItemID(itemType model.ItemType) (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}
	prefix := config.GetPrefixForType(string(itemType))
	idLen := config.IDLength
	if idLen == 0 {
		idLen = model.DefaultIDLength
	}

	for i := 0; i < maxIDRetries; i++ {
		id := model.GenerateIDWithPrefixN(prefix, itemType, idLen)
		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM items WHERE id = ?`, id).Scan(&count)
		if err != nil {
			return "", fmt.Errorf("failed to check ID uniqueness: %w", err)
		}
		if count == 0 {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique ID after %d attempts (consider increasing id_length in config)", maxIDRetries)
}

// GenerateItemIDStatic returns a new item ID without collision checking (for use without DB).
func GenerateItemIDStatic(itemType model.ItemType) (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}
	prefix := config.GetPrefixForType(string(itemType))
	return model.GenerateIDWithPrefixN(prefix, itemType, config.IDLength), nil
}
