package tasks

import (
	"context"
	"testing"
)

func TestCompleteUpdatedBoardReturnsArchivedMutationWithoutActiveReload(t *testing.T) {
	repository := &PostgresRepository{}
	archived := Board{
		ID:       "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Name:     "Campanha",
		Archived: true,
	}

	board, err := repository.completeUpdatedBoard(context.Background(), "account-1", archived)
	if err != nil {
		t.Fatalf("completeUpdatedBoard: %v", err)
	}
	if board.ID != archived.ID || !board.Archived {
		t.Fatalf("board arquivado deveria ser devolvido sem releitura ativa: %+v", board)
	}
}
