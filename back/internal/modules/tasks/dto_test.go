package tasks

import (
	"testing"
	"time"
)

// stringPtr e' um helper local para escrever ponteiros literais em tabelas de teste sem poluir
// o pacote com varias variaveis intermediarias.
func stringPtr(value string) *string {
	return &value
}

func TestBuildTaskDTO_AgencyKeepsClientAccount(t *testing.T) {
	service := NewService(nil, nil, nil, nil)

	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	clientAccountID := "00000000-0000-0000-0000-000000000999"
	responsibleUserID := "00000000-0000-0000-0000-000000000111"
	task := Task{
		ID:                "task-1",
		BoardID:           "board-1",
		AccountID:         "acc-agency",
		Title:             "Renomear card",
		Priority:          "media",
		ResponsibleUserID: &responsibleUserID,
		ClientAccountID:   &clientAccountID,
		UIMetadata:        nil,
		Version:           7,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	dto := service.BuildTaskDTO(task, PerspectiveAgency)

	if dto.ClientAccountID == nil || *dto.ClientAccountID != clientAccountID {
		t.Fatalf("perspective agency deve manter clientAccountId; got %v", dto.ClientAccountID)
	}
	if dto.Responsible == nil || dto.Responsible.ID != responsibleUserID {
		t.Fatalf("perspective agency deve carregar Responsible{ID}; got %+v", dto.Responsible)
	}
	if dto.UIMetadata == nil {
		t.Fatalf("UIMetadata deve ser objeto vazio (nao nil) — front depende disso para sobrescrever cache local")
	}
	if dto.CreatedAt == "" || dto.UpdatedAt == "" {
		t.Fatalf("CreatedAt/UpdatedAt devem ser ISO-8601 formatados; got %q / %q", dto.CreatedAt, dto.UpdatedAt)
	}
}

func TestBuildTaskDTO_ClientViewerOmitsClientAccount(t *testing.T) {
	service := NewService(nil, nil, nil, nil)

	clientAccountID := "00000000-0000-0000-0000-000000000999"
	task := Task{
		ID:              "task-1",
		BoardID:         "board-1",
		AccountID:       "acc-agency",
		Title:           "Renomear card",
		Priority:        "media",
		ClientAccountID: &clientAccountID,
		Version:         7,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	dto := service.BuildTaskDTO(task, PerspectiveClientViewer)

	if dto.ClientAccountID != nil {
		t.Fatalf("perspective client_viewer NAO deve expor clientAccountId; got %q", *dto.ClientAccountID)
	}
	if dto.Title != task.Title {
		t.Fatalf("client_viewer deve manter title e demais campos visiveis; got %q", dto.Title)
	}
}

func TestBuildTaskDTO_NilUIMetadataBecomesEmptyMap(t *testing.T) {
	service := NewService(nil, nil, nil, nil)

	dto := service.BuildTaskDTO(Task{ID: "t", CreatedAt: time.Now(), UpdatedAt: time.Now()}, PerspectiveAgency)
	if dto.UIMetadata == nil {
		t.Fatalf("UIMetadata deve ser map vazio quando Task.UIMetadata == nil (front sempre espera objeto)")
	}
	if len(dto.UIMetadata) != 0 {
		t.Fatalf("UIMetadata deve ser vazio quando entrada e nil; got %v", dto.UIMetadata)
	}
}

func TestBuildTaskDTO_FormatsDates(t *testing.T) {
	service := NewService(nil, nil, nil, nil)

	dueDate := time.Date(2026, 6, 1, 15, 30, 0, 0, time.UTC)
	startDate := time.Date(2026, 5, 30, 9, 0, 0, 0, time.UTC)
	task := Task{
		ID:        "t",
		BoardID:   "b",
		AccountID: "a",
		DueDate:   &dueDate,
		StartDate: &startDate,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	dto := service.BuildTaskDTO(task, PerspectiveAgency)
	if dto.DueDate == nil || *dto.DueDate != "2026-06-01T15:30:00Z" {
		t.Fatalf("DueDate deve virar ISO-8601 UTC; got %v", dto.DueDate)
	}
	if dto.StartDate == nil || *dto.StartDate != "2026-05-30T09:00:00Z" {
		t.Fatalf("StartDate deve virar ISO-8601 UTC; got %v", dto.StartDate)
	}
}
