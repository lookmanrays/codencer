package common

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agent-bridge/internal/domain"
)

// CollectStandardArtifacts reads the artifact directory and populates domain artifacts.
func CollectStandardArtifacts(ctx context.Context, attemptID string, artifactRoot string) ([]*domain.Artifact, error) {
	var artifacts []*domain.Artifact
	
	entries, err := os.ReadDir(artifactRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return artifacts, nil
		}
		return nil, fmt.Errorf("failed to read artifact root %s: %w", artifactRoot, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}

		path := filepath.Join(artifactRoot, entry.Name())
		artType := domain.ArtifactType("file")
		
		switch entry.Name() {
		case "stdout.log":
			artType = domain.ArtifactTypeStdout
		case "result.json":
			artType = domain.ArtifactType("result_json")
		case "stderr.log":
			artType = domain.ArtifactType("stderr")
		default:
			if filepath.Ext(entry.Name()) == ".patch" || filepath.Ext(entry.Name()) == ".diff" {
				artType = domain.ArtifactType("diff")
			}
		}

		artifacts = append(artifacts, &domain.Artifact{
			ID:        fmt.Sprintf("art-%s-%s", entry.Name(), attemptID),
			AttemptID: attemptID,
			Type:      artType,
			Path:      path,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}
	
	return artifacts, nil
}

// NormalizeStandardResult parses a standard result.json into domain.Result.
func NormalizeStandardResult(attemptID string, artifacts []*domain.Artifact) (*domain.Result, error) {
	var resultPath string
	for _, art := range artifacts {
		if art.Type == "result_json" {
			resultPath = art.Path
			break
		}
	}

	if resultPath == "" {
		return nil, fmt.Errorf("no result_json found for attempt %s", attemptID)
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read result JSON at %s: %w", resultPath, err)
	}

	var res domain.Result
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result JSON: %w", err)
	}

	if res.State == "" {
		res.State = domain.StepStateFailedTerminal
		res.Summary = "Invalid result: missing status field"
	}

	return &res, nil
}
