# Runbook de rollout R0–R5 — Omnichannel

Este roteiro executa o rollout definido em `E10_ROLLOUT.md`. Ele não autoriza deploy, conexão de
provider, envio real, ativação de workflow n8n nem mudança de coorte. Cada etapa exige ambiente,
owner e janela previamente autorizados.

## Regras invariáveis

1. PostgreSQL e API Go são autoritativos; n8n somente orquestra e nunca envia ao canal.
2. Trabalhe em uma account por vez e confirme o contexto antes de coletar evidência.
3. Comece com rollout explícito em `off`, `observe`, `shadow` ou `assist`; não use o fallback legado
   `active` em piloto novo.
4. Toda alteração registra revisão esperada, motivo e ator. Conflito `409` exige recarregar; nunca
   sobrescreva a configuração concorrente.
5. Antes de qualquer efeito externo, confirme health sem alerta crítico, outbox estável, provider e
   binding da mesma account e kill switch testado.
6. Segredos, QR, telefone completo, texto de conversa e URLs assinadas não entram nas evidências.
7. Qualquer vazamento cross-tenant, duplicata, resposta crítica insegura ou perda de ingestão pausa
   o rollout imediatamente, conforme `OPERACAO_E9.md`.

## Pacote de evidência por etapa

Registre em um arquivo datado ou chamado de mudança:

- ambiente, account, instância, provider, owner e janela;
- revisão e configuração do rollout antes/depois, sem segredos;
- resultado de `GET /v1/omnichannel/operations/health`;
- volume de inbound, deduplicação, outbox, DLQ, latência, custo e handoffs;
- amostra anonimizada e decisão `avançar`, `manter` ou `recuar`, com ator e horário;
- resultado do kill switch e do retorno ao modo anterior.

## Pré-voo comum

- Deploy/migrations foram aprovados e a API reporta a migration esperada.
- Membership, módulo, permissões e grants da account piloto estão revisados.
- Número, binding cliente×canal e automation profile pertencem ao mesmo escopo.
- Backup/restore e on-call de E9 têm evidência válida no ambiente alvo.
- Templates, consentimento, opt-out, horários e limites do canal foram aprovados.
- O painel “Saúde e rollout” carrega sem fallback implícito e o operador humano consegue atender.
- O owner consegue aplicar `paused` e confirmar que ingest/CRM/inbox humano permanecem ativos.

Se qualquer item falhar, não iniciar a etapa.

## R0 — demonstração interna

1. Use conta e número de teste, fixtures sem dados pessoais e modo inicial `observe`.
2. Demonstre inbound, mídia, reply humano, CRM, classificação, handoff e auditoria/custo.
3. Passe para `shadow` somente para demonstrar decisão da IA sem envio.
4. Opcionalmente use `assist` para gerar um draft; somente uma ação humana pode enviá-lo.
5. Aplique `paused` durante uma tentativa controlada e confirme ausência de efeito automático.

Aceite: fluxo reproduzível do zero, nenhuma saída IA não aprovada, nenhuma duplicata/vazamento e
evidência completa. Ao terminar, retornar ao modo seguro aprovado pelo owner.

## R1 — observe

1. Mantenha IA sem execução/envio por 24–48 h no número piloto.
2. Meça inbound, dedupe, mídia, latência, backlog, DLQ e capacidade humana.
3. Compare webhooks aceitos com mensagens/conversas persistidas e trate qualquer lacuna.

Aceite: zero perda, zero duplicata, zero cross-tenant e SLOs E9 estáveis durante a janela. Caso
contrário, manter `observe` ou aplicar `paused` e abrir incidente.

## R2 — shadow

1. Configure `shadow`, coorte/allowlists e teto de custo; `autoReplyPercent` não concede envio.
2. Processe o conjunto rotulado e uma amostra do tráfego piloto sem criar efeito externo.
3. Meça schema válido, intenção, rota, campos, falso handoff, segurança, latência e custo.

Aceite: 100% schema/fallback seguro, rota correta >= 90%, nenhuma resposta crítica insegura e custo
dentro do budget. Falha crítica retorna a `observe` ou `paused`.

## R3 — assist

1. Configure `assist` apenas para filas/horários aprovados.
2. Confirme que o draft persiste, aparece no inbox e não cria outbox automático.
3. Operadores registram uso, edição, descarte e motivo; respostas por fora devem expirar o draft.
4. Meça taxa de uso, edição, rejeição, qualidade, latência e handoff.

Aceite: nenhum envio sem POST humano, auditoria íntegra e qualidade aceita pelo owner. Ajustes de
prompt/policy geram nova versão; não editar versão ativa silenciosamente.

## R4 — auto piloto

1. Inicie em 5–10% de conversas elegíveis, horário comercial, filas/intents allowlisted e sem tags
   sensíveis.
2. Observe uma janela completa antes de cada promoção 10→25→50→100%.
3. Antes de promover, anexe métricas e decisão explícita do owner.
4. Teste `paused` em condição controlada e confirme que job atrasado também falha fechado.

Aceite por janela: zero duplicata/cross-tenant, nenhum crítico inseguro, SLOs e custo dentro do
limite, handoff utilizável >= 95% e backlog/DLQ sem crescimento sustentado.

## R5 — expansão

1. Adicione apenas uma dimensão por mudança: fila, horário, número ou account.
2. Repita pré-voo e uma janela reduzida R1→R4 para cada nova dimensão.
3. WhatsApp oficial é aprovado por número; Instagram começa por DM. Comentário público exige
   aprovação específica.
4. Não reutilize grant, credencial ou binding de outra account.

Aceite: cada expansão tem evidência própria, rollback testado e owner definido. “Funcionou na
account anterior” não substitui validação tenant-scoped.

## Rollback e encerramento

1. Aplique `paused` com motivo e revisão esperada.
2. Confirme que novos dispatches e claims automáticos cessaram e que o inbox humano segue ativo.
3. Drene somente efeitos já autorizados/idempotentes; não reprocese eventos em massa.
4. Reverta, conforme a causa, versão do modelo/prompt, modo da IA, workflow próprio ou provider por
   número. Não faça rollback destrutivo de migration aditiva.
5. Só retome após health estável, causa registrada, evidência do teste e nova decisão do owner.

## O que permanece necessariamente externo

R0–R5 só podem receber `PASS` global depois de executados no ambiente autorizado com provider,
operadores, métricas e janelas reais. Testes locais comprovam o mecanismo; não substituem App
Review, pareamento, canário, on-call, restauração de backup real ou aceite humano.
