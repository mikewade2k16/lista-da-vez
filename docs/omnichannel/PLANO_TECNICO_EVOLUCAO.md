# Plano técnico de evolução — atendimento omnichannel inteligente

Decisão vigente desde 2026-07-20. Este documento organiza a evolução posterior ao
piloto F0–F14 de `PLANO_ATENDIMENTO.md`. O plano antigo continua como histórico do port;
este é o roteiro executivo para tornar o atendimento funcional, multicanal e operável.

As especificações executáveis, contratos para agentes, matriz de dependências, migrations
propostas, APIs, componentes, testes, proibições e critérios de aceite ficam em
[`docs/omnichannel/evolucao/README.md`](evolucao/README.md). O plano continua sendo a visão
executiva; um agente só pode implementar uma fase quando receber também o contrato comum e um
pacote atômico daquela pasta.

## Proposta em validação — Customer Intelligence

A separação futura entre Omnichannel, dados determinísticos do cliente e runtime inteligente está
registrada em [`../customer-intelligence/GOVERNANCA.md`](../customer-intelligence/GOVERNANCA.md),
com blueprint em [`../customer-intelligence/SPECS_GERAIS.md`](../customer-intelligence/SPECS_GERAIS.md).
Enquanto esses documentos estiverem em `DRAFT`, eles não substituem as specs executáveis atuais
nem autorizam migration, cutover, workflow ou exclusão.

## Prioridade vigente — MVP de automação WhatsApp (2026-07-21)

Antes de retomar E9/E10, executar e validar o recorte
[`MVP_AUTOMACAO_ATENDIMENTO.md`](MVP_AUTOMACAO_ATENDIMENTO.md). O MVP não substitui os
contratos já implementados; organiza um caminho curto para provar em tela e em um número real:

1. perfil cliente↔número↔agente e policy configurável;
2. iniciar, sugerir encerramento e transferir com decisão final no Go;
3. página `/omnichannel/automacao` sem resposta pelo painel;
4. cards de intervenção;
5. piloto Evolution e, depois, cutover controlado para WhatsApp Cloud.

Durante esse recorte, não iniciar E9/E10, Instagram ou automação de comentários. Não tocar em
WAHA, workflow WhatsApp do módulo Automation, Calendar n8n ou workflows de outros módulos.

## 1. Objetivo final

Entregar um inbox único de WhatsApp e Instagram no qual:

1. todo evento entra pelo Go, é autenticado, deduplicado e persistido;
2. o contato é identificado ou criado automaticamente no CRM multicanal;
3. a IA conduz o primeiro atendimento, coleta dados e classifica intenção/etapa/origem;
4. regras determinísticas escolhem se a IA continua ou transfere para setor/fila;
5. o atendente assume com histórico, resumo, campos coletados e motivo do handoff;
6. toda resposta sai por outbox Go e pelo adapter do canal;
7. provider, modelo, chave, prompt, tools e políticas são configuráveis no painel;
8. workflows n8n são exportáveis sem credencial, PII, pin data ou memória de execução;
9. WhatsApp Cloud API substitui gradualmente a Evolution;
10. Instagram DM e comentários entram no mesmo contato/CRM, respeitando políticas Meta.

## 2. Arquitetura que não pode regredir

| Go/PostgreSQL | n8n |
|---|---|
| Evolution, WhatsApp Cloud e Instagram | debounce e agrupamento |
| webhooks, assinatura, dedupe e idempotência | montagem do contexto da IA |
| contatos, identidades, touchpoints e CRM | chamadas ao modelo |
| mensagens, mídias e estados | transcrição e visão |
| setores, filas, responsáveis e RBAC | consulta a tools autorizadas |
| auditoria, custos, retenção e LGPD | decisão estruturada de continuar/handoff |
| outbox e envio final | orquestração configurável |

Fluxo obrigatório:

```text
Canal -> adapter/webhook Go -> dedupe + persistência + CRM
      -> job de inteligência -> n8n -> resposta estruturada
      -> validação + FSM + routing Go -> outbox Go -> adapter do canal
```

O n8n nunca recebe webhook público como fonte autoritativa, nunca grava diretamente no
PostgreSQL do produto e nunca envia para Evolution ou Meta.

## 3. Estado de partida provado em 2026-07-20

### Entregue

- schema `messaging.*`, contatos, identidades, touchpoints e notas;
- inbox, mensagens, mídia em disco privado, realtime, outbox e worker FIFO;
- estados, setores, filas, membros, regras de roteamento e handoff;
- adapter mock e Evolution, webhook público protegido e sessão/QR;
- agente configurável, versões, runs, campos coletáveis, simulador e auditoria de custo;
- executor alternável `OMNI_AI_EXECUTOR=native|n8n`;
- workflow stateless `Omnichannel Brain` e esqueleto `Instagram First Contact`;
- contato/identidade/touchpoint criados na mesma transação do inbound;
- multi-turno inicial: `needs_human=false` mantém IA; `true` transfere; falha faz fail-open.

### Gaps funcionais imediatos

- mídia inbound real ainda pode ficar sem preview;
- quote/reply ainda precisa chegar como citação no provider;
- mensagem enviada pelo aparelho (`fromMe`) precisa ser espelhada sem duplicar;
- decisão da IA ainda usa booleano temporário em vez de contrato versionado completo;
- debounce, agrupamento, transcrição, visão e tools ainda não estão no cérebro novo;
- UI de CRM ainda não expõe identidades, origem, lifecycle, tags, notas e histórico;
- landing pages ainda não enviam atribuição completa ao CRM;
- adapters Meta WhatsApp Cloud e Instagram foram implementados localmente; permanecem pendentes
  apenas smoke/cutover controlados com credenciais reais e ativação gradual.

### Isolamento entre módulos n8n

- este plano só pode alterar `workflow-omnichannel-brain.json` e
  `workflow-instagram-first-contact.json`;
- `workflow-whatsapp.json` e WAHA pertencem ao módulo `automation` e não são legado do
  omnichannel;
- workflows de calendário, Operação e automação mantêm runtime, credenciais e ciclo de
  deploy próprios;
- script compartilhado deve aplicar validações por id/owner, nunca por suposição global.

## 4. Ordem executiva

| Fase | Resultado | Dependência | Saída obrigatória |
|---|---|---|---|
| E0 | ownership e fronteira dos workflows | — | nenhum efeito cross-module |
| E1 | piloto WhatsApp funcional | E0 | texto, mídia, quote e fromMe reais |
| E2 | cérebro n8n v2 | E1 | decisão estruturada, debounce e multi-turno |
| E3 | multimodal | E2 | áudio, imagem e documento com limites |
| E4 | CRM e atribuição inteligente | E1 | contato 360° pesquisável e editável |
| E5 | handoff operacional | E2, E4 | fila humana com contexto completo |
| E6 | tools e conhecimento | E2, E4 | consultas autorizadas e auditadas |
| E7 | WhatsApp Cloud API | E1–E6 | migração oficial por número |
| E8 | Instagram | E4–E7 | DM/comentários no inbox único |
| E9 | hardening e escala | transversal | SLO, LGPD, custo e recuperação |
| E10 | rollout | E1–E9 | piloto, expansão e rollback provados |

Não iniciar E7/E8 com o hot path E1 instável. E4 pode avançar em paralelo com E2 depois
que os contratos de contato e atribuição estiverem congelados.

## 5. Fases detalhadas

### E0 — ownership e fronteira dos workflows

Spec executável: [`E0_OWNERSHIP_WORKFLOWS.md`](evolucao/E0_OWNERSHIP_WORKFLOWS.md).

**Execução:** `DONE` em 2026-07-20, com 89 testes offline e hashes dos workflows externos
preservados. Nenhum runtime/deploy foi executado.

Entregas:

- registrar ownership explícito de cada workflow no AGENT do n8n;
- limitar toda tarefa aos workflows do módulo em escopo;
- bloquear por teste qualquer node de envio a canal somente nos workflows do omnichannel;
- preservar `workflow-whatsapp.json`, WAHA e todos os workflows de outros módulos;
- manter export/import/deploy compartilhados sem alterar estado ativo de owners não relacionados;
- remover apenas legado interno do omnichannel, como adapters de costura, quando a fase dona fechar.

Aceite:

- lista de workflows ativos permanece igual fora dos ids do omnichannel;
- busca versionada não encontra node WAHA/Meta/Evolution nos workflows do omnichannel;
- WhatsApp real recebe no máximo uma resposta por inbound;
- alteração do cérebro não gera diff nem mudança de estado em Calendar, Operation ou Automation.

### E1 — fechar o piloto WhatsApp funcional

Spec executável: [`E1_PILOTO_WHATSAPP.md`](evolucao/E1_PILOTO_WHATSAPP.md).

**Execução:** código e validação local concluídos em 2026-07-20. O aceite final permanece
pendente do smoke controlado de quote outbound e mídia inbound com a Evolution saudável. Ponto de
retomada: [`E1_PAUSA_2026-07-20.md`](evolucao/E1_PAUSA_2026-07-20.md). A preparação E2 já está em
execução local; o rollout externo continua bloqueado até o gateway seguro e o smoke controlado.

Entregas:

- baixar/persistir mídia inbound imediatamente e servir preview autenticado;
- mapear `quoted_message_id` interno para o payload correto do provider;
- ingerir `fromMe=true` como outbound espelhado e deduplicar pelo external id;
- reconciliar ACK/status, retry, dead-letter e mensagem presa em `PENDING`;
- paginação e busca de conversas/contatos nos hot paths;
- smoke automatizado do adapter mock e smoke manual controlado na Evolution.

Aceite:

- texto, imagem, áudio, documento, reply e mensagem pelo aparelho aparecem uma vez e na ordem;
- repetir webhook e repetir idempotency key não cria duplicata;
- provider indisponível não perde inbound e produz erro/dead-letter observável;
- assumir manualmente bloqueia a IA imediatamente.

### E2 — cérebro n8n v2

Spec executável: [`E2_CEREBRO_N8N_V2.md`](evolucao/E2_CEREBRO_N8N_V2.md).

**Execução atual:** `IN_PROGRESS`. Contrato, migrations `0216`, dispatch durável/outbox, worker,
policy strict (incluindo limite de turnos/confiança e fallback de silêncio) e parâmetros do painel estão implementados e testados localmente. Permanecem
pendentes a importação/ativação controlada do workflow, a timeline/badge de inspeção e o QA/shadow.
O executor n8n segue opt-in no boot: sem `OMNI_N8N_INTERNAL_TOKEN` ele usa o caminho nativo como rollback seguro; com o token e o webhook configurados, ativa o gateway cifrado `brain.result.v2`.

Entregas:

- disparo assíncrono por job após commit, não goroutine solta no hot path;
- janela de debounce configurável por conta/conversa e agrupamento idempotente;
- contrato `schema_version` com `continue_ai|handoff|no_reply`;
- contexto autoritativo com contato, origem, horário, resumo, histórico limitado, filas,
  campos pendentes e tools permitidas;
- saída com `reply_draft`, intenção, categorias, etapa do lead, setor/fila sugeridos,
  confiança, campos extraídos, motivo, uso e custo;
- validação do schema e catálogos no Go; baixa confiança e timeout fazem fail-open;
- teto de turnos, cooldown, cancelamento quando humano assume e circuit breaker do n8n;
- testes de contrato e fixtures anonimizadas.

Aceite:

- mensagens rápidas geram uma execução e uma resposta;
- falha/retry não responde duas vezes;
- alterar provider/modelo/prompt/chave no painel muda a próxima execução sem editar workflow;
- desligar n8n permite rollback temporário `native`, sem mudar dados/estado;
- workflow exportado tem `pinData={}`, `staticData=null`, save de execução desligado e zero node de canal.

### E3 — áudio, visão e documentos

Spec executável: [`E3_MULTIMODAL.md`](evolucao/E3_MULTIMODAL.md).

**Execução atual:** `IN_PROGRESS`. Contratos/fixtures, `media_config` versionado e migration
`0219_messaging_media_analyses` foram entregues e aplicados no Postgres local; a persistência
idempotente, consulta autorizada e stream interno `media-stream.v1` também estão codificados.
O policy/job que agenda análises, branches n8n, render no inbox e QA Evolution continuam
pendentes; nenhuma chamada multimodal é ativada por esta fatia.

Entregas:

- normalização de anexos no Go com MIME/tamanho/hash/tenant;
- transcrição e visão no n8n usando o modelo configurado no painel;
- OCR/extração de documento somente para tipos permitidos;
- resultado derivado persistido separadamente do binário e auditado;
- proteção contra prompt injection em arquivo, SSRF, decompression bomb e custo excessivo;
- fallback para humano quando mídia não puder ser interpretada.

Aceite:

- áudio/imagem chegam uma vez ao modelo e entram no contexto agrupado;
- arquivo acima do limite é recusado com mensagem acionável;
- chave, URL privada e base64 não aparecem em execution data ou export.

### E4 — CRM e atribuição 360°

Spec executável: [`E4_CRM_ATRIBUICAO.md`](evolucao/E4_CRM_ATRIBUICAO.md).

**Execução atual:** `IN_PROGRESS`. DB-02, API/BE-03/04, merge/undo BE-05, captura LP-06 e FE-07
(lista cursorizada, filtros, drawer 360°, edição inline, notas e merge assistido) estão implementados
localmente; migrations `0217/0218` foram aplicadas no Postgres do Compose. Ficam para a sequência
segmentos, export/consentimento, confirmação CRM/ERP, ingestão de identidade inbound e QA-08.
Nenhuma rota pública foi publicada na VPS.

Entregas:

- tela de contato com identidades WhatsApp/Instagram, touchpoints, lifecycle, tags,
  notas, campos customizados, consentimento e timeline;
- merge seguro de duplicatas com auditoria e undo operacional;
- regras autoritativas para `new_lead`, `known_lead`, `customer`, `inactive`;
- ingestão de landing page, formulário, campanha, anúncio, UTM e referrer;
- primeira/última interação calculadas pelos eventos persistidos;
- busca, filtros, segmentos e export respeitando conta/permissão/LGPD;
- endpoint de integração CRM/ERP para confirmar cliente existente.

Aceite:

- primeiro contato cria um CRM; contato recorrente reutiliza o mesmo cadastro;
- WhatsApp e Instagram podem convergir no mesmo contato por regra auditável;
- a IA recebe origem e status do cliente do banco, nunca os inventa;
- operador consegue corrigir classificação sem editar SQL.

### E5 — handoff e operação humana

Spec executável: [`E5_HANDOFF_OPERACIONAL.md`](evolucao/E5_HANDOFF_OPERACIONAL.md).

Entregas:

- políticas configuráveis por setor, horário, confiança, intenção, cliente e SLA;
- resumo de handoff com dados coletados, faltantes, motivo e última ação da IA;
- fila, atribuição, aceite, transferência, reabertura e encerramento auditados;
- aviso explícito ao cliente na transição quando a política exigir;
- SLA, prioridade, presença e notificação do responsável;
- bloqueio forte da IA em `human_active`, inclusive durante execução em voo.

Aceite:

- conversa chega à fila correta e só membros autorizados a veem;
- humano assume sem a IA responder depois;
- retransferência e encerramento preservam histórico e explicação da decisão.

### E6 — tools, automações e conhecimento

Spec executável: [`E6_TOOLS_CONHECIMENTO.md`](evolucao/E6_TOOLS_CONHECIMENTO.md).

**Fechamento local 2026-07-21:** `DONE` para a plataforma de tools e conhecimento. Aprovações e
evidências mascaradas foram integradas no Go e no painel, com `0225_messaging_ai_tool_approvals.sql`,
rotas tenant-scoped e retry assinado. A validação local passou; E7–E10 continuam pausadas e não
foram iniciadas.

Adapters corporativos sem contrato estável não foram inventados: bindings sem handler Go falham
fechado e auditado. Importação/ativação do workflow no runtime n8n, smoke Evolution e deploy não
fazem parte deste fechamento.

**Execução:** `DONE (local)`. Inventário, migrations `0222`–`0225`, CRUD de bindings/configuração,
gateway interno idempotente/auditado, ingestão manual com busca FTS/evidências, loop n8n assinado,
aprovações humanas e a aba de configuração de tools/conhecimento foram implementados e validados
localmente.

Entregas:

- registry Go de tools com schema, permissão, timeout, rate limit e auditoria;
- tools de leitura para catálogo, preço, estoque, agenda, CRM e status de pedido;
- tools de escrita passam por confirmação/política e endpoints Go idempotentes;
- base de conhecimento com versão, escopo de conta e fontes citáveis;
- n8n apenas solicita a tool; Go autentica, executa e devolve resultado mínimo;
- testes contra prompt injection e escalada de permissão.

Aceite:

- nenhuma tool acessa SQL diretamente ou usa conta vinda do modelo;
- toda chamada tem conta, agente, conversa, latência e resultado auditados;
- falha de tool não produz informação inventada e pode transferir para humano.

### E7 — WhatsApp Cloud API oficial

Spec executável: [`E7_WHATSAPP_CLOUD_API.md`](evolucao/E7_WHATSAPP_CLOUD_API.md).

**Execução:** `CODE-COMPLETE` local em 2026-07-21. Migration `0226`, adapter Meta, HMAC/challenge,
templates, janela de 24h, policy de template/outbox e configuração segura por número estão
implementados e testados. Falta somente o smoke/cutover com credenciais Meta reais.

Entregas:

- adapter `meta_whatsapp_cloud` implementando `channel.Provider`/capabilities;
- verificação do webhook Meta, HMAC, tokens cifrados e rotação;
- status, mídia, reply/context, templates e janela de 24h;
- configuração por número no painel e health/capability reais;
- runbook de migração, coexistência controlada e rollback por número;
- embedded signup somente depois do fluxo manual estar estável e do app review.

Regra de segurança: um número nunca fica ativo simultaneamente na Evolution e na Cloud API.

Aceite:

- inbound/outbound oficial passa pelos mesmos contratos, CRM, FSM e outbox;
- fora da janela a UI/IA só usa template permitido;
- migração e rollback não duplicam eventos nem perdem histórico.

### E8 — Instagram DM e comentários

Spec executável: [`E8_INSTAGRAM.md`](evolucao/E8_INSTAGRAM.md).

**Execução:** `CODE-COMPLETE` local em 2026-07-21. Migration `0227`, adapter Instagram, DM/comentário,
CRM/inbox único, moderação, outbox de ações, rotas account-scoped, painel e workflow owner-scoped
foram implementados. Comentário/menção nunca publica automaticamente: a IA só grava rascunho e a
aprovação humana dispara o job Go. Falta smoke Meta controlado e validação de capabilities/App Review.

Entregas:

- adapter Meta Instagram e webhooks validados no Go;
- identidade `instagram` vinculada ao CRM e ao inbox existente;
- workflow separado para primeiro contato, reutilizando o cérebro comum;
- políticas distintas para DM, menção e comentário público;
- allowlist/denylist, moderação, limite por post e aprovação humana inicial;
- correlação campanha/post/comentário -> touchpoint -> contato;
- respostas públicas/privadas somente pelas capabilities e outbox Go.

Aceite:

- DM aparece no mesmo painel com badge de canal e contato correto;
- comentário elegível é classificado sem publicação duplicada;
- conteúdo sensível, spam ou baixa confiança vai para moderação humana.

### E9 — segurança, LGPD, observabilidade e escala

Spec executável: [`E9_HARDENING_ESCALA.md`](evolucao/E9_HARDENING_ESCALA.md).

Entregas:

- métricas por etapa: webhook, dedupe, job, n8n, modelo, tool, routing e outbox;
- traces com correlation id sem conteúdo/PII; dashboards e alertas por SLO;
- retenção, export, anonimização, consentimento e direito do titular;
- orçamento/limite de tokens e custo por conta/agente/canal;
- rate limit distribuído, QR/cache compartilhado e execução com múltiplas APIs;
- backup/restore de banco, mídia, n8n e configurações; disaster recovery testado;
- testes de carga, chaos do provider/n8n e revisão de segurança multi-tenant.

Aceite:

- nenhuma mensagem fica invisivelmente presa;
- vazamento cross-tenant e replay são cobertos por testes;
- RPO/RTO, SLO e alertas estão documentados e provados.

### E10 — rollout e operação

Spec executável: [`E10_ROLLOUT.md`](evolucao/E10_ROLLOUT.md).

Ondas:

1. shadow: IA classifica, mas humano responde;
2. piloto interno: IA responde intents seguras e transfere o restante;
3. piloto de uma conta/número com métricas e rollback imediato;
4. expansão por setor/horário/intenção;
5. WhatsApp oficial por número;
6. Instagram DM; comentários em modo aprovação; depois automação por política.

Gates de promoção:

- zero duplicata/envio indevido no período definido;
- taxa de handoff, precisão de roteamento e satisfação dentro da meta;
- P95 de resposta, erro e custo dentro do orçamento;
- segurança/LGPD e rollback aprovados;
- suporte possui runbook e painel de diagnóstico.

## 6. Backlog priorizado imediato

### Agora — P0

1. consolidar E0 nos scripts e runbooks compartilhados sem mudar outros runtimes;
2. corrigir mídia inbound, quote e `fromMe`;
3. substituir disparo solto da IA por job idempotente;
4. fechar contrato versionado do cérebro e debounce;
5. provar o fluxo real Evolution ponta a ponta.

### Em seguida — P1

1. CRM 360° e atribuição de landing pages;
2. áudio/visão;
3. handoff operacional e SLA;
4. tools de leitura;
5. adapter WhatsApp Cloud.

### Depois — P2

1. smoke e cutover controlado dos adapters Meta já implementados;
2. comentários com aprovação e progressiva automação;
3. RAG avançado, tools de escrita e follow-ups proativos;
4. remover os adapters de costura e demais legados internos do próprio omnichannel.

## 7. Regra de atualização

Ao fechar uma fase, atualizar juntos:

- este plano;
- `docs/omnichannel/ESTADO.md` com evidência executada;
- `back/internal/modules/omnichannel/AGENT.md` e o AGENT do front quando aplicável;
- roadmap do painel;
- `docs/LEGADO.md` quando um vestígio for removido ou descoberto.
- a spec `docs/omnichannel/evolucao/E<n>_*.md` e sua matriz de dependências, quando o contrato ou
  a ordem tiverem mudado.

Código sem prova e documentação sincronizada não encerra a fase.
