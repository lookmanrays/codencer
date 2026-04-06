param(
    [Parameter(Mandatory = $true)]
    [string]$RunId,

    [string]$Project,

    [Parameter(Mandatory = $true)]
    [ValidateSet("task-file", "task-json", "prompt-file", "goal")]
    [string]$InputMode,

    [Parameter(Mandatory = $true)]
    [string]$TasksFile,

    [switch]$ContinueOnFailure,

    [string]$TitlePrefix = "Task",

    [string]$Adapter,

    [int]$Timeout,

    [string]$Policy,

    [string[]]$Validation = @(),

    [string[]]$Acceptance = @(),

    [string]$OrchestratorCtl = $(if ($env:ORCHESTRATORCTL) { $env:ORCHESTRATORCTL } else { "./bin/orchestratorctl" })
)

$ErrorActionPreference = "Stop"

function Write-Stderr {
    param([string]$Message)
    [Console]::Error.WriteLine($Message)
}

function Fail {
    param(
        [string]$Message,
        [int]$Code = 1
    )
    Write-Stderr $Message
    exit $Code
}

function Invoke-OrchestratorJson {
    param([string[]]$Args)

    $output = & $OrchestratorCtl @Args
    $exitCode = $LASTEXITCODE
    $stdoutText = ($output -join "`n")
    $payload = $null
    if (-not [string]::IsNullOrWhiteSpace($stdoutText)) {
        try {
            $payload = $stdoutText | ConvertFrom-Json -Depth 20
        } catch {
            $payload = $null
        }
    }
    return [PSCustomObject]@{
        ExitCode = $exitCode
        Stdout = $stdoutText
        Payload = $payload
    }
}

function Get-LatestStepId {
    $steps = Invoke-OrchestratorJson @("step", "list", $RunId, "--json")
    if ($steps.ExitCode -ne 0) {
        return ""
    }
    if ($steps.Payload -is [System.Array] -and $steps.Payload.Count -gt 0) {
        return [string]$steps.Payload[-1].id
    }
    return ""
}

function Ensure-Run {
    $state = Invoke-OrchestratorJson @("run", "state", $RunId, "--json")
    if ($state.ExitCode -eq 0) {
        if (-not $Project -and $state.Payload) {
            $script:Project = [string]$state.Payload.project_id
        }
        Write-Stderr "Reusing run $RunId"
        return
    }

    if ($state.ExitCode -eq 1 -and $state.Payload -and [string]$state.Payload.status -eq "404") {
        if (-not $Project) {
            Fail "Run $RunId does not exist. Re-run with --Project <project> so the wrapper can create it."
        }
        Write-Stderr "Creating run $RunId for project $Project"
        $create = Invoke-OrchestratorJson @("run", "start", $RunId, "--project", $Project, "--json")
        if ($create.ExitCode -ne 0) {
            Fail "Failed to create run $RunId. $($create.Stdout)" $create.ExitCode
        }
        return
    }

    Fail "Failed to query run $RunId. $($state.Stdout)" $state.ExitCode
}

if (-not (Test-Path -LiteralPath $TasksFile)) {
    Fail "Tasks file not found: $TasksFile"
}

$directArgsUsed = [bool]($Adapter -or $Policy -or $Validation.Count -gt 0 -or $Acceptance.Count -gt 0 -or $PSBoundParameters.ContainsKey("Timeout"))
if (($InputMode -eq "task-file" -or $InputMode -eq "task-json") -and $directArgsUsed) {
    Fail "Direct-mode flags (--Adapter, --Timeout, --Policy, --Validation, --Acceptance) are only supported with prompt-file and goal modes."
}

if ($InputMode -ne "goal" -and $PSBoundParameters.ContainsKey("TitlePrefix")) {
    if ($TitlePrefix -ne "Task") {
        Fail "--TitlePrefix is only supported with goal mode."
    }
}

$continueMode = $ContinueOnFailure.IsPresent -or $env:CODENCER_CONTINUE_ON_FAILURE -eq "1"

$taskItems = @()
Get-Content -LiteralPath $TasksFile -Encoding UTF8 | ForEach-Object {
    $line = $_.TrimEnd("`r")
    if ([string]::IsNullOrWhiteSpace($line)) {
        return
    }
    if ($line.TrimStart().StartsWith("#")) {
        return
    }
    $taskItems += $line
}

if ($taskItems.Count -eq 0) {
    Fail "No tasks found in $TasksFile after filtering blank lines and comments."
}

Ensure-Run

$results = @()
$tasksSucceeded = 0
$tasksFailed = 0
$firstNonZeroExit = 0

for ($i = 0; $i -lt $taskItems.Count; $i++) {
    $index = $i + 1
    $source = $taskItems[$i]
    Write-Stderr "[$index/$($taskItems.Count)] submitting $source"

    $submitArgs = @("submit", $RunId)
    switch ($InputMode) {
        "task-file" { $submitArgs += $source }
        "task-json" { $submitArgs += @("--task-json", $source) }
        "prompt-file" { $submitArgs += @("--prompt-file", $source) }
        "goal" {
            $submitArgs += @("--goal", $source, "--title", ("{0} {1:d2}" -f $TitlePrefix, $index))
        }
    }

    if ($InputMode -eq "prompt-file" -or $InputMode -eq "goal") {
        if ($Adapter) { $submitArgs += @("--adapter", $Adapter) }
        if ($PSBoundParameters.ContainsKey("Timeout")) { $submitArgs += @("--timeout", [string]$Timeout) }
        if ($Policy) { $submitArgs += @("--policy", $Policy) }
        foreach ($item in $Validation) {
            $submitArgs += @("--validation", $item)
        }
        foreach ($item in $Acceptance) {
            $submitArgs += @("--acceptance", $item)
        }
    }

    $submitArgs += @("--wait", "--json")
    $submit = Invoke-OrchestratorJson $submitArgs

    $state = if ($submit.Payload) {
        if ($submit.Payload.state) { [string]$submit.Payload.state } elseif ($submit.Payload.error) { [string]$submit.Payload.error } else { "unknown" }
    } else {
        "unknown"
    }

    $stepId = ""
    if ($submit.Payload -and $submit.Payload.step_id) {
        $stepId = [string]$submit.Payload.step_id
    }
    if (-not $stepId) {
        $stepId = Get-LatestStepId
    }

    $results += [PSCustomObject]@{
        index = $index
        source = $source
        step_id = $stepId
        state = $state
        exit_code = $submit.ExitCode
    }

    if ($submit.ExitCode -eq 0) {
        $tasksSucceeded++
    } else {
        $tasksFailed++
        if ($firstNonZeroExit -eq 0) {
            $firstNonZeroExit = $submit.ExitCode
        }
        Write-Stderr "[$index/$($taskItems.Count)] task failed with exit code $($submit.ExitCode) and state $state"
        if (-not $continueMode) {
            [PSCustomObject]@{
                run_id = $RunId
                project = $Project
                input_mode = $InputMode
                continue_on_failure = $false
                tasks_total = $taskItems.Count
                tasks_succeeded = $tasksSucceeded
                tasks_failed = $tasksFailed
                results = $results
                final_exit_code = $submit.ExitCode
            } | ConvertTo-Json -Depth 6
            exit $submit.ExitCode
        }
    }
}

[PSCustomObject]@{
    run_id = $RunId
    project = $Project
    input_mode = $InputMode
    continue_on_failure = $continueMode
    tasks_total = $taskItems.Count
    tasks_succeeded = $tasksSucceeded
    tasks_failed = $tasksFailed
    results = $results
    final_exit_code = $firstNonZeroExit
} | ConvertTo-Json -Depth 6

exit $firstNonZeroExit
