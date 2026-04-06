package antigravity

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/domain"
)

const (
	daemonDirRel = ".gemini/antigravity/daemon"
)

// Discovery handles finding active Antigravity instances.
type Discovery struct {
	client *http.Client
}

func NewDiscovery() *Discovery {
	return &Discovery{
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// Discover scans the daemon directory for active instance files.
func (d *Discovery) Discover(ctx context.Context) ([]domain.AGInstance, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("Antigravity: failed to get home dir: %w", err)
	}

	daemonDir := filepath.Join(home, daemonDirRel)
	files, err := os.ReadDir(daemonDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.AGInstance{}, nil
		}
		return nil, fmt.Errorf("Antigravity: failed to read daemon dir %q: %w", daemonDir, err)
	}

	var instances []domain.AGInstance
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "ls_") || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		path := filepath.Join(daemonDir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var inst domain.AGInstance
		if err := json.Unmarshal(data, &inst); err != nil {
			continue
		}

		// Simple health probe + workspace enrichment
		workspace, reachable := d.probeInstance(ctx, inst)
		inst.IsReachable = reachable
		inst.WorkspaceRoot = workspace

		instances = append(instances, inst)
	}

	return instances, nil
}

func (d *Discovery) probeInstance(ctx context.Context, inst domain.AGInstance) (string, bool) {
	url := fmt.Sprintf("https://127.0.0.1:%d/%s/GetWorkspaceInfos", inst.HTTPSPort, servicePrefix)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader("{}"))
	if err != nil {
		return "", false
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-codeium-csrf-token", inst.CSRFToken)

	resp, err := d.client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", true // Reachable but couldn't read body
	}

	var info struct {
		WorkspaceInfos []struct {
			WorkspaceUri string `json:"workspaceUri"`
		} `json:"workspaceInfos"`
	}

	if err := json.Unmarshal(body, &info); err != nil {
		return "", true
	}

	if len(info.WorkspaceInfos) > 0 {
		return info.WorkspaceInfos[0].WorkspaceUri, true
	}

	return "", true
}
