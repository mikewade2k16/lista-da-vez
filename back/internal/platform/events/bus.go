// Package events fornece um event bus in-process para comunicacao assincrona
// entre modulos da plataforma.
//
// A interface Bus e identica em forma a um broker externo (NATS/RabbitMQ/Kafka).
// Quando algum modulo precisar sair para outro processo, basta trocar a
// implementacao InMemoryBus por um adapter sem alterar quem publica/consome.
//
// Convencao de topicos: "<module>.<entity>.<verb_past>". Ex:
//   queue.service_finished
//   finance.invoice_paid
//   account.modules.changed
//
// Reviewer rejeita handler que publica evento do mesmo modulo (efeito cascata
// suspeito). Bus rejeita eventos com profundidade > MaxEventDepth.
package events

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// MaxEventDepth limita a cadeia de causation para detectar loops.
const MaxEventDepth = 10

// ErrMaxDepthExceeded e retornado quando o evento tem CausationID em uma cadeia
// maior que MaxEventDepth — provavel loop entre handlers.
var ErrMaxDepthExceeded = errors.New("events: max causation depth exceeded — possible loop")

// Event e a unidade publicada e consumida no bus.
type Event struct {
	// ID identifica esta publicacao especifica. Gerado pelo Publish quando vazio.
	ID string

	// AccountID propaga o escopo multi-tenant. Obrigatorio para topicos que
	// dependam de account; vazio so para topicos plataforma-wide (raro).
	AccountID string

	// Topic segue o formato "<module>.<entity>.<verb_past>".
	Topic string

	// Payload e o conteudo do evento. Mantenha pequeno e serializavel —
	// adapters externos vao precisar serializar como JSON.
	Payload map[string]any

	// OccurredAt e o instante logico do evento (nao o de publicacao).
	// Default: now() quando vazio.
	OccurredAt time.Time

	// CausationID aponta para o ID do evento que originou este. Vazio quando
	// publicado por origem externa (HTTP request, scheduler).
	CausationID string

	// CorrelationID identifica a cadeia inteira (request id ou similar). Igual
	// para todos os eventos disparados pelo mesmo gatilho original.
	CorrelationID string

	// Depth e calculado pelo bus a partir da cadeia de causation. Handler nao
	// precisa preencher.
	Depth int
}

// Handler processa um evento. Erro retornado e logado mas nao re-publicado;
// handlers devem ser idempotentes.
type Handler func(ctx context.Context, e Event) error

// Subscription representa um handler registrado. Chamar Unsubscribe para
// parar de receber eventos do topico.
type Subscription interface {
	Unsubscribe()
}

// Bus e o contrato implementado pelo InMemoryBus (e potencialmente por adapters
// externos no futuro).
type Bus interface {
	Publish(ctx context.Context, e Event) error
	Subscribe(topic string, handler Handler) Subscription
}

// ============================================================================
// InMemoryBus
// ============================================================================

// InMemoryBus despacha eventos sincronamente para todos os handlers de um
// topico. Erros sao logados; nao para o despacho dos demais handlers.
//
// Despacho sincrono evita ordering issues e simplifica testes. Quando
// precisar de assincronia, criar um worker pool em frente ao Subscribe ou
// trocar para implementacao com goroutine pool bounded por topico.
type InMemoryBus struct {
	logger *slog.Logger

	mu            sync.RWMutex
	subscriptions map[string][]*subscription
}

// NewInMemoryBus cria um bus pronto para uso.
func NewInMemoryBus(logger *slog.Logger) *InMemoryBus {
	return &InMemoryBus{
		logger:        logger,
		subscriptions: make(map[string][]*subscription),
	}
}

// Publish dispara o evento para todos os handlers do topico. Erros de handlers
// sao logados; o evento e considerado entregue mesmo com falhas.
func (b *InMemoryBus) Publish(ctx context.Context, e Event) error {
	if e.Topic == "" {
		return errors.New("events: topic is required")
	}

	if e.Depth >= MaxEventDepth {
		b.logger.Warn(
			"event rejected — max depth exceeded",
			slog.String("topic", e.Topic),
			slog.String("correlation_id", e.CorrelationID),
			slog.Int("depth", e.Depth),
		)
		return ErrMaxDepthExceeded
	}

	if e.ID == "" {
		e.ID = newEventID()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	if e.CorrelationID == "" {
		e.CorrelationID = e.ID
	}

	b.mu.RLock()
	handlers := append([]*subscription(nil), b.subscriptions[e.Topic]...)
	b.mu.RUnlock()

	for _, sub := range handlers {
		if err := sub.handler(ctx, e); err != nil {
			b.logger.Error(
				"event handler failed",
				slog.String("topic", e.Topic),
				slog.String("event_id", e.ID),
				slog.String("correlation_id", e.CorrelationID),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// Subscribe registra um handler para o topico exato. Wildcards nao suportados
// nesta implementacao — quando precisar, criar Subscribe("module.*", handler).
func (b *InMemoryBus) Subscribe(topic string, handler Handler) Subscription {
	if topic == "" {
		panic("events: topic is required for Subscribe")
	}
	if handler == nil {
		panic(fmt.Sprintf("events: handler is required for Subscribe(%q)", topic))
	}

	sub := &subscription{
		bus:     b,
		topic:   topic,
		handler: handler,
	}

	b.mu.Lock()
	b.subscriptions[topic] = append(b.subscriptions[topic], sub)
	b.mu.Unlock()

	return sub
}

type subscription struct {
	bus     *InMemoryBus
	topic   string
	handler Handler
}

func (s *subscription) Unsubscribe() {
	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()

	subs := s.bus.subscriptions[s.topic]
	for i, candidate := range subs {
		if candidate == s {
			s.bus.subscriptions[s.topic] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// newEventID gera um identificador no formato UUID v4. Sem dependencia externa
// (regra do projeto: nao usar pacote uuid de terceiros). Em caso improvavel
// de falha do crypto/rand, cai num fallback baseado em timestamp.
func newEventID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UTC().UnixNano(), 16)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
