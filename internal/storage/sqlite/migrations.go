package sqlite

import (
	"database/sql"
	"fmt"
)

// RunMigrations applies the base schema to the SQLite database.
func RunMigrations(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS runs (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	conversation_id TEXT,
	planner_id TEXT,
	executor_id TEXT,
	state TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	recovery_notes TEXT
);

CREATE TABLE IF NOT EXISTS phases (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	name TEXT NOT NULL,
	seq_order INTEGER NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS steps (
	id TEXT PRIMARY KEY,
	phase_id TEXT NOT NULL,
	title TEXT NOT NULL,
	goal TEXT NOT NULL,
	state TEXT NOT NULL,
	policy TEXT NOT NULL,
	adapter TEXT NOT NULL,
	timeout_seconds INTEGER NOT NULL DEFAULT 0,
	status_reason TEXT,
	validations TEXT,
	task_spec_snapshot TEXT,
	submission_provenance TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	FOREIGN KEY (phase_id) REFERENCES phases(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS attempts (
	id TEXT PRIMARY KEY,
	step_id TEXT NOT NULL,
	number INTEGER NOT NULL,
	adapter TEXT NOT NULL,
	state TEXT NOT NULL,
	summary TEXT NOT NULL,
	needs_human_decision BOOLEAN NOT NULL DEFAULT 0,
	warnings TEXT,
	questions TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	files_changed TEXT,
	raw_output TEXT,
	raw_output_ref TEXT,
	is_simulation BOOLEAN NOT NULL DEFAULT 0,
	retryable BOOLEAN NOT NULL DEFAULT 0,
	version TEXT,
	artifacts TEXT,
	provisioning TEXT,
	FOREIGN KEY (step_id) REFERENCES steps(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS artifacts (
	id TEXT PRIMARY KEY,
	attempt_id TEXT NOT NULL,
	type TEXT NOT NULL,
	name TEXT NOT NULL,
	path TEXT NOT NULL,
	size INTEGER NOT NULL,
	hash TEXT,
	mime_type TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	FOREIGN KEY (attempt_id) REFERENCES attempts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS gates (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	step_id TEXT NOT NULL,
	description TEXT NOT NULL,
	state TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	resolved_at DATETIME,
	FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE,
	FOREIGN KEY (step_id) REFERENCES steps(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS validations (
	attempt_id TEXT NOT NULL,
	name TEXT NOT NULL,
	command TEXT NOT NULL,
	state TEXT NOT NULL,
	passed BOOLEAN NOT NULL,
	exit_code INTEGER NOT NULL,
	stdout_ref TEXT,
	stderr_ref TEXT,
	error_msg TEXT,
	duration_ms INTEGER NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	PRIMARY KEY (attempt_id, name),
	FOREIGN KEY (attempt_id) REFERENCES attempts(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS benchmarks (
	id TEXT PRIMARY KEY,
	adapter TEXT NOT NULL,
	phase_id TEXT NOT NULL,
	attempt_id TEXT NOT NULL DEFAULT '',
	duration_ms INTEGER NOT NULL,
	validations_hit INTEGER NOT NULL,
	validations_max INTEGER NOT NULL,
	cost_cents REAL NOT NULL,
	failure_reason TEXT,
	is_simulation BOOLEAN NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
	key TEXT PRIMARY KEY,
	value TEXT
);
`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Ensure columns added in Batch 2-6 exist for legacy databases
	extraMigrations := []string{
		"ALTER TABLE artifacts ADD COLUMN name TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE artifacts ADD COLUMN updated_at DATETIME NOT NULL DEFAULT '1970-01-01'",
		"ALTER TABLE validations ADD COLUMN command TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE validations ADD COLUMN status TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE validations ADD COLUMN exit_code INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE validations ADD COLUMN stdout_ref TEXT",
		"ALTER TABLE validations ADD COLUMN stderr_ref TEXT",
		"ALTER TABLE validations ADD COLUMN duration_ms INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE validations ADD COLUMN updated_at DATETIME NOT NULL DEFAULT '1970-01-01'",
		"ALTER TABLE attempts ADD COLUMN warnings TEXT",
		"ALTER TABLE attempts ADD COLUMN questions TEXT",
		"ALTER TABLE runs ADD COLUMN recovery_notes TEXT",
		"ALTER TABLE benchmarks ADD COLUMN attempt_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE benchmarks ADD COLUMN is_simulation BOOLEAN NOT NULL DEFAULT 0",
		"ALTER TABLE attempts ADD COLUMN state TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE validations ADD COLUMN state TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE attempts ADD COLUMN files_changed TEXT",
		"ALTER TABLE attempts ADD COLUMN raw_output TEXT",
		"ALTER TABLE attempts ADD COLUMN raw_output_ref TEXT",
		"ALTER TABLE attempts ADD COLUMN is_simulation BOOLEAN NOT NULL DEFAULT 0",
		"ALTER TABLE attempts ADD COLUMN retryable BOOLEAN NOT NULL DEFAULT 0",
		"ALTER TABLE gates ADD COLUMN state TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE artifacts ADD COLUMN hash TEXT",
		"ALTER TABLE artifacts ADD COLUMN mime_type TEXT",
		"ALTER TABLE attempts ADD COLUMN version TEXT",
		"ALTER TABLE attempts ADD COLUMN artifacts TEXT",
		"ALTER TABLE steps ADD COLUMN timeout_seconds INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE runs ADD COLUMN conversation_id TEXT",
		"ALTER TABLE runs ADD COLUMN planner_id TEXT",
		"ALTER TABLE runs ADD COLUMN executor_id TEXT",
		"ALTER TABLE steps ADD COLUMN status_reason TEXT",
		"ALTER TABLE steps ADD COLUMN validations TEXT",
		"ALTER TABLE attempts ADD COLUMN provisioning TEXT",
		"ALTER TABLE steps ADD COLUMN task_spec_snapshot TEXT",
		"ALTER TABLE steps ADD COLUMN submission_provenance TEXT",
	}

	for _, m := range extraMigrations {
		// Ignore errors if columns/indexes already exist
		_, _ = db.Exec(m)
	}

	// Add indexes for Batch 2 metadata
	_, _ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runs_project_id ON runs(project_id)")
	_, _ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runs_conversation_id ON runs(conversation_id)")

	return nil
}
