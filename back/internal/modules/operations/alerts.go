package operations

import (
	"context"
	"time"
)

const (
	TriggerLongOpenService = "long_open_service"

	SignalLongOpenServiceTriggered = "long_open_service.triggered"
	SignalLongOpenServiceResolved  = "long_open_service.resolved"

	SignalLongQueueWaitTriggered = "long_queue_wait.triggered"
	SignalLongQueueWaitResolved  = "long_queue_wait.resolved"

	SignalLongPauseTriggered = "long_pause.triggered"
	SignalLongPauseResolved  = "long_pause.resolved"

	SignalIdleStoreTriggered = "idle_store.triggered"
	SignalIdleStoreResolved  = "idle_store.resolved"

	SignalOutsideBusinessHoursTriggered = "outside_business_hours.triggered"
	SignalOutsideBusinessHoursResolved  = "outside_business_hours.resolved"
)

type OperationalAlertRules struct {
	LongOpenServiceMinutes int
	NotifyDashboard        bool
	NotifyOperationContext bool
}

type OperationalAlertSignal struct {
	TenantID       string
	StoreID        string
	ServiceID      string
	ConsultantID   string
	SignalType     string
	TriggeredAt    time.Time
	Metadata       map[string]any
	ConsultantName string
	ElapsedMinutes int
	TriggerType    string
}

type AlertCoordinator interface {
	LoadOperationalRules(ctx context.Context, storeID string) (OperationalAlertRules, error)
	ReceiveOperationalSignals(ctx context.Context, signals []OperationalAlertSignal) error
}
