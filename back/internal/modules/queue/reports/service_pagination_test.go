package reports

import (
	"context"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

type fakeReportsRepository struct {
	history    []operations.ServiceHistoryEntry
	countTotal int
	countCalls int
	failCount  func()
}

func (repository *fakeReportsRepository) ListHistory(
	context.Context, string, repositoryFilters,
) ([]operations.ServiceHistoryEntry, error) {
	return append([]operations.ServiceHistoryEntry{}, repository.history...), nil
}

func (repository *fakeReportsRepository) ListHistoryByStores(
	context.Context, []string, repositoryFilters,
) ([]operations.ServiceHistoryEntry, error) {
	return append([]operations.ServiceHistoryEntry{}, repository.history...), nil
}

func (repository *fakeReportsRepository) CountHistory(
	context.Context, []string, repositoryFilters,
) (int, error) {
	repository.countCalls++
	if repository.failCount != nil {
		repository.failCount()
	}
	return repository.countTotal, nil
}

func (repository *fakeReportsRepository) ListLiveCounts(
	context.Context, []string,
) (map[string]StoreLiveCounts, error) {
	return map[string]StoreLiveCounts{}, nil
}

func (repository *fakeReportsRepository) ListPauseSessions(
	context.Context, []string, *int64, *int64, []string,
) ([]PauseSessionRow, error) {
	return []PauseSessionRow{}, nil
}

type fakeStoreFinder struct {
	store stores.StoreView
}

func (finder fakeStoreFinder) ListAccessible(
	context.Context, auth.Principal, stores.ListInput,
) ([]stores.StoreView, error) {
	return []stores.StoreView{finder.store}, nil
}

func (finder fakeStoreFinder) FindAccessible(
	context.Context, auth.Principal, string,
) (stores.StoreView, error) {
	return finder.store, nil
}

func makeHistory(count int) []operations.ServiceHistoryEntry {
	entries := make([]operations.ServiceHistoryEntry, 0, count)
	for index := 0; index < count; index++ {
		entries = append(entries, operations.ServiceHistoryEntry{
			ServiceID:     "service-" + string(rune('a'+index%26)),
			StoreID:       "store-1",
			FinishedAt:    int64(index),
			FinishOutcome: "compra",
		})
	}
	return entries
}

func ownerPrincipal() auth.Principal {
	return auth.Principal{Role: auth.RoleOwner}
}

func storeScopedFilters() Filters {
	return Filters{StoreID: "store-1"}
}

func TestNormalizeFiltersClampsLimit(t *testing.T) {
	cases := []struct {
		name     string
		input    int
		expected int
	}{
		{name: "zero uses default", input: 0, expected: defaultHistoryFetchLimit},
		{name: "above max clamps", input: 9999, expected: maxHistoryFetchLimit},
		{name: "within range kept", input: 300, expected: 300},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			normalized, repositoryInput, err := normalizeFilters(Filters{StoreID: "store-1", Limit: testCase.input})
			if err != nil {
				t.Fatalf("normalizeFilters error: %v", err)
			}
			if normalized.Limit != testCase.expected {
				t.Fatalf("expected normalized limit %d, got %d", testCase.expected, normalized.Limit)
			}
			if repositoryInput.Limit != testCase.expected {
				t.Fatalf("expected repository limit %d, got %d", testCase.expected, repositoryInput.Limit)
			}
		})
	}
}

func TestResultsMarksTruncatedWindow(t *testing.T) {
	limit := defaultHistoryFetchLimit
	repository := &fakeReportsRepository{
		history:    makeHistory(limit),
		countTotal: limit + 500,
	}
	service := NewService(repository, fakeStoreFinder{store: stores.StoreView{ID: "store-1", Name: "Loja 1"}})

	filters := storeScopedFilters()
	filters.PageSize = 50

	response, err := service.Results(context.Background(), ownerPrincipal(), filters)
	if err != nil {
		t.Fatalf("Results error: %v", err)
	}

	if !response.HistoryWindow.Truncated {
		t.Fatal("expected HistoryWindow.Truncated=true when count exceeds fetched")
	}
	if response.HistoryWindow.Total != limit+500 {
		t.Fatalf("expected total %d, got %d", limit+500, response.HistoryWindow.Total)
	}
	if response.HistoryWindow.Fetched != limit {
		t.Fatalf("expected fetched %d, got %d", limit, response.HistoryWindow.Fetched)
	}
	if response.HistoryWindow.Limit != limit {
		t.Fatalf("expected window limit %d, got %d", limit, response.HistoryWindow.Limit)
	}
	if len(response.Rows) != filters.PageSize {
		t.Fatalf("expected page size %d rows, got %d", filters.PageSize, len(response.Rows))
	}
	if repository.countCalls != 1 {
		t.Fatalf("expected CountHistory called once, got %d", repository.countCalls)
	}
}

func TestResultsSkipsCountWhenBelowLimit(t *testing.T) {
	limit := defaultHistoryFetchLimit
	repository := &fakeReportsRepository{
		history: makeHistory(limit - 1),
	}
	repository.failCount = func() {
		t.Fatal("CountHistory must not run when window is below the limit")
	}
	service := NewService(repository, fakeStoreFinder{store: stores.StoreView{ID: "store-1", Name: "Loja 1"}})

	response, err := service.Results(context.Background(), ownerPrincipal(), storeScopedFilters())
	if err != nil {
		t.Fatalf("Results error: %v", err)
	}

	if response.HistoryWindow.Truncated {
		t.Fatal("expected HistoryWindow.Truncated=false below the limit")
	}
	if response.HistoryWindow.Total != response.HistoryWindow.Fetched {
		t.Fatalf("expected total==fetched, got total=%d fetched=%d", response.HistoryWindow.Total, response.HistoryWindow.Fetched)
	}
	if response.HistoryWindow.Fetched != limit-1 {
		t.Fatalf("expected fetched %d, got %d", limit-1, response.HistoryWindow.Fetched)
	}
}
