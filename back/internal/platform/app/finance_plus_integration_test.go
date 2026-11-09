package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/core"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/knowledge"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type plusAuthFixture struct{ auth.UserRepository }

func (plusAuthFixture) LoadUserForAuth(_ context.Context, id string) (auth.User, error) {
	return auth.User{ID: id, Email: "synthetic@plus.test", Role: auth.RoleDirector, Active: true}, nil
}
func TestPlusHTTPAccountingAndFinanceWithoutModel(t *testing.T) {
	url := os.Getenv("PLUS_APP_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable plus_test_app database")
	}
	ctx := context.Background()
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if !strings.HasPrefix(p.Config().ConnConfig.Database, "plus_test_app") {
		t.Fatal("refusing non-disposable database")
	}
	_, e = p.Exec(ctx, `create schema if not exists core;
 create table if not exists core.accounts(id uuid primary key default gen_random_uuid(),name text not null default '',is_active boolean not null default true);
 create table if not exists core.users(id uuid primary key default gen_random_uuid(),email text,display_name text,is_active boolean not null default true,is_platform_admin boolean not null default false);
 create table if not exists core.account_users(account_id uuid,user_id uuid,is_active boolean not null default true);
 create table if not exists core.account_modules(account_id uuid,module_id text,enabled boolean);
 create table if not exists core.permissions(key text primary key,deprecated_at timestamptz);
 create table if not exists core.user_role_assignments(account_id uuid,user_id uuid,role_id uuid);
 create table if not exists core.roles(id uuid primary key,code text,cloned_from_template_id text);
 create table if not exists core.role_permissions(role_id uuid,permission_key text);
 create table if not exists core.user_permission_overrides(account_id uuid,user_id uuid,permission_key text,effect text,is_active boolean);`)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"0187_finance_module.sql", "0191_calendar_chat_conversations.sql", "0287_calendar_chat_proposal_executions.sql"} {
		plusApply(t, p, name)
	}
	if _, e = p.Exec(ctx, `alter table calendar.chat_conversations add column if not exists entry_surface text not null default 'calendar';`); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"0304_finance_user_fiscal_profile.sql", "0305_finance_assistant.sql", "0306_finance_document_drafts.sql", "0307_finance_document_reviews.sql", "0308_finance_access_conversation.sql", "0309_finance_portal_sessions.sql", "0310_accounting_foundation.sql", "0311_assistant_plus.sql", "0317_public_knowledge_conversation.sql"} {
		plusApply(t, p, name)
	}
	if _, e = p.Exec(ctx, `create schema if not exists messaging;create table if not exists messaging.ai_agents(id uuid primary key default gen_random_uuid(),account_id uuid,unique(account_id,id))`); e != nil {
		t.Fatal(e)
	}
	corpus, e := os.ReadFile("../database/migrations/0222_messaging_ai_tools_knowledge.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, string(corpus[strings.Index(string(corpus), "create table if not exists messaging.knowledge_bases"):])); e != nil {
		t.Fatal(e)
	}
	plusApply(t, p, "0312_accounting_knowledge.sql")
	plusApply(t, p, "0313_finance_plus_llm_runs.sql")
	var account, user, other string
	for _, v := range []struct {
		sql    string
		target *string
	}{{`insert into core.accounts default values returning id::text`, &account}, {`insert into core.users default values returning id::text`, &user}, {`insert into core.users default values returning id::text`, &other}} {
		if e = p.QueryRow(ctx, v.sql).Scan(v.target); e != nil {
			t.Fatal(e)
		}
	}
	for _, actor := range []string{user, other} {
		if _, e = p.Exec(ctx, `insert into core.account_users(account_id,user_id) values($1::uuid,$2::uuid)`, account, actor); e != nil {
			t.Fatal(e)
		}
	}
	for _, module := range []string{"finance", "accounting"} {
		if _, e = p.Exec(ctx, `insert into core.account_modules(account_id,module_id,enabled) values($1::uuid,$2,true)`, account, module); e != nil {
			t.Fatal(e)
		}
	}
	permissions := []string{"finance.sheets.view", "accounting.issuers.view", "accounting.issuers.manage", "accounting.documents.view", "accounting.documents.manage", "accounting.knowledge.view", "accounting.knowledge.manage"}
	for _, permission := range permissions {
		if _, e = p.Exec(ctx, `insert into core.permissions(key) values($1) on conflict do nothing`, permission); e != nil {
			t.Fatal(e)
		}
		for _, actor := range []string{user, other} {
			if _, e = p.Exec(ctx, `insert into core.user_permission_overrides(account_id,user_id,permission_key,effect,is_active) values($1::uuid,$2::uuid,$3,'allow',true)`, account, actor, permission); e != nil {
				t.Fatal(e)
			}
		}
	}
	checker, catalog := auth.NewPostgresAccountMemberChecker(p), core.NewModuleAccess(p)
	am := accounting.New(accountingPolicy{checker: checker, catalog: catalog}).WithKnowledgePorts(knowledge.Ports{PutOriginal: func(context.Context, knowledge.Scope, string, knowledge.Import) (string, error) {
		return "00000000-0000-4000-8000-000000000001", nil
	}})
	modelCalls := 0
	fm := finance.New().WithAssistantResolver(func(_ context.Context, _ string, cfg finance.AssistantConfig) (finance.AssistantRuntime, error) {
		return finance.AssistantRuntime{Provider: "openai", Model: cfg.Model, APIKey: "synthetic-model-key"}, nil
	}).WithPlusPorts(finance.PlusPorts{LLM: plusAppLLMFixture{calls: &modelCalls}, Authorize: plusAuthorizer(checker, catalog), Accounting: accountingPlusPort(am), Knowledge: accountingKnowledgePlusPort(am), ValidateEvidence: accountingEvidenceValidator(am)})
	handle, e := fm.Build(modules.Dependencies{Pool: p})
	if e != nil {
		t.Fatal(e)
	}
	defer handle.Close()
	ah, e := am.Build(modules.Dependencies{Pool: p})
	if e != nil {
		t.Fatal(e)
	}
	defer ah.Close()
	if _, e = fm.Assistant().SaveConfig(ctx, account, finance.AssistantConfigInput{Config: finance.AssistantConfig{HistoryWindow: 5, Plus: finance.PlusConfig{Enabled: true, ToolPolicyVersion: 1}}}); e != nil {
		t.Fatal(e)
	}
	svc := calendar.NewService(calendar.NewStore(p), nil).WithPlus(calendarPlusPorts(fm), p)
	tokens := auth.NewHMACTokenManager("synthetic-plus-http-key", time.Hour)
	authSvc := auth.NewService(plusAuthFixture{}, nil, tokens, nil, nil, nil, nil)
	middleware := auth.NewMiddleware(authSvc)
	middleware.SetAccountChecker(checker)
	mux := http.NewServeMux()
	calendar.RegisterChatRoutes(mux, svc, middleware)
	send := func(actor, key string, body map[string]any) (int, map[string]json.RawMessage) {
		t.Helper()
		session, e := tokens.Issue("", auth.User{ID: actor, Email: "synthetic@plus.test", Role: auth.RoleDirector, Active: true})
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := json.Marshal(body)
		r := httptest.NewRequestWithContext(ctx, "POST", "/v1/assistant/chat/ask", bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer "+session.AccessToken)
		r.Header.Set("X-Account-Id", account)
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		var out map[string]json.RawMessage
		if e = json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return w.Code, out
	}
	sc := accounting.Scope{AccountID: account, ActorID: user}
	issuer, e := am.Service().CreateIssuer(ctx, sc, "plus-issuer-key", accounting.Command{Profile: &accounting.Profile{LegalName: "Synthetic company"}})
	if e != nil {
		t.Fatal(e)
	}
	var ir struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(issuer, &ir)
	sc.IssuerID = ir.ID
	diagnosis, e := am.Service().SaveDiagnosis(ctx, sc, "plus-diagnosis-key", accounting.DiagnosisInput{ExpectedProfileVersion: 1, Intent: "service", Answers: map[string]accounting.Answer{}})
	if e != nil {
		t.Fatal(e)
	}
	var dr struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(diagnosis, &dr)
	body := map[string]any{"mode": "plus", "surface": "finance", "contractVersion": "1.0", "question": "Preparar documento", "issuerId": sc.IssuerID, "action": map[string]any{"toolId": "accounting.document.prepare.v1", "input": map[string]any{"expectedVersion": 0, "diagnosisId": dr.ID, "expectedDiagnosisVersion": 1, "expectedProfileVersion": 1, "kind": "nfse", "fields": map[string]any{}}}}
	code, out := send(user, "plus-http-prepare", body)
	if code != 200 {
		t.Fatalf("prepare %d: %s", code, out["error"])
	}
	var result struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(out["data"], &result); e != nil || result.ID == "" {
		t.Fatal("missing persisted draft")
	}
	if code, _ = send(user, "plus-http-prepare", body); code != 200 {
		t.Fatal("replay failed")
	}
	var count int
	if e = p.QueryRow(ctx, `select count(*) from accounting.documents where account_id=$1::uuid and issuer_id=$2::uuid`, account, sc.IssuerID).Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate preparation")
	}
	if code, _ = send(other, "plus-other-issuer", body); code != 404 {
		t.Fatalf("same account leaked issuer: %d", code)
	}
	body["question"] = "Alterar preparação"
	if code, _ = send(user, "plus-http-prepare", body); code != 409 {
		t.Fatal("changed replay accepted")
	}
	var sheet string
	if e = p.QueryRow(ctx, `insert into finance.sheets(account_id,title,period) values($1::uuid,'Synthetic','2026-11') returning id::text`, account).Scan(&sheet); e != nil {
		t.Fatal(e)
	}
	read := map[string]any{"mode": "plus", "surface": "finance", "contractVersion": "1.0", "question": "resumo", "financeSheetId": sheet}
	if code, _ = send(user, "plus-http-summary", read); code != 200 {
		t.Fatalf("Finance without LLM failed: %d", code)
	}
	if e = p.QueryRow(ctx, `select count(*) from accounting.submission_authorizations where account_id=$1::uuid`, account).Scan(&count); e != nil || count != 0 {
		t.Fatal("preparation authorized issuance")
	}
	// CT-06: actual persisted corpus → neutral service → Accounting policy → Plus HTTP.
	ks := am.Service().Knowledge()
	kScope := knowledge.Scope{AccountID: account, ActorID: user, ResourceID: sc.IssuerID}
	base, e := ks.CreateBase(ctx, kScope, "Synthetic fiscal knowledge")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ks.SetBase(ctx, kScope, base.ID, base.Version, true); e != nil {
		t.Fatal(e)
	}
	doc, e := ks.Import(ctx, kScope, knowledge.Import{BaseID: base.ID, Title: "Synthetic source", SourceRef: "manual://synthetic/plus", FileName: "synthetic.txt", ContentType: "text/plain", Content: []byte("Fonte sintética competência serviço"), Metadata: knowledge.Metadata{Authority: "Synthetic", Jurisdiction: "unknown", Regime: "unknown", Classification: "fixture"}})
	if e != nil {
		t.Fatal(e)
	}
	doc, e = ks.Review(ctx, kScope, doc.ID, knowledge.Review{ExpectedVersion: doc.LockVersion, Metadata: doc.Metadata, Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ks.Publish(ctx, kScope, doc.ID, doc.LockVersion, true); e != nil {
		t.Fatal(e)
	}
	lookup := map[string]any{"mode": "plus", "surface": "finance", "contractVersion": "1.0", "issuerId": sc.IssuerID, "question": "buscar fonte competência"}
	code, out = send(user, "plus-http-knowledge", lookup)
	if code != 200 {
		t.Fatalf("knowledge HTTP: %d %s", code, out["error"])
	}
	var evidence []knowledge.Evidence
	if e = json.Unmarshal(out["evidence"], &evidence); e != nil || len(evidence) != 1 || evidence[0].DocumentID != doc.ID {
		t.Fatalf("missing scoped evidence: %v %+v", e, out)
	}

	cfg, e := fm.Assistant().Config(ctx, account)
	if e != nil {
		t.Fatal(e)
	}
	cfg.Plus.Explanation = finance.ExplanationConfig{Enabled: true, CredentialID: "00000000-0000-4000-8000-000000000005", Model: "synthetic-model", TaskClass: "source_summary", MaxInputBytes: 8000, MaxOutputTokens: 256, TimeoutMS: 1000, DailyRequests: 2}
	if _, e = fm.Assistant().SaveConfig(ctx, account, finance.AssistantConfigInput{Config: cfg, ExpectedUpdatedAt: cfg.UpdatedAt}); e != nil {
		t.Fatal(e)
	}
	explanation := map[string]any{"mode": "plus", "surface": "finance", "contractVersion": "1.0", "issuerId": sc.IssuerID, "question": "resuma fonte competência"}
	code, answer := send(user, "plus-http-explanation", explanation)
	if code != 200 || modelCalls != 1 {
		t.Fatalf("grounded explanation: %d calls=%d %s", code, modelCalls, answer["error"])
	}
	if code, _ = send(user, "plus-http-explanation", explanation); code != 200 || modelCalls != 1 {
		t.Fatal("paid replay called model twice")
	}
	var runCount int
	if e = p.QueryRow(ctx, `select count(*) from finance.plus_llm_runs where account_id=$1::uuid`, account).Scan(&runCount); e != nil || runCount != 1 {
		t.Fatal("run ledger replay duplicated", e)
	}
	if code, _ = send(other, "plus-http-knowledge-denied", lookup); code != 404 {
		t.Fatal("issuer policy bypassed", code)
	}
	if _, e = p.Exec(ctx, `update core.user_permission_overrides set is_active=false where account_id=$1::uuid and user_id=$2::uuid and permission_key='accounting.knowledge.view'`, account, user); e != nil {
		t.Fatal(e)
	}
	if code, _ = send(user, "plus-http-explanation", explanation); code != 403 {
		t.Fatal("replay leaked revoked evidence", code)
	}

	var conversation string
	if e = json.Unmarshal(answer["conversationId"], &conversation); e != nil {
		t.Fatal(e)
	}
	session, e := tokens.Issue("", auth.User{ID: user, Email: "synthetic@plus.test", Role: auth.RoleDirector, Active: true})
	if e != nil {
		t.Fatal(e)
	}
	historyRequest := httptest.NewRequestWithContext(ctx, "GET", "/v1/assistant/chat/conversations/"+conversation+"?mode=plus&issuerId="+sc.IssuerID, nil)
	historyRequest.Header.Set("Authorization", "Bearer "+session.AccessToken)
	historyRequest.Header.Set("X-Account-Id", account)
	historyResponse := httptest.NewRecorder()
	mux.ServeHTTP(historyResponse, historyRequest)
	if historyResponse.Code != 403 {
		t.Fatal("history leaked revoked source", historyResponse.Code)
	}
	if code, _ = send(user, "plus-http-knowledge-revoked", lookup); code != 403 {
		t.Fatal("knowledge permission revocation bypassed", code)
	}
}
func plusApply(t *testing.T, p *pgxpool.Pool, name string) {
	t.Helper()
	raw, e := os.ReadFile("../database/migrations/" + name)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(context.Background(), string(raw)); e != nil {
		t.Fatal(e)
	}
}
