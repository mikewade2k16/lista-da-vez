package modules

import (
	"context"
	"strings"
)

type RelationRef struct {
	ModuleID     string
	ResourceType string
	ResourceID   string
	Metadata     map[string]any
}

type RelationResult struct {
	ModuleID     string
	ResourceType string
	ResourceID   string
	Label        string
	URL          string
	Status       string
	Metadata     map[string]any
}

type RelationResolver interface {
	ModuleID() string
	ResolveMany(ctx context.Context, accountID string, refs []RelationRef) ([]RelationResult, error)
}

type RelationRegistry struct {
	byModule map[string]RelationResolver
}

func NewRelationRegistry(resolvers ...RelationResolver) *RelationRegistry {
	registry := &RelationRegistry{byModule: make(map[string]RelationResolver)}
	for _, resolver := range resolvers {
		registry.Register(resolver)
	}
	return registry
}

func (registry *RelationRegistry) Register(resolver RelationResolver) {
	if registry == nil || resolver == nil {
		return
	}
	if registry.byModule == nil {
		registry.byModule = make(map[string]RelationResolver)
	}
	moduleID := strings.TrimSpace(resolver.ModuleID())
	if moduleID == "" {
		return
	}
	registry.byModule[moduleID] = resolver
}

func (registry *RelationRegistry) Resolver(moduleID string) RelationResolver {
	if registry == nil {
		return nil
	}
	return registry.byModule[strings.TrimSpace(moduleID)]
}
