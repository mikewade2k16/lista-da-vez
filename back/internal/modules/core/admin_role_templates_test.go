package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// admin_role_templates_test.go — cobre (a) o CONTRATO das queries (fragmentos
// SQL: schema-qualificado, parametrizado, criterio de scope/deprecated) e (b) a
// LOGICA do service contra um fake do RoleTemplateAdminRepository (validacao de
// id/perms, bloqueio de is_system, replace de permissoes). Sem Postgres — mesmo
// padrao de admin_users_test.go / store_postgres_test.go.

// ============================================================================
// Contrato SQL
// ============================================================================

func TestRoleTemplateQueriesAreSchemaQualifiedAndParameterized(t *testing.T) {
	checks := []struct {
		name     string
		query    string
		mustHave []string
	}{
		{
			name:  "list templates aggregates permission keys",
			query: listRoleTemplatesQuery,
			mustHave: []string{
				"from core.role_templates rt",
				"left join core.role_template_permissions rtp",
				"array_agg(rtp.permission_key order by rtp.permission_key)",
				"order by rt.sort_order asc, rt.id asc",
			},
		},
		{
			name:  "available permissions excludes platform + deprecated",
			query: listAvailablePermissionsQuery,
			mustHave: []string{
				"from core.permissions",
				"deprecated_at is null",
				"scope <> 'platform'",
			},
		},
		{
			name:  "find template by id is parameterized",
			query: findRoleTemplateQuery,
			mustHave: []string{
				"from core.role_templates",
				"where id = $1",
			},
		},
		{
			name:  "invalid keys checks catalog, deprecated and scope",
			query: invalidTemplatePermissionKeysQuery,
			mustHave: []string{
				"unnest($1::text[])",
				"from core.permissions p",
				"p.deprecated_at is null",
				"p.scope <> 'platform'",
			},
		},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			for _, frag := range c.mustHave {
				if !strings.Contains(c.query, frag) {
					t.Fatalf("query missing %q:\n%s", frag, c.query)
				}
			}
		})
	}
}

// ============================================================================
// Fake repository
// ============================================================================

type fakeRoleTemplateRepo struct {
	templates map[string]RoleTemplate
	available []AvailablePermission
	invalid   []string // keys que InvalidPermissionKeys devolve como invalidas

	createdInput *CreateRoleTemplateInput
	patchedID    string
	patchedInput *PatchRoleTemplateInput
	replacedID   string
	replacedKeys []string
	deletedID    string
}

func newFakeRepo() *fakeRoleTemplateRepo {
	return &fakeRoleTemplateRepo{templates: map[string]RoleTemplate{}}
}

func (f *fakeRoleTemplateRepo) ListRoleTemplates(_ context.Context) ([]RoleTemplate, error) {
	out := make([]RoleTemplate, 0, len(f.templates))
	for _, t := range f.templates {
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeRoleTemplateRepo) ListAvailablePermissions(_ context.Context) ([]AvailablePermission, error) {
	return f.available, nil
}

func (f *fakeRoleTemplateRepo) FindRoleTemplate(_ context.Context, id string) (RoleTemplate, error) {
	t, ok := f.templates[id]
	if !ok {
		return RoleTemplate{}, ErrTemplateNotFound
	}
	return t, nil
}

func (f *fakeRoleTemplateRepo) InvalidPermissionKeys(_ context.Context, _ []string) ([]string, error) {
	return f.invalid, nil
}

func (f *fakeRoleTemplateRepo) CreateRoleTemplate(_ context.Context, in CreateRoleTemplateInput) (RoleTemplate, error) {
	if _, exists := f.templates[in.ID]; exists {
		return RoleTemplate{}, ErrRoleTemplateConflict
	}
	f.createdInput = &in
	t := RoleTemplate{
		ID: in.ID, ModuleID: roleTemplateCustomModuleID, Label: in.Label,
		Description: in.Description, IsSystem: false, IsLocked: false,
		SortOrder: roleTemplateCustomSortOrder, PermissionKeys: in.PermissionKeys,
	}
	f.templates[in.ID] = t
	return t, nil
}

func (f *fakeRoleTemplateRepo) PatchRoleTemplate(_ context.Context, id string, in PatchRoleTemplateInput) (RoleTemplate, error) {
	t, ok := f.templates[id]
	if !ok {
		return RoleTemplate{}, ErrTemplateNotFound
	}
	f.patchedID = id
	f.patchedInput = &in
	if in.Label != nil {
		t.Label = *in.Label
	}
	return t, nil
}

func (f *fakeRoleTemplateRepo) ReplaceTemplatePermissions(_ context.Context, id string, keys []string) (RoleTemplate, error) {
	t, ok := f.templates[id]
	if !ok {
		return RoleTemplate{}, ErrTemplateNotFound
	}
	f.replacedID = id
	f.replacedKeys = keys
	t.PermissionKeys = keys
	return t, nil
}

func (f *fakeRoleTemplateRepo) DeleteRoleTemplate(_ context.Context, id string) error {
	if _, ok := f.templates[id]; !ok {
		return ErrTemplateNotFound
	}
	f.deletedID = id
	delete(f.templates, id)
	return nil
}

// ============================================================================
// Service: Create
// ============================================================================

func TestCreateRoleTemplateValidatesAndFixesSystemFields(t *testing.T) {
	repo := newFakeRepo()
	svc := NewRoleTemplateAdminService(repo)

	got, err := svc.Create(context.Background(), CreateRoleTemplateInput{
		ID:             "  custom.suporte  ",
		Label:          "  Suporte  ",
		PermissionKeys: []string{"workspace.operacao.view", "workspace.operacao.view", " "},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.IsSystem {
		t.Fatal("created template must be is_system=false")
	}
	if got.ModuleID != "core" || got.SortOrder != 200 {
		t.Fatalf("expected module core/sort 200, got %s/%d", got.ModuleID, got.SortOrder)
	}
	if repo.createdInput.ID != "custom.suporte" || repo.createdInput.Label != "Suporte" {
		t.Fatalf("expected trimmed id/label, got %q/%q", repo.createdInput.ID, repo.createdInput.Label)
	}
	// dedup + trim: 1 key valida.
	if len(repo.createdInput.PermissionKeys) != 1 || repo.createdInput.PermissionKeys[0] != "workspace.operacao.view" {
		t.Fatalf("expected deduped single key, got %v", repo.createdInput.PermissionKeys)
	}
}

func TestCreateRoleTemplateRejectsBadID(t *testing.T) {
	svc := NewRoleTemplateAdminService(newFakeRepo())
	for _, bad := range []string{"Custom.Role", "has space", "emoji_x!", ""} {
		_, err := svc.Create(context.Background(), CreateRoleTemplateInput{ID: bad, Label: "x"})
		if !errors.Is(err, ErrRoleTemplateInvalidID) {
			t.Fatalf("id %q: expected ErrRoleTemplateInvalidID, got %v", bad, err)
		}
	}
}

func TestCreateRoleTemplateRequiresLabel(t *testing.T) {
	svc := NewRoleTemplateAdminService(newFakeRepo())
	_, err := svc.Create(context.Background(), CreateRoleTemplateInput{ID: "custom.x", Label: "   "})
	if !errors.Is(err, ErrRoleTemplateLabelRequired) {
		t.Fatalf("expected ErrRoleTemplateLabelRequired, got %v", err)
	}
}

func TestCreateRoleTemplateRejectsInvalidPermissions(t *testing.T) {
	repo := newFakeRepo()
	repo.invalid = []string{"workspace.bogus.view"}
	svc := NewRoleTemplateAdminService(repo)
	_, err := svc.Create(context.Background(), CreateRoleTemplateInput{
		ID: "custom.x", Label: "X", PermissionKeys: []string{"workspace.bogus.view"},
	})
	if !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("expected ErrInvalidPermission, got %v", err)
	}
	if repo.createdInput != nil {
		t.Fatal("must not reach repo.Create when permissions are invalid")
	}
}

func TestCreateRoleTemplateConflict(t *testing.T) {
	repo := newFakeRepo()
	repo.templates["custom.x"] = RoleTemplate{ID: "custom.x"}
	svc := NewRoleTemplateAdminService(repo)
	_, err := svc.Create(context.Background(), CreateRoleTemplateInput{ID: "custom.x", Label: "X"})
	if !errors.Is(err, ErrRoleTemplateConflict) {
		t.Fatalf("expected ErrRoleTemplateConflict, got %v", err)
	}
}

// ============================================================================
// Service: bloqueio de is_system em patch/replace/delete
// ============================================================================

func TestSystemTemplateCannotBePatchedReplacedOrDeleted(t *testing.T) {
	repo := newFakeRepo()
	repo.templates["core.owner"] = RoleTemplate{ID: "core.owner", IsSystem: true}
	svc := NewRoleTemplateAdminService(repo)

	label := "novo"
	if _, err := svc.Patch(context.Background(), "core.owner", PatchRoleTemplateInput{Label: &label}); !errors.Is(err, ErrRoleTemplateSystem) {
		t.Fatalf("patch system: expected ErrRoleTemplateSystem, got %v", err)
	}
	if _, err := svc.ReplacePermissions(context.Background(), "core.owner", []string{"workspace.operacao.view"}); !errors.Is(err, ErrRoleTemplateSystem) {
		t.Fatalf("replace system: expected ErrRoleTemplateSystem, got %v", err)
	}
	if err := svc.Delete(context.Background(), "core.owner"); !errors.Is(err, ErrRoleTemplateSystem) {
		t.Fatalf("delete system: expected ErrRoleTemplateSystem, got %v", err)
	}
	// Nenhuma mutacao deve ter alcancado o repo.
	if repo.patchedInput != nil || repo.replacedID != "" || repo.deletedID != "" {
		t.Fatal("system template reached a mutating repo method")
	}
}

func TestPatchReplaceDeleteNotFound(t *testing.T) {
	svc := NewRoleTemplateAdminService(newFakeRepo())
	if _, err := svc.Patch(context.Background(), "ghost", PatchRoleTemplateInput{}); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}
	if err := svc.Delete(context.Background(), "ghost"); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}
}

// ============================================================================
// Service: replace de permissoes em template custom
// ============================================================================

func TestReplacePermissionsOnCustomTemplate(t *testing.T) {
	repo := newFakeRepo()
	repo.templates["custom.x"] = RoleTemplate{ID: "custom.x", IsSystem: false}
	svc := NewRoleTemplateAdminService(repo)

	got, err := svc.ReplacePermissions(context.Background(), "custom.x",
		[]string{"workspace.operacao.view", "workspace.operacao.edit", "workspace.operacao.view"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.replacedID != "custom.x" {
		t.Fatalf("expected replace on custom.x, got %q", repo.replacedID)
	}
	// dedup: 2 keys unicas.
	if len(repo.replacedKeys) != 2 {
		t.Fatalf("expected 2 deduped keys, got %v", repo.replacedKeys)
	}
	if len(got.PermissionKeys) != 2 {
		t.Fatalf("response should carry replaced keys, got %v", got.PermissionKeys)
	}
}

func TestReplacePermissionsRejectsInvalidKeys(t *testing.T) {
	repo := newFakeRepo()
	repo.templates["custom.x"] = RoleTemplate{ID: "custom.x"}
	repo.invalid = []string{"core.account.view"} // platform/invalid for templates
	svc := NewRoleTemplateAdminService(repo)
	_, err := svc.ReplacePermissions(context.Background(), "custom.x", []string{"core.account.view"})
	if !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("expected ErrInvalidPermission, got %v", err)
	}
	if repo.replacedID != "" {
		t.Fatal("must not replace when keys are invalid")
	}
}

// ============================================================================
// Service: List devolve templates + available
// ============================================================================

func TestListReturnsTemplatesAndAvailable(t *testing.T) {
	repo := newFakeRepo()
	repo.templates["custom.x"] = RoleTemplate{ID: "custom.x", PermissionKeys: []string{"workspace.operacao.view"}}
	repo.available = []AvailablePermission{
		{Key: "workspace.operacao.view", Label: "Ver Operacao", ModuleID: "core", Scope: "account"},
	}
	svc := NewRoleTemplateAdminService(repo)

	resp, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Templates) != 1 || resp.Templates[0].ID != "custom.x" {
		t.Fatalf("expected 1 template custom.x, got %+v", resp.Templates)
	}
	if len(resp.Available) != 1 || resp.Available[0].Key != "workspace.operacao.view" {
		t.Fatalf("expected available catalog, got %+v", resp.Available)
	}
}

// ============================================================================
// Charset do id
// ============================================================================

func TestRoleTemplateIDCharset(t *testing.T) {
	valid := []string{"queue.owner", "custom_role", "a-b.c_d", "x1", "store.terminal-2"}
	for _, v := range valid {
		if !isValidRoleTemplateID(v) {
			t.Fatalf("expected %q valid", v)
		}
	}
	invalid := []string{"", "Queue.Owner", "has space", "weird/slash", "tab\tx", "a:b"}
	for _, v := range invalid {
		if isValidRoleTemplateID(v) {
			t.Fatalf("expected %q invalid", v)
		}
	}
}
