# E8 — Instagram DM e comentários

**Status:** `CODE-COMPLETE` local em 2026-07-21; smoke Meta/App Review ainda pendente

**Resultado:** DMs e comentários de contas profissionais entram no domínio/CRM/inbox único; a IA
tria e recomenda resposta, enquanto Go aplica janela, moderação, rate limit e outbox.

## 1. Referência e decisões fechadas

Referência primária: [coleção oficial Instagram API da Meta](https://www.postman.com/meta/instagram/documentation/6yqw8pt/instagram-api).
Ela documenta o endpoint de mensagens, webhooks e o fluxo de private reply a comentário. A
capability atual deve ser reconfirmada no dispatch da implementação, pois permissões/App Review e
versões da Graph API mudam.

- P0: receber DM, responder DM iniciada pelo usuário, ingerir comentário e produzir sugestão;
- resposta pública automática começa em modo aprovação; private reply usa policy específica;
- DM e comentário são eventos distintos, mas a pessoa usa o mesmo contato/identidade E4;
- n8n Instagram recomenda/classifica; adapter Go envia;
- não existe cold DM genérico: toda ação exige evento/capability/janela permitidos;
- workflow próprio: `workflow-instagram-first-contact.json`, ID `instafirst000001`.

## 2. Banco

Criar `messaging.instagram_accounts`:

`id/account_id/ig_user_id/username/display_name/page_id/provider_config/credentials_ciphertext/
is_active/webhook_status/created_at/updated_at`, unique `(account_id,ig_user_id)`. Segredos
cifrados; front mascarado.

Reutilizar `conversations/messages/contacts/contact_identities/contact_touchpoints/outbox` com
`channel='INSTAGRAM'`. Não criar inbox ou mensagem paralela.

Criar `messaging.instagram_comments`:

`id/account_id/instagram_account_id/external_comment_id/external_media_id/parent_comment_id/
contact_id/author_scoped_id/username/text/event_kind/status/is_live/occurred_at/metadata/created_at/updated_at`,
unique por conta/account/comment. `status`: visible, hidden, deleted, pending_review.
`event_kind` distingue `comment` e `mention`; ambos mantêm política/capability próprias.

Criar `messaging.instagram_comment_actions`:

`id/account_id/comment_id/action_kind/status/proposed_text/approved_text/ai_run_id/
approved_by_user_id/external_message_id/idempotency_key/last_error/created_at/executed_at`.

`action_kind`: `public_reply`, `private_reply`, `hide`, `ignore`; status fechado. Unique por
account/idempotency. Guardar `private_reply_expires_at` calculado do evento e capability versionada.

## 3. Webhook e normalização

- GET verifica challenge/token; POST valida assinatura app secret antes do parse de domínio;
- resolver conta Instagram por recipient/account ID dentro do accountSlug configurado;
- dedupe em `webhook_events` na mesma transação da mensagem/comentário;
- DM normaliza para evento canônico E1, incluindo reply/media/reaction quando suportado;
- comentário cria/atualiza contato, identidade e touchpoint `instagram_comment`;
- echo/fromMe de DM reconcilia outbound e nunca dispara nova IA;
- evento removido/oculto atualiza status monotonicamente.

## 4. Policy de comentários e DMs

A coleção oficial atual descreve private reply a comentário pelo endpoint de mensagens e informa
limites como uma resposta privada por comentário, janela de até 7 dias para post/reel e, em Live,
somente durante a transmissão; follow-up depende de resposta do destinatário e da janela aplicável.
Essas regras ficam em policy Go, parametrizadas por capability/version e cobertas por fixtures.

Para cada ação:

1. confirmar conta, comentário/DM, status e opt-out;
2. validar prazo/capability/permissão App Review;
3. validar que ação equivalente não foi executada;
4. aplicar moderação/approval e rate limit;
5. gravar outbox;
6. adapter envia e reconcilia external ID/webhook;
7. erro de policy fica visível e não sofre retry inútil.

Resposta pública automática só pode ser habilitada por regra explícita com allowlist de intenção,
confidence, palavras bloqueadas e amostragem. Casos sensíveis, reclamação, ameaça, dado pessoal,
ambiguidade ou baixa confidence vão à aprovação humana.
Rate limit inclui conta, autor e post/janela; cada regra tem teto por post e denylist/allowlist
versionadas no banco. Estouro encaminha para moderação/ignore auditado, nunca dispara em rajada.

## 5. Workflow n8n Instagram

Entrada `instagram.first_contact.request.v1`: account/context IDs, tipo `dm|comment`, texto/mídia
analisada, contato, origem/campanha e policy capabilities. Saída:

```json
{
  "schemaVersion": "instagram.first_contact.result.v1",
  "classification": {"intent": "...", "confidence": 0.0, "sentiment": "..."},
  "contactUpdates": {"tags": [], "fields": {}},
  "recommendedAction": "reply_dm|reply_public|reply_private|handoff|ignore",
  "replyDraft": "...",
  "moderation": {"requiresApproval": true, "reasonCodes": []}
}
```

O workflow não chama Graph API, não grava banco e não decide validade da janela. Export sem
credenciais. O brain WhatsApp não é alterado por este pacote.

## 6. Frontend

- canal Instagram aparece na mesma lista/filtros com ícone e account correta;
- DM usa o chat existente por capabilities;
- comentário mostra post/media context, thread, autor, texto, prazo de private reply e status;
- fila de moderação permite editar/aprovar/rejeitar draft e registra ator;
- drawer do contato mostra identidade Instagram e touchpoint post/reel/campaign;
- config conecta conta, testa permissões/webhook e mostra recursos aprovados/indisponíveis;
- botão fica desabilitado com razão do backend quando janela/capability fechou.

## 7. Pacotes atômicos

| Pacote | Entrega |
|---|---|
| `E8-CONTRACT-01` | fixtures/capabilities/policy DM-comment |
| `E8-DB-02` | accounts/comments/actions |
| `E8-BE-03` | webhook/parser/dedupe/CRM |
| `E8-N8N-04` | first-contact workflow estruturado |
| `E8-BE-05` | policy/outbox/adapter de ações |
| `E8-FE-06` | inbox Instagram e moderação |
| `E8-QA-07` | DM/comment/private/public/expiry/tenant |

## 8. Aceite

- webhook duplicado cria uma DM/comentário;
- echo outbound não chama IA;
- comentário cria touchpoint e classificação automática com fonte/confiança;
- menção cria touchpoint próprio e não herda automaticamente permissão de reply de comentário;
- private reply é bloqueada após prazo ou segunda tentativa equivalente;
- Live encerrada não recebe private reply;
- baixa confiança vai à moderação e nada é enviado antes da aprovação;
- conta A não vê/responde comentário da B;
- retry de outbox não duplica reply;
- workflows WhatsApp/Automação/Calendário permanecem sem diff.
