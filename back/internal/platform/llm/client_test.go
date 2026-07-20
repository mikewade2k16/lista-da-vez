package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc intercepta a request ANTES da rede: testa o caminho real de Complete
// (host canonico do provider passa na allowlist) sem discar. httptest nao serve aqui
// porque liga em 127.0.0.1, que o guard de SSRF (isInternalHost) barra de proposito.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// clientWith monta um Client cujo transporte e o handler dado. O ultimo request
// interceptado fica em *captured para assercao.
func clientWith(t *testing.T, captured **http.Request, status int, body string) Client {
	t.Helper()
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if captured != nil {
			*captured = r
		}
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
	return NewWithHTTPClient(hc, nil)
}

// baseReq usa BaseURL vazia => default canonico do provider (api.openai.com), que passa
// na allowlist sem registrar host nenhum.
func baseReq() Request {
	return Request{
		Provider:   "openai",
		Model:      "gpt-4o-mini",
		UserPrompt: "oi",
		APIKey:     "sk-super-secreta-123",
		AccountID:  "acc-1",
	}
}

func TestComplete_TextOK(t *testing.T) {
	var got *http.Request
	c := clientWith(t, &got, http.StatusOK,
		`{"choices":[{"message":{"content":"ola"}}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`)

	resp, err := c.Complete(context.Background(), baseReq())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "ola" {
		t.Errorf("Text = %q", resp.Text)
	}
	if resp.Usage.TotalTokens != 4 {
		t.Errorf("TotalTokens = %d", resp.Usage.TotalTokens)
	}
	if got.Header.Get("Authorization") != "Bearer sk-super-secreta-123" {
		t.Errorf("Authorization = %q", got.Header.Get("Authorization"))
	}
	if !strings.HasSuffix(got.URL.Path, "/chat/completions") {
		t.Errorf("path = %q", got.URL.Path)
	}
	if got.URL.Host != "api.openai.com" {
		t.Errorf("host = %q (base vazia deveria virar o default canonico)", got.URL.Host)
	}
}

func TestComplete_SchemaValidated(t *testing.T) {
	schema := &Schema{
		Name:    "triage",
		Version: 1,
		Definition: json.RawMessage(`{
			"type":"object",
			"properties":{"intent":{"type":"string"}},
			"required":["intent"]
		}`),
	}

	t.Run("valido preenche JSON e pede json_object", func(t *testing.T) {
		var got *http.Request
		c := clientWith(t, &got, http.StatusOK,
			`{"choices":[{"message":{"content":"{\"intent\":\"compra\"}"}}]}`)
		req := baseReq()
		req.Schema = schema
		resp, err := c.Complete(context.Background(), req)
		if err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if string(resp.JSON) != `{"intent":"compra"}` {
			t.Errorf("JSON = %q", resp.JSON)
		}
		body, _ := io.ReadAll(got.Body)
		if !strings.Contains(string(body), `"json_object"`) {
			t.Errorf("esperava response_format json_object no body: %s", body)
		}
	})

	t.Run("fora do schema vira ErrSchemaViolation", func(t *testing.T) {
		c := clientWith(t, nil, http.StatusOK,
			`{"choices":[{"message":{"content":"{\"outro\":1}"}}]}`)
		req := baseReq()
		req.Schema = schema
		_, err := c.Complete(context.Background(), req)
		if !errors.Is(err, ErrSchemaViolation) {
			t.Fatalf("erro = %v, queria ErrSchemaViolation", err)
		}
	})
}

func TestComplete_StatusErrorCarriesCode(t *testing.T) {
	c := clientWith(t, nil, http.StatusTooManyRequests, `rate limited`)
	_, err := c.Complete(context.Background(), baseReq())
	var se *StatusError
	if !errors.As(err, &se) {
		t.Fatalf("erro = %v, queria *StatusError", err)
	}
	if se.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d", se.StatusCode)
	}
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Errorf("StatusError deveria desembrulhar ErrProviderUnavailable")
	}
}

func TestComplete_PrechecksSemRede(t *testing.T) {
	// Estes erros tem que sair ANTES de qualquer chamada de rede — transporte que
	// explode prova que nao houve request.
	explode := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("nao deveria ter feito request de rede")
		return nil, nil
	})}
	c := NewWithHTTPClient(explode, nil)

	cases := []struct {
		name string
		mut  func(*Request)
		want error
	}{
		{"provider invalido", func(r *Request) { r.Provider = "claude" }, ErrInvalidProvider},
		{"modelo vazio", func(r *Request) { r.Model = "" }, ErrInvalidModel},
		{"chave ausente", func(r *Request) { r.APIKey = "" }, ErrKeyMissing},
		{"ssrf host interno", func(r *Request) { r.BaseURL = "http://169.254.169.254/latest" }, ErrBaseURLNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := baseReq()
			tc.mut(&req)
			_, err := c.Complete(context.Background(), req)
			if !errors.Is(err, tc.want) {
				t.Fatalf("erro = %v, queria %v", err, tc.want)
			}
		})
	}
}

func TestComplete_ChaveNuncaVazaNoErro(t *testing.T) {
	// Provedor devolve a chave no corpo do erro (alguns fazem eco). O client NAO pode
	// propagar corpo cru — o erro nunca deve conter a chave.
	req := baseReq()
	c := clientWith(t, nil, http.StatusInternalServerError, "erro com Bearer "+req.APIKey)
	_, err := c.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("esperava erro")
	}
	if strings.Contains(err.Error(), req.APIKey) {
		t.Fatalf("a chave vazou no erro: %v", err)
	}
}
