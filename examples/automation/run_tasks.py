#!/usr/bin/env python3
import argparse
import json
import os
import subprocess
import sys
from pathlib import Path


def log(message: str) -> None:
    print(message, file=sys.stderr)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Sequential Codencer wrapper for ordered task lists."
    )
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--project")
    parser.add_argument(
        "--input-mode",
        required=True,
        choices=["task-file", "task-json", "prompt-file", "goal"],
    )
    parser.add_argument("--tasks-file", required=True)
    parser.add_argument("--continue-on-failure", action="store_true")
    parser.add_argument("--title-prefix", default="Task")
    parser.add_argument("--adapter")
    parser.add_argument("--timeout", type=int)
    parser.add_argument("--policy")
    parser.add_argument("--validation", action="append", default=[])
    parser.add_argument("--acceptance", action="append", default=[])
    parser.add_argument("--orchestratorctl", default=os.environ.get("ORCHESTRATORCTL", "./bin/orchestratorctl"))
    return parser.parse_args()


def run_cli(cli_path: str, args: list[str]) -> tuple[int, str]:
    completed = subprocess.run(
        [cli_path, *args],
        check=False,
        capture_output=True,
        text=True,
    )
    if completed.stderr:
        sys.stderr.write(completed.stderr)
    return completed.returncode, completed.stdout


def parse_json(stdout: str) -> dict | list | None:
    text = stdout.strip()
    if not text:
        return None
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return None


def latest_step_id(cli_path: str, run_id: str) -> str:
    exit_code, stdout = run_cli(cli_path, ["step", "list", run_id, "--json"])
    if exit_code != 0:
        return ""
    payload = parse_json(stdout)
    if isinstance(payload, list) and payload:
        item = payload[-1]
        if isinstance(item, dict):
            return str(item.get("id", ""))
    return ""


def ensure_run(cli_path: str, run_id: str, project: str | None) -> str:
    exit_code, stdout = run_cli(cli_path, ["run", "state", run_id, "--json"])
    payload = parse_json(stdout)

    if exit_code == 0 and isinstance(payload, dict):
        log(f"Reusing run {run_id}")
        return str(project or payload.get("project_id", ""))

    status = None
    if isinstance(payload, dict):
        status = payload.get("status")

    if exit_code == 1 and status == 404:
        if not project:
            raise SystemExit(
                f"Run {run_id} does not exist. Re-run with --project <project> so the wrapper can create it."
            )
        log(f"Creating run {run_id} for project {project}")
        create_exit, create_stdout = run_cli(
            cli_path, ["run", "start", run_id, "--project", project, "--json"]
        )
        if create_exit != 0:
            raise SystemExit(f"Failed to create run {run_id}: {create_stdout.strip()}")
        return project

    raise SystemExit(f"Failed to query run {run_id}: {stdout.strip()}")


def load_tasks(tasks_path: Path) -> list[str]:
    if not tasks_path.is_file():
        raise SystemExit(f"Tasks file not found: {tasks_path}")

    items: list[str] = []
    for raw_line in tasks_path.read_text(encoding="utf-8").splitlines():
        line = raw_line.rstrip("\r")
        if not line.strip():
            continue
        if line.lstrip().startswith("#"):
            continue
        items.append(line)

    if not items:
        raise SystemExit(f"No tasks found in {tasks_path} after filtering blank lines and comments.")
    return items


def validate_mode_args(args: argparse.Namespace) -> None:
    direct_args_used = any(
        [
            args.adapter,
            args.timeout is not None,
            args.policy,
            args.validation,
            args.acceptance,
        ]
    )

    if args.input_mode in {"task-file", "task-json"} and direct_args_used:
        raise SystemExit(
            "Direct-mode flags (--adapter, --timeout, --policy, --validation, --acceptance) are only supported with prompt-file and goal modes."
        )
    if args.input_mode != "goal" and args.title_prefix != "Task":
        raise SystemExit("--title-prefix is only supported with goal mode.")


def build_submit_args(args: argparse.Namespace, source: str, index: int) -> list[str]:
    submit_args = ["submit", args.run_id]
    if args.input_mode == "task-file":
        submit_args.append(source)
    elif args.input_mode == "task-json":
        submit_args.extend(["--task-json", source])
    elif args.input_mode == "prompt-file":
        submit_args.extend(["--prompt-file", source])
    elif args.input_mode == "goal":
        submit_args.extend(["--goal", source, "--title", f"{args.title_prefix} {index:02d}"])
    else:
        raise SystemExit(f"Unsupported input mode: {args.input_mode}")

    if args.input_mode in {"prompt-file", "goal"}:
        if args.adapter:
            submit_args.extend(["--adapter", args.adapter])
        if args.timeout is not None:
            submit_args.extend(["--timeout", str(args.timeout)])
        if args.policy:
            submit_args.extend(["--policy", args.policy])
        for validation in args.validation:
            submit_args.extend(["--validation", validation])
        for acceptance in args.acceptance:
            submit_args.extend(["--acceptance", acceptance])

    submit_args.extend(["--wait", "--json"])
    return submit_args


def main() -> int:
    args = parse_args()
    if not args.continue_on_failure and os.environ.get("CODENCER_CONTINUE_ON_FAILURE") == "1":
        args.continue_on_failure = True

    validate_mode_args(args)
    tasks = load_tasks(Path(args.tasks_file))
    project = ensure_run(args.orchestratorctl, args.run_id, args.project)

    results: list[dict] = []
    tasks_succeeded = 0
    tasks_failed = 0
    first_nonzero_exit = 0

    for index, source in enumerate(tasks, start=1):
        log(f"[{index}/{len(tasks)}] submitting {source}")
        exit_code, stdout = run_cli(args.orchestratorctl, build_submit_args(args, source, index))
        payload = parse_json(stdout)

        step_id = ""
        state = "unknown"
        if isinstance(payload, dict):
            step_id = str(payload.get("id") or payload.get("step_id") or "")
            state = str(payload.get("state") or payload.get("error") or "unknown")
        if not step_id:
            step_id = latest_step_id(args.orchestratorctl, args.run_id)

        result = {
            "index": index,
            "source": source,
            "step_id": step_id,
            "state": state,
            "exit_code": exit_code,
        }
        results.append(result)

        if exit_code == 0:
            tasks_succeeded += 1
        else:
            tasks_failed += 1
            if first_nonzero_exit == 0:
                first_nonzero_exit = exit_code
            log(f"[{index}/{len(tasks)}] task failed with exit code {exit_code} and state {state}")
            if not args.continue_on_failure:
                summary = {
                    "run_id": args.run_id,
                    "project": project,
                    "input_mode": args.input_mode,
                    "continue_on_failure": False,
                    "tasks_total": len(tasks),
                    "tasks_succeeded": tasks_succeeded,
                    "tasks_failed": tasks_failed,
                    "results": results,
                    "final_exit_code": exit_code,
                }
                print(json.dumps(summary, indent=2))
                return exit_code

    summary = {
        "run_id": args.run_id,
        "project": project,
        "input_mode": args.input_mode,
        "continue_on_failure": args.continue_on_failure,
        "tasks_total": len(tasks),
        "tasks_succeeded": tasks_succeeded,
        "tasks_failed": tasks_failed,
        "results": results,
        "final_exit_code": first_nonzero_exit,
    }
    print(json.dumps(summary, indent=2))
    return first_nonzero_exit


if __name__ == "__main__":
    raise SystemExit(main())
