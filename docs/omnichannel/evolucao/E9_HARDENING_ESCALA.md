# E9 — segurança, LGPD, observabilidade e escala

**Status:** `DRAFT`; requisitos locais são aplicados desde E0

**Resultado:** o módulo possui SLOs mensuráveis, isolamento/segredos testados, retenção executada,
backlog/custo operáveis e restauração de banco+mídia provada.

## 1. Princípio de execução

Esta fase começa com auditoria do que já existe: F13, `messaging.purge_runs`, logmask, secretbox,
outbox/jobs, rate limit, SSRF, métricas e backup. Não criar controle duplicado. Cada gap vira pacote
pequeno com owner e evidência.

## 2. SLOs iniciais e indicadores

Valores são baseline do piloto; o owner pode apertar após medir, nunca afrouxar silenciosamente:

| Indicador | SLO piloto | Medição |
|---|---:|---|
| ACK do webhook válido | p95 < 500 ms | entrada HTTP até resposta, sem download/modelo |
| evento até aparecer no inbox | p95 < 2 s | provider occurredAt/receivedAt até realtime/read |
| resposta IA texto | p95 < 15 s | última msg agregada até outbox `done` |
| handoff até fila | p95 < 3 s | decision handoff até state queued |
| envio pelo provider | >= 99% sem falha permanente | outbox por provider/tipo |
| duplicata persistida/enviada | 0 | conflitos dedupe + auditoria amostral |
| dispatch IA preso | 0 > timeout+grace | ai_dispatches processing |
| outbox presa | 0 > lock timeout | outbox processing |
| restauração | RPO 24h/RTO medido | exercício documentado |

Métricas usam IDs técnicos, tenant/provider/status e histogramas; não levam texto, telefone, nome,
email, prompt, URL assinada ou segredo. Cardinalidade é limitada: conversation/message ID fica em
log de investigação controlado, não label de métrica.

## 3. Observabilidade operacional

Dashboards/queries mínimas:

- webhook rate/invalid signature/dedupe/latência por provider;
- tamanho/idade/falha/dead-letter da outbox;
- dispatch AI por outcome, schema error, timeout, tokens/custo/modelo;
- mídia queued/failed/bytes/tempo e espaço em volume;
- conversas por state/fila/SLA;
- tool calls por status/latência/approval;
- Meta/Instagram quality, window/template/policy blocks;
- purge último sucesso/erro/linhas/bytes por conta;
- realtime conectado/fallback polling;
- QR/session cache hit, expiração e inconsistência entre réplicas, sem conteúdo do QR em métrica.

Alertas têm threshold, janela, severidade, owner e runbook. Alerta sem ação associada não é aceite.
Endpoints de health distinguem processo vivo, dependências prontas e degradação de provider/n8n.

## 4. Segurança

### 4.1 Isolamento

- testes cross-tenant para todo novo store/endpoint/job/realtime;
- `account_id` em toda query, inclusive UPDATE/DELETE e subquery;
- IDs em payload de job são revalidados contra a conta produtora;
- websocket/ticket não assina canal de outra conta;
- filas/membros/tools/bases/instâncias validam relacionamento no banco.

### 4.2 Segredos e chamadas internas

- inventário de ciphertext/last4/rotation/last-used; nenhuma leitura devolve segredo;
- chave mestra tem versionamento e runbook de rotação;
- Go↔n8n: HMAC/mTLS conforme infraestrutura, timestamp, nonce, clock skew, replay cache, timeout;
- tokens Meta/Instagram com menor escopo possível e validação periódica;
- export n8n e fixtures passam scanner de segredo.

### 4.3 Entrada não confiável

- limite antes de JSON/multipart; MIME por conteúdo; filename ignorado para path;
- SSRF em URLs de provider/tool/knowledge, redirects e DNS rebinding;
- HMAC do webhook antes de parse/persistência do body;
- schema fechado para output IA/tool;
- prompt injection e conteúdo malicioso não ampliam tool/policy;
- rate limit por IP no público e por account/provider/action no domínio.

Se o rate limiter atual for somente memória e houver mais de uma réplica, criar/adotar backend
compartilhado na abstração `platform/ratelimit`; não espalhar implementação em handlers.

## 5. LGPD e retenção

Preservar a política já implementada, validando-a com dados reais:

- `audit`: 365 dias;
- `conversation`: 180 dias, ancorada em `last_message_at`;
- `ai_io`: 90 dias, scrub de input/output preservando tokens/custo;
- `ephemeral`: 30 dias;
- órfão de mídia: janela de carência antes da exclusão.

Primeiro run em ambiente real é dry-run e produz `purge_runs`. Poda é tenant-scoped, em batches,
com teto de tempo, arquivo antes da linha e contagem de bytes. Export/anonimização do titular é
pacote separado após decisão jurídica sobre formato, autorização e entrega; não improvisar delete.

O pacote de direitos do titular deve implementar, quando essas decisões estiverem aprovadas:

- busca determinística do titular e verificação/autorização do pedido;
- export assíncrono de contato, identities, consents, touchpoints, conversations/messages e mídia
  conforme escopo legal, em artefato privado cifrado, auditado e com expiração;
- anonimização idempotente preservando métricas legais/financeiras sem PII, com tratamento explícito
  de conversa aberta, external refs, arquivos, logs e backups;
- revogação/consentimento por propósito e canal alimentando policy de envio;
- trilha do pedido, aprovador, execução, falhas e evidência de conclusão.

## 6. Backup e recuperação

O backup deve cobrir:

- PostgreSQL com migrations/status;
- volume `OMNICHANNEL_MEDIA_DIR`/uploads;
- configuração exportável dos dois workflows próprios sem credenciais;
- inventário cifrado de configuração necessário para reconstrução;
- runbooks de provider/webhook/DNS/proxy.

Teste de restore ocorre em ambiente isolado: restaurar banco, mídia, iniciar API, validar migrations,
abrir conversa/mídia, processar outbox segura e manter workflows não próprios intocados. Medir RTO,
timestamp do último dado (RPO) e checksums. Backup nunca é provado só por “arquivo existe”.

## 7. Escala e degradação

- cargas com distribuição realista por conversa; preservar FIFO por ordering key;
- claim com `SKIP LOCKED`, lease/heartbeat e recuperação de worker morto;
- backpressure por conta impede tenant ruidoso de monopolizar workers;
- circuit breaker/retry-after para n8n/model/provider; erro permanente vai dead-letter;
- budget mensal e limite de concorrência por conta/modelo;
- mídia grande não ocupa memória inteira; stream e limites;
- queries hot path passam EXPLAIN com cardinalidade representativa;
- degradação: IA/n8n fora transfere ou segura segundo config; inbox humano continua.

Para múltiplas réplicas da API, QR/session cache e leases não podem depender de mapa local. Evoluir
a abstração existente para store compartilhado com TTL e compare-and-swap, sem persistir QR além do
necessário. Testar duas APIs concorrentes: uma cria/atualiza, outra lê/expira e ambas concordam.

## 8. Frontend administrativo

Uma visão de Saúde do Omnichannel mostra estado de providers/workflows/outbox/AI/mídia, atraso,
último webhook, últimos erros mascarados, custo/limite e último purge/backup conhecido. Ações de
retry/requeue exigem permissão, confirmação, idempotência e auditoria. Nunca oferecer “retry all”
sem filtro/limite.

## 9. Pacotes atômicos

| Pacote | Entrega |
|---|---|
| `E9-AUDIT-01` | matriz gap×controle×evidência, somente leitura |
| `E9-OBS-02` | métricas, health, alertas e runbooks |
| `E9-SEC-03` | tenant/segredos/webhook/internal-call hardening |
| `E9-LGPD-04` | dry-run/purge/masking/retention comprovados |
| `E9-PRIVACY-05` | consentimento, export e anonimização após decisão jurídica |
| `E9-BACKUP-06` | backup de mídia + restore isolado |
| `E9-SCALE-07` | carga, fairness, leases, QR cache e backpressure |
| `E9-FE-08` | painel de saúde/custo/DLQ |
| `E9-QA-09` | fault injection e checklist independente |

## 10. Aceite

- SLOs têm consulta/dashboard e alerta acionável;
- falha de n8n/provider/worker não perde evento nem bloqueia atendimento humano;
- restart recupera dispatch/outbox sem duplicar envio;
- suite cross-tenant cobre todas as entidades E1–E8;
- secret scan não encontra credenciais em Git/export/log;
- dry-run e purge por conta produzem evidência sem afetar conta vizinha;
- export/anonimização/consentimento têm autorização, idempotência e evidência, quando liberados;
- restore de banco+mídia abre conteúdo e registra RPO/RTO;
- carga não quebra FIFO por conversa nem monopoliza por tenant; duas APIs compartilham QR/leases;
- painel não expõe PII/segredo e retry é auditado.
