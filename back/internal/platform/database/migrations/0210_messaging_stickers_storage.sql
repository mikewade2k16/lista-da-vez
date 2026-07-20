-- 0210 — F12: figurinhas em DISCO (media_storage), não base64 no Postgres.
--
-- A F2 (0200) criou messaging.saved_stickers com data_url TEXT NOT NULL (base64 no banco, como o
-- legado Prisma — o que a decisão D-B/D2 rejeita: mídia vai pro disco privado, não pro Postgres).
-- A F12 grava os bytes via media_storage (raiz OMNICHANNEL_MEDIA_DIR) e guarda só o PATH relativo
-- em storage_key; o data URL base64 é REMONTADO na serialização, relendo o arquivo. Por isso
-- data_url passa a ser nullable (linhas novas só preenchem storage_key).
--
-- SQL plano e IDEMPOTENTE (add column if not exists; drop not null é no-op se já nullable). Sem
-- `-- +goose Down`: o migrator roda o arquivo INTEIRO — um Down com DROP se auto-destruiria.
-- Schema-qualificado.

alter table messaging.saved_stickers add column if not exists storage_key text;
alter table messaging.saved_stickers alter column data_url drop not null;
