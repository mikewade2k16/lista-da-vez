package finance

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

const nationalPortalLogin = "https://www.nfse.gov.br/EmissorNacional/Login"

type portalCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type PortalCheck struct {
	Status         string `json:"status"`
	Message        string `json:"message"`
	CheckedAt      string `json:"checkedAt"`
	ProfileVersion string `json:"profileVersion"`
}

var portalInputTag = regexp.MustCompile(`(?is)<input\b[^>]*>`)
var portalAttribute = regexp.MustCompile(`(?i)([a-z_][a-z0-9_]*)\s*=\s*["']([^"']*)["']`)
var portalScripts = regexp.MustCompile(`(?is)<(?:script|style)\b[^>]*>.*?</(?:script|style)\s*>`)
var portalTags = regexp.MustCompile(`(?s)<[^>]+>`)
var portalChallengeElement = regexp.MustCompile(`(?is)<(?:div|iframe|input)\b[^>]*(?:class|id|name)\s*=\s*["'][^"']*(?:g-recaptcha|h-captcha|cf-turnstile|captcha-challenge|captcha-container)[^"']*["']`)
var portalLogout = regexp.MustCompile(`(?is)<a\b[^>]*href\s*=\s*["'][^"']*(?:/sair|/logout|/logoff)[^"']*["']`)

func portalInput(body, name string) string {
	for _, tag := range portalInputTag.FindAllString(body, -1) {
		attrs := map[string]string{}
		for _, a := range portalAttribute.FindAllStringSubmatch(tag, -1) {
			attrs[strings.ToLower(a[1])] = html.UnescapeString(a[2])
		}
		if attrs["name"] == name {
			return attrs["value"]
		}
	}
	return ""
}
func probeNationalPortal(ctx context.Context, c portalCredentials) PortalCheck {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return portalCheck("unavailable")
	}
	client := newPortalHTTPClient(jar)
	return probePortalWithClient(ctx, client, nationalPortalLogin, c)
}
func probePortalWithClient(ctx context.Context, client *http.Client, login string, c portalCredentials) PortalCheck {
	body, response, err := portalRequest(ctx, client, http.MethodGet, login, nil) //nolint:bodyclose // portalRequest reads and closes the body.
	if err != nil || response.StatusCode != http.StatusOK {
		return portalCheck("unavailable")
	}
	token := portalInput(body, "__RequestVerificationToken")
	if token == "" {
		return portalCheck("manual_required")
	}
	values := url.Values{"__RequestVerificationToken": {token}, "Inscricao": {c.Username}, "Senha": {c.Password}}
	body, response, err = portalRequest(ctx, client, http.MethodPost, login, values) //nolint:bodyclose // portalRequest reads and closes the body.
	if err != nil {
		return portalCheck("unavailable")
	}
	return classifyPortalResponse(body, response)
}
func classifyPortalResponse(body string, response *http.Response) PortalCheck {
	lower := strings.ToLower(body)
	loginForm := strings.Contains(lower, `name="senha"`) || strings.Contains(lower, `name='senha'`)
	// Authenticated pages may load CAPTCHA libraries without presenting a challenge.
	if response.StatusCode == http.StatusOK && portalLogout.MatchString(body) && !loginForm {
		return portalCheck("authenticated")
	}
	visible := strings.ToLower(html.UnescapeString(portalTags.ReplaceAllString(portalScripts.ReplaceAllString(body, ""), " ")))
	if portalChallengeElement.MatchString(body) || strings.Contains(visible, "captcha") || strings.Contains(visible, "autenticação em duas") {
		return portalCheck("manual_required")
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		return portalCheck("manual_required")
	}
	if response.StatusCode == http.StatusOK && loginForm {
		return portalCheck("rejected")
	}
	return portalCheck("unavailable")
}
func portalRequest(ctx context.Context, client *http.Client, method, target string, values url.Values) (string, *http.Response, error) {
	var input io.Reader
	if values != nil {
		input = strings.NewReader(values.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, target, input)
	if err != nil {
		return "", nil, err
	}
	if values != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	res, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return string(raw), res, err
}
func portalCheck(status string) PortalCheck {
	messages := map[string]string{
		"authenticated":   "O último teste autenticou o acesso no Emissor Nacional. O teste não emitiu nota; a sessão é temporária e não abre o seu Chrome.",
		"rejected":        "O portal não confirmou o acesso. Confira CPF/CNPJ e senha próprios do Emissor Nacional; a senha gov.br usa outra entrada.",
		"manual_required": "O acesso exige uma etapa no navegador (gov.br, CAPTCHA ou confirmação). Este teste não consegue confirmar essa sessão.",
		"unavailable":     "Não foi possível confirmar o acesso agora. O portal pode estar indisponível ou ter alterado o formulário.",
		"unsupported":     "O teste automático atende somente ao Emissor Nacional. Confira o endereço salvo.",
	}
	return PortalCheck{Status: status, Message: messages[status]}
}
