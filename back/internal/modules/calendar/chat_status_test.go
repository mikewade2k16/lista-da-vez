package calendar

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatHealthEndpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		askURL  string
		want    string
		wantErr bool
	}{
		{name: "docker n8n", askURL: "http://n8n:5678/webhook/calendar-chat", want: "http://n8n:5678/healthz"},
		{name: "public n8n with subpath", askURL: "https://example.com/n8n/webhook/calendar-chat?x=1", want: "https://example.com/n8n/healthz"},
		{name: "invalid relative url", askURL: "/webhook/calendar-chat", wantErr: true},
		{name: "invalid scheme", askURL: "ftp://n8n/webhook/calendar-chat", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := chatHealthEndpoint(tt.askURL)
			if tt.wantErr {
				if !errors.Is(err, errChatUpstream) {
					t.Fatalf("expected errChatUpstream, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPingChatUpstreamUsesHealthEndpointOnly(t *testing.T) {
	t.Parallel()
	called := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	svc := &Service{chat: chatConfig{
		askURL: server.URL + "/webhook/calendar-chat",
		client: server.Client(),
	}}
	if err := svc.pingChatUpstream(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := <-called; got != "GET /healthz" {
		t.Fatalf("called %q, want GET /healthz", got)
	}
}
