package omnichannel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type AutomationService struct {
	store           automationProfileRepository
	permissions     automationPermissionChecker
	clients         AutomationClientCatalog
	businessContext AutomationBusinessContextProvider
	domain          automationDomainController
}

type automationDomainController interface {
	RequestAutomationHandoff(context.Context, string, string, string, HandoffRequest) (HandoffView, error)
}

type automationPermissionChecker interface {
	requirePermission(context.Context, string, auth.Principal, string) error
}

func NewAutomationService(store automationProfileRepository, permissions automationPermissionChecker, clients AutomationClientCatalog, businessContext AutomationBusinessContextProvider, domain ...automationDomainController) *AutomationService {
	service := &AutomationService{store: store, permissions: permissions, clients: clients, businessContext: businessContext}
	if len(domain) > 0 {
		service.domain = domain[0]
	}
	return service
}

func (s *AutomationService) ListProfiles(ctx context.Context, accountID string, p auth.Principal) ([]AutomationProfileView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return nil, err
	}
	clients, err := s.accessibleClients(ctx, p)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListAutomationProfiles(ctx, accountID)
	if err != nil {
		return nil, err
	}
	byClient := make(map[string]automationProfileRow, len(rows))
	for _, row := range rows {
		byClient[row.ClientAccountID] = row
	}
	out := make([]AutomationProfileView, 0, len(clients))
	for _, client := range clients {
		row, ok := byClient[client.ID]
		if !ok {
			out = append(out, emptyAutomationProfile(client))
			continue
		}
		out = append(out, automationProfileView(row, client))
	}
	return out, nil
}

func (s *AutomationService) GetProfile(ctx context.Context, accountID string, p auth.Principal, clientID string) (AutomationProfileView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return AutomationProfileView{}, err
	}
	client, err := s.accessibleClient(ctx, p, clientID)
	if err != nil {
		return AutomationProfileView{}, err
	}
	row, err := s.store.GetAutomationProfile(ctx, accountID, client.ID)
	var out AutomationProfileView
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		out = emptyAutomationProfile(client)
	case err != nil:
		return AutomationProfileView{}, err
	default:
		out = automationProfileView(row, client)
	}
	contextView, err := s.loadBusinessContext(ctx, accountID, client.ID)
	if err != nil {
		return AutomationProfileView{}, err
	}
	out.StrategicContext = &contextView
	return out, nil
}

func (s *AutomationService) PutProfile(ctx context.Context, accountID string, p auth.Principal, clientID string, in AutomationProfileInput) (AutomationProfileView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return AutomationProfileView{}, err
	}
	client, err := s.accessibleClient(ctx, p, clientID)
	if err != nil {
		return AutomationProfileView{}, err
	}
	instanceID := strings.TrimSpace(in.WhatsAppInstanceID)
	agentID := strings.TrimSpace(in.AIAgentID)
	if !omnichannelUUIDPattern.MatchString(instanceID) || !omnichannelUUIDPattern.MatchString(agentID) {
		return AutomationProfileView{}, ErrValidation
	}
	readiness, err := s.store.AutomationBindingReadiness(ctx, accountID, instanceID, agentID)
	if err != nil {
		return AutomationProfileView{}, err
	}
	if !readiness.InstanceFound || !readiness.AgentFound {
		return AutomationProfileView{}, ErrNotFound
	}
	if in.Enabled && (!readiness.InstanceReady || !readiness.AgentReady) {
		return AutomationProfileView{}, ErrAutomationNotReady
	}
	policy, err := normalizeAutomationClosePolicy(in.ClosePolicy)
	if err != nil {
		return AutomationProfileView{}, err
	}
	row, err := s.store.UpsertAutomationProfile(ctx, accountID, client.ID, p.UserID, automationProfileWrite{
		WhatsAppInstanceID: instanceID, AIAgentID: agentID, Enabled: in.Enabled,
		AutoCloseEnabled: policy.AutoCloseEnabled, AutoCloseMinConfidence: policy.MinimumConfidence,
		AutoCloseRequireAllFields:  policy.RequireAllRequiredFields,
		AutoCloseBlockHumanRequest: policy.BlockOnHumanRequest,
		AutoCloseBlockSensitive:    policy.BlockSensitiveTopics,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return AutomationProfileView{}, ErrConflict
		}
		return AutomationProfileView{}, translate(err)
	}
	out := automationProfileView(row, client)
	contextView, err := s.loadBusinessContext(ctx, accountID, client.ID)
	if err != nil {
		return AutomationProfileView{}, err
	}
	out.StrategicContext = &contextView
	return out, nil
}

func (s *AutomationService) requireManage(ctx context.Context, accountID string, p auth.Principal) error {
	if s.permissions == nil {
		return ErrForbidden
	}
	return s.permissions.requirePermission(ctx, accountID, p, "omnichannel.settings.manage")
}

func (s *AutomationService) accessibleClients(ctx context.Context, p auth.Principal) ([]AutomationClientRef, error) {
	if s.clients == nil {
		return []AutomationClientRef{}, nil
	}
	return s.clients.ListAccessible(ctx, p)
}

func (s *AutomationService) accessibleClient(ctx context.Context, p auth.Principal, clientID string) (AutomationClientRef, error) {
	clientID = strings.TrimSpace(clientID)
	if !omnichannelUUIDPattern.MatchString(clientID) {
		return AutomationClientRef{}, ErrNotFound
	}
	clients, err := s.accessibleClients(ctx, p)
	if err != nil {
		return AutomationClientRef{}, err
	}
	for _, client := range clients {
		if strings.EqualFold(client.ID, clientID) {
			return client, nil
		}
	}
	return AutomationClientRef{}, ErrNotFound
}

func (s *AutomationService) loadBusinessContext(ctx context.Context, accountID, clientID string) (AutomationBusinessContextView, error) {
	out := AutomationBusinessContextView{Source: "calendar.client_profiles", Profile: AutomationBusinessContext{ClientID: clientID}}
	if s.businessContext == nil {
		return out, nil
	}
	profile, available, err := s.businessContext.Load(ctx, accountID, clientID)
	if err != nil {
		return AutomationBusinessContextView{}, err
	}
	if strings.TrimSpace(profile.ClientID) == "" {
		profile.ClientID = clientID
	}
	out.Available = available
	out.Profile = profile
	out.Filled = businessContextFilled(profile)
	return out, nil
}

func businessContextFilled(p AutomationBusinessContext) bool {
	values := []string{p.Segment, p.Positioning, p.Description, p.History, p.SiteURL,
		p.Instagram, p.Address, p.Objectives, p.BrandVoice, p.Extra.Audience, p.Extra.Offer,
		p.Extra.Pillars, p.Extra.Cadence, p.Extra.Restrictions, p.Extra.Performance, p.Extra.Assets}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func defaultAutomationClosePolicy() AutomationClosePolicyView {
	return AutomationClosePolicyView{
		MinimumConfidence: defaultAutoCloseMinConfidence, RequireAllRequiredFields: true,
		BlockOnHumanRequest: true, BlockSensitiveTopics: true, ValidGenerationRequired: true,
	}
}

func normalizeAutomationClosePolicy(in *AutomationClosePolicyInput) (AutomationClosePolicyView, error) {
	out := defaultAutomationClosePolicy()
	if in == nil {
		return out, nil
	}
	out.AutoCloseEnabled = in.AutoCloseEnabled
	if in.MinimumConfidence != nil {
		out.MinimumConfidence = *in.MinimumConfidence
	}
	if in.RequireAllRequiredFields != nil {
		out.RequireAllRequiredFields = *in.RequireAllRequiredFields
	}
	if in.BlockOnHumanRequest != nil {
		out.BlockOnHumanRequest = *in.BlockOnHumanRequest
	}
	if in.BlockSensitiveTopics != nil {
		out.BlockSensitiveTopics = *in.BlockSensitiveTopics
	}
	if out.MinimumConfidence < 0 || out.MinimumConfidence > 1 {
		return AutomationClosePolicyView{}, ErrValidation
	}
	return out, nil
}

func emptyAutomationProfile(client AutomationClientRef) AutomationProfileView {
	return AutomationProfileView{Client: client, ReadinessIssues: []string{"not_configured"}, ClosePolicy: defaultAutomationClosePolicy()}
}

func automationProfileView(row automationProfileRow, client AutomationClientRef) AutomationProfileView {
	issues := make([]string, 0, 3)
	if !row.Enabled {
		issues = append(issues, "automation_disabled")
	}
	if !row.InstanceActive {
		issues = append(issues, "instance_inactive")
	}
	if !row.AgentReady {
		issues = append(issues, "agent_not_ready")
	}
	createdAt, updatedAt := row.CreatedAt, row.UpdatedAt
	return AutomationProfileView{
		ID: row.ID, Configured: true, Client: client, Enabled: row.Enabled,
		Ready: row.Enabled && row.InstanceActive && row.AgentReady, ReadinessIssues: issues,
		WhatsAppInstance: &AutomationInstanceView{ID: row.WhatsAppInstanceID, InstanceName: row.InstanceName,
			Provider: row.InstanceProvider, DisplayName: row.InstanceDisplayName,
			PhoneNumber: row.InstancePhoneNumber, Active: row.InstanceActive},
		AIAgent: &AutomationAgentView{ID: row.AIAgentID, Name: row.AgentName,
			Enabled: row.AgentEnabled, ActiveVersionID: row.AgentActiveVersionID},
		ClosePolicy: AutomationClosePolicyView{AutoCloseEnabled: row.AutoCloseEnabled,
			MinimumConfidence:        row.AutoCloseMinConfidence,
			RequireAllRequiredFields: row.AutoCloseRequireAllFields,
			BlockOnHumanRequest:      row.AutoCloseBlockHumanRequest,
			BlockSensitiveTopics:     row.AutoCloseBlockSensitive,
			ValidGenerationRequired:  true},
		Revision: row.Revision, CreatedAt: &createdAt, UpdatedAt: &updatedAt,
	}
}
