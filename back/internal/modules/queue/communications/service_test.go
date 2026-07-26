package communications

import (
	"context"
	"errors"
	"testing"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
)

type fakeRepository struct {
	items       map[string]Communication
	validStores bool
}

func (repository *fakeRepository) List(
	_ context.Context,
	accountID string,
	_ ListFilter,
) ([]Communication, error) {
	items := make([]Communication, 0)
	for _, item := range repository.items {
		if item.AccountID == accountID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (repository *fakeRepository) Get(
	_ context.Context,
	accountID, communicationID string,
) (Communication, error) {
	item, ok := repository.items[communicationID]
	if !ok || item.AccountID != accountID {
		return Communication{}, ErrNotFound
	}
	return item, nil
}

func (repository *fakeRepository) StoresBelongToAccount(
	_ context.Context,
	_ string,
	_ []string,
) (bool, error) {
	return repository.validStores, nil
}

func (repository *fakeRepository) Create(
	_ context.Context,
	item Communication,
) (Communication, error) {
	item.ID = "communication-1"
	item.CreatedAt = time.Now()
	item.UpdatedAt = item.CreatedAt
	repository.items[item.ID] = item
	return item, nil
}

func (repository *fakeRepository) Update(
	_ context.Context,
	item Communication,
) (Communication, error) {
	if _, ok := repository.items[item.ID]; !ok {
		return Communication{}, ErrNotFound
	}
	repository.items[item.ID] = item
	return item, nil
}

func (repository *fakeRepository) Archive(
	_ context.Context,
	accountID, communicationID, _ string,
) error {
	item, ok := repository.items[communicationID]
	if !ok || item.AccountID != accountID {
		return ErrNotFound
	}
	delete(repository.items, communicationID)
	return nil
}

func managerAccess() AccessContext {
	return AccessContext{
		UserID:              "00000000-0000-0000-0000-000000000001",
		AccountID:           "00000000-0000-0000-0000-000000000002",
		Role:                "manager",
		StoreIDs:            []string{"00000000-0000-0000-0000-000000000003"},
		Permissions:         []string{accesscontrol.PermissionOperationsView, accesscontrol.PermissionQueueCommunicationsManage},
		PermissionsResolved: true,
	}
}

func TestCreateCommunicationNormalizesTargets(t *testing.T) {
	repository := &fakeRepository{
		items:       make(map[string]Communication),
		validStores: true,
	}
	service := NewService(repository)
	access := managerAccess()

	created, err := service.Create(context.Background(), access, UpsertInput{
		Title:            "  Aviso importante  ",
		Body:             "  Conteudo do aviso.  ",
		IsPublished:      true,
		TargetsAllStores: false,
		StoreIDs:         []string{access.StoreIDs[0], access.StoreIDs[0]},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Title != "Aviso importante" || created.Body != "Conteudo do aviso." {
		t.Fatalf("Create() did not normalize text: %#v", created)
	}
	if len(created.StoreIDs) != 1 || created.StoreIDs[0] != access.StoreIDs[0] {
		t.Fatalf("Create() storeIds = %#v", created.StoreIDs)
	}
}

func TestCreateCommunicationRejectsStoreOutsideScope(t *testing.T) {
	repository := &fakeRepository{
		items:       make(map[string]Communication),
		validStores: true,
	}
	service := NewService(repository)

	_, err := service.Create(context.Background(), managerAccess(), UpsertInput{
		Title:            "Aviso",
		Body:             "Conteudo",
		TargetsAllStores: false,
		StoreIDs:         []string{"00000000-0000-0000-0000-000000000999"},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create() error = %v, want ErrForbidden", err)
	}
}

func TestConsultantCannotManageCommunications(t *testing.T) {
	repository := &fakeRepository{
		items:       make(map[string]Communication),
		validStores: true,
	}
	service := NewService(repository)
	access := managerAccess()
	access.Role = "consultant"
	access.Permissions = []string{accesscontrol.PermissionOperationsView}

	_, err := service.Create(context.Background(), access, UpsertInput{
		Title:            "Aviso",
		Body:             "Conteudo",
		TargetsAllStores: true,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create() error = %v, want ErrForbidden", err)
	}
}

func TestPlatformAdminCanManageBeforePermissionRefresh(t *testing.T) {
	repository := &fakeRepository{
		items:       make(map[string]Communication),
		validStores: true,
	}
	service := NewService(repository)
	access := managerAccess()
	access.Role = "platform_admin"
	access.Permissions = []string{accesscontrol.PermissionOperationsView}

	if _, err := service.Create(context.Background(), access, UpsertInput{
		Title:            "Aviso",
		Body:             "Conteudo",
		TargetsAllStores: true,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestListPublishedForStoreRequiresAccessibleStore(t *testing.T) {
	repository := &fakeRepository{
		items:       make(map[string]Communication),
		validStores: true,
	}
	service := NewService(repository)

	_, err := service.List(context.Background(), managerAccess(), ListFilter{
		StoreID:       "00000000-0000-0000-0000-000000000999",
		PublishedOnly: true,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("List() error = %v, want ErrForbidden", err)
	}
}
