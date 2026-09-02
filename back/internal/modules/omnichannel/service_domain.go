package omnichannel

import (
	"context"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// ============================================================================
// F8 — Service de config: CRUD de setores/filas/membros/regras (Contrato 6)
// ============================================================================
//
// Todo metodo recebe o accountID resolvido do Principal (nunca do body, principio 2) e
// exige `omnichannel.settings.manage` (feature gate => 403; escopo => 404). platform_admin
// passa mesmo com has()=false no front (armadilha da spec).

// ============================================================================
// Setores
// ============================================================================

func (s *Service) CreateDepartment(ctx context.Context, accountID string, p auth.Principal, in DepartmentInput) (DepartmentView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return DepartmentView{}, err
	}
	name := strings.TrimSpace(in.Name)
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	if name == "" || slug == "" {
		return DepartmentView{}, ErrValidation
	}
	d, err := s.store.CreateDepartment(ctx, accountID, slug, name, in.IsDefault)
	if isUniqueViolation(err) {
		return DepartmentView{}, ErrConflict
	}
	return d, err
}

func (s *Service) ListDepartments(ctx context.Context, accountID string, p auth.Principal) ([]DepartmentView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return nil, err
	}
	return s.store.ListDepartments(ctx, accountID)
}

func (s *Service) UpdateDepartment(ctx context.Context, accountID string, p auth.Principal, id string, patch DepartmentPatch) (DepartmentView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return DepartmentView{}, err
	}
	if patch.Name != nil && strings.TrimSpace(*patch.Name) == "" {
		return DepartmentView{}, ErrValidation
	}
	d, err := s.store.UpdateDepartment(ctx, accountID, id, patch)
	return d, translate(err)
}

func (s *Service) DeleteDepartment(ctx context.Context, accountID string, p auth.Principal, id string) error {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return err
	}
	return s.store.SoftDeleteDepartment(ctx, accountID, id)
}

// ============================================================================
// Filas
// ============================================================================

func (s *Service) CreateQueue(ctx context.Context, accountID string, p auth.Principal, in QueueInput) (QueueView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return QueueView{}, err
	}
	departmentID := strings.TrimSpace(in.DepartmentID)
	name := strings.TrimSpace(in.Name)
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	if departmentID == "" || name == "" || slug == "" {
		return QueueView{}, ErrValidation
	}
	// Setor de outra conta => 404 (FK cruzada validada contra a conta, spec Seguranca).
	ok, err := s.store.departmentExists(ctx, accountID, departmentID)
	if err != nil {
		return QueueView{}, err
	}
	if !ok {
		return QueueView{}, ErrNotFound
	}
	q, err := s.store.CreateQueue(ctx, accountID, departmentID, slug, name, in.IsDefault)
	if isUniqueViolation(err) {
		return QueueView{}, ErrConflict
	}
	return q, err
}

func (s *Service) ListQueues(ctx context.Context, accountID string, p auth.Principal, departmentID string) ([]QueueView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return nil, err
	}
	return s.store.ListQueues(ctx, accountID, strings.TrimSpace(departmentID))
}

func (s *Service) UpdateQueue(ctx context.Context, accountID string, p auth.Principal, id string, patch QueuePatch) (QueueView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return QueueView{}, err
	}
	if patch.Name != nil && strings.TrimSpace(*patch.Name) == "" {
		return QueueView{}, ErrValidation
	}
	q, err := s.store.UpdateQueue(ctx, accountID, id, patch)
	return q, translate(err)
}

func (s *Service) DeleteQueue(ctx context.Context, accountID string, p auth.Principal, id string) error {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return err
	}
	return s.store.SoftDeleteQueue(ctx, accountID, id)
}

// ============================================================================
// Membros da fila (gate de dado — POST/DELETE incremental, nao PUT de conjunto)
// ============================================================================

func (s *Service) ListQueueMembers(ctx context.Context, accountID string, p auth.Principal, queueID string) ([]QueueMemberView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return nil, err
	}
	if err := s.assertQueueInAccount(ctx, accountID, queueID); err != nil {
		return nil, err
	}
	return s.store.ListQueueMembers(ctx, accountID, queueID)
}

func (s *Service) AddQueueMember(ctx context.Context, accountID string, p auth.Principal, queueID string, in QueueMemberInput) (QueueMemberView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return QueueMemberView{}, err
	}
	userID := strings.TrimSpace(in.UserID)
	if userID == "" {
		return QueueMemberView{}, ErrValidation
	}
	if err := s.assertQueueInAccount(ctx, accountID, queueID); err != nil {
		return QueueMemberView{}, err
	}
	// Usuario nao-membro da conta => 404 (spec Contrato 6).
	ok, err := s.store.userInAccount(ctx, accountID, userID)
	if err != nil {
		return QueueMemberView{}, err
	}
	if !ok {
		return QueueMemberView{}, ErrNotFound
	}
	member, err := s.store.AddQueueMember(ctx, accountID, queueID, userID)
	if err != nil {
		return QueueMemberView{}, err
	}
	s.publisher.PublishOmnichannelEvent(ctx, newInvalidationSignal(
		accountID, RealtimeInvalidationReasonAccessScopeChanged, time.Now().UTC()))
	return member, nil
}

func (s *Service) RemoveQueueMember(ctx context.Context, accountID string, p auth.Principal, queueID, userID string) error {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return err
	}
	if err := s.assertQueueInAccount(ctx, accountID, queueID); err != nil {
		return err
	}
	if err := s.store.RemoveQueueMember(ctx, accountID, queueID, strings.TrimSpace(userID)); err != nil {
		return err
	}
	s.publisher.PublishOmnichannelEvent(ctx, newInvalidationSignal(
		accountID, RealtimeInvalidationReasonAccessScopeChanged, time.Now().UTC()))
	return nil
}

func (s *Service) assertQueueInAccount(ctx context.Context, accountID, queueID string) error {
	ok, err := s.store.queueInAccount(ctx, accountID, strings.TrimSpace(queueID))
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Regras de roteamento
// ============================================================================

func (s *Service) CreateRoutingRule(ctx context.Context, accountID string, p auth.Principal, in RoutingRuleInput) (RoutingRuleView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return RoutingRuleView{}, err
	}
	if strings.TrimSpace(in.Name) == "" || !validConditions(in.Conditions) {
		return RoutingRuleView{}, ErrValidation
	}
	if err := s.assertActiveQueue(ctx, accountID, strings.TrimSpace(in.TargetQueueID)); err != nil {
		return RoutingRuleView{}, err
	}
	if in.Priority == 0 {
		in.Priority = 100
	}
	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	return s.store.CreateRoutingRule(ctx, accountID, in, isActive)
}

func (s *Service) ListRoutingRules(ctx context.Context, accountID string, p auth.Principal) ([]RoutingRuleView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return nil, err
	}
	return s.store.ListRoutingRules(ctx, accountID)
}

func (s *Service) UpdateRoutingRule(ctx context.Context, accountID string, p auth.Principal, id string, patch RoutingRulePatch) (RoutingRuleView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return RoutingRuleView{}, err
	}
	if patch.Conditions != nil && !validConditions(*patch.Conditions) {
		return RoutingRuleView{}, ErrValidation
	}
	if patch.TargetQueueID != nil {
		if err := s.assertActiveQueue(ctx, accountID, strings.TrimSpace(*patch.TargetQueueID)); err != nil {
			return RoutingRuleView{}, err
		}
	}
	r, err := s.store.UpdateRoutingRule(ctx, accountID, id, patch)
	return r, translate(err)
}

func (s *Service) DeleteRoutingRule(ctx context.Context, accountID string, p auth.Principal, id string) error {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return err
	}
	return s.store.SoftDeleteRoutingRule(ctx, accountID, id)
}

func (s *Service) ReorderRoutingRules(ctx context.Context, accountID string, p auth.Principal, ruleIDs []string) error {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return err
	}
	if len(ruleIDs) == 0 {
		return ErrValidation
	}
	return s.store.ReorderRoutingRules(ctx, accountID, ruleIDs)
}

// assertActiveQueue: target de regra inativo/de outra conta => 404 (spec Contrato 6).
func (s *Service) assertActiveQueue(ctx context.Context, accountID, queueID string) error {
	if queueID == "" {
		return ErrValidation
	}
	ok, err := s.store.activeQueueInAccount(ctx, accountID, queueID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// validConditions valida o conjunto fechado de operadores e que todo field e nao-vazio.
func validConditions(conditions []Condition) bool {
	for _, c := range conditions {
		if strings.TrimSpace(c.Field) == "" || !validOp(c.Op) {
			return false
		}
	}
	return true
}
