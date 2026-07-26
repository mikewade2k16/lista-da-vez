package core

import (
	"context"
	"encoding/json"
	"fmt"
)

// ExperimentalFeaturesService lê e persiste o rollout global de recursos
// experimentais. A autorização de escrita continua na borda HTTP.
type ExperimentalFeaturesService struct {
	repo platformSettingsRepository
}

func NewExperimentalFeaturesService(repo platformSettingsRepository) *ExperimentalFeaturesService {
	return &ExperimentalFeaturesService{repo: repo}
}

func (s *ExperimentalFeaturesService) Get(ctx context.Context) (ExperimentalFeaturesResponse, error) {
	raw, updatedAt, updatedBy, err := s.repo.GetByKey(ctx, experimentalFeaturesKey)
	if err != nil {
		return ExperimentalFeaturesResponse{}, err
	}
	if len(raw) == 0 {
		return ExperimentalFeaturesResponse{
			Features: defaultExperimentalFeatures(),
		}, nil
	}

	var features ExperimentalFeatures
	if err := json.Unmarshal(raw, &features); err != nil {
		return ExperimentalFeaturesResponse{}, fmt.Errorf("unmarshal experimental_features: %w", err)
	}
	normalizeExperimentalFeatures(&features)

	return ExperimentalFeaturesResponse{
		Features:  features,
		UpdatedAt: updatedAt,
		UpdatedBy: updatedBy,
	}, nil
}

func (s *ExperimentalFeaturesService) Save(
	ctx context.Context,
	features ExperimentalFeatures,
	userID string,
) (ExperimentalFeaturesResponse, error) {
	normalizeExperimentalFeatures(&features)

	raw, err := json.Marshal(features)
	if err != nil {
		return ExperimentalFeaturesResponse{}, fmt.Errorf("marshal experimental_features: %w", err)
	}

	updatedAt, err := s.repo.Upsert(ctx, experimentalFeaturesKey, raw, userID)
	if err != nil {
		return ExperimentalFeaturesResponse{}, err
	}

	updatedBy := userID
	return ExperimentalFeaturesResponse{
		Features:  features,
		UpdatedAt: &updatedAt,
		UpdatedBy: &updatedBy,
	}, nil
}
