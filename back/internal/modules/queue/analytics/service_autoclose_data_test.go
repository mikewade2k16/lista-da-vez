package analytics

import (
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

func TestBuildAutoCloseDataGroupsAuditByConsultantAndStore(t *testing.T) {
	bundles := []bundle{
		{
			storeID:   "store-1",
			storeView: stores.StoreView{ID: "store-1", Name: "Loja Centro"},
			snapshot: operations.SnapshotState{
				ServiceHistory: []operations.ServiceHistoryEntry{
					{
						ServiceID:        "service-validated",
						StoreID:          "store-1",
						PersonID:         "consultant-1",
						PersonName:       "Ana",
						CloseReason:      "auto",
						ValidationStatus: "validated",
						ValidatedBy:      "manager-1",
						ValidatedAt:      3000,
						ValidationReason: "Consultora esqueceu de encerrar",
						FinishedAt:       2000,
						SnoozeCount:      1,
					},
					{
						ServiceID:        "service-pending",
						StoreID:          "store-1",
						PersonID:         "consultant-1",
						PersonName:       "Ana",
						CloseReason:      "auto",
						ValidationStatus: "pending",
						FinishedAt:       4000,
					},
					{
						ServiceID:        "service-manual",
						StoreID:          "store-1",
						PersonID:         "consultant-2",
						PersonName:       "Bia",
						CloseReason:      "manual",
						ValidationStatus: "validated",
						FinishedAt:       5000,
					},
				},
			},
		},
	}

	result := buildAutoCloseData(bundles, map[string]string{"manager-1": "Gerente Maria"})

	if result.Summary.Total != 2 || result.Summary.Pending != 1 || result.Summary.Validated != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if len(result.ByConsultant) != 1 || result.ByConsultant[0].ConsultantName != "Ana" || result.ByConsultant[0].Total != 2 {
		t.Fatalf("unexpected consultant rows: %+v", result.ByConsultant)
	}
	if len(result.ByStore) != 1 || result.ByStore[0].StoreName != "Loja Centro" || result.ByStore[0].Total != 2 {
		t.Fatalf("unexpected store rows: %+v", result.ByStore)
	}
	if len(result.Recent) != 2 || result.Recent[0].ServiceID != "service-pending" {
		t.Fatalf("unexpected recent order: %+v", result.Recent)
	}
	if result.Recent[1].ClosedByName != "Gerente Maria" || result.Recent[1].Reason == "" {
		t.Fatalf("expected validator and reason in audit: %+v", result.Recent[1])
	}
}

func TestCollectAutoCloseValidatorIDsDeduplicatesAndIgnoresManual(t *testing.T) {
	bundles := []bundle{{
		snapshot: operations.SnapshotState{ServiceHistory: []operations.ServiceHistoryEntry{
			{CloseReason: "auto", ValidatedBy: "manager-2"},
			{CloseReason: "auto", ValidatedBy: "manager-1"},
			{CloseReason: "auto", ValidatedBy: "manager-2"},
			{CloseReason: "manual", ValidatedBy: "manager-3"},
		}},
	}}

	ids := collectAutoCloseValidatorIDs(bundles)
	if len(ids) != 2 || ids[0] != "manager-1" || ids[1] != "manager-2" {
		t.Fatalf("unexpected validator ids: %+v", ids)
	}
}
