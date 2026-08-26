package tasks

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const (
	testBoardID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	testSourceBoardID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func TestUpdateBoardSelectedTaskSourcesNormalizesAndValidates(t *testing.T) {
	mode := " SELECTED "
	ids := []string{" " + strings.ToUpper(testSourceBoardID) + " ", testSourceBoardID, testBoardID}
	validated := false
	repository := &repositoryMock{
		onGetBoard: func(_ context.Context, _ AccessContext, _ string) (Board, error) {
			return Board{ID: testBoardID, TaskSourceMode: TaskSourceOwn}, nil
		},
		onValidateBoardTaskSources: func(_ context.Context, accountID, boardID string, sourceBoardIDs []string) error {
			validated = true
			if accountID != "acc-agency" || boardID != testBoardID {
				t.Fatalf("escopo incorreto: account=%q board=%q", accountID, boardID)
			}
			if len(sourceBoardIDs) != 1 || sourceBoardIDs[0] != testSourceBoardID {
				t.Fatalf("paginas de origem deveriam ser normalizadas: %v", sourceBoardIDs)
			}
			return nil
		},
		onUpdateBoard: func(_ context.Context, _ string, input UpdateBoardInput) (Board, error) {
			return Board{
				ID:                 input.ID,
				TaskSourceMode:     *input.TaskSourceMode,
				TaskSourceBoardIDs: *input.TaskSourceBoardIDs,
			}, nil
		},
	}

	service := NewService(repository, nil, nil, nil)
	board, err := service.UpdateBoard(context.Background(), agencyAccess(), UpdateBoardInput{
		ID:                 testBoardID,
		TaskSourceMode:     &mode,
		TaskSourceBoardIDs: &ids,
	})
	if err != nil {
		t.Fatalf("UpdateBoard: %v", err)
	}
	if !validated || board.TaskSourceMode != TaskSourceSelected || len(board.TaskSourceBoardIDs) != 1 {
		t.Fatalf("origens selecionadas nao persistidas: validated=%v board=%+v", validated, board)
	}
}

func TestUpdateBoardSelectedTaskSourcesRequiresAnotherBoard(t *testing.T) {
	mode := TaskSourceSelected
	ids := []string{testBoardID}
	repository := &repositoryMock{
		onGetBoard: func(_ context.Context, _ AccessContext, _ string) (Board, error) {
			return Board{ID: testBoardID, TaskSourceMode: TaskSourceOwn}, nil
		},
		onUpdateBoard: func(_ context.Context, _ string, _ UpdateBoardInput) (Board, error) {
			t.Fatal("repository nao deve atualizar origem selecionada vazia")
			return Board{}, nil
		},
	}

	service := NewService(repository, nil, nil, nil)
	_, err := service.UpdateBoard(context.Background(), agencyAccess(), UpdateBoardInput{
		ID:                 testBoardID,
		TaskSourceMode:     &mode,
		TaskSourceBoardIDs: &ids,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("origem selecionada vazia deveria falhar validacao; got %v", err)
	}
}

func TestBoardTaskSourceClauseKeepsOwnAndIncludesConfiguredBoards(t *testing.T) {
	for _, expected := range []string{
		"t.board_id = target.id",
		"target.task_source_mode = 'all'",
		"target.task_source_mode = 'selected'",
		"t.board_id = any(target.task_source_board_ids)",
	} {
		if !strings.Contains(boardTaskSourceClause, expected) {
			t.Fatalf("filtro de origem do board deveria conter %q", expected)
		}
	}
	if strings.Contains(boardTaskListScopeClause, "t.board_id = $2") {
		t.Fatal("consulta base nao pode restringir ao board alvo antes de aplicar as origens configuradas")
	}
}

func TestSaveUserPreferencesUsesAuthenticatedScope(t *testing.T) {
	repository := &repositoryMock{
		onSaveUserPreferences: func(_ context.Context, accountID, userID, lastBoardID string) (UserPreferences, error) {
			if accountID != "acc-agency" || userID != "user-agency" || lastBoardID != testBoardID {
				t.Fatalf("preferencia fora do principal: account=%q user=%q board=%q", accountID, userID, lastBoardID)
			}
			return UserPreferences{LastBoardID: lastBoardID}, nil
		},
	}

	service := NewService(repository, nil, nil, nil)
	preferences, err := service.SaveUserPreferences(context.Background(), agencyAccess(), UpdateUserPreferencesInput{LastBoardID: strings.ToUpper(testBoardID)})
	if err != nil {
		t.Fatalf("SaveUserPreferences: %v", err)
	}
	if preferences.LastBoardID != testBoardID {
		t.Fatalf("ultima pagina nao normalizada: %+v", preferences)
	}
}
