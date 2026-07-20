# Plano técnico de evolução — atendimento omnichannel inteligente

Decisão vigente desde 2026-07-20. Este documento organiza a evolução posterior ao
piloto F0–F14 de `PLANO_ATENDIMENTO.md`. O plano antigo continua como histórico do port;
este é o roteiro executivo para tornar o atendimento funcional, multicanal e operável.

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
- WhatsApp Cloud e Instagram ainda não possuem adapters reais.

### Legado em corte

- o workflow n8n `Whatsapp`/WAHA de envio direto foi desativado localmente em 2026-07-20
  e removido do conjunto versionado/importável;
- o container WAHA e o módulo Go `automation` permanecem temporariamente porque a tela
  antiga ainda usa QR/status. Eles não podem voltar a responder mensagens;
- volumes WAHA permanecem intactos para rollback até a migração da tela antiga.

## 4. Ordem executiva

| Fase | Resultado | Dependência | Saída obrigatória |
|---|---|---|---|
| E0 | corte seguro do sender legado | — | um único dono de envio |
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

### E0 — cortar o legado sem perder rollback

Entregas:

- desativar o workflow `lzhb5JjN5kdcVuRR` no n8n dev/prod;
- remover `workflow-whatsapp.json` do export/import/deploy;
- bloquear por teste/checagem qualquer node n8n que envie a canal no conjunto omnichannel;
- documentar que WAHA é somente dependência transitória da tela `/automation`;
- mapear consumidores de `back/internal/modules/automation` antes de remover serviço/volume;
- decidir e executar limpeza do histórico Git dos backups com PII, seguida de rotação de segredos.

Aceite:

- `n8n list:workflow --active=true` não mostra o workflow legado;
- busca versionada não encontra node WAHA/Meta/Evolution nos workflows novos;
- WhatsApp real recebe no máximo uma resposta por inbound;
- rollback é reativar explicitamente o registro preservado, nunca reimport automático.

### E1 — fechar o piloto WhatsApp funcional

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

1. concluir E0 também em produção;
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

1. Instagram DM;
2. comentários com aprovação e progressiva automação;
3. RAG avançado, tools de escrita e follow-ups proativos;
4. remover definitivamente WAHA, módulo `/automation` duplicado e adapters de costura.

## 7. Regra de atualização

Ao fechar uma fase, atualizar juntos:

- este plano;
- `docs/omnichannel/ESTADO.md` com evidência executada;
- `back/internal/modules/omnichannel/AGENT.md` e o AGENT do front quando aplicável;
- roadmap do painel;
- `docs/LEGADO.md` quando um vestígio for removido ou descoberto.

Código sem prova e documentação sincronizada não encerra a fase.
