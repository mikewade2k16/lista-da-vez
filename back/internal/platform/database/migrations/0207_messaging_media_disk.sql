-- Omnichannel F6 — colunas de midia em DISCO em messaging.messages (spec OMNI-F6 C2/D2).
--
-- A F2 (0200) nasceu com media_url / media_mime_type / media_file_name / media_file_size_bytes
-- / media_caption / media_duration_seconds, mas SEM media_storage_key nem media_source_kind:
-- estas duas sao o que separa o storage em disco (raiz privada) do data URL base64 do legado.
-- Aditivo, idempotente, schema-qualificado, SEM `-- +goose Down` (o migrator roda o arquivo
-- inteiro; um Down se auto-destruiria — ver 0200/0147).
--
--   media_storage_key: path RELATIVO a OMNICHANNEL_MEDIA_DIR ({acct}/{conv}/{random}.{ext}).
--                      NUNCA serializado no JSON (nao entra no messageCols) — so o /media o le.
--   media_source_kind: 'disk' (gravado aqui) | 'url_encrypted' (inbound a rehidratar via provider).
--
-- media_url continua sendo a URL do ENDPOINT autenticado quando ha storage key (spec C2):
-- nunca data URL, nunca o path de disco. Sem migration para ela (a coluna ja existe).

alter table messaging.messages
    add column if not exists media_storage_key text;

alter table messaging.messages
    add column if not exists media_source_kind text;
