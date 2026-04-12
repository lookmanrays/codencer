package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/relayproto"
)

type SharedInstance struct {
	Info         domain.InstanceInfo
	DaemonURL    string
	ManifestPath string
}

type Registry struct {
	cfg           *Config
	clientFactory func(baseURL string) *CodencerClient
}

func NewRegistry(cfg *Config) *Registry {
	return &Registry{
		cfg: cfg,
		clientFactory: func(baseURL string) *CodencerClient {
			return NewCodencerClient(baseURL)
		},
	}
}

func (r *Registry) SharedInstances(ctx context.Context) ([]SharedInstance, error) {
	discovered, _ := r.discoverByInstanceID()
	if len(r.cfg.Instances) == 0 && r.cfg.DaemonURL != "" {
		r.cfg.UpsertSharedInstance(SharedInstanceConfig{DaemonURL: r.cfg.DaemonURL, Share: true})
	}

	var out []SharedInstance
	seen := map[string]struct{}{}
	for _, candidate := range r.cfg.Instances {
		if !candidate.Share {
			continue
		}
		instance, err := r.resolveInstance(ctx, candidate, discovered)
		if err != nil {
			continue
		}
		if _, ok := seen[instance.Info.ID]; ok {
			continue
		}
		seen[instance.Info.ID] = struct{}{}
		out = append(out, instance)
	}
	return out, nil
}

func (r *Registry) ResolveInstance(ctx context.Context, instanceID string) (*SharedInstance, error) {
	instances, err := r.SharedInstances(ctx)
	if err != nil {
		return nil, err
	}
	for _, instance := range instances {
		if instance.Info.ID == instanceID {
			return &instance, nil
		}
	}
	return nil, fmt.Errorf("instance %s is not shared by this connector", instanceID)
}

func (r *Registry) Advertisements(ctx context.Context) ([]relayproto.InstanceAdvertisement, []string, error) {
	instances, err := r.SharedInstances(ctx)
	if err != nil {
		return nil, nil, err
	}
	ads := make([]relayproto.InstanceAdvertisement, 0, len(instances))
	instanceIDs := make([]string, 0, len(instances))
	for _, instance := range instances {
		payload, err := json.Marshal(instance.Info)
		if err != nil {
			return nil, nil, err
		}
		ads = append(ads, relayproto.InstanceAdvertisement{Instance: payload})
		instanceIDs = append(instanceIDs, instance.Info.ID)
	}
	return ads, instanceIDs, nil
}

func (r *Registry) resolveInstance(ctx context.Context, candidate SharedInstanceConfig, discovered map[string]string) (SharedInstance, error) {
	manifestPath := candidate.ManifestPath
	if manifestPath == "" && candidate.InstanceID != "" {
		manifestPath = discovered[candidate.InstanceID]
	}

	var info domain.InstanceInfo
	if manifestPath != "" {
		loaded, err := loadManifest(manifestPath)
		if err == nil {
			info = *loaded
		}
	}

	daemonURL := candidate.DaemonURL
	if daemonURL == "" && info.BaseURL != "" {
		daemonURL = info.BaseURL
	}
	if daemonURL == "" {
		return SharedInstance{}, fmt.Errorf("no daemon url for shared instance")
	}

	liveInfo, err := r.clientFactory(daemonURL).GetInstance(ctx)
	if err == nil {
		info = *liveInfo
	}
	if info.ID == "" {
		return SharedInstance{}, fmt.Errorf("could not resolve instance identity")
	}
	return SharedInstance{
		Info:         info,
		DaemonURL:    daemonURL,
		ManifestPath: manifestPath,
	}, nil
}

func (r *Registry) discoverByInstanceID() (map[string]string, error) {
	found := map[string]string{}
	for _, root := range r.cfg.DiscoveryRoots {
		if root == "" {
			continue
		}
		err := filepath.WalkDir(root, func(current string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				switch d.Name() {
				case ".git", "node_modules":
					return filepath.SkipDir
				}
				return nil
			}
			if d.Name() != "instance.json" || filepath.Base(filepath.Dir(current)) != ".codencer" {
				return nil
			}
			info, err := loadManifest(current)
			if err != nil || info.ID == "" {
				return nil
			}
			found[info.ID] = current
			return nil
		})
		if err != nil {
			return found, err
		}
	}
	return found, nil
}

func loadManifest(path string) (*domain.InstanceInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info domain.InstanceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
