package sqlite

import (
	"database/sql"
)

// In MVP, we can persist validations to a 'validations' table if needed,
// but for now we store the attempt validations as a structured artifact or just
// accept that step.go stores them. To follow the prompt closely, we create a
// simplified ValidationsRepo stub for now that could serialize them.
type ValidationsRepo struct {
	db *sql.DB
}

func NewValidationsRepo(db *sql.DB) *ValidationsRepo {
	return &ValidationsRepo{db: db}
}

// In a real implementation this would map ValidationResult to DB rows.
// For the MVP, we skip full struct implementation to focus on the policy engine gates.
