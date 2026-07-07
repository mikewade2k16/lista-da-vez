-- Modulo Calendario — optimistic locking de eventos (Wave 2, contrato C12)
-- Plano: docs/CALENDARIO_PLAN.md; contrato C12 em docs/CALENDARIO_SPECS2.md.
--
-- Adiciona a coluna version (contador de revisao) em calendar.events. O PUT do
-- evento pode enviar If-Match: <version>; quando divergente, a api devolve 409
-- version_conflict (o front avisa "alterado por outra pessoa" e oferece recarregar).
-- Toda escrita bem-sucedida faz version = version + 1. Linhas antigas nascem com 1.
-- Idempotente, schema qualificado, sem `-- +goose Down` (o migrator roda o arquivo inteiro).

alter table calendar.events add column if not exists version integer not null default 1;
