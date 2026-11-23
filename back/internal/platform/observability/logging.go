package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// NewLogger cria o logger da aplicação: JSON por padrão (LOG_FORMAT=text volta ao
// formato chave=valor), com correlation ID automático quando o log recebe
// contexto e sanitização de atributos de texto.
func NewLogger(output io.Writer) *slog.Logger {
	if output == nil {
		output = os.Stdout
	}
	var base slog.Handler
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_FORMAT")), "text") {
		base = slog.NewTextHandler(output, nil)
	} else {
		base = slog.NewJSONHandler(output, nil)
	}
	return slog.New(&sanitizingHandler{next: base})
}

type sanitizingHandler struct {
	next slog.Handler
}

func (h *sanitizingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *sanitizingHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	hasRequestID := false
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "request_id" {
			hasRequestID = true
		}
		clean.AddAttrs(sanitizeAttr(attr))
		return true
	})
	if !hasRequestID {
		if id := CorrelationID(ctx); id != "" {
			clean.AddAttrs(slog.String("request_id", id))
		}
	}
	return h.next.Handle(ctx, clean)
}

func (h *sanitizingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		clean = append(clean, sanitizeAttr(attr))
	}
	return &sanitizingHandler{next: h.next.WithAttrs(clean)}
}

func (h *sanitizingHandler) WithGroup(name string) slog.Handler {
	return &sanitizingHandler{next: h.next.WithGroup(name)}
}

func sanitizeAttr(attr slog.Attr) slog.Attr {
	if isIDKey(attr.Key) {
		return attr // ids internos são opacos; mascarar quebraria a correlação
	}
	if IsSensitiveKey(attr.Key) {
		return slog.String(attr.Key, masked)
	}
	value := attr.Value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		return slog.String(attr.Key, SanitizeText(value.String(), 4000))
	case slog.KindAny:
		if err, ok := value.Any().(error); ok && err != nil {
			return slog.String(attr.Key, SanitizeText(err.Error(), 4000))
		}
		return attr
	case slog.KindGroup:
		group := value.Group()
		clean := make([]any, 0, len(group))
		for _, item := range group {
			clean = append(clean, sanitizeAttr(item))
		}
		return slog.Group(attr.Key, clean...)
	default:
		return attr
	}
}

// Chaves de id (ex.: document_id, phone_number_id) são identificadores internos,
// não o dado pessoal em si; mantê-las ajuda a depurar sem expor PII.
func isIDKey(key string) bool {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
		return false
	}
	return strings.HasSuffix(lower, "_id") || strings.HasSuffix(lower, "id")
}
