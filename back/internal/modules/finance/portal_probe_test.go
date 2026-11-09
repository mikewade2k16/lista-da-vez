package finance

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
)

func TestPortalProbeRequiresAuthenticatedEvidence(t *testing.T) {
	for _, tc := range []struct{ name, body, status string }{{"accepted", `<a href="/EmissorNacional/Login/Sair">Sair</a>`, "authenticated"}, {"authenticated with captcha library", `<script src="captcha.js"></script><script>const captcha = true;</script><a href="/EmissorNacional/Login/Sair">Sair</a>`, "authenticated"}, {"rejected with captcha library", `<script src="captcha.js"></script><input name="Senha" type="password">Senha inválida`, "rejected"}, {"interactive challenge", `<div class="g-recaptcha"></div><input name="Senha">`, "manual_required"}, {"bad password", `<input name="Senha" type="password">Senha inválida`, "rejected"}, {"empty response", `<html>ok</html>`, "unavailable"}, {"challenge", `<div>captcha</div>`, "manual_required"}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					http.SetCookie(w, &http.Cookie{Name: "csrf", Value: "test", Path: "/"})
					_, _ = w.Write([]byte(`<input name="__RequestVerificationToken" value="qa&amp;token">`))
					return
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				cookie, e := r.Cookie("csrf")
				if e != nil || cookie.Value != "test" || r.PostForm.Get("__RequestVerificationToken") != "qa&token" || r.PostForm.Get("Inscricao") != "qa-user" || r.PostForm.Get("Senha") != "qa-secret" {
					t.Error("login did not use cookie/CSRF/form credentials")
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			jar, _ := cookiejar.New(nil)
			client := &http.Client{Jar: jar}
			result := probePortalWithClient(context.Background(), client, server.URL, portalCredentials{Username: "qa-user", Password: "qa-secret"})
			if result.Status != tc.status {
				t.Fatalf("expected %s, got %s", tc.status, result.Status)
			}
		})
	}
}
func TestAccessConversationDoesNotAcceptPasswordAsMessage(t *testing.T) {
	for _, text := range []string{"minha senha é abc123", "senha: abc123", "password=abc123"} {
		_, action, method := accessReply(text, FiscalProfileView{})
		if action != "access" || method != "" {
			t.Fatal("credential text became command")
		}
	}
	_, _, method := accessReply("CPF/CNPJ e senha do Emissor Nacional", FiscalProfileView{})
	if method != "portal" {
		t.Fatal("portal method was not understood")
	}
}
