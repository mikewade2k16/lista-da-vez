# Runbook operacional E9 — Omnichannel

Use este runbook para os alertas retornados por
`GET /v1/omnichannel/operations/health`. A rota é account-scoped e exige
`omnichannel.audit.view` ou `omnichannel.settings.manage`.

## Primeiros passos

1. Confirme a account ativa no shell; nunca copie `accountId` de outra sessão.
2. Atualize “Saúde e rollout” e anote código, severidade, horário e revisão do rollout.
3. Se houver risco de envio incorreto, cross-tenant, duplicata ou custo fora do limite, aplique
   `paused` com motivo explícito. O inbox humano deve permanecer ativo.
4. Não reenvie eventos, não limpe dead-letter e não troque provider antes de identificar a causa.
5. Segredos, telefone, texto, prompt, QR e URL assinada não entram no registro do incidente.

## Ações por alerta

| Código | Verificação | Ação segura |
|---|---|---|
| `database_unavailable` | readiness da API e PostgreSQL | manter rollout pausado; restaurar a dependência antes de liberar workers |
| `outbox_dead` | idade, tipo, provider e erro mascarado da outbox | corrigir causa; reprocessar somente IDs autorizados e idempotentes |
| `outbox_stale` | locks e worker ativo | confirmar lease expirada; nunca rodar dois consumidores manuais |
| `ai_dispatch_stuck` | dispatch, geração e estado da conversa | pausar IA; deixar o recovery canônico expirar/reencaminhar o job |
| `provider_credentials_missing` | instância ativa e credencial cifrada configurada | manter provider sem envio; corrigir pelo painel, sem logar token |
| `automation_binding_mismatch` | instance, binding e automation profile da mesma account/cliente | alinhar o vínculo; o runtime deve continuar fail-closed |
| `retention_overdue` | último `purge_runs`, erro e política da account | executar primeiro em dry-run; respeitar batch, cutoff e mídia |

## Kill switch

- `paused` bloqueia novos dispatches e saídas automáticas, incrementa a geração e devolve conversas
  ativas pela IA para operação humana.
- O gate é reavaliado antes da inferência, depois dela e na transação final antes do provider.
- Para sair de `paused`, registre motivo, revisão esperada e evidência de que o alerta foi eliminado.
- Nunca use alteração visual do frontend como mecanismo de pausa.

## Backup e restore

O exercício isolado precisa cobrir PostgreSQL, migrations, mídia e grants. Registre:

- timestamp do backup e último dado restaurado (RPO);
- duração de dump, restore e readiness (RTO);
- checksum de uma amostra de mídia;
- migration mais recente;
- contagem account-scoped de conversa/grant;
- confirmação de que nenhuma outbox insegura foi executada.

Não restaure sobre produção. Não importe workflows e não ligue providers durante o exercício.

## Encerramento do incidente

Só encerre após health estável, backlog sem crescimento, causa registrada, ação auditada e janela de
observação definida. Avanço de rollout é uma decisão separada; recuperação não implica promoção.
