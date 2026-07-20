// Package jobs e o engine GENERICO de outbox durável da plataforma (OMNI-F3.2):
// claim com `FOR UPDATE SKIP LOCKED`, FIFO por ordering_key, retry classificado com
// backoff, dead-letter e monitor de presas com filtro de conta.
//
// O engine nao conhece a tabela concreta: fala com a interface Store. No omnichannel
// o Store concreto e messaging.outbox (criada pela migration 0200_messaging_schema.sql,
// F2) e ordering_key = conversation_id. O produtor de job e a F6.
//
// Estado e 100% do BANCO — sem BullMQ, sem Redis (principio 1: fonte unica de verdade).
//
// SEGURANCA: account_id vem SEMPRE do Principal, nunca do body. Todo select/update do
// Store filtra por account_id tambem no repositorio (defesa em profundidade). payload e
// last_error sao MASCARADOS: nunca vao para log, erro ou trace.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Status e o ciclo de vida do job na tabela (check da 0200).
type Status string

const (
	// StatusPending: aguardando claim (inclui job em backoff, com run_after futuro).
	StatusPending Status = "pending"
	// StatusProcessing: reivindicado por um worker (locked_at/locked_by preenchidos).
	StatusProcessing Status = "processing"
	// StatusDone: concluido com sucesso.
	StatusDone Status = "done"
	// StatusFailed: reservado para falha terminal marcada pelo consumidor.
	StatusFailed Status = "failed"
	// StatusDead: dead-letter — esgotou tentativas ou classe unrecoverable.
	StatusDead Status = "dead"
)

// Job e uma unidade de trabalho reivindicada do outbox.
type Job struct {
	ID          string
	AccountID   string
	OrderingKey string // FIFO por chave. No omnichannel = conversation_id.
	Kind        string // despacha o handler
	Payload     json.RawMessage
	Attempts    int // ja incrementado pelo claim (1 = primeira tentativa)
	MaxAttempts int
}

// NewJob e a entrada do produtor (F6). AccountID tem de vir do Principal.
type NewJob struct {
	AccountID      string
	OrderingKey    string
	IdempotencyKey string // unique (account_id, idempotency_key) — POR CONTA. Vai cru.
	Kind           string
	Payload        json.RawMessage
	RunAfter       time.Time // zero = agora
	MaxAttempts    int       // <=0 => defaultMaxAttempts; o retry reclassifica na falha
}

// defaultMaxAttempts espelha o default da coluna max_attempts (0200).
const defaultMaxAttempts = 3

// Handler processa um job. Erro devolvido => o engine CLASSIFICA (ver retry.go) e
// decide retry com backoff ou dead-letter. Handler deve ser idempotente: uma presa
// liberada pelo monitor pode reexecutar um job cujo efeito ja ocorreu.
type Handler interface {
	Handle(ctx context.Context, job Job) error
}

// HandlerFunc adapta uma func a Handler.
type HandlerFunc func(ctx context.Context, job Job) error

// Handle implementa Handler.
func (f HandlerFunc) Handle(ctx context.Context, job Job) error { return f(ctx, job) }

var (
	// ErrNoHandler: kind sem handler registrado. E unrecoverable — reprocessar nao
	// resolve, so codigo novo resolve. Vai direto para dead-letter.
	ErrNoHandler = errors.New("jobs: nenhum handler registrado para o kind")

	// ErrInvalidJob: NewJob sem account_id/ordering_key/kind.
	ErrInvalidJob = errors.New("jobs: job invalido (account_id, ordering_key e kind sao obrigatorios)")

	// ErrInvalidTable: nome de tabela fora do formato schema.tabela.
	ErrInvalidTable = errors.New("jobs: nome de tabela invalido (esperado schema.tabela)")
)

// StatusError e o erro que carrega o status HTTP da resposta do provider, para a
// classificacao do retry. Quem chama um provider HTTP devolve isto; sem ele o engine
// trata como "sem status" (4 tentativas).
type StatusError struct {
	// StatusCode e o HTTP status da resposta (0 = nao houve resposta HTTP).
	StatusCode int

	// Unrecoverable marca um 400/422 CONHECIDO (payload rejeitado por regra do
	// provider): reenviar o mesmo payload da o mesmo erro. A tabela do canonico §8
	// ja trata 400/422 como unrecoverable por default; esta flag deixa o consumidor
	// marcar explicitamente outros status como terminais.
	Unrecoverable bool

	// Err e a causa. NUNCA deve carregar payload cru nem segredo: o texto vai para
	// last_error, que e persistido.
	Err error
}

// Error implementa error. Nao interpola struct — so status e causa.
func (e *StatusError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("jobs: provider respondeu status %d", e.StatusCode)
	}
	if e.StatusCode == 0 {
		return e.Err.Error()
	}
	return fmt.Sprintf("status %d: %s", e.StatusCode, e.Err.Error())
}

// Unwrap permite errors.Is/As atravessarem o StatusError.
func (e *StatusError) Unwrap() error { return e.Err }
