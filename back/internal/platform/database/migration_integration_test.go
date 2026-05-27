package database_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

// TestAllMigrationsApply aplica todas as migrations em um banco limpo.
//
// Reproduz fielmente o ambiente de deploy: cria a view public.consultants
// via migration 0104 e qualquer outra estrutura que afete migrations futuras.
// Qualquer DDL sem schema qualificado (ex: ALTER TABLE consultants em vez de
// ALTER TABLE queue.consultants) vai falhar aqui antes de chegar em produção.
//
// Para rodar:
//
//	TEST_DATABASE_URL="postgres://user:pass@localhost:5432/testdb" go test ./internal/platform/database/... -run TestAllMigrationsApply -v
func TestAllMigrationsApply(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido — pulando teste de integração de migrations")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("conectar ao banco de teste: %v", err)
	}
	defer pool.Close()

	if err := database.ApplyMigrationsWithOptions(context.Background(), pool, database.MigrationOptions{
		SkipDataSeeds: true,
	}); err != nil {
		t.Fatalf("migrations falharam: %v", err)
	}
}
