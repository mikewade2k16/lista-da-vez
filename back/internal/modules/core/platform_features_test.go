package core

import (
	"context"
	"encoding/json"
	"testing"
)

type fakePlatformSettingsRepository struct {
	raw       []byte
	updatedAt *string
	updatedBy *string
	savedKey  string
	savedRaw  []byte
	savedBy   string
}

func (f *fakePlatformSettingsRepository) GetByKey(
	_ context.Context,
	_ string,
) ([]byte, *string, *string, error) {
	return f.raw, f.updatedAt, f.updatedBy, nil
}

func (f *fakePlatformSettingsRepository) Upsert(
	_ context.Context,
	key string,
	config []byte,
	updatedBy string,
) (string, error) {
	f.savedKey = key
	f.savedRaw = config
	f.savedBy = updatedBy
	return "2026-07-24T12:00:00Z", nil
}

func TestExperimentalFeaturesDefaultToDisabled(t *testing.T) {
	service := NewExperimentalFeaturesService(&fakePlatformSettingsRepository{})

	response, err := service.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if response.Features.Version != 1 {
		t.Fatalf("Version = %d, want 1", response.Features.Version)
	}
	if response.Features.AttendanceAudioRecording {
		t.Fatal("AttendanceAudioRecording must default to false")
	}
}

func TestExperimentalFeaturesReadPersistedValue(t *testing.T) {
	updatedAt := "2026-07-24T11:00:00Z"
	updatedBy := "user-1"
	service := NewExperimentalFeaturesService(&fakePlatformSettingsRepository{
		raw:       []byte(`{"version":1,"attendanceAudioRecording":true}`),
		updatedAt: &updatedAt,
		updatedBy: &updatedBy,
	})

	response, err := service.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !response.Features.AttendanceAudioRecording {
		t.Fatal("AttendanceAudioRecording = false, want true")
	}
	if response.UpdatedAt == nil || *response.UpdatedAt != updatedAt {
		t.Fatalf("UpdatedAt = %v, want %q", response.UpdatedAt, updatedAt)
	}
}

func TestExperimentalFeaturesSaveUsesDedicatedKeyAndRehydratesResponse(t *testing.T) {
	repository := &fakePlatformSettingsRepository{}
	service := NewExperimentalFeaturesService(repository)

	response, err := service.Save(context.Background(), ExperimentalFeatures{
		AttendanceAudioRecording: true,
	}, "user-1")
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if repository.savedKey != experimentalFeaturesKey {
		t.Fatalf("saved key = %q, want %q", repository.savedKey, experimentalFeaturesKey)
	}
	if repository.savedBy != "user-1" {
		t.Fatalf("saved by = %q, want user-1", repository.savedBy)
	}

	var saved ExperimentalFeatures
	if err := json.Unmarshal(repository.savedRaw, &saved); err != nil {
		t.Fatalf("saved config is invalid JSON: %v", err)
	}
	if saved.Version != 1 || !saved.AttendanceAudioRecording {
		t.Fatalf("saved features = %+v", saved)
	}
	if response.UpdatedAt == nil || response.UpdatedBy == nil {
		t.Fatal("Save() response must include authoritative update metadata")
	}
	if response.Features != saved {
		t.Fatalf("response features = %+v, saved = %+v", response.Features, saved)
	}
}
