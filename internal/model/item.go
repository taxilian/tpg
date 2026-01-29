// Package model defines the core data types for the tasks system.
package model

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"
)

const idAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// DefaultIDLength is the default number of characters in the random portion of an ID.
const DefaultIDLength = 3

// GenerateID returns a new ID with a type-specific prefix.
func GenerateID(itemType ItemType) string {
	return GenerateIDWithPrefixN("", itemType, DefaultIDLength)
}

// GenerateIDWithPrefixN returns a new ID with the provided prefix and n random chars from [0-9a-z].
func GenerateIDWithPrefixN(prefix string, itemType ItemType, n int) string {
	p := strings.TrimSpace(prefix)
	p = strings.TrimSuffix(p, "-")
	if p == "" {
		if itemType == ItemTypeEpic {
			p = "ep"
		} else {
			p = "ts"
		}
	}
	return p + "-" + randomAlpha(n)
}

// GenerateIDWithPrefix returns a new ID with the provided prefix and default length.
// Kept for backward compatibility.
func GenerateIDWithPrefix(prefix string) string {
	return GenerateIDWithPrefixN(prefix, ItemTypeTask, DefaultIDLength)
}

func randomAlpha(n int) string {
	alphabetLen := big.NewInt(int64(len(idAlphabet)))
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		b[i] = idAlphabet[idx.Int64()]
	}
	return string(b)
}

type ItemType string

const (
	ItemTypeTask ItemType = "task"
	ItemTypeEpic ItemType = "epic"
)

func (t ItemType) IsValid() bool {
	return t == ItemTypeTask || t == ItemTypeEpic
}

type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusDone       Status = "done"
	StatusCanceled   Status = "canceled"
)

func (s Status) IsValid() bool {
	return s == StatusOpen || s == StatusInProgress || s == StatusBlocked || s == StatusDone || s == StatusCanceled
}

// Item represents a task or epic in the system.
type Item struct {
	ID              string            // Unique identifier
	Project         string            // Project scope (e.g., "gaia", "myapp")
	Type            ItemType          // "task" or "epic"
	Title           string            // Short description
	Description     string            // Full context, notes, handoff info
	Status          Status            // Current state
	Priority        int               // 1=high, 2=medium, 3=low
	ParentID        *string           // Optional parent epic ID
	AgentID         *string           // Agent currently working on this (if in_progress)
	AgentLastActive *time.Time        // Last time agent was active on this
	TemplateID      string            // Template identifier (if templated)
	StepIndex       *int              // Step index within template (nil if none)
	TemplateVars    map[string]string // Template variables (if templated)
	TemplateHash    string            // Hash of template at instantiation
	Results         string            // Results message when done
	Labels          []string          // Attached label names (populated separately)
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Log is a timestamped audit trail entry for an item.
type Log struct {
	ID        int64
	ItemID    string
	Message   string
	CreatedAt time.Time
}

// Dep represents a dependency relationship where ItemID depends on DependsOn.
// ItemID is blocked until DependsOn has status "done".
type Dep struct {
	ItemID    string
	DependsOn string
}

// Project represents a named project that groups related items.
type Project struct {
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// LearningStatus represents the lifecycle state of a learning.
type LearningStatus string

const (
	LearningStatusActive   LearningStatus = "active"
	LearningStatusStale    LearningStatus = "stale"
	LearningStatusArchived LearningStatus = "archived"
)

func (s LearningStatus) IsValid() bool {
	return s == LearningStatusActive || s == LearningStatusStale || s == LearningStatusArchived
}

// Concept represents a knowledge category within a project.
type Concept struct {
	ID            string // con-XXXXXX
	Name          string
	Project       string
	Summary       string
	LastUpdated   time.Time
	LearningCount int // Derived from learning_concepts join
}

// Learning represents a piece of knowledge discovered during work.
type Learning struct {
	ID        string // lrn-XXXXXX
	Project   string
	CreatedAt time.Time
	UpdatedAt time.Time
	TaskID    *string // Optional link to the task that discovered this
	Summary   string  // One-liner
	Detail    string  // Full context
	Files     []string
	Status    LearningStatus
	Concepts  []string // Associated concept names
}

// GenerateLearningID returns a new learning ID with lrn- prefix.
func GenerateLearningID() string {
	return "lrn-" + randomAlpha(DefaultIDLength)
}

// GenerateConceptID returns a new concept ID with con- prefix.
func GenerateConceptID() string {
	return "con-" + randomAlpha(DefaultIDLength)
}

// Label represents a tag that can be attached to items for categorization.
// Labels are project-scoped and identified by name (IDs are internal).
type Label struct {
	ID        string // lbl-XXXXXX (internal)
	Name      string // User-facing identifier, unique per project
	Project   string
	Color     string // Optional hex color for UI display
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GenerateLabelID returns a new label ID with lbl- prefix.
func GenerateLabelID() string {
	return "lbl-" + randomAlpha(DefaultIDLength)
}
