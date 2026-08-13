package app

import (
	"errors"
	"testing"

	objectstorage "github.com/mikewade2k16/lista-da-vez/back/internal/modules/storage"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tasks"
)

func TestMapTaskStorageErrorPreservesOperationalReason(t *testing.T) {
	tests := []struct {
		name string
		in   error
		want error
	}{
		{name: "analytics", in: objectstorage.ErrAnalyticsUnavailable, want: tasks.ErrVideoMetricsUnavailable},
		{name: "storage quota", in: objectstorage.ErrStorageQuotaExceeded, want: tasks.ErrVideoQuotaExceeded},
		{name: "class A quota", in: objectstorage.ErrClassAQuotaExceeded, want: tasks.ErrVideoQuotaExceeded},
		{name: "provider", in: objectstorage.ErrProviderMismatch, want: tasks.ErrVideoUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := mapTaskStorageError(test.in); !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}
