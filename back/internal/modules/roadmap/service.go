package roadmap

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func defaultRoadmapPermissionsForRole(role auth.Role) []string {
	switch role {
	case auth.RolePlatformAdmin, auth.RoleOwner, auth.RoleDirector:
		return []string{PermRoadmapView, PermRoadmapManage}
	case auth.RoleManager, auth.RoleMarketing:
		return []string{PermRoadmapView}
	default:
		return nil
	}
}

func (service *Service) ResolveAccessContext(ctx context.Context, principal auth.Principal, accountID string) (AccessContext, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return AccessContext{}, ErrAccountRequired
	}

	exists, err := service.repository.AccountExists(ctx, accountID)
	if err != nil {
		return AccessContext{}, err
	}
	if !exists {
		return AccessContext{}, ErrAccountNotFound
	}

	isPlatformAdmin := principal.Role == auth.RolePlatformAdmin
	var permKeys []string
	if isPlatformAdmin {
		permKeys = []string{PermRoadmapView, PermRoadmapManage}
	} else {
		isMember, err := service.repository.IsAccountMember(ctx, accountID, principal.UserID)
		if err != nil {
			return AccessContext{}, err
		}
		if !isMember {
			return AccessContext{}, ErrAccountNotFound
		}

		permKeys, err = service.repository.ListPermissionsForUser(ctx, accountID, principal.UserID)
		if err != nil {
			return AccessContext{}, err
		}
		if len(permKeys) == 0 {
			permKeys = defaultRoadmapPermissionsForRole(principal.Role)
		}
	}

	permissions := make(map[string]struct{}, len(permKeys))
	for _, key := range permKeys {
		key = strings.TrimSpace(key)
		if key != "" {
			permissions[key] = struct{}{}
		}
	}

	return AccessContext{
		UserID:          strings.TrimSpace(principal.UserID),
		AccountID:       accountID,
		IsPlatformAdmin: isPlatformAdmin,
		Permissions:     permissions,
	}, nil
}

func (service *Service) ListModules(ctx context.Context, access AccessContext) ([]ModuleRecord, error) {
	if !access.Has(PermRoadmapView) {
		return nil, ErrForbidden
	}
	modules, err := service.repository.ListModules(ctx, access.AccountID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(modules, func(i, j int) bool {
		if modules[i].Priority != modules[j].Priority {
			return modules[i].Priority < modules[j].Priority
		}
		return modules[i].SortOrder < modules[j].SortOrder
	})
	return modules, nil
}

func (service *Service) CreateOrUpsertModule(ctx context.Context, access AccessContext, input UpsertModuleInput) (*ModuleRecord, error) {
	if !access.Has(PermRoadmapManage) {
		return nil, ErrForbidden
	}
	if err := validateModuleInput(input); err != nil {
		return nil, err
	}
	input = normalizeModuleInput(input)
	return service.repository.UpsertModuleForAccount(ctx, access.AccountID, input)
}

func (service *Service) UpdateModule(ctx context.Context, access AccessContext, id string, input UpsertModuleInput) (*ModuleRecord, error) {
	if !access.Has(PermRoadmapManage) {
		return nil, ErrForbidden
	}
	if err := validateModuleInput(input); err != nil {
		return nil, err
	}
	input = normalizeModuleInput(input)

	existing, err := service.repository.GetModule(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.IsGlobal {
		input.SourceID = existing.SourceID
		return service.repository.UpsertModuleForAccount(ctx, access.AccountID, input)
	}
	if existing.AccountID != nil && *existing.AccountID != access.AccountID {
		return nil, ErrForbidden
	}
	input.SourceID = existing.SourceID
	return service.repository.UpdateModule(ctx, id, input)
}

func (service *Service) DeleteModule(ctx context.Context, access AccessContext, id string) error {
	if !access.Has(PermRoadmapManage) {
		return ErrForbidden
	}
	existing, err := service.repository.GetModule(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsGlobal {
		return ErrCannotDeleteGlobal
	}
	if existing.AccountID != nil && *existing.AccountID != access.AccountID {
		return ErrForbidden
	}
	return service.repository.DeleteModule(ctx, id)
}

func (service *Service) ListRules(ctx context.Context, access AccessContext) ([]Rule, error) {
	if !access.Has(PermRoadmapView) {
		return nil, ErrForbidden
	}
	rules, err := service.repository.ListRules(ctx, access.AccountID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Category != rules[j].Category {
			return categoryOrder(rules[i].Category) < categoryOrder(rules[j].Category)
		}
		return rules[i].SortOrder < rules[j].SortOrder
	})
	return rules, nil
}

func (service *Service) CreateOrUpsertRule(ctx context.Context, access AccessContext, input UpsertRuleInput) (*Rule, error) {
	if !access.Has(PermRoadmapManage) {
		return nil, ErrForbidden
	}
	if err := validateRuleInput(input); err != nil {
		return nil, err
	}
	input = normalizeRuleInput(input)
	return service.repository.UpsertRuleForAccount(ctx, access.AccountID, input)
}

func (service *Service) UpdateRule(ctx context.Context, access AccessContext, id string, input UpsertRuleInput) (*Rule, error) {
	if !access.Has(PermRoadmapManage) {
		return nil, ErrForbidden
	}
	if err := validateRuleInput(input); err != nil {
		return nil, err
	}
	input = normalizeRuleInput(input)

	existing, err := service.repository.GetRule(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.IsGlobal {
		input.SourceID = existing.SourceID
		return service.repository.UpsertRuleForAccount(ctx, access.AccountID, input)
	}
	if existing.AccountID != nil && *existing.AccountID != access.AccountID {
		return nil, ErrForbidden
	}
	input.SourceID = existing.SourceID
	return service.repository.UpdateRule(ctx, id, input)
}

func (service *Service) DeleteRule(ctx context.Context, access AccessContext, id string) error {
	if !access.Has(PermRoadmapManage) {
		return ErrForbidden
	}
	existing, err := service.repository.GetRule(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsGlobal {
		return ErrCannotDeleteGlobal
	}
	if existing.AccountID != nil && *existing.AccountID != access.AccountID {
		return ErrForbidden
	}
	return service.repository.DeleteRule(ctx, id)
}

func (service *Service) Dashboard(ctx context.Context, access AccessContext) ([]DashboardModule, error) {
	if !access.Has(PermRoadmapView) {
		return nil, ErrForbidden
	}
	modules, err := service.repository.ListModules(ctx, access.AccountID)
	if err != nil {
		return nil, err
	}
	tasksByModule, err := service.repository.ListDashboardTasks(ctx, access.AccountID)
	if err != nil {
		return nil, err
	}

	out := make([]DashboardModule, 0, len(modules))
	for _, m := range modules {
		tasks := tasksByModule[m.ID]
		if tasks == nil {
			tasks = []DashboardTask{}
		}
		out = append(out, DashboardModule{
			Module: m,
			Tasks:  tasks,
			Counts: countDashboardTasks(tasks, m.Status),
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Module.Priority != out[j].Module.Priority {
			return out[i].Module.Priority < out[j].Module.Priority
		}
		return out[i].Module.SortOrder < out[j].Module.SortOrder
	})
	return out, nil
}

func countDashboardTasks(tasks []DashboardTask, _ string) DashboardCounts {
	counts := DashboardCounts{Total: len(tasks)}
	for _, t := range tasks {
		statusValue := normalizeDashboardTaskStatus(t.Status)
		switch statusValue {
		case "idea", "ideia":
			counts.Idea++
		case "planning", "planejamento", "todo", "backlog", "raw", "standby", "rotina":
			counts.Planning++
		case "in_progress", "doing", "em_andamento", "running", "execucao", "aguardando_aprovacao", "aprovada", "approved":
			counts.InProgress++
		case "concluida", "finalizada", "finished", "complete", "completed":
			counts.Done++
		case "done", "concluido", "concluído", "finalizado":
			counts.Done++
		default:
			counts.Planning++
		}
	}
	return counts
}

func normalizeDashboardTaskStatus(status *string) string {
	if status == nil {
		return ""
	}
	value := strings.ToLower(strings.TrimSpace(*status))
	replacer := strings.NewReplacer(
		" ", "_",
		"-", "_",
		"/", "_",
	)
	return replacer.Replace(value)
}

func (service *Service) ExportRulesMarkdown(ctx context.Context, access AccessContext) (string, error) {
	rules, err := service.ListRules(ctx, access)
	if err != nil {
		return "", err
	}
	return BuildMarkdown(rules), nil
}

func BuildMarkdown(rules []Rule) string {
	var sb strings.Builder
	sb.WriteString("# AGENT_RULES.md\n\n")
	sb.WriteString("Regras canonicas que todo agente/IA deve ler antes de iniciar qualquer tarefa neste projeto.\n\n")
	fmt.Fprintf(&sb, "Gerado em %s a partir do backend roadmap.\n\n", time.Now().UTC().Format(time.RFC3339))

	for _, cat := range categoryOrderList() {
		filtered := filterRulesByCategory(rules, cat)
		if len(filtered) == 0 {
			continue
		}
		sb.WriteString("## ")
		sb.WriteString(categoryLabel(cat))
		sb.WriteString("\n\n")
		for _, r := range filtered {
			sb.WriteString("### ")
			sb.WriteString(r.Title)
			sb.WriteString("\n")
			sb.WriteString(r.Body)
			sb.WriteString("\n\n")
			if r.Why != "" {
				sb.WriteString("- **Por que:** ")
				sb.WriteString(r.Why)
				sb.WriteString("\n")
			}
			if r.AppliesWhen != "" {
				sb.WriteString("- **Aplica quando:** ")
				sb.WriteString(r.AppliesWhen)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
		sb.WriteString("---\n\n")
	}
	return sb.String()
}

func filterRulesByCategory(rules []Rule, category string) []Rule {
	out := make([]Rule, 0)
	for _, r := range rules {
		if r.Category == category {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out
}

func categoryOrderList() []string {
	return []string{
		CategoryFrontend,
		CategoryBackend,
		CategoryBanco,
		CategoryLinguagens,
		CategoryDeploy,
		CategoryPadroesGerais,
	}
}

func categoryOrder(category string) int {
	for i, c := range categoryOrderList() {
		if c == category {
			return i
		}
	}
	return 999
}

func categoryLabel(category string) string {
	switch category {
	case CategoryFrontend:
		return "Frontend"
	case CategoryBackend:
		return "Backend"
	case CategoryBanco:
		return "Banco"
	case CategoryLinguagens:
		return "Linguagens"
	case CategoryDeploy:
		return "Deploy"
	case CategoryPadroesGerais:
		return "Padroes Gerais"
	default:
		return category
	}
}

func validateModuleInput(input UpsertModuleInput) error {
	if strings.TrimSpace(input.SourceID) == "" {
		return fmt.Errorf("%w: sourceId required", ErrInvalid)
	}
	if strings.TrimSpace(input.Label) == "" {
		return fmt.Errorf("%w: label required", ErrInvalid)
	}
	if !isValidStatus(input.Status) {
		return fmt.Errorf("%w: status invalid", ErrInvalid)
	}
	if !isValidPriority(input.Priority) {
		return fmt.Errorf("%w: priority invalid", ErrInvalid)
	}
	return nil
}

func validateRuleInput(input UpsertRuleInput) error {
	if strings.TrimSpace(input.SourceID) == "" {
		return fmt.Errorf("%w: sourceId required", ErrInvalid)
	}
	if strings.TrimSpace(input.Title) == "" {
		return fmt.Errorf("%w: title required", ErrInvalid)
	}
	if strings.TrimSpace(input.Body) == "" {
		return fmt.Errorf("%w: body required", ErrInvalid)
	}
	if !isValidCategory(input.Category) {
		return fmt.Errorf("%w: category invalid", ErrInvalid)
	}
	return nil
}

func isValidStatus(status string) bool {
	switch status {
	case StatusPending, StatusInProgress, StatusBeta, StatusDone:
		return true
	}
	return false
}

func isValidPriority(priority string) bool {
	switch priority {
	case PriorityP0, PriorityP1, PriorityP2, PriorityP3:
		return true
	}
	return false
}

func isValidCategory(category string) bool {
	for _, c := range categoryOrderList() {
		if c == category {
			return true
		}
	}
	return false
}

func normalizeModuleInput(input UpsertModuleInput) UpsertModuleInput {
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.Label = strings.TrimSpace(input.Label)
	input.Route = strings.TrimSpace(input.Route)
	input.Description = strings.TrimSpace(input.Description)
	input.Category = strings.TrimSpace(input.Category)
	input.Scope = normalizeStringSlice(input.Scope)
	input.DependsOn = normalizeStringSlice(input.DependsOn)
	if input.Scope == nil {
		input.Scope = []string{}
	}
	if input.DependsOn == nil {
		input.DependsOn = []string{}
	}
	return input
}

func normalizeRuleInput(input UpsertRuleInput) UpsertRuleInput {
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	input.Why = strings.TrimSpace(input.Why)
	input.AppliesWhen = strings.TrimSpace(input.AppliesWhen)
	input.Category = strings.TrimSpace(input.Category)
	return input
}
