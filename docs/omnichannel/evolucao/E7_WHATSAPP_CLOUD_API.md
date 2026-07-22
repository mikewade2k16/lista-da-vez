# E7 — WhatsApp Cloud API oficial

**Status:** `CODE-COMPLETE` local em 2026-07-21; smoke/cutover Meta real ainda pendente

**Resultado:** um número migra da Evolution para a API oficial da Meta sem duplicar webhook/envio,
mantendo o mesmo domínio, CRM, IA, fila, outbox e inbox.

## 1. Referência e decisões fechadas

A implementação deve reconferir a versão suportada da Graph API no momento do pacote. Não fixar
uma versão antiga no código por copiar exemplo. Referências primárias atuais:

- [coleção oficial WhatsApp Cloud API da Meta](https://www.postman.com/meta/whatsapp-business-platform/documentation/wlk6lh4/whatsapp-cloud-api);
- [mensagens e tipos suportados](https://www.postman.com/meta/whatsapp-business-platform/folder/o48mro7/messages);
- [webhooks e payloads de status](https://www.postman.com/meta/whatsapp-business-platform/folder/vzaxn16/webhook-payload-reference);
- [mídia e permissões](https://www.postman.com/meta/whatsapp-business-platform/folder/13382743-ecb27be5-4d27-4763-bbee-6a8002c04bf3);
- [templates](https://www.postman.com/meta/whatsapp-business-platform/request/o65u5m5/send-message-template-text).

Decisões:

- migração é por número; nunca Evolution e Cloud ativos para o mesmo número;
- o adapter implementa a mesma interface de canal já usada pelo domínio;
- política da janela/template é Go/PostgreSQL; n8n não decide nem envia;
- P0 usa configuração manual assistida no painel; Embedded Signup é pacote P1;
- token/app secret/verify token ficam cifrados; IDs públicos ficam em `provider_config` validado;
- Graph API version é config de plataforma allowlisted, não texto livre por tenant.

## 2. Banco

Reutilizar `messaging.whatsapp_instances` com `provider='meta_whatsapp_cloud'`. Validar schema do
`provider_config`:

```json
{
  "wabaId": "...",
  "phoneNumberId": "...",
  "businessPortfolioId": "...",
  "appId": "...",
  "graphVersion": "allowlisted",
  "webhookMode": "waba_override"
}
```

`credentials_ciphertext` guarda envelope versionado com access token, app secret e verify token;
front recebe apenas `{set,last4/updatedAt}` por segredo.

Criar `messaging.whatsapp_templates`:

`id/account_id/instance_id/meta_template_id/name/language/category/status/components/quality_rating/
last_synced_at/created_at/updated_at`, unique por instance/name/language. Status é sincronizado da
Meta e nunca “aprovado” manualmente.

Criar `messaging.channel_windows` genérica:

`account_id/conversation_id/provider/window_kind/opened_at/expires_at/source_message_id/
updated_at`, unique por conversation/provider/window_kind. Inbound válido atualiza a janela de
atendimento de forma monotônica. A policy sempre reavalia na hora do claim da outbox.

Adicionar campos de provider message/status somente se E1 não cobrir; não criar tabela paralela de
mensagens.

## 3. Adapter Meta

Implementar `channel/meta_whatsapp` separado em arquivos pequenos:

- `parse.go`: webhook Meta → evento canônico E1;
- `verify.go`: handshake GET e HMAC `X-Hub-Signature-256` do POST;
- `client.go`: Graph API com timeout, auth, retry-after e erros tipados;
- `sender.go`: text, reply, media, template e mark-read conforme capability;
- `media.go`: obter URL por media ID e baixar autenticado imediatamente;
- `templates.go`: listar/sincronizar status e componentes;
- fixtures/testes sem rede real.

O endpoint público deve suportar verificação GET e evento POST. O accountSlug só seleciona
configuração de callback; o payload `phone_number_id` precisa casar com a instância daquela conta.
Assinatura inválida falha antes de persistir. Event ID/dedupe usa IDs Meta e tipo/timestamp sem
hash de body instável.

A documentação oficial registra que mensagens usam `/{phone-number-id}/messages`, status chegam
por webhook e mídia recebida traz media ID; a URL de download é temporária e exige token. Portanto,
persistência de bytes segue o pipeline E1 e ACKs alimentam o mesmo status monotônico.

## 4. Policy de envio

No claim da outbox:

1. carregar provider/capabilities/janela atuais;
2. se dentro da customer-service window, permitir free-form suportado;
3. fora da janela, exigir template aprovado e parâmetros válidos;
4. se não houver template/policy, bloquear com erro acionável no inbox; não tentar free-form;
5. respeitar opt-out, qualidade/limite/rate e estado da instância;
6. enviar pelo adapter e reconciliar `wamid`/status via webhook.

Template não é texto livre. UI seleciona template aprovado, idioma e parâmetros conforme
components. Retry mantém idempotency key local; como provider pode não oferecer exatamente-once,
reconciliação consulta external message ID antes de novo side effect quando houver incerteza.

## 5. Painel e inbox

- wizard pede IDs e segredos em etapas, testa conexão sem salvar segredo em browser/log;
- mostra número, nome verificado, WABA, qualidade, webhook e capabilities;
- ação “validar configuração” faz chamadas read-only e explica item a item;
- template manager sincroniza status, idioma/componentes e não permite marcar aprovado;
- composer detecta janela fechada pelo backend, desabilita free-form e abre seletor de template;
- badges distinguem Evolution e Oficial sem bifurcar o inbox;
- provider switch exige checklist, confirmação e permissão administrativa.

## 6. Cutover por número

1. inventário e backup de config/estado;
2. configurar instância Meta inativa e validar webhook/credenciais/templates;
3. rodar webhook em shadow sem responder e provar dedupe/persistência;
4. drenar outbox Evolution daquele número;
5. congelar envio no provider antigo do omnichannel;
6. garantir um único callback/provider/sender ativo;
7. ativar Meta e executar smoke texto/mídia/reply/ACK/template/falha;
8. observar SLO/DLQ/duplicatas;
9. rollback reponta provider e webhook, sem reprocessar eventos já deduplicados.

WAHA/Automação não participa e não pode ser alterado.

## 7. Pacotes atômicos

| Pacote | Entrega |
|---|---|
| `E7-CONTRACT-01` | capabilities, config e fixtures Meta |
| `E7-DB-02` | templates/windows/complementos |
| `E7-BE-03` | verificação/webhook/parser/dedupe |
| `E7-BE-04` | client/sender/media/status |
| `E7-BE-05` | policy janela/template/outbox |
| `E7-FE-06` | wizard, health, templates e composer |
| `E7-RUNBOOK-07` | cutover/rollback por número |
| `E7-QA-08` | contract tests + canário controlado |

## 8. Aceite

- assinatura/verify token inválidos não criam evento;
- phone number ID de outra conta não é aceito;
- texto, reply, mídia e ACK convergem no mesmo modelo E1;
- URL de mídia expirada é renovada/retry sem perder mensagem;
- fora da janela, free-form é bloqueado e template aprovado funciona;
- token rotacionado não exige editar workflow;
- Meta e Evolution nunca enviam para o mesmo número no teste de cutover;
- rollback preserva histórico, dedupe e outbox;
- logs/exports não contêm access token, app secret ou verify token.
