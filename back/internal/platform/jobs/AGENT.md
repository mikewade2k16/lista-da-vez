# Jobs — infraestrutura de fila durável

Store/Worker genéricos, tabela concreta escolhida pelo owner. Não adicionar regras
fiscais neste pacote. `PostgresStore.EnqueueTx(ctx,pgx.Tx,NewJob)` compartilha a
transação do consumidor; `Enqueue` e a interface Store preservam compatibilidade.
Deduplicação técnica é account+idempotency_key; consumidor valida hash de negócio.
Rollback deve remover também o job. `TestEnqueueTxRollbackAndLegacyCompatibility`
usa a tabela efêmera da suíte FIFO e TEST_DATABASE_URL descartável.

Claim/liberação de presa/retry podem reexecutar handler. Consumidor deve proteger
efeitos externos com estado/lease/fencing e reconciliação próprios; MarkDone não
prova exactly-once. Nunca incluir payload, credenciais ou resposta bruta em logs.
