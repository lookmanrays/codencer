package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"agent-bridge/internal/release"
)

func main() {
	version := flag.String("version", "", "Release version")
	dist := flag.String("dist", "dist", "Distribution directory")
	dryRun := flag.Bool("dry-run", false, "Write manifest/checksum skeleton without building archives")
	targets := flag.String("targets", release.DefaultTargetSpec, "Comma-separated release targets or host")
	requireTargets := flag.String("require-targets", "", "Comma-separated required release targets; defaults to selected targets")
	allowPartial := flag.Bool("allow-partial", false, "Allow a partial release when required targets are unavailable")
	dockerImage := flag.String("docker-image", release.DefaultDockerImage, "Docker image for Linux release builds from non-Linux hosts")
	jsonOut := flag.Bool("json", false, "Print JSON report")
	flag.Parse()
	report, err := release.Snapshot(release.Options{
		Version:        *version,
		DistDir:        *dist,
		DryRun:         *dryRun,
		Targets:        *targets,
		RequireTargets: *requireTargets,
		AllowPartial:   *allowPartial,
		DockerImage:    *dockerImage,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(report)
		if !report.OK {
			os.Exit(1)
		}
		return
	}
	fmt.Printf("dist: %s\nmanifest: %s\nchecksums: %s\npartial: %t\n", report.DistDir, report.ManifestPath, report.ChecksumsPath, report.Partial)
	for _, artifact := range report.Artifacts {
		fmt.Printf("%s %s %s/%s %s\n", artifact.Status, artifact.Name, artifact.OS, artifact.Arch, artifact.Message)
	}
	for _, err := range report.Errors {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}
	if !report.OK {
		os.Exit(1)
	}
}
