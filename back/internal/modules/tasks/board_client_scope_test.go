package tasks

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestUpdateBoardSelectedClientScopeNormalizesAndValidates(t *testing.T) {
	clientID := "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"
	mode := " SELECTED "
	ids := []string{" " + clientID + " ", strings.ToLower(clientID), "invalido"}
	validated := false
	repository := &repositoryMock{
		onGetBoard: func(_ context.Context, _ AccessContext, _ string) (Board, error) {
			return Board{ID: "board-1", ClientScopeMode: ClientScopeActive}, nil
		},
		onValidateBoardClientScope: func(_ context.Context, accountID, boardID string, clientIDs []string) error {
			validated = true
			if accountID != "acc-agency" || boardID != "board-1" {
				t.Fatalf("escopo incorreto: account=%q board=%q", accountID, boardID)
			}
			if len(clientIDs) != 1 || clientIDs[0] != strings.ToLower(clientID) {
				t.Fatalf("clientes deveriam ser UUIDs normalizados e deduplicados; got %v", clientIDs)
			}
			return nil
		},
		onUpdateBoard: func(_ context.Context, _ string, input UpdateBoardInput) (Board, error) {
			if input.ClientScopeMode == nil || *input.ClientScopeMode != ClientScopeSelected {
				t.Fatalf("modo selecionado nao normalizado: %v", input.ClientScopeMode)
			}
			if input.ClientScopeIDs == nil || len(*input.ClientScopeIDs) != 1 {
				t.Fatalf("ids selecionados nao normalizados: %v", input.ClientScopeIDs)
			}
			return Board{ID: input.ID, ClientScopeMode: *input.ClientScopeMode, ClientScopeIDs: *input.ClientScopeIDs}, nil
		},
	}

	service := NewService(repository, nil, nil, nil)
	board, err := service.UpdateBoard(context.Background(), agencyAccess(), UpdateBoardInput{
		ID:              "board-1",
		ClientScopeMode: &mode,
		ClientScopeIDs:  &ids,
	})
	if err != nil {
		t.Fatalf("UpdateBoard: %v", err)
	}
	if !validated || board.ClientScopeMode != ClientScopeSelected || len(board.ClientScopeIDs) != 1 {
		t.Fatalf("escopo selecionado nao persistido: validated=%v board=%+v", validated, board)
	}
}

func TestUpdateBoardSelectedClientScopeRequiresClient(t *testing.T) {
	mode := ClientScopeSelected
	ids := []string{}
	repository := &repositoryMock{
		onGetBoard: func(_ context.Context, _ AccessContext, _ string) (Board, error) {
			return Board{ID: "board-1", ClientScopeMode: ClientScopeActive}, nil
		},
		onUpdateBoard: func(_ context.Context, _ string, _ UpdateBoardInput) (Board, error) {
			t.Fatal("repository nao deve atualizar escopo selecionado vazio")
			return Board{}, nil
		},
	}

	service := NewService(repository, nil, nil, nil)
	_, err := service.UpdateBoard(context.Background(), agencyAccess(), UpdateBoardInput{
		ID:              "board-1",
		ClientScopeMode: &mode,
		ClientScopeIDs:  &ids,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("escopo selecionado vazio deveria falhar validacao; got %v", err)
	}
}

func TestBoardClientScopeClauseKeepsUnassignedAndFiltersConfiguredClients(t *testing.T) {
	for _, expected := range []string{
		"target.client_scope_mode <> 'selected' and t.client_account_id is null",
		"target.client_scope_mode = 'all'",
		"target.client_scope_mode = 'active' and client.is_active = true",
		"target.client_scope_mode = 'selected'",
		"t.client_account_id = any(target.client_scope_ids)",
	} {
		if !strings.Contains(boardClientScopeClause, expected) {
			t.Fatalf("filtro de clientes do board deveria conter %q", expected)
		}
	}
}
