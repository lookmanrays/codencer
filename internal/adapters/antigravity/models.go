package antigravity

import "encoding/json"

const servicePrefix = "exa.language_server_pb.LanguageServerService"

// StartCascadeRequest defines the payload for creating a new task execution.
type StartCascadeRequest struct {
	UserPrompt                 string          `json:"userPrompt"`
	WorkspaceFolderAbsoluteUri string          `json:"workspaceFolderAbsoluteUri,omitempty"`
	Source                     int             `json:"source,omitempty"`
	Metadata                   CascadeMetadata `json:"metadata"`
	CascadeConfig              CascadeConfig   `json:"cascadeConfig,omitempty"`
}

const (
	// Antigravity requires a CortexTrajectorySource on StartCascade.
	// CASCADE_CLIENT is the normal IDE/client-initiated cascade source.
	CortexTrajectorySourceCascadeClient = 1

	// MODEL_GOOGLE_GEMINI_2_5_FLASH in the Antigravity model enum.
	// This is the current public Antigravity default used for read-only proof runs.
	DefaultRequestedModelGoogleGemini25Flash = 312
)

type SendUserCascadeMessageRequest struct {
	CascadeId           string               `json:"cascadeId"`
	Items               []CascadeMessageItem `json:"items"`
	CascadeConfig       CascadeConfig        `json:"cascadeConfig,omitempty"`
	WaitForLSClientInit bool                 `json:"waitForLsClientInit"`
}

type CascadeMessageItem struct {
	Text string `json:"text"`
}

type HandleCascadeUserInteractionRequest struct {
	CascadeId   string                 `json:"cascadeId"`
	Interaction CascadeUserInteraction `json:"interaction"`
}

type CascadeUserInteraction struct {
	TrajectoryId string                 `json:"trajectoryId"`
	StepIndex    int                    `json:"stepIndex"`
	Interaction  map[string]interface{} `json:"interaction"`
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
	Planning       interface{} `json:"planning,omitempty"`
	Conversational interface{} `json:"conversational,omitempty"`
}

type RequestedModel struct {
	Model int `json:"model"`
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
	StepIndex            int                           `json:"stepIndex,omitempty"`
	Type                 string                        `json:"type,omitempty"`
	Status               string                        `json:"status,omitempty"`
	Metadata             CascadeStepMetadata           `json:"metadata,omitempty"`
	Items                []CascadeItem                 `json:"items,omitempty"`
	RequestedInteraction *CascadeRequestedInteraction  `json:"requestedInteraction,omitempty"`
	ErrorMessage         *CascadeErrorMessageContainer `json:"errorMessage,omitempty"`
	PlannerResponse      *CascadePlannerResponse       `json:"plannerResponse,omitempty"`
	Message              *CascadeMessage               `json:"message,omitempty"`
}

type CascadeStepMetadata struct {
	SourceTrajectoryStepInfo CascadeSourceTrajectoryStepInfo `json:"sourceTrajectoryStepInfo,omitempty"`
	ToolCall                 *CascadeToolCall                `json:"toolCall,omitempty"`
}

type CascadeSourceTrajectoryStepInfo struct {
	TrajectoryId string `json:"trajectoryId,omitempty"`
	StepIndex    int    `json:"stepIndex,omitempty"`
	CascadeId    string `json:"cascadeId,omitempty"`
}

type CascadeToolCall struct {
	Name          string `json:"name,omitempty"`
	OriginalName  string `json:"originalName,omitempty"`
	ArgumentsJSON string `json:"argumentsJson,omitempty"`
}

type CascadeRequestedInteraction struct {
	Permission          *CascadePermissionInteraction `json:"permission,omitempty"`
	ApprovalInteraction map[string]interface{}        `json:"approvalInteraction,omitempty"`
}

type CascadePermissionInteraction struct {
	Resource *CascadePermissionResource `json:"resource,omitempty"`
}

type CascadePermissionResource struct {
	Action string `json:"action,omitempty"`
	Target string `json:"target,omitempty"`
}

type CascadeErrorMessageContainer struct {
	Error CascadeErrorPayload `json:"error,omitempty"`
}

type CascadeErrorPayload struct {
	UserErrorMessage  string `json:"userErrorMessage,omitempty"`
	ModelErrorMessage string `json:"modelErrorMessage,omitempty"`
	ShortError        string `json:"shortError,omitempty"`
	FullError         string `json:"fullError,omitempty"`
}

type CascadePlannerResponse struct {
	Text string `json:"text,omitempty"`
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
	StatusIdle      = "CASCADE_RUN_STATUS_IDLE"

	StepStatusWaiting = "CORTEX_STEP_STATUS_WAITING"
)

type antigravityToolArguments struct {
	Action string `json:"Action"`
	Target string `json:"Target"`
}

func decodeToolArguments(raw string) antigravityToolArguments {
	var args antigravityToolArguments
	if raw == "" {
		return args
	}
	_ = json.Unmarshal([]byte(raw), &args)
	return args
}
