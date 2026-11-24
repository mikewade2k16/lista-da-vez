package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// DefaultShutdownTimeout e o prazo padrao do desligamento gracioso (env SHUTDOWN_TIMEOUT).
const DefaultShutdownTimeout = 20 * time.Second

// Serve atende em ln ate ctx ser cancelado; entao chama Shutdown (espera as requisicoes
// em andamento) por ate timeout e, se estourar, fecha as conexoes restantes.
// Devolve nil em desligamento limpo; erro do Serve se o servidor cair antes.
// timedOut indica que o prazo estourou e conexoes foram cortadas.
func Serve(ctx context.Context, srv *http.Server, ln net.Listener, timeout time.Duration) (timedOut bool, err error) {
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return false, nil
		}
		return false, err
	case <-ctx.Done():
	}

	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		<-serveErr
		return true, nil
	}
	<-serveErr
	return false, nil
}
