package antigravity

const servicePrefix = "exa.language_server_pb.LanguageServerService"

// StartCascadeRequest defines the payload for creating a new task execution.
type StartCascadeRequest struct {
	UserPrompt                 string         `json:"userPrompt"`
	WorkspaceFolderAbsoluteUri string         `json:"workspaceFolderAbsoluteUri,omitempty"`
	Metadata                   CascadeMetadata `json:"metadata"`
	CascadeConfig              CascadeConfig  `json:"cascadeConfig,omitempty"`
}

type CascadeMetadata struct {
	FileAccessGranted bool `json:"fileAccessGranted"`
}

type CascadeConfig struct {
	PlannerConfig PlannerConfig `json:"plannerConfig"`
}

type PlannerConfig struct {
	PlannerTypeConfig PlannerTypeConfig `json:"plannerTypeConfig,omitempty"`
	RequestedModel    RequestedModel    `json:"requestedModel,omitempty"`
}

type PlannerTypeConfig struct {
	Planning      interface{} `json:"planning,omitempty"`
	Conversational interface{} `json:"conversational,omitempty"`
}

type RequestedModel struct {
	Model string `json:"model"`
}

// StartCascadeResponse contains the identifier for the new trajectory.
type StartCascadeResponse struct {
	CascadeId string `json:"cascadeId"`
}

// GetCascadeTrajectoryRequest defines the payload for fetching execution status.
type GetCascadeTrajectoryRequest struct {
	CascadeId string `json:"cascadeId"`
}

// GetCascadeTrajectoryResponse contains the full state of the execution.
type GetCascadeTrajectoryResponse struct {
	Status    string `json:"status"`
	CascadeId string `json:"cascadeId"`
}

// GetCascadeTrajectoryStepsRequest defines the payload for fetching full step history.
type GetCascadeTrajectoryStepsRequest struct {
	CascadeId  string `json:"cascadeId"`
	StepOffset int    `json:"stepOffset"`
}

// GetCascadeTrajectoryStepsResponse contains the detailed steps of execution.
type GetCascadeTrajectoryStepsResponse struct {
	Steps []CascadeStep `json:"steps"`
}

type CascadeStep struct {
	StepIndex int           `json:"stepIndex"`
	Metadata  interface{}   `json:"metadata"`
	Items     []CascadeItem `json:"items"`
}

type CascadeItem struct {
	Message      *CascadeMessage      `json:"message,omitempty"`
	CallProposed *CascadeCallProposed `json:"callProposed,omitempty"`
	CallResult   *CascadeCallResult   `json:"callResult,omitempty"`
	Error        *CascadeError        `json:"error,omitempty"`
}

type CascadeMessage struct {
	Text string `json:"text"`
}

type CascadeCallProposed struct {
	FunctionCall interface{} `json:"functionCall"`
}

type CascadeCallResult struct {
	FunctionCall interface{} `json:"functionCall"`
}

type CascadeError struct {
	Message string `json:"message"`
}

// Terminal statuses from Antigravity LS
const (
	StatusCompleted = "CASCADE_RUN_STATUS_COMPLETED"
	StatusFailed    = "CASCADE_RUN_STATUS_FAILED"
	StatusAborted   = "CASCADE_RUN_STATUS_ABORTED"
	StatusRunning   = "CASCADE_RUN_STATUS_RUNNING"
)
