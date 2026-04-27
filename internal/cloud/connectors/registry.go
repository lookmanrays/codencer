package connectors

import (
	"fmt"
	"sort"
	"sync"
)

type Registry struct {
	mu         sync.RWMutex
	connectors map[Provider]Connector
}

func NewRegistry() *Registry {
	r := &Registry{
		connectors: make(map[Provider]Connector),
	}
	r.MustRegister(NewGitHubConnector(nil))
	r.MustRegister(NewGitLabConnector(nil))
	r.MustRegister(NewJiraConnector(nil))
	r.MustRegister(NewLinearConnector(nil))
	r.MustRegister(NewSlackConnector(nil))
	return r
}

func (r *Registry) Register(connector Connector) error {
	if connector == nil {
		return fmt.Errorf("connector is nil")
	}
	provider := connector.Provider()
	if provider == "" {
		return fmt.Errorf("connector provider is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.connectors[provider]; exists {
		return fmt.Errorf("connector %q already registered", provider)
	}
	r.connectors[provider] = connector
	return nil
}

func (r *Registry) MustRegister(connector Connector) {
	if err := r.Register(connector); err != nil {
		panic(err)
	}
}

func (r *Registry) Get(provider Provider) (Connector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	connector, ok := r.connectors[provider]
	return connector, ok
}

func (r *Registry) List() []Connector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := make([]string, 0, len(r.connectors))
	for provider := range r.connectors {
		providers = append(providers, string(provider))
	}
	sort.Strings(providers)
	connectors := make([]Connector, 0, len(providers))
	for _, name := range providers {
		connectors = append(connectors, r.connectors[Provider(name)])
	}
	return connectors
}

func (r *Registry) Names() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.connectors))
	for provider := range r.connectors {
		names = append(names, string(provider))
	}
	sort.Strings(names)
	result := make([]Provider, 0, len(names))
	for _, name := range names {
		result = append(result, Provider(name))
	}
	return result
}
