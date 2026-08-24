package tasks

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateAssistantTaskSnapshotAcceptsInitialVersionZero(t *testing.T) {
	t.Parallel()
	task := Task{ID: "task-1", AccountID: "account-1", Version: 0, Title: "Primeira task"}
	raw, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	version := 0
	if err := validateAssistantTaskSnapshot(task, AssistantTaskMutationInput{
		ExpectedVersion: &version,
		BeforeHash:      hash[:],
	}); err != nil {
		t.Fatalf("version zero is a valid initial task snapshot: %v", err)
	}
}

func TestValidateAssistantTaskSnapshotRejectsMissingOrStaleSnapshot(t *testing.T) {
	t.Parallel()
	task := Task{ID: "task-1", AccountID: "account-1", Version: 0, Title: "Primeira task"}
	if err := validateAssistantTaskSnapshot(task, AssistantTaskMutationInput{}); !errors.Is(err, ErrAssistantSnapshotMissing) {
		t.Fatalf("missing snapshot error = %v", err)
	}
	version := 0
	wrongHash := sha256.Sum256([]byte("outro estado"))
	if err := validateAssistantTaskSnapshot(task, AssistantTaskMutationInput{
		ExpectedVersion: &version,
		BeforeHash:      wrongHash[:],
	}); !errors.Is(err, ErrAssistantSnapshotStale) {
		t.Fatalf("stale snapshot error = %v", err)
	}
}
