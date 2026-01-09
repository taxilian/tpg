package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/baiirun/prog/internal/model"
)

// CreateLearning inserts a new learning and its concept associations.
// Creates concepts that don't exist yet.
func (db *DB) CreateLearning(l *model.Learning) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Serialize files to JSON
	filesJSON := "[]"
	if len(l.Files) > 0 {
		b, err := json.Marshal(l.Files)
		if err != nil {
			return fmt.Errorf("failed to marshal files: %w", err)
		}
		filesJSON = string(b)
	}

	// Insert learning
	_, err = tx.Exec(`
		INSERT INTO learnings (id, project, created_at, updated_at, task_id, summary, detail, files, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, l.ID, l.Project, l.CreatedAt, l.UpdatedAt, l.TaskID, l.Summary, l.Detail, filesJSON, l.Status)
	if err != nil {
		return fmt.Errorf("failed to insert learning: %w", err)
	}

	// Ensure concepts exist and create associations
	for _, conceptName := range l.Concepts {
		// Insert or update concept
		_, err = tx.Exec(`
			INSERT INTO concepts (name, project, last_updated)
			VALUES (?, ?, ?)
			ON CONFLICT (name, project) DO UPDATE SET last_updated = excluded.last_updated
		`, conceptName, l.Project, l.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to upsert concept %q: %w", conceptName, err)
		}

		// Create association
		_, err = tx.Exec(`
			INSERT INTO learning_concepts (learning_id, concept, project)
			VALUES (?, ?, ?)
		`, l.ID, conceptName, l.Project)
		if err != nil {
			return fmt.Errorf("failed to create concept association: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetLearning retrieves a learning by ID.
func (db *DB) GetLearning(id string) (*model.Learning, error) {
	var l model.Learning
	var filesJSON string
	var taskID *string

	err := db.QueryRow(`
		SELECT id, project, created_at, updated_at, task_id, summary, detail, files, status
		FROM learnings WHERE id = ?
	`, id).Scan(&l.ID, &l.Project, &l.CreatedAt, &l.UpdatedAt, &taskID, &l.Summary, &l.Detail, &filesJSON, &l.Status)
	if err != nil {
		return nil, fmt.Errorf("learning not found: %s", id)
	}
	l.TaskID = taskID

	// Parse files JSON
	if filesJSON != "" && filesJSON != "[]" {
		if err := json.Unmarshal([]byte(filesJSON), &l.Files); err != nil {
			return nil, fmt.Errorf("failed to unmarshal files: %w", err)
		}
	}

	// Get associated concepts
	rows, err := db.Query(`
		SELECT concept FROM learning_concepts WHERE learning_id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get concepts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var concept string
		if err := rows.Scan(&concept); err != nil {
			return nil, fmt.Errorf("failed to scan concept: %w", err)
		}
		l.Concepts = append(l.Concepts, concept)
	}

	return &l, nil
}

// GetCurrentTaskID returns the ID of the first in-progress task for a project.
// Returns nil if no task is in progress.
func (db *DB) GetCurrentTaskID(project string) (*string, error) {
	var taskID string
	err := db.QueryRow(`
		SELECT id FROM items
		WHERE status = 'in_progress' AND project = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, project).Scan(&taskID)
	if err != nil {
		return nil, nil // No task in progress, not an error
	}
	return &taskID, nil
}

// ListConcepts returns all concepts for a project.
func (db *DB) ListConcepts(project string) ([]model.Concept, error) {
	rows, err := db.Query(`
		SELECT c.name, c.project, c.summary, c.last_updated,
			(SELECT COUNT(*) FROM learning_concepts lc WHERE lc.concept = c.name AND lc.project = c.project) as count
		FROM concepts c
		WHERE c.project = ?
		ORDER BY count DESC, c.name
	`, project)
	if err != nil {
		return nil, fmt.Errorf("failed to list concepts: %w", err)
	}
	defer rows.Close()

	var concepts []model.Concept
	for rows.Next() {
		var c model.Concept
		var summary *string
		var count int
		if err := rows.Scan(&c.Name, &c.Project, &summary, &c.LastUpdated, &count); err != nil {
			return nil, fmt.Errorf("failed to scan concept: %w", err)
		}
		if summary != nil {
			c.Summary = *summary
		}
		concepts = append(concepts, c)
	}

	return concepts, nil
}

// EnsureConcept creates a concept if it doesn't exist.
func (db *DB) EnsureConcept(name, project string) error {
	_, err := db.Exec(`
		INSERT INTO concepts (name, project, last_updated)
		VALUES (?, ?, ?)
		ON CONFLICT (name, project) DO NOTHING
	`, name, project, time.Now())
	return err
}
