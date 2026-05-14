package tasks

import (
	"context"
	"strings"
	"time"

	platformmodules "github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

const relationCacheTTL = 60 * time.Second

func (service *Service) ExpandRelations(ctx context.Context, access AccessContext, taskID string) ([]Relation, error) {
	if !access.Has(PermTasksView) {
		return nil, ErrForbidden
	}

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrValidation
	}

	relations, err := service.repository.ListRelations(ctx, access, taskID)
	if err != nil {
		return nil, err
	}
	if len(relations) == 0 || service.relations == nil {
		return relations, nil
	}

	now := time.Now().UTC()
	staleRefsByModule := make(map[string][]platformmodules.RelationRef)
	relationIndexByKey := make(map[string]int, len(relations))
	for index, relation := range relations {
		key := relationKey(relation.Module, relation.ResourceType, relation.ResourceID)
		relationIndexByKey[key] = index
		if !relationNeedsRefresh(relation, now) {
			continue
		}
		if service.relations.Resolver(relation.Module) == nil {
			continue
		}
		staleRefsByModule[relation.Module] = append(staleRefsByModule[relation.Module], platformmodules.RelationRef{
			ModuleID:     relation.Module,
			ResourceType: relation.ResourceType,
			ResourceID:   relation.ResourceID,
			Metadata:     cloneRelationMetadata(relation.MetadataCache),
		})
	}

	for moduleID, refs := range staleRefsByModule {
		resolver := service.relations.Resolver(moduleID)
		if resolver == nil {
			continue
		}

		resolved, err := resolver.ResolveMany(ctx, access.AccountID, refs)
		resolvedByKey := make(map[string]platformmodules.RelationResult, len(resolved))
		if err == nil {
			for _, result := range resolved {
				resolvedByKey[relationKey(result.ModuleID, result.ResourceType, result.ResourceID)] = result
			}
		}

		for _, ref := range refs {
			key := relationKey(ref.ModuleID, ref.ResourceType, ref.ResourceID)
			index, ok := relationIndexByKey[key]
			if !ok {
				continue
			}

			result, ok := resolvedByKey[key]
			if !ok {
				result = platformmodules.RelationResult{
					ModuleID:     ref.ModuleID,
					ResourceType: ref.ResourceType,
					ResourceID:   ref.ResourceID,
					Status:       "unknown",
					Metadata:     map[string]any{"status": "unknown"},
				}
			}

			merged := mergeRelationResolution(relations[index], result, now)
			persisted, err := service.repository.AddRelation(ctx, access.AccountID, AddRelationInput{
				TaskID:        merged.TaskID,
				Module:        merged.Module,
				ResourceType:  merged.ResourceType,
				ResourceID:    merged.ResourceID,
				LabelCache:    merged.LabelCache,
				MetadataCache: merged.MetadataCache,
			})
			if err != nil {
				return nil, err
			}
			relations[index] = persisted
		}
	}

	return relations, nil
}

func relationNeedsRefresh(relation Relation, now time.Time) bool {
	if strings.TrimSpace(relation.LabelCache) == "" || len(relation.MetadataCache) == 0 {
		return true
	}
	if relation.RefreshedAt.IsZero() {
		return true
	}
	if now.Sub(relation.RefreshedAt) >= relationCacheTTL {
		return true
	}
	status, ok := relation.MetadataCache["status"].(string)
	return !ok || strings.TrimSpace(status) == ""
}

func mergeRelationResolution(relation Relation, result platformmodules.RelationResult, resolvedAt time.Time) Relation {
	label := strings.TrimSpace(result.Label)
	if label == "" {
		label = strings.TrimSpace(relation.LabelCache)
	}
	if label == "" {
		label = strings.TrimSpace(relation.ResourceID)
	}

	metadata := cloneRelationMetadata(relation.MetadataCache)
	for key, value := range result.Metadata {
		metadata[key] = value
	}
	if status := strings.TrimSpace(result.Status); status != "" {
		metadata["status"] = status
	}
	if resolvedModule := strings.TrimSpace(result.ModuleID); resolvedModule != "" {
		metadata["resolvedModule"] = resolvedModule
	}
	if resolvedURL := strings.TrimSpace(result.URL); resolvedURL != "" {
		metadata["url"] = resolvedURL
	}
	metadata["resolvedAt"] = resolvedAt

	relation.LabelCache = label
	relation.MetadataCache = metadata
	relation.RefreshedAt = resolvedAt
	return relation
}

func relationKey(moduleID string, resourceType string, resourceID string) string {
	return strings.TrimSpace(moduleID) + "|" + strings.TrimSpace(resourceType) + "|" + strings.TrimSpace(resourceID)
}

func cloneRelationMetadata(source map[string]any) map[string]any {
	if len(source) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
