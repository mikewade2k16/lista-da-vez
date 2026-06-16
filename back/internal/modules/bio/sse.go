package bio

import (
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// sseSlugRe valida o slug do stream sem tocar o banco (so formato).
var sseSlugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// ssePingInterval mantem a conexao viva atraves de proxies (comentario SSE).
const ssePingInterval = 25 * time.Second

// sseBroker e um hub em memoria de assinantes por slug. Quando uma bio e
// (re)publicada/despublicada, notify(slug) acorda as conexoes SSE daquele slug,
// que entao emitem `event: updated`. Push real (sem polling) — alinhado ao
// ENGINEERING_PRINCIPLES §6 (WebSocket/SSE para tempo real, nao polling).
type sseBroker struct {
	mu   sync.RWMutex
	subs map[string]map[chan struct{}]struct{}
}

func newSSEBroker() *sseBroker {
	return &sseBroker{subs: make(map[string]map[chan struct{}]struct{})}
}

func (b *sseBroker) subscribe(slug string) chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[slug] == nil {
		b.subs[slug] = make(map[chan struct{}]struct{})
	}
	b.subs[slug][ch] = struct{}{}
	return ch
}

func (b *sseBroker) unsubscribe(slug string, ch chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if m := b.subs[slug]; m != nil {
		delete(m, ch)
		if len(m) == 0 {
			delete(b.subs, slug)
		}
	}
	close(ch)
}

// notify acorda todas as conexoes do slug. Envio nao-bloqueante: se o canal ja
// tem um aviso pendente, descarta (o cliente vai refetchar de qualquer forma).
func (b *sseBroker) notify(slug string) {
	if slug == "" {
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs[slug] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// handleStream serve GET /v1/public/bio/{slug}/stream (SSE, sem auth, CORS do
// middleware /v1/public/*). Nao envia conteudo da bio — so o sinal de que mudou;
// o front refetcha o endpoint publico. Conexao ociosa: zero trafego sem evento.
func handleStream(broker *sseBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := strings.ToLower(strings.TrimSpace(r.PathValue("slug")))
		if !sseSlugRe.MatchString(slug) {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Bio nao encontrada.")
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "no_stream", "Streaming nao suportado.")
			return
		}

		// SSE e long-lived: remove o WriteTimeout global (server.go) so para esta
		// conexao, senao o servidor a fecharia a cada ~30s e o EventSource ficaria
		// reconectando. Zero = sem deadline.
		if rc := http.NewResponseController(w); rc != nil {
			_ = rc.SetWriteDeadline(time.Time{})
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		ch := broker.subscribe(slug)
		defer broker.unsubscribe(slug, ch)

		_, _ = w.Write([]byte("event: ready\ndata: {}\n\n"))
		flusher.Flush()

		ping := time.NewTicker(ssePingInterval)
		defer ping.Stop()
		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				_, _ = w.Write([]byte("event: updated\ndata: {}\n\n"))
				flusher.Flush()
			case <-ping.C:
				_, _ = w.Write([]byte(": ping\n\n"))
				flusher.Flush()
			}
		}
	}
}
