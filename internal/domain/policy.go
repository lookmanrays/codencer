package domain

// Policy defines constraints and thresholds for evaluation.
type Policy struct {
	Version      string `json:"version"`
	Name         string `json:"name"`
	ContinueWhen struct {
		AllValidationsPass        bool `json:"all_validations_pass"`
		MaxChangedFiles           int  `json:"max_changed_files"`
		NoForbiddenPathsTouched   bool `json:"no_forbidden_paths_touched"`
		NoMigrationsDetected      bool `json:"no_migrations_detected"`
	} `json:"continue_when"`
	GateWhen struct {
		AnyValidationFails           bool `json:"any_validation_fails"`
		DependencyFilesChanged       bool `json:"dependency_files_changed"`
		MigrationsDetected           bool `json:"migrations_detected"`
		ChangedFilesOver             int  `json:"changed_files_over"`
		UnresolvedQuestionsPresent   bool `json:"unresolved_questions_present"`
	} `json:"gate_when"`
	RetryWhen struct {
		AdapterProcessFailed bool `json:"adapter_process_failed"`
		TimeoutOnce          bool `json:"timeout_once"`
	} `json:"retry_when"`
	FailWhen struct {
		TimeoutCountOver          int  `json:"timeout_count_over"`
		ArtifactPersistenceFailed bool `json:"artifact_persistence_failed"`
	} `json:"fail_when"`
}
