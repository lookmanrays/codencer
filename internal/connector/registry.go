package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/local"
	projectpkg "agent-bridge/internal/project"
	"agent-bridge/internal/relayproto"
)

type SharedInstance struct {
	Info         domain.InstanceInfo
	DaemonURL    string
	ManifestPath string
}

type SharedProject struct {
	Project   projectpkg.Project
	Info      domain.InstanceInfo
	DaemonURL string
}

type AdvertisementSet struct {
	Instances   []relayproto.InstanceAdvertisement
	Projects    []relayproto.ProjectAdvertisement
	InstanceIDs []string
	ProjectIDs  []string
	Warnings    []string
}

type DiscoveredManifest struct {
	Path string
	Info domain.InstanceInfo
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

func (r *Registry) SharedProjects(ctx context.Context) ([]SharedProject, []string, error) {
	paths, err := r.localPaths()
	if err != nil {
		return nil, []string{err.Error()}, nil
	}
	cfg, err := local.LoadConfig(paths.ConfigFile)
	if err != nil {
		return nil, []string{err.Error()}, nil
	}
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
	if err != nil {
		return nil, []string{err.Error()}, nil
	}
	machine, _, machineErr := local.EnsureMachine(paths.MachineFile, time.Now().UTC())
	if machineErr == nil {
		if projectpkg.BackfillMachineMetadata(registry, machine.MachineID, machine.HostLabel, machine.Hostname, time.Now().UTC()) {
			if err := projectpkg.SaveRegistry(paths.ProjectsFile, registry); err != nil {
				return nil, []string{err.Error()}, nil
			}
		}
	} else {
		return nil, []string{machineErr.Error()}, nil
	}

	out := make([]SharedProject, 0, len(registry.Projects))
	warnings := []string{}
	seen := map[string]struct{}{}
	for _, candidate := range projectpkg.ListProjects(registry) {
		if !candidate.SharedToRelay {
			continue
		}
		daemonURL := strings.TrimRight(strings.TrimSpace(candidate.DaemonURL), "/")
		if daemonURL == "" {
			daemonURL = strings.TrimRight(strings.TrimSpace(cfg.DefaultDaemonURL), "/")
		}
		if daemonURL == "" {
			warnings = append(warnings, fmt.Sprintf("project %s is shared but has no daemon url", candidate.ID))
			continue
		}
		info, err := r.clientFactory(daemonURL).GetInstance(ctx)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("project %s skipped: daemon %s unavailable: %v", candidate.ID, daemonURL, err))
			continue
		}
		if info.ID == "" {
			warnings = append(warnings, fmt.Sprintf("project %s skipped: daemon %s did not report an instance id", candidate.ID, daemonURL))
			continue
		}
		if candidate.RelayInstanceID != "" && candidate.RelayInstanceID != info.ID {
			warnings = append(warnings, fmt.Sprintf("project %s skipped: relay_instance_id %s does not match live instance %s", candidate.ID, candidate.RelayInstanceID, info.ID))
			continue
		}
		key := candidate.ID + "|" + info.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, SharedProject{Project: candidate, Info: *info, DaemonURL: daemonURL})
	}
	return out, warnings, nil
}

func (r *Registry) ResolveProject(ctx context.Context, projectID string) (*SharedProject, error) {
	projects, warnings, err := r.SharedProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, shared := range projects {
		if shared.Project.ID == projectID {
			return &shared, nil
		}
	}
	if len(warnings) > 0 {
		return nil, fmt.Errorf("project %s is not shared by this connector; warnings: %s", projectID, strings.Join(warnings, "; "))
	}
	return nil, fmt.Errorf("project %s is not shared by this connector", projectID)
}

func (r *Registry) ConfiguredSharedProject(projectID string) (projectpkg.Project, error) {
	paths, err := r.localPaths()
	if err != nil {
		return projectpkg.Project{}, err
	}
	registry, err := projectpkg.LoadRegistry(paths.ProjectsFile)
	if err != nil {
		return projectpkg.Project{}, err
	}
	project, err := projectpkg.GetProject(registry, projectID)
	if err != nil {
		return projectpkg.Project{}, err
	}
	if !project.SharedToRelay {
		return projectpkg.Project{}, fmt.Errorf("project %s is not shared by this connector", projectID)
	}
	return project, nil
}

func (r *Registry) Advertisements(ctx context.Context) (AdvertisementSet, error) {
	instances, err := r.SharedInstances(ctx)
	if err != nil {
		return AdvertisementSet{}, err
	}
	projects, warnings, err := r.SharedProjects(ctx)
	if err != nil {
		return AdvertisementSet{}, err
	}
	set := AdvertisementSet{Warnings: warnings}
	ads := make([]relayproto.InstanceAdvertisement, 0, len(instances))
	instanceIDs := make([]string, 0, len(instances))
	for _, instance := range instances {
		payload, err := json.Marshal(instance.Info)
		if err != nil {
			return AdvertisementSet{}, err
		}
		ads = append(ads, relayproto.InstanceAdvertisement{Instance: payload})
		instanceIDs = append(instanceIDs, instance.Info.ID)
	}
	projectAds := make([]relayproto.ProjectAdvertisement, 0, len(projects))
	projectIDs := make([]string, 0, len(projects))
	for _, shared := range projects {
		payload, err := json.Marshal(shared.Project)
		if err != nil {
			return AdvertisementSet{}, err
		}
		projectAds = append(projectAds, relayproto.ProjectAdvertisement{
			ProjectID:   shared.Project.ID,
			ProjectName: shared.Project.Name,
			InstanceID:  shared.Info.ID,
			MachineID:   shared.Project.MachineID,
			HostLabel:   shared.Project.HostLabel,
			Hostname:    shared.Project.Hostname,
			Status:      "available",
			Project:     payload,
		})
		projectIDs = append(projectIDs, shared.Project.ID)
	}
	set.Instances = ads
	set.Projects = projectAds
	set.InstanceIDs = instanceIDs
	set.ProjectIDs = projectIDs
	return set, nil
}

func (r *Registry) localPaths() (local.Paths, error) {
	home := ""
	if r != nil && r.cfg != nil {
		home = r.cfg.CodencerHome
	}
	return local.ResolvePathsForHome("", "", home)
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
	manifests, err := r.DiscoveredManifests()
	if err != nil {
		return nil, err
	}
	found := map[string]string{}
	for _, manifest := range manifests {
		if manifest.Info.ID == "" {
			continue
		}
		found[manifest.Info.ID] = manifest.Path
	}
	return found, nil
}

func (r *Registry) DiscoveredManifests() ([]DiscoveredManifest, error) {
	if r == nil || r.cfg == nil {
		return nil, nil
	}
	roots := normalizeDiscoveryRoots(r.cfg.DiscoveryRoots, nil)
	out := make([]DiscoveredManifest, 0)
	for _, root := range roots {
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
			out = append(out, DiscoveredManifest{Path: current, Info: *info})
			return nil
		})
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func matchManifest(candidate SharedInstanceConfig, manifests []DiscoveredManifest) *DiscoveredManifest {
	for i := range manifests {
		manifest := &manifests[i]
		if candidate.InstanceID != "" && manifest.Info.ID == candidate.InstanceID {
			return manifest
		}
		if candidate.ManifestPath != "" && manifest.Path == candidate.ManifestPath {
			return manifest
		}
		if candidate.ManifestPath != "" && manifest.Info.ManifestPath != "" && manifest.Info.ManifestPath == candidate.ManifestPath {
			return manifest
		}
		if candidate.DaemonURL != "" && manifest.Info.BaseURL != "" && candidate.DaemonURL == manifest.Info.BaseURL {
			return manifest
		}
	}
	return nil
}

func resolveDiscoveryLiveInfo(ctx context.Context, candidate SharedInstanceConfig, clientFactory func(string) *CodencerClient) *domain.InstanceInfo {
	if strings.TrimSpace(candidate.DaemonURL) == "" {
		return nil
	}
	if clientFactory == nil {
		clientFactory = func(baseURL string) *CodencerClient { return NewCodencerClient(baseURL) }
	}
	info, err := clientFactory(candidate.DaemonURL).GetInstance(ctx)
	if err != nil {
		return nil
	}
	return info
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
