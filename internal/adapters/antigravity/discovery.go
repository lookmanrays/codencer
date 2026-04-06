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
	daemonDirs, err := d.getDaemonDirs()
	if err != nil {
		return nil, err
	}

	return d.scanDirs(ctx, daemonDirs)
}

func (d *Discovery) scanDirs(ctx context.Context, dirs []string) ([]domain.AGInstance, error) {
	instanceMap := make(map[int]domain.AGInstance)
	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			// Log warning but continue with other directories
			continue
		}

		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "ls_") || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			path := filepath.Join(dir, file.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			var inst domain.AGInstance
			if err := json.Unmarshal(data, &inst); err != nil {
				continue
			}

			if inst.PID == 0 || inst.HTTPSPort == 0 {
				continue
			}

			// Deduplicate by PID
			if _, exists := instanceMap[inst.PID]; exists {
				continue
			}

			// Simple health probe + workspace enrichment
			workspace, reachable := d.probeInstance(ctx, inst)
			inst.IsReachable = reachable
			inst.WorkspaceRoot = workspace

			instanceMap[inst.PID] = inst
		}
	}

	var instances []domain.AGInstance
	for _, inst := range instanceMap {
		instances = append(instances, inst)
	}

	return instances, nil
}

func (d *Discovery) getDaemonDirs() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}

	dirs := []string{filepath.Join(home, daemonDirRel)}

	// WSL Detection & Cross-Side Discovery
	if content, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		if strings.Contains(strings.ToLower(string(content)), "microsoft") {
			// In WSL, attempt to reach the Windows host home directory.
			// Default path: /mnt/c/Users/<user>
			user := os.Getenv("USER")
			if user != "" {
				winHome := filepath.Join("/mnt/c/Users", user)
				winDaemonDir := filepath.Join(winHome, daemonDirRel)
				if info, err := os.Stat(winDaemonDir); err == nil && info.IsDir() {
					dirs = append(dirs, winDaemonDir)
				}
			}
		}
	}

	return dirs, nil
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
