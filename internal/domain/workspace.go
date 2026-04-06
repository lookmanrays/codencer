package domain

// ProvisioningSpec defines how a workspace/worktree should be prepared
// before an executor starts.
type ProvisioningSpec struct {
	// Copy paths from base repo to attempt worktree (e.g. .env)
	Copy []string `json:"copy"`
	
	// Symlink paths from base repo to attempt worktree (e.g. node_modules)
	Symlinks []string `json:"symlinks"`
	
	// ProvisioningHooks defines shell commands to run during setup
	Hooks ProvisioningHooks `json:"hooks"`
}

// ProvisioningHooks defines available lifecycle hooks for provisioning.
type ProvisioningHooks struct {
	// PostCreate runs after files are copied/symlinked but before executor starts.
	PostCreate string `json:"post_create"`
}
