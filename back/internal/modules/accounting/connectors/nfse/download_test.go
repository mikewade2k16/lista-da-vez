package nfse

import (
	"context"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type localUsers struct{ auth.UserRepository }

func (localUsers) LoadUserForAuth(_ context.Context, id string) (auth.User, error) {
	return auth.User{ID: id, Email: "synthetic@ct08.test", Active: true, Role: auth.RoleDirector}, nil
}

type localMembership struct{ account string }

func (m localMembership) IsMember(_ context.Context, account, _ string) (bool, error) {
	return account == m.account, nil
}

type privateReaderFunc func(context.Context, accounting.Scope, string) (io.ReadCloser, error)

func (f privateReaderFunc) OpenPrivate(ctx context.Context, sc accounting.Scope, id string) (io.ReadCloser, error) {
	return f(ctx, sc, id)
}
func TestPrivateArtifactHTTPAndRevocationDuringRead(t *testing.T) {
	s, sc, d, _, _, storage, p := motorFixture(t, "ok")
	raw, _ := queueDocument(t, s, sc, d)
	execution := field(t, raw, "executionId")
	if e := s.Handle(context.Background(), jobs.Job{AccountID: sc.AccountID, Payload: []byte(`{"executionId":"` + execution + `"}`)}); e != nil {
		t.Fatal(e)
	}
	var artifact string
	if e := p.QueryRow(context.Background(), `select id::text from accounting.artifacts where account_id=$1::uuid and issuer_id=$2::uuid and execution_id=$3::uuid and kind='xml'`, sc.AccountID, sc.IssuerID, execution).Scan(&artifact); e != nil {
		t.Fatal(e)
	}
	tokens := auth.NewHMACTokenManager("synthetic-local-ct08-only", time.Hour)
	middleware := auth.NewMiddleware(auth.NewService(localUsers{}, nil, tokens, nil, nil, nil, nil))
	middleware.SetAccountChecker(localMembership{account: sc.AccountID})
	mux := http.NewServeMux()
	accounting.RegisterRoutes(mux, s, middleware)
	path := "/v1/accounting/issuers/" + sc.IssuerID + "/executions/" + execution + "/artifacts/" + artifact
	for _, tc := range []struct {
		actor string
		code  int
	}{{sc.ActorID, 200}, {"", 401}, {testExecution, 404}} {
		r := httptest.NewRequestWithContext(context.Background(), "GET", path, nil)
		r.Header.Set("X-Account-Id", sc.AccountID)
		if tc.actor != "" {
			session, e := tokens.Issue("", auth.User{ID: tc.actor, Email: "synthetic@ct08.test", Active: true, Role: auth.RoleDirector})
			if e != nil {
				t.Fatal(e)
			}
			r.Header.Set("Authorization", "Bearer "+session.AccessToken)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatal("download authorization", w.Code, tc.code, w.Body.String())
		}
		if tc.code == 200 && (w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Type") != "application/xml") {
			t.Fatal("unsafe download headers")
		}
	}
	s.WithArtifactReader(privateReaderFunc(func(ctx context.Context, scope accounting.Scope, id string) (io.ReadCloser, error) {
		body, e := storage.OpenPrivate(ctx, scope, id)
		if e != nil {
			return nil, e
		}
		if _, e = p.Exec(ctx, `update accounting.issuer_members set status='inactive' where account_id=$1::uuid and issuer_id=$2::uuid and user_id=$3::uuid`, sc.AccountID, sc.IssuerID, sc.ActorID); e != nil {
			_ = body.Close()
			return nil, e
		}
		return body, nil
	}))
	value, e := s.DownloadArtifact(context.Background(), sc, execution, artifact)
	if e == nil || len(value.Content) != 0 {
		t.Fatal("revoked access exposed in-flight artifact")
	}
}
