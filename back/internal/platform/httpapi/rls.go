package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

// RLSScope descreve o tenant ativo da request para o Row-Level Security. E
// extraido do Principal pelo app.go (httpapi nao pode importar auth — ciclo) e
// vira os GUCs app.account_id / app.bypass_rls na conexao da request.
type RLSScope struct {
	// AccountID e o id do tenant ativo (core.accounts.id). Vira o GUC
	// app.account_id, que as policies comparam contra tenant_id/account_id.
	AccountID string
	// Bypass libera a policy (platform_admin, que ve todas as contas). Vira o
	// GUC app.bypass_rls = 'on'.
	Bypass bool
}

// RLSConnGuard injeta uma conexao por request com o GUC do tenant setado, para
// que as policies de Row-Level Security do Postgres filtrem por tenant mesmo
// que a query da aplicacao esqueca o WHERE. Aplicado SO no grupo de rotas que
// ja migrou para RLS (fase 1: /v1/feedback), nunca global, e SEMPRE apos o
// RequireAuth (precisa do Principal resolvido).
type RLSConnGuard struct {
	pool *pgxpool.Pool
	// scope extrai o tenant ativo do request. Definido pelo app.go a partir do
	// Principal (httpapi nao importa auth). Quando nil ou retorna AccountID
	// vazio sem bypass, a request segue no pool direto (sem RLS) — fail-safe
	// para nao trancar rotas antes do wiring completo.
	scope func(*http.Request) (RLSScope, bool)
}

// NewRLSConnGuard cria o guard com o pool de conexoes.
func NewRLSConnGuard(pool *pgxpool.Pool) *RLSConnGuard {
	return &RLSConnGuard{pool: pool}
}

// SetScopeResolver define o extrator do tenant ativo a partir do request.
func (g *RLSConnGuard) SetScopeResolver(fn func(*http.Request) (RLSScope, bool)) {
	g.scope = fn
}

// Wrap embrulha um handler: adquire uma conexao do pool, seta o GUC do tenant
// (e o bypass para platform_admin), poe a conexao no context e garante, via
// defer, o reset do GUC + release da conexao SEMPRE (mesmo em panic/erro),
// senao a conexao vaza do pool.
func (g *RLSConnGuard) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sem resolver ou sem escopo de tenant: segue no pool direto. As queries
		// continuam com o filtro por tenant na aplicacao (defesa em profundidade
		// existente); o RLS so nao atua nesta request.
		if g.scope == nil {
			next.ServeHTTP(w, r)
			return
		}
		scope, ok := g.scope(r)
		if !ok || (scope.AccountID == "" && !scope.Bypass) {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		conn, err := g.pool.Acquire(ctx)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "internal_error",
				"Erro ao obter conexao com o banco.")
			return
		}
		// CRITICO: release SEMPRE. O reset all limpa os GUCs de sessao antes de a
		// conexao voltar ao pool, para nao vazar o tenant para a proxima request.
		defer func() {
			_, _ = conn.Exec(context.Background(), "reset all")
			conn.Release()
		}()

		// set_config(..., false) e nivel de sessao (dura a conexao), sem precisar
		// de transacao. O tenant entra como parametro ($1) — nunca concatenado.
		if _, err := conn.Exec(ctx, "select set_config('app.account_id', $1, false)", scope.AccountID); err != nil {
			WriteError(w, r, http.StatusInternalServerError, "internal_error",
				"Erro ao configurar o escopo da sessao.")
			return
		}
		if scope.Bypass {
			if _, err := conn.Exec(ctx, "select set_config('app.bypass_rls', 'on', false)"); err != nil {
				WriteError(w, r, http.StatusInternalServerError, "internal_error",
					"Erro ao configurar o escopo da sessao.")
				return
			}
		}

		ctx = database.WithConn(ctx, conn)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
