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
	state TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
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
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	FOREIGN KEY (phase_id) REFERENCES phases(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS attempts (
	id TEXT PRIMARY KEY,
	step_id TEXT NOT NULL,
	number INTEGER NOT NULL,
	adapter TEXT NOT NULL,
	status TEXT NOT NULL,
	summary TEXT NOT NULL,
	needs_human_decision BOOLEAN NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	FOREIGN KEY (step_id) REFERENCES steps(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS artifacts (
	id TEXT PRIMARY KEY,
	attempt_id TEXT NOT NULL,
	type TEXT NOT NULL,
	path TEXT NOT NULL,
	size INTEGER NOT NULL,
	created_at DATETIME NOT NULL,
	FOREIGN KEY (attempt_id) REFERENCES attempts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS gates (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	step_id TEXT NOT NULL,
	description TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	resolved_at DATETIME,
	FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE,
	FOREIGN KEY (step_id) REFERENCES steps(id) ON DELETE CASCADE
);
`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
