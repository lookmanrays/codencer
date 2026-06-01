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
	jsonOut := flag.Bool("json", false, "Print JSON report")
	flag.Parse()
	report, err := release.Snapshot(release.Options{Version: *version, DistDir: *dist, DryRun: *dryRun})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(report)
		return
	}
	fmt.Printf("dist: %s\nmanifest: %s\nchecksums: %s\n", report.DistDir, report.ManifestPath, report.ChecksumsPath)
	for _, artifact := range report.Artifacts {
		fmt.Printf("%s %s %s/%s %s\n", artifact.Status, artifact.Name, artifact.OS, artifact.Arch, artifact.Message)
	}
	if !report.OK {
		os.Exit(1)
	}
}
