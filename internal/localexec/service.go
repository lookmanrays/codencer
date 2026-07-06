package localexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/local"
	manifestpkg "agent-bridge/internal/manifest"
	profilepkg "agent-bridge/internal/profile"
	projectpkg "agent-bridge/internal/project"
	"agent-bridge/internal/security"
	"agent-bridge/internal/validation"
)

const (
	defaultPollInterval = 500 * time.Millisecond
	defaultWaitTimeout  = 10 * time.Minute
)

type Service struct {
	HTTPClient   *http.Client
	Now          func() time.Time
	PollInterval time.Duration
	WaitTimeout  time.Duration
	Stdin        io.Reader
}

type BaseOptions struct {
	ProjectID    string
	RepoRoot     string
	ConfigPath   string
	CodencerHome string
}

type RunOptions struct {
	BaseOptions
	RunID string
}

type SubmitOptions struct {
	BaseOptions
	RunID          string
	Goal           string
	TaskFile       string
	PromptFile     string
	UseStdin       bool
	SourceKind     domain.SubmissionSourceKind
	SourceName     string
	Content        []byte
	Wait           bool
	Adapter        string
	Profile        string
	AdapterProfile string
	Title          string
	TimeoutSeconds int
}

type RunPlanOptions struct {
	BaseOptions
	ManifestPath string
	Manifest     *manifestpkg.Manifest
	ManifestName string
	Wait         bool
}

type RunPlanReportOptions struct {
	BaseOptions
	RunID string
}

type ReportError struct {
	Code    int
	Message string
	Blocker *Blocker
}

func (e *ReportError) Error() string { return e.Message }

func NewService() *Service {
	return &Service{}
}

func (s *Service) ListProfiles() ExecutionReport {
	return ExecutionReport{
		OK:       true,
		Status:   "ok",
		Profiles: profilepkg.List(),
		ExitCode: ExitSuccess,
	}
}

func (s *Service) GetProfile(id string) (ExecutionReport, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ExecutionReport{}, reportError(ExitInvalidInput, BlockerInvalidInput, "profile id is required")
	}
	profile, ok := profilepkg.Get(id)
	if !ok {
		return ExecutionReport{}, reportError(ExitInvalidInput, BlockerInvalidInput, fmt.Sprintf("unknown profile %q", id))
	}
	return ExecutionReport{
		OK:       true,
		Status:   "ok",
		Profile:  &profile,
		ExitCode: ExitSuccess,
	}, nil
}

func (s *Service) StartRun(ctx context.Context, opts RunOptions) (ExecutionReport, error) {
	resolved, err := s.resolve(ctx, opts.BaseOptions)
	if err != nil {
		return ExecutionReport{}, err
	}
	run, err := resolved.client.StartRun(ctx, resolved.project.ID)
	if err != nil {
		return resolved.daemonReport(err), nil
	}
	return resolved.report(ExecutionReport{
		OK:       true,
		Status:   "started",
		Run:      run,
		ExitCode: ExitSuccess,
	}), nil
}

func (s *Service) ListRuns(ctx context.Context, opts RunOptions) (ExecutionReport, error) {
	resolved, err := s.resolve(ctx, opts.BaseOptions)
	if err != nil {
		return ExecutionReport{}, err
	}
	runs, err := resolved.client.ListRuns(ctx, resolved.project.ID)
	if err != nil {
		return resolved.daemonReport(err), nil
	}
	return resolved.report(ExecutionReport{
		OK:       true,
		Status:   "ok",
		Runs:     runs,
		ExitCode: ExitSuccess,
	}), nil
}

func (s *Service) GetRun(ctx context.Context, opts RunOptions) (ExecutionReport, error) {
	if strings.TrimSpace(opts.RunID) == "" {
		return ExecutionReport{}, reportError(ExitInvalidInput, BlockerInvalidInput, "run id is required")
	}
	resolved, err := s.resolve(ctx, opts.BaseOptions)
	if err != nil {
		return ExecutionReport{}, err
	}
	run, err := resolved.client.GetRun(ctx, opts.RunID)
	if err != nil {
		return resolved.daemonReport(err), nil
	}
	steps, stepErr := resolved.client.GetRunSteps(ctx, opts.RunID)
	report := resolved.report(ExecutionReport{
		OK:       true,
		Status:   string(run.State),
		Run:      run,
		Steps:    steps,
		ExitCode: ExitSuccess,
	})
	if stepErr != nil {
		report.Blocker = daemonBlocker(stepErr)
		report.EvidenceWarning("could not fetch run steps: " + stepErr.Error())
	}
	return report, nil
}

func (s *Service) Status(ctx context.Context, opts RunOptions) (ExecutionReport, error) {
	if strings.TrimSpace(opts.RunID) != "" {
		return s.GetRun(ctx, opts)
	}
	resolved, err := s.resolve(ctx, opts.BaseOptions)
	if err != nil {
		return ExecutionReport{}, err
	}
	runs, err := resolved.client.ListRuns(ctx, resolved.project.ID)
	if err != nil {
		return resolved.daemonReport(err), nil
	}
	return resolved.report(ExecutionReport{
		OK:       true,
		Status:   "ok",
		Runs:     runs,
		ExitCode: ExitSuccess,
	}), nil
}

func (s *Service) Events(ctx context.Context, opts RunOptions) (ExecutionReport, error) {
	if strings.TrimSpace(opts.RunID) == "" {
		return ExecutionReport{}, reportError(ExitInvalidInput, BlockerInvalidInput, "run id is required")
	}
	resolved, err := s.resolve(ctx, opts.BaseOptions)
	if err != nil {
		return ExecutionReport{}, err
	}
	run, err := resolved.client.GetRun(ctx, opts.RunID)
	if err != nil {
		return resolved.daemonReport(err), nil
	}
	steps, err := resolved.client.GetRunSteps(ctx, opts.RunID)
	if err != nil {
		return resolved.daemonReport(err), nil
	}
	return resolved.report(ExecutionReport{
		OK:       true,
		Status:   "ok",
		Run:      run,
		Steps:    steps,
		Events:   eventsFromRunAndSteps(run, steps),
		ExitCode: ExitSuccess,
	}), nil
}

func (s *Service) CancelRun(ctx context.Context, opts RunOptions) (ExecutionReport, error) {
	if strings.TrimSpace(opts.RunID) == "" {
		return ExecutionReport{}, reportError(ExitInvalidInput, BlockerInvalidInput, "run id is required")
	}
	resolved, err := s.resolve(ctx, opts.BaseOptions)
	if err != nil {
		return ExecutionReport{}, err
	}
	if err := resolved.client.AbortRun(ctx, opts.RunID); err != nil {
		return resolved.daemonReport(err), nil
	}
	report := ExecutionReport{
		OK:       true,
		Status:   "cancel_requested",
		ExitCode: ExitSuccess,
	}
	if run, err := resolved.client.GetRun(ctx, opts.RunID); err == nil {
		report.Run = run
		report.Status = string(run.State)
	}
	return resolved.report(report), nil
}

func (s *Service) ResumeRun(ctx context.Context, opts RunOptions) (ExecutionReport, error) {
	if strings.TrimSpace(opts.RunID) == "" {
		return ExecutionReport{}, reportError(ExitInvalidInput, BlockerInvalidInput, "run id is required")
	}
	resolved, err := s.resolve(ctx, opts.BaseOptions)
	if err != nil {
		return ExecutionReport{}, err
	}
	run, err := resolved.client.ResumeRun(ctx, opts.RunID)
	if err == nil {
		return resolved.report(ExecutionReport{
			OK:     true,
			Status: string(run.State),
			Run:    run,
			Events: []RunEvent{{
				Type:      "run_resumed",
				RunID:     run.ID,
				State:     string(run.State),
				Summary:   run.RecoveryNotes,
				CreatedAt: formatEventTime(run.UpdatedAt),
			}},
			ExitCode: ExitSuccess,
		}), nil
	}
	if daemonErr, ok := err.(*DaemonError); ok && daemonErr.Kind == BlockerDaemonNotRunning {
		return resolved.daemonReport(err), nil
	}
	blocker := &Blocker{
		Type:                 BlockerUnsupportedOperation,
		Message:              "run resume is not supported for the current daemon/run state; approve/reject pending gates or start a new task",
		NeedsPlannerDecision: true,
		Retryable:            false,
		ObservedFacts:        []string{daemonErrorMessage(err)},
	}
	interrupt := humanInterruptFromBlocker(blocker, resolved.project.ID, opts.RunID, "", resolved.project.AdapterProfile, "", "")
	if interrupt != nil {
		blocker.Interrupt = interrupt
	}
	return resolved.report(ExecutionReport{
		OK:              false,
		Status:          "blocked",
		Blocker:         blocker,
		HumanInterrupts: interruptList(interrupt),
		Events: []RunEvent{{
			Type:    "run_resume_blocked",
			RunID:   opts.RunID,
			State:   "blocked",
			Summary: blocker.Message,
		}},
		ExitCode: ExitBlocked,
	}), nil
}

func daemonErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if daemonErr, ok := err.(*DaemonError); ok {
		return strings.TrimSpace(daemonErr.Message)
	}
	return strings.TrimSpace(err.Error())
}

func eventsFromRunAndSteps(run *domain.Run, steps []*domain.Step) []RunEvent {
	events := []RunEvent{}
	if run != nil {
		events = append(events, RunEvent{
			Type:      "run_created",
			RunID:     run.ID,
			State:     string(run.State),
			CreatedAt: formatEventTime(run.CreatedAt),
		})
		if run.State != "" && run.State != domain.RunStateCreated {
			events = append(events, RunEvent{
				Type:      "run_" + string(run.State),
				RunID:     run.ID,
				State:     string(run.State),
				Summary:   run.RecoveryNotes,
				CreatedAt: formatEventTime(run.UpdatedAt),
			})
		}
	}
	for _, step := range steps {
		if step == nil {
			continue
		}
		runID := ""
		if run != nil {
			runID = run.ID
		}
		events = append(events, RunEvent{
			Type:      "step_" + string(step.State),
			RunID:     runID,
			StepID:    step.ID,
			State:     string(step.State),
			Summary:   step.StatusReason,
			CreatedAt: formatEventTime(step.UpdatedAt),
		})
		if step.State == domain.StepStateNeedsApproval || step.State == domain.StepStateNeedsManualAttention {
			events = append(events, RunEvent{
				Type:      "human_interrupt_created",
				RunID:     runID,
				StepID:    step.ID,
				State:     "waiting_for_human",
				Summary:   step.StatusReason,
				CreatedAt: formatEventTime(step.UpdatedAt),
			})
		}
	}
	return events
}

func formatEventTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func (s *Service) Submit(ctx context.Context, opts SubmitOptions) (ExecutionReport, error) {
	resolved, err := s.resolve(ctx, opts.BaseOptions)
	if err != nil {
		return ExecutionReport{}, err
	}
	resolution, err := resolveProfile(resolved.project, opts.Profile, opts.AdapterProfile, opts.Adapter, "")
	if err != nil {
		return ExecutionReport{}, profileReportError(err)
	}

	runID := strings.TrimSpace(opts.RunID)
	run := &domain.Run{ID: runID, ProjectID: resolved.project.ID}
	if runID == "" {
		run, err = resolved.client.StartRun(ctx, resolved.project.ID)
		if err != nil {
			return resolved.daemonReport(err), nil
		}
	} else {
		run, err = resolved.client.GetRun(ctx, runID)
		if err != nil {
			return suppliedRunLookupReport(resolved, runID, err), nil
		}
		if run == nil {
			return suppliedRunInvalidReport(resolved, runID, fmt.Sprintf("run %s not found", runID)), nil
		}
		if strings.TrimSpace(run.ID) == "" {
			run.ID = runID
		}
		if run.ProjectID != "" && run.ProjectID != resolved.project.ID {
			return suppliedRunInvalidReport(resolved, runID, fmt.Sprintf("run %s belongs to project %s, not %s", runID, run.ProjectID, resolved.project.ID)), nil
		}
	}

	task, taskID, err := s.normalizeSubmit(opts, run.ID)
	if err != nil {
		return ExecutionReport{}, reportError(ExitInvalidInput, BlockerInvalidInput, err.Error())
	}
	enrichTask(task, resolved.project, run.ID, taskID, resolution, opts.TimeoutSeconds)

	step, err := resolved.client.SubmitTask(ctx, run.ID, task)
	if err != nil {
		return resolved.daemonReport(err), nil
	}
	if !opts.Wait {
		taskReport := taskReportFromStep(taskID, resolved.project.ID, run.ID, step, resolution, nil, Evidence{}, ExitSuccess)
		_, err := writeSubmitRunReport(resolved, run, taskReport)
		if err != nil {
			return ExecutionReport{}, reportError(ExitInternal, BlockerBridgeError, err.Error())
		}
		return resolved.report(ExecutionReport{
			OK:              true,
			Status:          "submitted",
			Run:             run,
			Task:            &taskReport,
			HumanInterrupts: taskReport.HumanInterrupts,
			ExitCode:        ExitSuccess,
		}), nil
	}
	taskReport := s.waitForTask(ctx, resolved.client, resolved.project.ID, run.ID, taskID, step.ID, resolution)
	if _, err := writeSubmitRunReport(resolved, run, taskReport); err != nil {
		return ExecutionReport{}, reportError(ExitInternal, BlockerBridgeError, err.Error())
	}
	report := resolved.report(ExecutionReport{
		OK:              taskReport.OK,
		Status:          taskReport.Status,
		Run:             run,
		Task:            &taskReport,
		Blocker:         taskReport.Blocker,
		HumanInterrupts: taskReport.HumanInterrupts,
		ExitCode:        taskReport.ExitCode,
	})
	return report, nil
}

func writeSubmitRunReport(resolved *resolvedContext, run *domain.Run, taskReport TaskReport) (string, error) {
	if resolved == nil {
		return "", fmt.Errorf("resolved context is required")
	}
	if run == nil {
		run = &domain.Run{ID: taskReport.RunID, ProjectID: taskReport.ProjectID}
	}
	report := RunPlanReport{
		OK:              taskReport.OK,
		Status:          taskReport.Status,
		Project:         resolved.projectSummary(taskReport.Profile),
		Run:             run,
		Tasks:           []TaskReport{taskReport},
		Blocker:         taskReport.Blocker,
		HumanInterrupts: taskReport.HumanInterrupts,
		Evidence:        taskReport.Evidence,
		ExitCode:        taskReport.ExitCode,
	}
	path, err := writeRunPlanReport(resolved.paths.ArtifactsDir, run.ID, report)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) RunPlan(ctx context.Context, opts RunPlanOptions) (RunPlanReport, error) {
	if opts.Manifest == nil && strings.TrimSpace(opts.ManifestPath) == "" {
		return RunPlanReport{}, reportError(ExitInvalidInput, BlockerInvalidInput, "manifest path is required")
	}
	manifest := opts.Manifest
	manifestPath := strings.TrimSpace(opts.ManifestPath)
	if manifest == nil {
		var err error
		manifest, _, err = manifestpkg.Load(manifestPath)
		if err != nil {
			return RunPlanReport{}, reportError(ExitInvalidInput, BlockerInvalidInput, err.Error())
		}
	} else if manifestPath == "" {
		manifestPath = strings.TrimSpace(opts.ManifestName)
		if manifestPath == "" {
			manifestPath = "inline-manifest"
		}
	}
	base := opts.BaseOptions
	base.ProjectID = manifestpkg.ProjectID(opts.ProjectID, manifest)
	resolved, err := s.resolve(ctx, base)
	if err != nil {
		return RunPlanReport{}, err
	}
	run, err := resolved.client.StartRun(ctx, resolved.project.ID)
	if err != nil {
		report := RunPlanReport{
			OK:           false,
			Status:       "blocked",
			ManifestPath: manifestPath,
			Project:      resolved.projectSummary(""),
			Blocker:      daemonBlocker(err),
			ExitCode:     daemonExitCode(err),
		}
		return report, nil
	}

	report := RunPlanReport{
		OK:           true,
		Status:       "completed",
		ManifestPath: manifestPath,
		Project:      resolved.projectSummary(""),
		Run:          run,
		Tasks:        []TaskReport{},
		ExitCode:     ExitSuccess,
	}
	for i, task := range manifest.Tasks {
		taskID := strings.TrimSpace(task.ID)
		if taskID == "" {
			taskID = fmt.Sprintf("task-%d", i+1)
		}
		taskReport := s.executeManifestTask(ctx, resolved, run.ID, taskID, task, manifest.Execution, manifestPath, manifest.Policy.Retry)
		report.Tasks = append(report.Tasks, taskReport)
		if taskReport.ExitCode == ExitSuccess {
			continue
		}
		report.OK = false
		report.Status = taskReport.Status
		report.Blocker = taskReport.Blocker
		report.HumanInterrupts = append(report.HumanInterrupts, taskReport.HumanInterrupts...)
		report.StoppedAtTask = taskID
		report.ExitCode = taskReport.ExitCode

		if taskReport.ExitCode == ExitBlocked && !manifestpkg.StopOnBlocker(manifest.Policy) {
			continue
		}
		if taskReport.ExitCode != ExitBlocked && !manifestpkg.StopOnFailure(manifest.Policy) {
			continue
		}
		break
	}
	if len(report.Tasks) == 0 {
		report.OK = false
		report.Status = "invalid_input"
		report.ExitCode = ExitInvalidInput
		report.Blocker = &Blocker{Type: BlockerInvalidInput, Message: "manifest has no tasks", NeedsPlannerDecision: true}
	}
	if len(report.HumanInterrupts) == 0 {
		for _, task := range report.Tasks {
			report.HumanInterrupts = append(report.HumanInterrupts, task.HumanInterrupts...)
		}
	}
	report.ReportPath, err = writeRunPlanReport(resolved.paths.ArtifactsDir, run.ID, report)
	if err != nil {
		return RunPlanReport{}, reportError(ExitInternal, BlockerBridgeError, err.Error())
	}
	return report, nil
}

func suppliedRunLookupReport(resolved *resolvedContext, runID string, err error) ExecutionReport {
	var daemonErr *DaemonError
	if errors.As(err, &daemonErr) && daemonErr.StatusCode == http.StatusNotFound {
		return suppliedRunInvalidReport(resolved, runID, fmt.Sprintf("run %s not found", runID))
	}
	return resolved.daemonReport(err)
}

func suppliedRunInvalidReport(resolved *resolvedContext, runID, message string) ExecutionReport {
	blocker := &Blocker{
		Type:                 BlockerInvalidInput,
		Message:              message,
		NeedsPlannerDecision: true,
		Retryable:            false,
	}
	return resolved.report(ExecutionReport{
		OK:       false,
		Status:   "blocked",
		Run:      &domain.Run{ID: runID, ProjectID: resolved.project.ID},
		Blocker:  blocker,
		ExitCode: ExitInvalidInput,
	})
}

func (s *Service) GetRunPlanReport(ctx context.Context, opts RunPlanReportOptions) (RunPlanReport, error) {
	if strings.TrimSpace(opts.RunID) == "" {
		return RunPlanReport{}, reportError(ExitInvalidInput, BlockerInvalidInput, "run id is required")
	}
	resolved, err := s.resolve(ctx, opts.BaseOptions)
	if err != nil {
		return RunPlanReport{}, err
	}
	path := filepath.Join(resolved.paths.ArtifactsDir, "run-plans", opts.RunID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RunPlanReport{}, reportError(ExitInvalidInput, BlockerBridgeError, "run-plan report not found")
		}
		return RunPlanReport{}, reportError(ExitInternal, BlockerBridgeError, err.Error())
	}
	var report RunPlanReport
	if err := json.Unmarshal(data, &report); err != nil {
		return RunPlanReport{}, reportError(ExitInternal, BlockerBridgeError, err.Error())
	}
	if report.Project == nil {
		report.Project = resolved.projectSummary("")
	}
	if report.ReportPath == "" {
		report.ReportPath = path
	}
	refreshed, changed := s.refreshNonTerminalRunPlanReport(ctx, resolved, report)
	if changed {
		refreshed.ReportPath = ""
		writtenPath, err := writeRunPlanReport(resolved.paths.ArtifactsDir, opts.RunID, refreshed)
		if err != nil {
			return RunPlanReport{}, reportError(ExitInternal, BlockerBridgeError, err.Error())
		}
		refreshed.ReportPath = writtenPath
		return refreshed, nil
	}
	return report, nil
}

func (s *Service) refreshNonTerminalRunPlanReport(ctx context.Context, resolved *resolvedContext, report RunPlanReport) (RunPlanReport, bool) {
	if resolved == nil || resolved.client == nil || report.Run == nil || strings.TrimSpace(report.Run.ID) == "" || len(report.Tasks) == 0 {
		return report, false
	}
	if runPlanReportTerminal(report) {
		return report, false
	}
	runID := report.Run.ID
	tasks := make([]TaskReport, 0, len(report.Tasks))
	changed := false
	for _, task := range report.Tasks {
		next, ok := s.refreshTaskReport(ctx, resolved, runID, task)
		if ok {
			changed = true
		}
		tasks = append(tasks, next)
	}
	if !changed {
		return report, false
	}
	report.Tasks = tasks
	applyTaskReportsToRunPlan(&report)
	return report, true
}

func (s *Service) refreshTaskReport(ctx context.Context, resolved *resolvedContext, runID string, task TaskReport) (TaskReport, bool) {
	if strings.TrimSpace(task.StepID) == "" || taskReportTerminal(task) {
		return task, false
	}
	step, err := resolved.client.GetStep(ctx, task.StepID)
	if err != nil || step == nil {
		return task, false
	}
	resolution := profilepkg.Resolution{
		ProfileID:     firstNonEmpty(task.Profile, task.AdapterProfile, resolved.project.AdapterProfile),
		Adapter:       firstNonEmpty(task.Adapter, resolved.project.DefaultAdapter),
		DaemonAdapter: firstNonEmpty(task.AdapterProfile, task.Adapter, resolved.project.DefaultAdapter),
	}
	if step.State.IsTerminal() || step.State == domain.StepStateNeedsApproval || step.State == domain.StepStateNeedsManualAttention {
		evidence := fetchEvidence(context.Background(), resolved.client, task.StepID)
		result := evidence.Result
		if result == nil {
			result = &domain.ResultSpec{State: step.State, Summary: step.StatusReason}
		}
		blocker, exitCode := blockerForResult(step, result, evidence)
		return taskReportFromStep(firstNonEmpty(task.TaskID, "task"), firstNonEmpty(task.ProjectID, resolved.project.ID), runID, step, resolution, blocker, evidence, exitCode), true
	}
	task.Status = string(step.State)
	task.Step = step
	task.Title = firstNonEmpty(step.Title, task.Title)
	task.Summary = firstNonEmpty(step.StatusReason, task.Summary)
	return task, true
}

func applyTaskReportsToRunPlan(report *RunPlanReport) {
	if report == nil || len(report.Tasks) == 0 {
		return
	}
	report.OK = true
	report.Status = "completed"
	report.ExitCode = ExitSuccess
	report.Blocker = nil
	report.HumanInterrupts = nil
	report.Evidence = Evidence{}
	for _, task := range report.Tasks {
		report.HumanInterrupts = append(report.HumanInterrupts, task.HumanInterrupts...)
		if !taskReportTerminal(task) {
			report.Status = firstNonEmpty(task.Status, "submitted")
			report.OK = true
			report.ExitCode = ExitSuccess
			return
		}
		if task.ExitCode != ExitSuccess {
			report.OK = false
			report.Status = task.Status
			report.ExitCode = task.ExitCode
			report.Blocker = task.Blocker
			report.Evidence = task.Evidence
			return
		}
	}
	first := report.Tasks[0]
	report.Status = first.Status
	report.Evidence = first.Evidence
}

func runPlanReportTerminal(report RunPlanReport) bool {
	if terminalReportStatus(report.Status) {
		return true
	}
	if len(report.Tasks) == 0 {
		return false
	}
	for _, task := range report.Tasks {
		if !taskReportTerminal(task) {
			return false
		}
	}
	return true
}

func taskReportTerminal(task TaskReport) bool {
	return terminalReportStatus(task.Status)
}

func terminalReportStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "completed_with_warnings", "failed_validation", "failed_adapter", "failed_bridge", "failed_retryable", "cancelled", "blocked", BlockerValidationFailed, BlockerManualApproval, BlockerQuestion, BlockerUnknown, BlockerAdapterError, BlockerBridgeError, BlockerTimeout, BlockerFailedTerminal, BlockerUnsafeAction, BlockerInvalidInput, BlockerConfigurationRequired, BlockerUnsupportedOperation:
		return true
	default:
		return false
	}
}

func (s *Service) executeManifestTask(ctx context.Context, resolved *resolvedContext, runID, taskID string, task manifestpkg.Task, execution manifestpkg.Execution, manifestPath string, retry manifestpkg.RetryPolicy) TaskReport {
	maxAttempts := 1
	if retry.Enabled && retry.MaxAttempts > 1 {
		maxAttempts = retry.MaxAttempts
	}
	var last TaskReport
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		profileID := firstNonEmpty(task.Profile, execution.Profile)
		adapterID := firstNonEmpty(task.Adapter, execution.Adapter)
		resolution, err := resolveProfile(resolved.project, profileID, "", adapterID, "")
		if err != nil {
			return TaskReport{
				OK:        false,
				Status:    "blocked",
				TaskID:    taskID,
				ProjectID: resolved.project.ID,
				RunID:     runID,
				Blocker:   profileBlocker(err),
				ExitCode:  ExitInvalidInput,
			}
		}
		spec, err := manifestTaskSpec(task, manifestPath, runID)
		if err != nil {
			return TaskReport{
				OK:        false,
				Status:    "invalid_input",
				TaskID:    taskID,
				ProjectID: resolved.project.ID,
				RunID:     runID,
				Blocker:   &Blocker{Type: BlockerInvalidInput, Message: err.Error(), NeedsPlannerDecision: true},
				ExitCode:  ExitInvalidInput,
			}
		}
		spec.StepID = manifestStepID(taskID, attempt)
		timeout, err := manifestpkg.TimeoutSeconds(firstNonEmpty(task.Timeout, execution.Timeout))
		if err != nil {
			return TaskReport{
				OK:        false,
				Status:    "invalid_input",
				TaskID:    taskID,
				ProjectID: resolved.project.ID,
				RunID:     runID,
				Blocker:   &Blocker{Type: BlockerInvalidInput, Message: err.Error(), NeedsPlannerDecision: true},
				ExitCode:  ExitInvalidInput,
			}
		}
		enrichTask(spec, resolved.project, runID, taskID, resolution, timeout)
		step, err := resolved.client.SubmitTask(ctx, runID, spec)
		if err != nil {
			last = daemonTaskReport(taskID, resolved.project.ID, runID, resolution, err)
		} else {
			last = s.waitForTask(ctx, resolved.client, resolved.project.ID, runID, taskID, step.ID, resolution)
		}
		if last.ExitCode == ExitSuccess || attempt == maxAttempts || !retryableForManifest(last.Blocker) {
			return last
		}
	}
	return last
}

func (s *Service) waitForTask(ctx context.Context, client *Client, projectID, runID, taskID, stepID string, resolution profilepkg.Resolution) TaskReport {
	waitTimeout := s.WaitTimeout
	if waitTimeout == 0 {
		waitTimeout = defaultWaitTimeout
	}
	pollInterval := s.PollInterval
	if pollInterval == 0 {
		pollInterval = defaultPollInterval
	}
	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	var step *domain.Step
	for {
		next, err := client.GetStep(waitCtx, stepID)
		if err != nil {
			return daemonTaskReport(taskID, projectID, runID, resolution, err)
		}
		step = next
		if step.State.IsTerminal() || step.State == domain.StepStateNeedsApproval || step.State == domain.StepStateNeedsManualAttention {
			break
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			blocker := &Blocker{
				Type:                 BlockerTimeout,
				Message:              fmt.Sprintf("wait timed out after %s", waitTimeout),
				NeedsPlannerDecision: true,
				Retryable:            true,
			}
			return taskReportFromStep(taskID, projectID, runID, step, resolution, blocker, Evidence{}, ExitTimeout)
		case <-timer.C:
		}
	}

	evidence := fetchEvidence(context.Background(), client, stepID)
	result := evidence.Result
	if result == nil {
		result = &domain.ResultSpec{State: step.State, Summary: step.StatusReason}
	}
	blocker, exitCode := blockerForResult(step, result, evidence)
	return taskReportFromStep(taskID, projectID, runID, step, resolution, blocker, evidence, exitCode)
}

func (s *Service) normalizeSubmit(opts SubmitOptions, runID string) (*domain.TaskSpec, string, error) {
	kind, sourceName, content, err := s.submitSource(opts)
	if err != nil {
		return nil, "", err
	}
	normalized, err := validation.NormalizeTaskInput(validation.NormalizeTaskInputRequest{
		RunID:      runID,
		SourceKind: kind,
		SourceName: sourceName,
		Content:    content,
		Direct: validation.DirectTaskOptions{
			Title:          opts.Title,
			TimeoutSeconds: opts.TimeoutSeconds,
		},
	})
	if err != nil {
		return nil, "", err
	}
	taskID := normalized.Task.StepID
	if taskID == "" {
		taskID = "task"
	}
	return normalized.Task, taskID, nil
}

func (s *Service) submitSource(opts SubmitOptions) (domain.SubmissionSourceKind, string, []byte, error) {
	count := 0
	if len(opts.Content) > 0 || opts.SourceKind != "" {
		count++
	}
	if strings.TrimSpace(opts.Goal) != "" {
		count++
	}
	if strings.TrimSpace(opts.TaskFile) != "" {
		count++
	}
	if strings.TrimSpace(opts.PromptFile) != "" {
		count++
	}
	if opts.UseStdin {
		count++
	}
	if count != 1 {
		return "", "", nil, fmt.Errorf("exactly one of --goal, --task-file, --prompt-file, or --stdin is required")
	}
	if len(opts.Content) > 0 || opts.SourceKind != "" {
		sourceKind := opts.SourceKind
		if sourceKind == "" {
			sourceKind = domain.SubmissionSourceStdin
		}
		sourceName := strings.TrimSpace(opts.SourceName)
		if sourceName == "" {
			sourceName = "inline"
		}
		return sourceKind, sourceName, opts.Content, nil
	}
	if strings.TrimSpace(opts.Goal) != "" {
		return domain.SubmissionSourceGoal, "goal", []byte(opts.Goal), nil
	}
	if strings.TrimSpace(opts.TaskFile) != "" {
		content, err := os.ReadFile(opts.TaskFile)
		if err != nil {
			return "", "", nil, fmt.Errorf("read task file: %w", err)
		}
		return domain.SubmissionSourceTaskFile, opts.TaskFile, content, nil
	}
	if strings.TrimSpace(opts.PromptFile) != "" {
		content, err := os.ReadFile(opts.PromptFile)
		if err != nil {
			return "", "", nil, fmt.Errorf("read prompt file: %w", err)
		}
		return domain.SubmissionSourcePromptFile, opts.PromptFile, content, nil
	}
	reader := s.Stdin
	if reader == nil {
		reader = os.Stdin
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", "", nil, fmt.Errorf("read stdin: %w", err)
	}
	return domain.SubmissionSourceStdin, "stdin", content, nil
}

type resolvedContext struct {
	paths            local.Paths
	config           local.Config
	registry         *projectpkg.Registry
	project          projectpkg.Project
	resolution       string
	daemonURL        string
	client           *Client
	projectCount     int
	currentProjectID string
}

func (s *Service) resolve(ctx context.Context, opts BaseOptions) (*resolvedContext, error) {
	repoRoot := strings.TrimSpace(opts.RepoRoot)
	if repoRoot == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, reportError(ExitInternal, BlockerBridgeError, err.Error())
		}
		repoRoot = wd
	}
	paths, err := local.ResolvePathsForHome(repoRoot, opts.ConfigPath, opts.CodencerHome)
	if err != nil {
		return nil, reportError(ExitInvalidInput, BlockerInvalidInput, err.Error())
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return nil, reportError(ExitInvalidInput, BlockerInvalidInput, err.Error())
	}
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
	if err != nil {
		return nil, reportError(ExitInvalidInput, BlockerInvalidInput, err.Error())
	}
	projectResult, err := projectpkg.ResolveProject(registry, projectpkg.ResolveOptions{
		ExplicitID: opts.ProjectID,
		CWD:        repoRoot,
	})
	if err != nil {
		return nil, reportError(ExitInvalidInput, BlockerConfigurationRequired, err.Error())
	}
	daemonURL := strings.TrimSpace(projectResult.Project.DaemonURL)
	if daemonURL == "" {
		daemonURL = cfg.DefaultDaemonURL
	}
	return &resolvedContext{
		paths:            paths,
		config:           cfg,
		registry:         registry,
		project:          projectResult.Project,
		resolution:       projectResult.Source,
		daemonURL:        daemonURL,
		client:           NewClient(daemonURL, s.HTTPClient),
		projectCount:     len(registry.Projects),
		currentProjectID: registry.CurrentProjectID,
	}, nil
}

func (r *resolvedContext) report(report ExecutionReport) ExecutionReport {
	report.Project = r.projectSummary("")
	report.DaemonURL = r.daemonURL
	report.ProjectCount = r.projectCount
	report.CurrentProjectID = r.currentProjectID
	return report
}

func (r *resolvedContext) projectSummary(plannerProfile string) *ProjectSummary {
	if plannerProfile == "" {
		plannerProfile = r.project.AdapterProfile
	}
	summary := summarizeProject(r.project, r.resolution, r.daemonURL, plannerProfile)
	return &summary
}

func (r *resolvedContext) daemonReport(err error) ExecutionReport {
	blocker := daemonBlocker(err)
	return r.report(ExecutionReport{
		OK:       false,
		Status:   "blocked",
		Blocker:  blocker,
		ExitCode: daemonExitCode(err),
	})
}

func (r *ExecutionReport) EvidenceWarning(message string) {
	if r.Task == nil {
		r.Task = &TaskReport{}
	}
	r.Task.Evidence.Warnings = append(r.Task.Evidence.Warnings, message)
}

func resolveProfile(p projectpkg.Project, explicitProfile, adapterProfile, adapterOverride, taskAdapterProfile string) (profilepkg.Resolution, error) {
	profileID := strings.TrimSpace(explicitProfile)
	if profileID == "" {
		profileID = strings.TrimSpace(adapterProfile)
	}
	if profileID == "" {
		profileID = strings.TrimSpace(taskAdapterProfile)
	}
	adapter := strings.TrimSpace(adapterOverride)
	if adapter == "" && profileID == "" {
		adapter = p.DefaultAdapter
	}
	return profilepkg.Resolve(profilepkg.ResolveOptions{
		ProfileID:             profileID,
		Adapter:               adapter,
		ProjectDefaultAdapter: p.DefaultAdapter,
		ProjectProfile:        p.AdapterProfile,
		AllowDangerousBypass:  os.Getenv(profilepkg.DangerousBypassEnv) == "1",
	})
}

func profileReportError(err error) error {
	blocker := profileBlocker(err)
	return &ReportError{Code: ExitInvalidInput, Message: err.Error(), Blocker: blocker}
}

func profileBlocker(err error) *Blocker {
	blockerType := BlockerInvalidInput
	if strings.Contains(err.Error(), profilepkg.DangerousBypassEnv) {
		blockerType = BlockerUnsafeAction
	}
	return &Blocker{Type: blockerType, Message: err.Error(), NeedsPlannerDecision: true}
}

func enrichTask(task *domain.TaskSpec, p projectpkg.Project, runID, taskID string, resolution profilepkg.Resolution, timeoutSeconds int) {
	if task.Version == "" {
		task.Version = "v1"
	}
	task.ProjectID = p.ID
	task.RunID = runID
	if task.StepID == "" || task.StepID == "task" {
		task.StepID = uniqueStepID(taskID)
	}
	if task.PhaseID == "" {
		task.PhaseID = "phase-execution-" + runID
	}
	if task.Title == "" {
		task.Title = taskID
	}
	task.AdapterProfile = resolution.DaemonAdapter
	if timeoutSeconds > 0 {
		task.TimeoutSeconds = timeoutSeconds
	}
	if len(task.AllowedPaths) == 0 {
		task.AllowedPaths = append([]string(nil), p.AllowedPaths...)
	}
	if len(task.ForbiddenPaths) == 0 {
		task.ForbiddenPaths = append([]string(nil), p.ForbiddenPaths...)
	}
	if len(task.Validations) == 0 && len(p.DefaultValidations) > 0 {
		task.Validations = make([]domain.ValidationCommand, 0, len(p.DefaultValidations))
		for _, validationCommand := range p.DefaultValidations {
			task.Validations = append(task.Validations, domain.ValidationCommand{
				Name:    validationCommand.Name,
				Command: validationCommand.Command,
			})
		}
	}
	task.IsSimulation = strings.HasPrefix(resolution.DaemonAdapter, "fake-")
}

func uniqueStepID(prefix string) string {
	prefix = sanitizeID(prefix)
	if prefix == "" {
		prefix = "step"
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}

func manifestStepID(taskID string, attempt int) string {
	taskID = sanitizeID(taskID)
	if attempt <= 1 {
		return uniqueStepID(taskID)
	}
	return uniqueStepID(fmt.Sprintf("%s-attempt-%d", taskID, attempt))
}

func sanitizeID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), ".-_")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func manifestTaskSpec(task manifestpkg.Task, manifestPath, runID string) (*domain.TaskSpec, error) {
	goal := strings.TrimSpace(task.Goal)
	sourceKind := domain.SubmissionSourceGoal
	sourceName := task.ID
	if goal == "" && strings.TrimSpace(task.Prompt) != "" {
		goal = task.Prompt
	}
	if goal == "" && strings.TrimSpace(task.PromptFile) != "" {
		promptPath := task.PromptFile
		if !filepath.IsAbs(promptPath) {
			promptPath = filepath.Join(filepath.Dir(manifestPath), promptPath)
		}
		content, err := os.ReadFile(promptPath)
		if err != nil {
			return nil, fmt.Errorf("read prompt file for task %q: %w", task.ID, err)
		}
		goal = string(content)
		sourceKind = domain.SubmissionSourcePromptFile
		sourceName = promptPath
	}
	if strings.TrimSpace(goal) == "" {
		return nil, fmt.Errorf("task %q must set goal, prompt, or prompt_file", task.ID)
	}
	normalized, err := validation.NormalizeTaskInput(validation.NormalizeTaskInputRequest{
		RunID:      runID,
		SourceKind: sourceKind,
		SourceName: sourceName,
		Content:    []byte(goal),
		Direct: validation.DirectTaskOptions{
			Title: task.Title,
		},
	})
	if err != nil {
		return nil, err
	}
	spec := normalized.Task
	spec.Validations = append([]domain.ValidationCommand(nil), task.Validations...)
	return spec, nil
}

func fetchEvidence(ctx context.Context, client *Client, stepID string) Evidence {
	evidence := Evidence{}
	result, err := client.GetResult(ctx, stepID)
	if err != nil {
		evidence.Warnings = append(evidence.Warnings, "result unavailable: "+err.Error())
	} else {
		evidence.Result = result
	}
	artifacts, err := client.GetArtifacts(ctx, stepID)
	if err != nil {
		evidence.Warnings = append(evidence.Warnings, "artifacts unavailable: "+err.Error())
	} else {
		evidence.Artifacts = artifacts
		for _, artifact := range artifacts {
			if artifact.Type == domain.ArtifactTypeStdout || artifact.Type == domain.ArtifactTypeStderr {
				evidence.LogsRef = artifact.Path
				break
			}
		}
	}
	validations, err := client.GetValidations(ctx, stepID)
	if err != nil {
		evidence.Warnings = append(evidence.Warnings, "validations unavailable: "+err.Error())
	} else {
		evidence.Validations = validations
	}
	logs, err := client.GetLogs(ctx, stepID)
	if err != nil {
		evidence.Warnings = append(evidence.Warnings, "logs unavailable: "+err.Error())
	} else {
		evidence.Logs = logs
	}
	return evidence
}

func blockerForResult(step *domain.Step, result *domain.ResultSpec, evidence Evidence) (*Blocker, int) {
	state := result.State
	if step != nil && (step.State == domain.StepStateNeedsApproval || step.State == domain.StepStateNeedsManualAttention) {
		state = step.State
	}
	summary := strings.TrimSpace(result.Summary)
	if summary == "" && step != nil {
		summary = step.StatusReason
	}
	if summary == "" {
		summary = string(state)
	}
	switch state {
	case domain.StepStateCompleted, domain.StepStateCompletedWithWarnings:
		return nil, ExitSuccess
	case domain.StepStateFailedValidation:
		return blockerFromEvidence(BlockerValidationFailed, summary, false, result, evidence), ExitValidationFailed
	case domain.StepStateNeedsApproval:
		return blockerFromEvidence(BlockerManualApproval, summary, false, result, evidence), ExitBlocked
	case domain.StepStateNeedsManualAttention:
		blockerType := BlockerUnknown
		if len(result.Questions) > 0 || result.NeedsHumanDecision {
			blockerType = BlockerQuestion
		}
		return blockerFromEvidence(blockerType, summary, false, result, evidence), ExitBlocked
	case domain.StepStateFailedAdapter:
		return blockerFromEvidence(BlockerAdapterError, summary, true, result, evidence), ExitAdapterFailed
	case domain.StepStateFailedBridge:
		return blockerFromEvidence(BlockerBridgeError, summary, true, result, evidence), ExitDaemonFailed
	case domain.StepStateTimeout:
		return blockerFromEvidence(BlockerTimeout, summary, true, result, evidence), ExitTimeout
	case domain.StepStateFailedRetryable:
		return blockerFromEvidence(BlockerFailedTerminal, summary, true, result, evidence), ExitFailedTerminal
	default:
		return blockerFromEvidence(BlockerFailedTerminal, summary, false, result, evidence), ExitFailedTerminal
	}
}

func blockerFromEvidence(blockerType, message string, retryable bool, result *domain.ResultSpec, evidence Evidence) *Blocker {
	blocker := &Blocker{
		Type:                 blockerType,
		Message:              message,
		NeedsPlannerDecision: true,
		Retryable:            retryable || (result != nil && result.Retryable),
	}
	if result != nil {
		blocker.Questions = append(blocker.Questions, result.Questions...)
		if result.RawOutputRef != "" {
			blocker.EvidenceRefs = append(blocker.EvidenceRefs, result.RawOutputRef)
		}
		for _, ref := range result.Artifacts {
			if strings.TrimSpace(ref) != "" {
				blocker.EvidenceRefs = append(blocker.EvidenceRefs, ref)
			}
		}
	}
	if evidence.LogsRef != "" {
		blocker.EvidenceRefs = append(blocker.EvidenceRefs, evidence.LogsRef)
	}
	for _, warning := range evidence.Warnings {
		blocker.ObservedFacts = append(blocker.ObservedFacts, warning)
	}
	return blocker
}

func taskReportFromStep(taskID, projectID, runID string, step *domain.Step, resolution profilepkg.Resolution, blocker *Blocker, evidence Evidence, exitCode int) TaskReport {
	status := "completed"
	ok := exitCode == ExitSuccess
	if step != nil && step.State != "" {
		status = string(step.State)
	}
	if blocker != nil && !ok {
		status = "blocked"
		if exitCode != ExitBlocked {
			status = blocker.Type
		}
	}
	stepID := ""
	title := ""
	summary := ""
	if step != nil {
		stepID = step.ID
		title = step.Title
		summary = step.StatusReason
	}
	if evidence.Result != nil && evidence.Result.Summary != "" {
		summary = evidence.Result.Summary
	}
	interrupt := humanInterruptFromBlocker(blocker, projectID, runID, stepID, resolution.ProfileID, formatEventTime(stepCreatedAt(step)), formatEventTime(stepUpdatedAt(step)))
	if interrupt != nil {
		blocker.Interrupt = interrupt
	}
	return TaskReport{
		OK:              ok,
		Status:          status,
		TaskID:          taskID,
		ProjectID:       projectID,
		RunID:           runID,
		StepID:          stepID,
		Adapter:         resolution.Adapter,
		Profile:         resolution.ProfileID,
		AdapterProfile:  resolution.DaemonAdapter,
		Title:           title,
		Summary:         summary,
		Step:            step,
		Blocker:         blocker,
		HumanInterrupts: interruptList(interrupt),
		Evidence:        evidence,
		ExitCode:        exitCode,
	}
}

func daemonTaskReport(taskID, projectID, runID string, resolution profilepkg.Resolution, err error) TaskReport {
	blocker := daemonBlocker(err)
	interrupt := humanInterruptFromBlocker(blocker, projectID, runID, "", resolution.ProfileID, "", "")
	if interrupt != nil {
		blocker.Interrupt = interrupt
	}
	return TaskReport{
		OK:              false,
		Status:          "blocked",
		TaskID:          taskID,
		ProjectID:       projectID,
		RunID:           runID,
		Adapter:         resolution.Adapter,
		Profile:         resolution.ProfileID,
		AdapterProfile:  resolution.DaemonAdapter,
		Blocker:         blocker,
		HumanInterrupts: interruptList(interrupt),
		ExitCode:        daemonExitCode(err),
	}
}

func humanInterruptFromBlocker(blocker *Blocker, projectID, runID, stepID, executorProfile, createdAt, updatedAt string) *HumanInterrupt {
	if blocker == nil {
		return nil
	}
	interruptType, requestedAction, allowed := humanInterruptContract(blocker.Type)
	if interruptType == "" {
		return nil
	}
	prompt := strings.TrimSpace(blocker.Message)
	if prompt == "" && len(blocker.Questions) > 0 {
		prompt = blocker.Questions[0]
	}
	interrupt := &HumanInterrupt{
		RunID:            strings.TrimSpace(runID),
		StepID:           strings.TrimSpace(stepID),
		ProjectID:        strings.TrimSpace(projectID),
		ExecutorProfile:  strings.TrimSpace(executorProfile),
		Type:             interruptType,
		Status:           "waiting_for_human",
		Prompt:           safeHumanInterruptText(prompt),
		RequestedAction:  requestedAction,
		AllowedResponses: append([]string(nil), allowed...),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}
	if interrupt.UpdatedAt == "" {
		interrupt.UpdatedAt = interrupt.CreatedAt
	}
	return interrupt
}

func humanInterruptContract(blockerType string) (string, string, []string) {
	switch blockerType {
	case BlockerManualApproval:
		return "planning_approval_required", "approve_or_reject", []string{"approve", "reject", "cancel"}
	case BlockerQuestion:
		return "clarifying_question_required", "answer_question", []string{"answer", "cancel"}
	case BlockerUnsafeAction:
		return "permission_request_required", "confirm_or_deny", []string{"confirm", "deny", "cancel"}
	case BlockerDaemonNotRunning:
		return "os_system_human_action_required", "start_or_configure_daemon", []string{"retry", "cancel"}
	case BlockerUnsupportedOperation:
		return "executor_specific_human_decision_required", "choose_alternate_action", []string{"cancel", "start_new_task"}
	default:
		return "", "", nil
	}
}

func interruptList(interrupt *HumanInterrupt) []HumanInterrupt {
	if interrupt == nil {
		return nil
	}
	return []HumanInterrupt{*interrupt}
}

func stepCreatedAt(step *domain.Step) time.Time {
	if step == nil {
		return time.Time{}
	}
	return step.CreatedAt
}

func stepUpdatedAt(step *domain.Step) time.Time {
	if step == nil {
		return time.Time{}
	}
	return step.UpdatedAt
}

func safeHumanInterruptText(value string) string {
	value = strings.TrimSpace(security.Redact(value))
	if value == "" {
		return ""
	}
	data, _ := json.Marshal(map[string]any{"value": value})
	var decoded map[string]string
	if json.Unmarshal(security.SanitizeRemoteJSON(data), &decoded) == nil {
		return strings.TrimSpace(decoded["value"])
	}
	return value
}

func daemonBlocker(err error) *Blocker {
	var daemonErr *DaemonError
	if errors.As(err, &daemonErr) {
		blockerType := daemonErr.Kind
		if blockerType == "" {
			blockerType = BlockerBridgeError
		}
		message := daemonErr.Message
		if message == "" {
			message = err.Error()
		}
		return &Blocker{
			Type:                 blockerType,
			Message:              message,
			NeedsPlannerDecision: true,
			Retryable:            blockerType == BlockerDaemonNotRunning || blockerType == BlockerBridgeError,
		}
	}
	return &Blocker{
		Type:                 BlockerBridgeError,
		Message:              err.Error(),
		NeedsPlannerDecision: true,
		Retryable:            true,
	}
}

func daemonExitCode(err error) int {
	var daemonErr *DaemonError
	if errors.As(err, &daemonErr) {
		if daemonErr.Kind == BlockerInvalidInput {
			return ExitInvalidInput
		}
		if daemonErr.Kind == BlockerDaemonNotRunning || daemonErr.Kind == BlockerBridgeError {
			return ExitDaemonFailed
		}
	}
	return ExitDaemonFailed
}

func retryableForManifest(blocker *Blocker) bool {
	if blocker == nil {
		return false
	}
	switch blocker.Type {
	case BlockerTimeout, BlockerAdapterError, BlockerBridgeError, BlockerDaemonNotRunning:
		return true
	default:
		return blocker.Retryable
	}
}

func writeRunPlanReport(artifactsDir, runID string, report RunPlanReport) (string, error) {
	dir := filepath.Join(artifactsDir, "run-plans")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create run-plan artifact dir: %w", err)
	}
	path := filepath.Join(dir, runID+".json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode run-plan report: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return "", fmt.Errorf("create run-plan temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write run-plan temp file: %w", err)
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("chmod run-plan temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("close run-plan temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("replace run-plan report: %w", err)
	}
	return path, nil
}

func reportError(code int, blockerType, message string) error {
	return &ReportError{
		Code:    code,
		Message: message,
		Blocker: &Blocker{
			Type:                 blockerType,
			Message:              message,
			NeedsPlannerDecision: true,
		},
	}
}

func ErrorReportFor(err error) ErrorReport {
	var reportErr *ReportError
	if errors.As(err, &reportErr) {
		return ErrorReport{
			OK:       false,
			Status:   "error",
			Error:    reportErr.Message,
			Blocker:  reportErr.Blocker,
			ExitCode: reportErr.Code,
		}
	}
	return ErrorReport{
		OK:       false,
		Status:   "error",
		Error:    err.Error(),
		Blocker:  &Blocker{Type: BlockerBridgeError, Message: err.Error(), NeedsPlannerDecision: true},
		ExitCode: ExitInternal,
	}
}

func ExitCodeForError(err error) int {
	var reportErr *ReportError
	if errors.As(err, &reportErr) {
		return reportErr.Code
	}
	return ExitInternal
}
