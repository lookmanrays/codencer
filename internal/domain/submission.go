package domain

// SubmissionSourceKind identifies the primary submit input source selected by the caller.
type SubmissionSourceKind string

const (
	SubmissionSourceTaskFile   SubmissionSourceKind = "task_file"
	SubmissionSourceTaskJSON   SubmissionSourceKind = "task_json"
	SubmissionSourcePromptFile SubmissionSourceKind = "prompt_file"
	SubmissionSourceGoal       SubmissionSourceKind = "goal"
	SubmissionSourceStdin      SubmissionSourceKind = "stdin"
)

// SubmissionProvenance captures the exact input source and deterministic defaults
// used to normalize submit input into the canonical TaskSpec.
type SubmissionProvenance struct {
	SourceKind      SubmissionSourceKind `json:"source_kind,omitempty"`
	SourceName      string               `json:"source_name,omitempty"`
	OriginalFormat  string               `json:"original_format,omitempty"`
	OriginalInput   string               `json:"original_input,omitempty"`
	DefaultsApplied []string             `json:"defaults_applied,omitempty"`
}
