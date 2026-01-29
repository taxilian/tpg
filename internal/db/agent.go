package db

import (
	"database/sql"
	"os"
	"time"
)

// AgentContext holds current agent information from environment variables
type AgentContext struct {
	ID   string // From $AGENT_ID, empty if not set
	Type string // From $AGENT_TYPE: "primary", "subagent", or ""
}

// GetAgentContext reads agent information from environment variables
func GetAgentContext() AgentContext {
	return AgentContext{
		ID:   os.Getenv("AGENT_ID"),
		Type: os.Getenv("AGENT_TYPE"),
	}
}

// IsActive returns true if this agent has an ID set
func (a AgentContext) IsActive() bool {
	return a.ID != ""
}

// IsSubagent returns true if this is a subagent
func (a AgentContext) IsSubagent() bool {
	return a.Type == "subagent"
}

// RecordAgentProjectAccess updates the agent's last access time for a project
// No-op if agentID is empty
func (db *DB) RecordAgentProjectAccess(agentID, project string) error {
	if agentID == "" || project == "" {
		return nil
	}

	now := sqlTime(time.Now())
	_, err := db.Exec(`
		INSERT INTO agent_sessions (agent_id, project, last_active)
		VALUES (?, ?, ?)
		ON CONFLICT(agent_id, project) 
		DO UPDATE SET last_active = ?
	`, agentID, project, now, now)

	return err
}

// GetAgentLastProject returns the most recent project accessed by this agent
// Returns empty string if agentID is empty or no history found
func (db *DB) GetAgentLastProject(agentID string) (string, error) {
	if agentID == "" {
		return "", nil
	}

	var project string
	err := db.QueryRow(`
		SELECT project FROM agent_sessions
		WHERE agent_id = ?
		ORDER BY last_active DESC
		LIMIT 1
	`, agentID).Scan(&project)

	if err == sql.ErrNoRows {
		return "", nil
	}
	return project, err
}

// CleanupOldAgentSessions keeps only the 20 most recent agent-project pairs
// This prevents unbounded growth of the agent_sessions table
func (db *DB) CleanupOldAgentSessions() error {
	_, err := db.Exec(`
		DELETE FROM agent_sessions
		WHERE rowid NOT IN (
			SELECT rowid FROM agent_sessions
			ORDER BY last_active DESC
			LIMIT 20
		)
	`)
	return err
}
