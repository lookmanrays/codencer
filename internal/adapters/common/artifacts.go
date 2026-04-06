package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"agent-bridge/internal/domain"
)

// CollectStandardArtifacts reads the artifact directory and populates domain artifacts.
func CollectStandardArtifacts(ctx context.Context, attemptID string, attemptArtifactRoot string) ([]*domain.Artifact, error) {
	var artifacts []*domain.Artifact
	
	entries, err := os.ReadDir(attemptArtifactRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return artifacts, nil
		}
		return nil, fmt.Errorf("failed to read artifact root %s: %w", attemptArtifactRoot, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}

		path := filepath.Join(attemptArtifactRoot, entry.Name())
		
		// 1. Calculate Hash and Detect MIME Type
		hash, mimeType, err := calculateMetadata(path)
		if err != nil {
			// Log and skip if file is unreadable (could be a transient issue or incomplete write)
			continue
		}

		// 2. Classify Type
		artType := domain.ArtifactType("file")
		switch entry.Name() {
		case "stdout.log":
			artType = domain.ArtifactTypeStdout
		case "result.json":
			artType = domain.ArtifactTypeResultJSON
		case "stderr.log":
			artType = domain.ArtifactTypeStderr
		default:
			if filepath.Ext(entry.Name()) == ".patch" || filepath.Ext(entry.Name()) == ".diff" {
				artType = domain.ArtifactTypeDiff
			}
		}

		artifacts = append(artifacts, &domain.Artifact{
			ID:        fmt.Sprintf("art-%s-%s", hash[:12], entry.Name()),
			AttemptID: attemptID,
			Type:      artType,
			Name:      entry.Name(),
			Path:      path,
			Size:      info.Size(),
			Hash:      hash,
			MimeType:  mimeType,
			CreatedAt: info.ModTime(),
			UpdatedAt: time.Now().UTC(),
		})
	}
	
	return artifacts, nil
}

func calculateMetadata(path string) (string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	// 1. Detect MIME Type (sample first 512 bytes)
	sample := make([]byte, 512)
	n, _ := file.Read(sample)
	mimeType := http.DetectContentType(sample[:n])

	// 2. Calculate SHA-256
	if _, err := file.Seek(0, 0); err != nil {
		return "", "", err
	}
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", "", err
	}
	hash := hex.EncodeToString(h.Sum(nil))

	return hash, mimeType, nil
}

// NormalizeStandardResult parses a standard result.json into domain.ResultSpec.
func NormalizeStandardResult(attemptID string, artifacts []*domain.Artifact) (*domain.ResultSpec, error) {
	var resultPath string
	for _, art := range artifacts {
		if art.Type == domain.ArtifactTypeResultJSON {
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

	var res domain.ResultSpec
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result JSON: %w", err)
	}

	if res.State == "" {
		res.State = domain.StepStateFailedTerminal
		res.Summary = "Invalid result: missing status field"
	}

	return &res, nil
}
