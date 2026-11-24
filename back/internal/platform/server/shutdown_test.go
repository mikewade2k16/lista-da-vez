package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func TestServeShutdownWaitsForInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	srv := New("", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(200 * time.Millisecond)
		_, _ = io.WriteString(w, "ok")
	}))
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	var timedOut bool
	go func() {
		var err error
		timedOut, err = Serve(ctx, srv, ln, 3*time.Second)
		result <- err
	}()

	body := make(chan string, 1)
	go func() {
		resp, err := httpGet("http://" + ln.Addr().String())
		if err != nil {
			body <- "erro: " + err.Error()
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		body <- string(b)
	}()
	<-started
	cancel() // SIGTERM com a requisicao em voo

	if got := <-body; got != "ok" {
		t.Fatalf("requisicao em andamento deveria concluir, got %q", got)
	}
	if err := <-result; err != nil || timedOut {
		t.Fatalf("shutdown limpo esperado: err=%v timedOut=%v", err, timedOut)
	}
}

func TestServeShutdownTimeoutClosesStuckRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	srv := New("", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}))
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() {
		timedOut, _ := Serve(ctx, srv, ln, 100*time.Millisecond)
		done <- timedOut
	}()
	go func() {
		if resp, err := httpGet("http://" + ln.Addr().String()); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	cancel()
	select {
	case timedOut := <-done:
		if !timedOut {
			t.Fatal("esperava timedOut=true")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve nao voltou apos o timeout")
	}
}

// httpGet faz GET com contexto (regra noctx do lint).
func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}
