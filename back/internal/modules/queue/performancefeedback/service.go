package performancefeedback

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

const (
	maxFeedbackSections   = 20
	maxSectionTitleLength = 160
	maxRichTextLength     = 200_000
)

type Service struct {
	repository      Repository
	storeFinder     StoreFinder
	metricsProvider MetricsProvider
	now             func() time.Time
}

func NewService(repository Repository, storeFinder StoreFinder, metricsProvider MetricsProvider) *Service {
	return &Service{
		repository:      repository,
		storeFinder:     storeFinder,
		metricsProvider: metricsProvider,
		now:             time.Now,
	}
}

func (service *Service) Context(ctx context.Context, principal auth.Principal, input ContextInput) (ContextView, error) {
	if !canView(principal) {
		return ContextView{}, ErrForbidden
	}

	storeView, err := service.resolveStore(ctx, principal, input.StoreID)
	if err != nil {
		return ContextView{}, err
	}

	settings, err := service.loadSettings(ctx, storeView.TenantID)
	if err != nil {
		return ContextView{}, err
	}
	if settings.Cadence == CadenceMonthly {
		input.Week = 0
	}

	period, err := resolvePeriod(input.Month, input.Week, service.now())
	if err != nil {
		return ContextView{}, err
	}

	userScope := ""
	if principal.Role == auth.RoleConsultant {
		userScope = strings.TrimSpace(principal.UserID)
	}
	consultantRows, err := service.repository.ListConsultants(ctx, storeView.TenantID, storeView.ID, userScope)
	if err != nil {
		return ContextView{}, err
	}

	view := ContextView{
		Store:       storeView,
		Consultants: consultantRows,
		Period:      period,
		History:     []HistoryItem{},
		CanManage:   canManage(principal),
		Settings:    settings,
	}
	if len(consultantRows) == 0 {
		return view, nil
	}

	selectedID := strings.TrimSpace(input.ConsultantID)
	if selectedID == "" {
		selectedID = consultantRows[0].ID
	}
	selected, ok := findConsultant(consultantRows, selectedID)
	if !ok {
		return ContextView{}, ErrConsultantNotFound
	}
	view.Selected = &selected
	view.CanRespond = selected.UserID != "" && selected.UserID == strings.TrimSpace(principal.UserID)

	review, err := service.repository.FindByPeriod(ctx, storeView.TenantID, storeView.ID, selected.ID, period)
	switch {
	case err == nil:
		view.Review = &review
		view.Metrics = &review.Metrics
	case errors.Is(err, ErrNotFound):
		metrics, loadErr := service.buildMetrics(ctx, principal, storeView, selected, period)
		if loadErr != nil {
			return ContextView{}, loadErr
		}
		view.Metrics = &metrics
	default:
		return ContextView{}, err
	}

	history, err := service.repository.ListHistory(ctx, storeView.TenantID, storeView.ID, selected.ID, 12)
	if err != nil {
		return ContextView{}, err
	}
	view.History = history
	return view, nil
}

func (service *Service) SaveManager(ctx context.Context, principal auth.Principal, input ManagerInput) (Review, error) {
	if !canManage(principal) {
		return Review{}, ErrForbidden
	}
	sections, ok := normalizeFeedbackSections(input.FeedbackSections)
	if !ok {
		return Review{}, ErrValidation
	}

	storeView, err := service.resolveStore(ctx, principal, input.StoreID)
	if err != nil {
		return Review{}, err
	}
	settings, err := service.loadSettings(ctx, storeView.TenantID)
	if err != nil {
		return Review{}, err
	}
	if settings.Cadence == CadenceMonthly {
		input.Week = 0
	}
	period, err := resolvePeriod(input.Month, input.Week, service.now())
	if err != nil {
		return Review{}, err
	}
	consultant, err := service.repository.FindConsultant(
		ctx,
		storeView.TenantID,
		storeView.ID,
		strings.TrimSpace(input.ConsultantID),
	)
	if err != nil {
		return Review{}, err
	}

	status := strings.TrimSpace(input.Status)
	if status != StatusDraft && status != StatusShared {
		return Review{}, ErrValidation
	}

	existing, findErr := service.repository.FindByPeriod(ctx, storeView.TenantID, storeView.ID, consultant.ID, period)
	if findErr != nil && !errors.Is(findErr, ErrNotFound) {
		return Review{}, findErr
	}

	var metrics Metrics
	switch {
	case input.MetricsSnapshot != nil:
		var ok bool
		metrics, ok = normalizeMetricsSnapshot(*input.MetricsSnapshot)
		if !ok {
			return Review{}, ErrValidation
		}
	case findErr == nil:
		metrics = existing.Metrics
	default:
		metrics, err = service.buildMetrics(ctx, principal, storeView, consultant, period)
		if err != nil {
			return Review{}, err
		}
	}

	review := Review{
		TenantID:         storeView.TenantID,
		StoreID:          storeView.ID,
		StoreName:        storeView.Name,
		ConsultantID:     consultant.ID,
		ConsultantUserID: consultant.UserID,
		ConsultantName:   consultant.Name,
		Period:           period,
		Status:           status,
		FeedbackSections: sections,
		Metrics:          metrics,
		UpdatedByUserID:  strings.TrimSpace(principal.UserID),
	}
	if findErr == nil {
		review.ID = existing.ID
		review.ConsultantNotesHTML = existing.ConsultantNotesHTML
		review.CreatedByUserID = existing.CreatedByUserID
		review.SharedAt = existing.SharedAt
		review.AcknowledgedAt = existing.AcknowledgedAt
		review.CreatedAt = existing.CreatedAt
		review.Version = existing.Version
	} else {
		review.CreatedByUserID = strings.TrimSpace(principal.UserID)
	}
	if status == StatusShared && review.SharedAt == nil {
		now := service.now().UTC()
		review.SharedAt = &now
	}

	return service.repository.UpsertManager(ctx, review, input.ExpectedVersion)
}

func (service *Service) SaveSettings(ctx context.Context, principal auth.Principal, input SettingsInput) (Settings, error) {
	if !canManage(principal) {
		return Settings{}, ErrForbidden
	}
	storeView, err := service.resolveStore(ctx, principal, input.StoreID)
	if err != nil {
		return Settings{}, err
	}
	settings, ok := normalizeSettings(Settings{
		TenantID:        storeView.TenantID,
		Cadence:         input.Cadence,
		DefaultSections: input.DefaultSections,
		Configured:      true,
		UpdatedByUserID: strings.TrimSpace(principal.UserID),
	})
	if !ok {
		return Settings{}, ErrValidation
	}
	return service.repository.UpsertSettings(ctx, settings, input.ExpectedVersion)
}

func (service *Service) loadSettings(ctx context.Context, tenantID string) (Settings, error) {
	settings, err := service.repository.FindSettings(ctx, tenantID)
	if errors.Is(err, ErrNotFound) {
		return defaultSettings(tenantID), nil
	}
	return settings, err
}

func (service *Service) SaveConsultant(ctx context.Context, principal auth.Principal, input ConsultantInput) (Review, error) {
	if !canView(principal) || principal.Role != auth.RoleConsultant {
		return Review{}, ErrForbidden
	}
	if !validRichText(input.ConsultantNotesHTML) {
		return Review{}, ErrValidation
	}

	tenantID := strings.TrimSpace(principal.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(principal.AccountID)
	}
	if tenantID == "" {
		return Review{}, ErrNotFound
	}

	review, err := service.repository.FindByID(ctx, tenantID, strings.TrimSpace(input.ReviewID))
	if err != nil {
		return Review{}, err
	}
	if review.ConsultantUserID == "" || review.ConsultantUserID != strings.TrimSpace(principal.UserID) {
		return Review{}, ErrNotFound
	}
	if review.Status != StatusShared && review.Status != StatusAcknowledged {
		return Review{}, ErrConflict
	}

	now := service.now().UTC()
	review.ConsultantNotesHTML = strings.TrimSpace(input.ConsultantNotesHTML)
	review.Status = StatusAcknowledged
	review.AcknowledgedAt = &now
	review.UpdatedByUserID = strings.TrimSpace(principal.UserID)
	return service.repository.UpdateConsultant(ctx, review, input.ExpectedVersion)
}

func (service *Service) buildMetrics(
	ctx context.Context,
	principal auth.Principal,
	storeView stores.StoreView,
	consultant Consultant,
	period Period,
) (Metrics, error) {
	metrics, err := service.metricsProvider.LoadConsultantMetrics(
		ctx,
		principal,
		storeView.ID,
		consultant.ID,
		period.DateFrom,
		period.DateTo,
	)
	if err != nil {
		return Metrics{}, err
	}

	goal, err := service.repository.LoadGoal(ctx, storeView.TenantID, storeView.ID, consultant.ID, period)
	if err != nil {
		return Metrics{}, err
	}
	metrics.SalesGoal = goal.SalesGoal
	metrics.TicketGoal = goal.TicketGoal
	metrics.ConversionGoal = goal.ConversionGoal
	metrics.PAGoal = goal.PAGoal

	transcriptionScore, samples, err := service.repository.LoadTranscriptionScore(
		ctx,
		storeView.TenantID,
		storeView.ID,
		consultant.ID,
		period,
	)
	if err != nil {
		return Metrics{}, err
	}
	metrics.TranscriptionScore = transcriptionScore
	metrics.TranscriptionSamples = samples
	return metrics, nil
}

func (service *Service) resolveStore(ctx context.Context, principal auth.Principal, requestedStoreID string) (stores.StoreView, error) {
	storeID := strings.TrimSpace(requestedStoreID)
	if storeID != "" {
		storeView, err := service.storeFinder.FindAccessible(ctx, principal, storeID)
		if err != nil {
			return stores.StoreView{}, mapStoreError(err)
		}
		return storeView, nil
	}

	rows, err := service.storeFinder.ListAccessible(ctx, principal, stores.ListInput{})
	if err != nil {
		return stores.StoreView{}, mapStoreError(err)
	}
	if len(rows) != 1 {
		return stores.StoreView{}, ErrStoreRequired
	}
	return rows[0], nil
}

func canView(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionPerformanceFeedbackView)
	}
	return principal.Role == auth.RolePlatformAdmin || principal.Role == auth.RoleOwner || principal.Role == auth.RoleManager || principal.Role == auth.RoleConsultant
}

func canManage(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionPerformanceFeedbackEdit)
	}
	return principal.Role == auth.RolePlatformAdmin || principal.Role == auth.RoleOwner || principal.Role == auth.RoleManager
}

func validRichText(value string) bool {
	return len(value) <= maxRichTextLength
}

func defaultSettings(tenantID string) Settings {
	return Settings{
		TenantID:   strings.TrimSpace(tenantID),
		Cadence:    CadenceMonthly,
		Configured: false,
		Version:    0,
		DefaultSections: []FeedbackSection{
			{ID: "strengths-and-opportunities", Title: "Pontos fortes e oportunidades"},
			{ID: "action-plan", Title: "Plano de ação e combinados"},
		},
	}
}

func normalizeSettings(settings Settings) (Settings, bool) {
	settings.TenantID = strings.TrimSpace(settings.TenantID)
	settings.Cadence = strings.TrimSpace(settings.Cadence)
	if settings.TenantID == "" || (settings.Cadence != CadenceMonthly && settings.Cadence != CadenceWeekly) {
		return Settings{}, false
	}
	if len(settings.DefaultSections) == 0 {
		return Settings{}, false
	}
	sections := make([]FeedbackSection, len(settings.DefaultSections))
	for index, section := range settings.DefaultSections {
		section.ContentHTML = ""
		sections[index] = section
	}
	normalized, ok := normalizeFeedbackSections(sections)
	if !ok {
		return Settings{}, false
	}
	settings.DefaultSections = normalized
	return settings, true
}

func normalizeMetricsSnapshot(metrics Metrics) (Metrics, bool) {
	floatValues := []float64{
		metrics.SoldValue,
		metrics.ConversionRate,
		metrics.TicketAverage,
		metrics.PAScore,
		metrics.QualityScore,
		metrics.AvgDurationMs,
		metrics.QueueJumpRate,
		metrics.CancellationRate,
		metrics.SalesGoal,
		metrics.TicketGoal,
		metrics.ConversionGoal,
		metrics.PAGoal,
	}
	for _, value := range floatValues {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return Metrics{}, false
		}
	}
	intValues := []int{
		metrics.Attendances,
		metrics.Conversions,
		metrics.NonConversions,
		metrics.NonClientConversions,
		metrics.QueueJumpServices,
		metrics.ERPOrders,
		metrics.TranscriptionSamples,
	}
	for _, value := range intValues {
		if value < 0 {
			return Metrics{}, false
		}
	}
	if metrics.TranscriptionScore != nil && (math.IsNaN(*metrics.TranscriptionScore) || math.IsInf(*metrics.TranscriptionScore, 0) || *metrics.TranscriptionScore < 0 || *metrics.TranscriptionScore > 10) {
		return Metrics{}, false
	}
	metrics.SoldValueSource = strings.TrimSpace(metrics.SoldValueSource)
	metrics.TicketAverageSource = strings.TrimSpace(metrics.TicketAverageSource)
	metrics.PAScoreSource = strings.TrimSpace(metrics.PAScoreSource)
	if len(metrics.SoldValueSource) > 40 || len(metrics.TicketAverageSource) > 40 || len(metrics.PAScoreSource) > 40 {
		return Metrics{}, false
	}
	return metrics, true
}

func normalizeFeedbackSections(sections []FeedbackSection) ([]FeedbackSection, bool) {
	if len(sections) > maxFeedbackSections {
		return nil, false
	}

	normalized := make([]FeedbackSection, 0, len(sections))
	seenIDs := make(map[string]struct{}, len(sections))
	totalContentLength := 0
	for _, section := range sections {
		section.ID = strings.TrimSpace(section.ID)
		section.Title = strings.TrimSpace(section.Title)
		section.ContentHTML = strings.TrimSpace(section.ContentHTML)
		if section.ID == "" || len(section.ID) > 80 || section.Title == "" || len(section.Title) > maxSectionTitleLength {
			return nil, false
		}
		if _, exists := seenIDs[section.ID]; exists {
			return nil, false
		}
		seenIDs[section.ID] = struct{}{}
		totalContentLength += len(section.ContentHTML)
		if totalContentLength > maxRichTextLength {
			return nil, false
		}
		normalized = append(normalized, section)
	}
	return normalized, true
}

func findConsultant(rows []Consultant, consultantID string) (Consultant, bool) {
	for _, row := range rows {
		if row.ID == consultantID {
			return row, true
		}
	}
	return Consultant{}, false
}

func mapStoreError(err error) error {
	if errors.Is(err, stores.ErrStoreNotFound) || errors.Is(err, stores.ErrForbidden) || errors.Is(err, stores.ErrTenantForbidden) {
		return ErrNotFound
	}
	return err
}
