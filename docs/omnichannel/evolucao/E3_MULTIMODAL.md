# E3 — áudio, visão e documentos

**Status:** `READY_FOR_ROLLOUT` — pipeline Go+n8n, cofre de credenciais e configuração por função implementados; migração, importação e ativação continuam operações explícitas por ambiente

**Resultado:** áudio, imagem e documento viram contexto estruturado auditável para a triagem, com
limites, consentimento/política, custo e fallback; a mídia original continua privada no Go.

## 1. Escopo fechado

Entrega atual: contratos por tipo, configuração versionada, persistência idempotente, gateway
privado `media-stream.v1`, policy determinística e o ramo `omnichannel-brain-media` do workflow
próprio. O job de mídia chama o n8n; o n8n interpreta; o Go valida e persiste o resultado antes de
entregá-lo ao modelo de resposta. A ativação real exige aplicar a migration `0234`, importar e
ativar o workflow, configurar os endpoints internos e salvar as versões com credenciais nomeadas.

- áudio: transcrição; imagem: descrição/OCR orientado ao atendimento; documento: extração de texto
  em formatos permitidos e dentro de limites;
- análise enriquece contexto; não substitui a mensagem original;
- bytes não vão ao PostgreSQL nem ao JSON exportado do workflow;
- o n8n acessa mídia por URL interna assinada, curta, tenant-scoped e de uso limitado;
- modelo, provider, habilitação, limites e instruções vêm da versão/config do agente;
- falha multimodal não bloqueia a fila humana nem apaga a mídia.

## 2. Configuração versionada

Adicionar `media_config jsonb not null default '{}'` a `ai_agent_versions` apenas se a auditoria
confirmar que `layers` não possui contrato equivalente. Shape fechado na API:

```json
{
  "audio": {"enabled": true, "credentialId": "uuid", "provider": "openai", "model": "...", "maxSeconds": 600},
  "image": {"enabled": true, "credentialId": "uuid", "provider": "openai", "model": "...", "maxBytes": 5242880},
  "video": {"enabled": true, "credentialId": "uuid", "provider": "gemini", "model": "...", "maxBytes": 62914560},
  "document": {"enabled": false, "credentialId": "uuid", "provider": "gemini", "model": "...", "maxPages": 20},
  "retentionDays": 90,
  "includeInReply": true
}
```

Chave não entra neste JSON. `credentialId` referencia `messaging.ai_credentials`, cofre cifrado e
account-scoped. A versão também fixa `response_credential_id` para o modelo que redige a resposta.
Uma mesma credencial nomeada pode ser reutilizada por vários agentes sem duplicar o segredo.

## 3. Banco

Criar `messaging.media_analyses`:

| Campo | Regra |
|---|---|
| `id`, `account_id`, `message_id`, `conversation_id` | FKs e escopo explícito |
| `analysis_kind` | `transcription`, `vision`, `video_summary`, `document_text` |
| `content_hash` | SHA-256 dos bytes; dedupe com kind/model/version |
| `status` | `queued`, `processing`, `completed`, `failed`, `blocked` |
| `provider`, `model`, `credential_id`, `agent_version_id` | reprodutibilidade e vínculo com o cofre |
| `result_text` | resultado limitado; PII operacional, sujeito à retenção |
| `result_json` | idioma, confiança, páginas/timestamps, sem bytes |
| `prompt_tokens`, `completion_tokens`, `cost_usd`, `latency_ms` | custo congelado |
| `attempts`, `last_error` | erro mascarado/classificado |
| `created_at`, `completed_at`, `expires_at` | retenção |

Unique `(account_id,message_id,analysis_kind,content_hash,provider,model,agent_version_id)` evita
cobrança duplicada. Índices por mensagem e por `status/created_at`.

## 4. Pipeline

1. E1 termina ingest e publica `media.ready` após commit.
2. Policy verifica AIAllowed, config, MIME, tamanho/duração/páginas e orçamento.
3. Cria análise idempotente e job durável.
4. Go emite token assinado contendo account/message/analysis/expiry/nonce.
5. Workflow brain chama rota interna de mídia com esse token; rota valida escopo e streama com
   limite, sem revelar path.
6. n8n chama o modelo configurado e retorna resultado estruturado.
7. Go valida tamanho/schema, grava análise/custo e reabre/continua dispatch E2 da generation certa.
8. Em falha, grava `failed|blocked`, inclui placeholder seguro no contexto e segue fallback.

Token expira em minutos, não é logado e não pode buscar outra mensagem. Redirect externo é
proibido. Documento protegido por senha ou tipo não suportado vira `blocked`. Extração acontece
com limites de bytes descompactados, quantidade de entradas, páginas, pixels e tempo para impedir
ZIP/PDF decompression bomb; o MIME declarado nunca basta sem sniffing seguro.

## 5. Contratos por tipo

- transcrição: `{text, language, durationSeconds, segments?[start,end,text], confidence?}`;
- visão: `{summary, visibleText, objects[], safetyFlags[], confidence?}`;
- vídeo: `{summary, visibleText, scenes[], safetyFlags[], durationSeconds?, confidence?}`;
- documento: `{summary, extractedText, pageCount, truncated, warnings[]}`.

O texto máximo por análise e o total inserido no prompt são limitados. Documento truncado declara
`truncated=true`; nunca finge leitura completa.

## 6. Frontend

- mensagem exibe estado “analisando”, “transcrito”, “não analisado por política” ou “falhou”;
- transcrição/descrição abre em bloco recolhível, deixando claro que foi gerada por IA;
- retry só para permissão administrativa e não cria custo duplicado se análise já completou;
- config do agente oferece toggles, modelos e limites com ajuda sobre custo/privacidade;
- histórico de run liga a análise ao dispatch que a consumiu;
- nenhum signed URL persiste em store/localStorage.

## 7. Pacotes atômicos

| Pacote | Entrega |
|---|---|
| `E3-CONTRACT-01` | schemas por tipo e fixtures |
| `E3-DB-02` | config versionada + `media_analyses` |
| `E3-BE-03` | policy, job, signed stream e persistência |
| `E3-N8N-04` | branches multimodais no brain próprio |
| `E3-FE-05` | config e render de análise |
| `E3-QA-06` | MIME/limite/SSRF/custo/fallback end-to-end |

## 8. Aceite e proibições

- áudio válido transcreve uma vez e participa da mesma triagem;
- imagem com texto entrega OCR/summary; modelo sem visão gera fallback explícito;
- PDF acima do limite é bloqueado antes da chamada paga;
- retry de webhook/job não duplica análise/custo;
- token expirado, conta errada e message ID alterado retornam 404/401 seguro;
- resultado atrasado não reativa IA depois do handoff;
- bytes/path/token/chave não aparecem em banco de auditoria, log ou workflow;
- proibido OCR caseiro no handler, base64 no banco, URL pública permanente e download direto pelo
  browser/n8n sem assinatura.
