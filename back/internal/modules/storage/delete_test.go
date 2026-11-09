package storage

import (
	"context"
	"errors"
	"testing"
)

type deletingRepository struct {
	fakeRepository
	marked []string
}

func (repository *deletingRepository) MarkDeleted(_ context.Context, accountID, objectID, sourceModule string) error {
	repository.marked = append(repository.marked, accountID+"/"+objectID+"/"+sourceModule)
	return nil
}

type deletingClient struct {
	fakeClient
	deleted []string
}

func (client *deletingClient) DeleteObject(_ context.Context, key string) error {
	client.deleted = append(client.deleted, key)
	return nil
}

func TestDeleteForSourceRemovesOnlyAvailableObjectOfTheSameModule(t *testing.T) {
	cfg := validConfig()
	object := Object{ID: "obj1", AccountID: "acc", SourceModule: "finance", ObjectKey: "accounts/acc/finance/2026/obj1--a.pdf", Status: "available"}
	repository := &deletingRepository{fakeRepository: fakeRepository{state: ProviderState{AccountID: cfg.AccountID, Bucket: cfg.Bucket}, reserveObject: object}}
	client := &deletingClient{}
	service := newTestService(cfg, repository, client)

	if err := service.DeleteForSource(context.Background(), "acc", "obj1", "calendar"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("other module deleted object: %v", err)
	}
	if err := service.DeleteForSource(context.Background(), "acc", "obj1", "finance"); err != nil {
		t.Fatal(err)
	}
	if len(client.deleted) != 1 || client.deleted[0] != object.ObjectKey || len(repository.marked) != 1 {
		t.Fatalf("deleted=%v marked=%v", client.deleted, repository.marked)
	}
	repository.reserveObject.Status = "pending"
	if err := service.DeleteForSource(context.Background(), "acc", "obj1", "finance"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("pending object deleted: %v", err)
	}
}

func TestDeleteForSourceRequiresDeleteCapableProvider(t *testing.T) {
	cfg := validConfig()
	repository := &fakeRepository{state: ProviderState{AccountID: cfg.AccountID, Bucket: cfg.Bucket}}
	service := newTestService(cfg, repository, &fakeClient{})
	if err := service.DeleteForSource(context.Background(), "acc", "obj1", "finance"); !errors.Is(err, ErrDeleteUnsupported) {
		t.Fatal(err)
	}
	cfg.Enabled = false
	if err := newTestService(cfg, repository, &fakeClient{}).DeleteForSource(context.Background(), "acc", "obj1", "finance"); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
}
