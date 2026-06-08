package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPathHasSegmentPrefix(t *testing.T) {
	cases := []struct {
		path   string
		prefix string
		want   bool
	}{
		{"/v1/tasks", "/v1/tasks", true},
		{"/v1/tasks/123", "/v1/tasks", true},
		{"/v1/task-boards", "/v1/tasks", false}, // limite de segmento: nao casa
		{"/v1/tasksx", "/v1/tasks", false},
		{"/v1/erp/sync", "/v1/erp", true},
		{"/v1/admin/accounts", "/v1/erp", false},
	}
	for _, c := range cases {
		if got := pathHasSegmentPrefix(c.path, c.prefix); got != c.want {
			t.Errorf("pathHasSegmentPrefix(%q,%q)=%v want %v", c.path, c.prefix, got, c.want)
		}
	}
}

func TestMatchModuleRule_LongestWins(t *testing.T) {
	rules := []ModulePathRule{
		{Prefix: "/v1/tasks", ModuleID: "tasks"},
		{Prefix: "/v1/task-boards", ModuleID: "tasks"},
		{Prefix: "/v1/erp", ModuleID: "crm"},
	}
	if got := matchModuleRule(rules, "/v1/erp/sync"); got != "crm" {
		t.Errorf("got %q want crm", got)
	}
	if got := matchModuleRule(rules, "/v1/task-boards/1"); got != "tasks" {
		t.Errorf("got %q want tasks", got)
	}
	if got := matchModuleRule(rules, "/v1/admin/accounts"); got != "" {
		t.Errorf("got %q want empty (rota nao listada)", got)
	}
}

func TestRequireModuleByPath(t *testing.T) {
	guard := NewAccountModulesGuard(nil)
	// Semeia o cache (evita DB): account "acc-crm" tem crm habilitado, tasks nao.
	guard.store("acc-crm", map[string]struct{}{"crm": {}})

	rules := []ModulePathRule{
		{Prefix: "/v1/erp", ModuleID: "crm"},
		{Prefix: "/v1/tasks", ModuleID: "tasks"},
	}
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := guard.RequireModuleByPath(rules)(okHandler)

	do := func(path, accountID string) int {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		if accountID != "" {
			req.Header.Set("X-Account-Id", accountID)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do("/v1/erp/sync", "acc-crm"); code != http.StatusOK {
		t.Errorf("crm habilitado: got %d want 200", code)
	}
	if code := do("/v1/tasks/1", "acc-crm"); code != http.StatusForbidden {
		t.Errorf("tasks desabilitado: got %d want 403", code)
	}
	if code := do("/v1/erp/sync", ""); code != http.StatusBadRequest {
		t.Errorf("sem X-Account-Id: got %d want 400", code)
	}
	if code := do("/v1/admin/accounts", ""); code != http.StatusOK {
		t.Errorf("rota nao listada deve passar: got %d want 200", code)
	}
}

// okStatusHandler responde 200 — usado para confirmar que o request passou pelo guard.
func okStatusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// requestStatus dispara um GET path com (opcional) X-Account-Id e devolve o status.
func requestStatus(handler http.Handler, path, accountID string) int {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	if accountID != "" {
		req.Header.Set("X-Account-Id", accountID)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code
}

// ceilingRules e o gate-list usado nos testes de teto (espelha moduleGatingRules do app).
func ceilingRules() []ModulePathRule {
	return []ModulePathRule{
		{Prefix: "/v1/operations", ModuleID: "queue"},
		{Prefix: "/v1/erp", ModuleID: "crm"},
		{Prefix: "/v1/tasks", ModuleID: "tasks"},
	}
}

// TestAccountModuleCeiling_Matrix codifica a regra de TETO: o modulo contratado
// pela ACCOUNT e o limite de acesso — independe de qual usuario/papel faz o
// request (todos os usuarios de uma account mandam o mesmo X-Account-Id). Papel
// nunca contorna porque o guard roda ANTES do handler no Chain: modulo off => 403
// antes de qualquer checagem de permissao de papel.
//
// Espelha a matriz acordada (casos 1, 2, 4, 7).
func TestAccountModuleCeiling_Matrix(t *testing.T) {
	guard := NewAccountModulesGuard(nil)
	// Perola contratou queue+tasks (NAO crm). Duby contratou tasks+crm (NAO queue).
	guard.store("perola", map[string]struct{}{"queue": {}, "tasks": {}})
	guard.store("duby", map[string]struct{}{"tasks": {}, "crm": {}})

	handler := guard.RequireModuleByPath(ceilingRules())(okStatusHandler())

	cases := []struct {
		name      string
		path      string
		accountID string
		want      int
	}{
		{"perola acessa queue contratado", "/v1/operations/list", "perola", http.StatusOK},
		{"perola acessa tasks contratado", "/v1/tasks/1", "perola", http.StatusOK},
		{"perola NAO contratou crm -> teto bloqueia (qualquer papel)", "/v1/erp/dashboard", "perola", http.StatusForbidden},
		{"duby acessa crm contratado", "/v1/erp/dashboard", "duby", http.StatusOK},
		{"duby NAO contratou queue -> teto bloqueia", "/v1/operations/list", "duby", http.StatusForbidden},
		{"rota gateada sem X-Account-Id", "/v1/erp/dashboard", "", http.StatusBadRequest},
		{"rota de gestao (nao gateada) passa", "/v1/admin/accounts", "perola", http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := requestStatus(handler, c.path, c.accountID); got != c.want {
				t.Errorf("got %d want %d", got, c.want)
			}
		})
	}
}

// TestAccountModuleCeiling_CrossAccountUsesHeader garante que o teto e por
// account do header — um usuario que manda X-Account-Id de outra account so
// passa se ESSA account tiver o modulo (o membership e validado a parte pelo
// RequireAuthWithAccount; aqui validamos a dimensao de modulo).
func TestAccountModuleCeiling_CrossAccountUsesHeader(t *testing.T) {
	guard := NewAccountModulesGuard(nil)
	guard.store("perola", map[string]struct{}{"crm": {}})
	guard.store("duby", map[string]struct{}{}) // Duby sem nenhum modulo

	handler := guard.RequireModuleByPath(ceilingRules())(okStatusHandler())

	if code := requestStatus(handler, "/v1/erp/x", "perola"); code != http.StatusOK {
		t.Errorf("perola tem crm: got %d want 200", code)
	}
	if code := requestStatus(handler, "/v1/erp/x", "duby"); code != http.StatusForbidden {
		t.Errorf("duby nao tem crm: got %d want 403", code)
	}
}

// TestAccountModuleCeiling_CascadeOnDisable prova a cascata: ao desabilitar o
// modulo da account e invalidar o cache (como faz o handler de
// account.modules.changed), TODOS os requests daquela account passam a 403 na
// hora — sem esperar o TTL.
func TestAccountModuleCeiling_CascadeOnDisable(t *testing.T) {
	guard := NewAccountModulesGuard(nil)
	guard.store("perola", map[string]struct{}{"queue": {}, "crm": {}})

	handler := guard.RequireModuleByPath(ceilingRules())(okStatusHandler())

	if code := requestStatus(handler, "/v1/erp/x", "perola"); code != http.StatusOK {
		t.Fatalf("crm habilitado: got %d want 200", code)
	}

	// Admin desabilita crm: o evento account.modules.changed limpa o cache
	// (Invalidate); o proximo lookup recarrega do banco — aqui simulamos a recarga
	// com o novo estado (sem crm) via store apos o Invalidate, ja que o pool e nil.
	guard.Invalidate("perola")
	guard.store("perola", map[string]struct{}{"queue": {}})

	if code := requestStatus(handler, "/v1/erp/x", "perola"); code != http.StatusForbidden {
		t.Errorf("crm desabilitado + invalidate -> 403 na hora: got %d want 403", code)
	}
	// queue continua valendo.
	if code := requestStatus(handler, "/v1/operations/x", "perola"); code != http.StatusOK {
		t.Errorf("queue segue habilitado: got %d want 200", code)
	}
}
