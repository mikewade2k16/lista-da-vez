# Plano de execução — acesso por WhatsApp, limpeza de histórico e finalização do Omnichannel

Status: P0–P4 concluídos localmente; P6 validado no escopo local; núcleo local de P7/P8 implementado e validado
Data-base da análise: 2026-08-27
Escopo deste arquivo: orientar implementação, testes, revisão, rollout e rollback
Última atualização de execução: 2026-08-28

## Atualização executiva — 2026-08-28

- **P0 PASS local:** reset lógico por conexão, cutoff uniforme, auditoria e invalidação opaca.
- **P1 PASS local:** grants relacionais, resolver canônico e realtime fail-closed.
- **P2 PASS local:** API/painel de grants, revisão otimista e remoção dos gates de papel.
- **P3 PASS local:** número próprio pertence à account do cliente; agência só acessa cliente com
  `core.account_users` explícito, permissões e grants válidos.
- **P4 PASS local:** binding cliente×canal×automação é validado antes do dispatch e novamente antes
  do efeito externo; divergência falha fechada.
- **P6 local PASS:** suites Go/frontend, integrações PostgreSQL e guardas de ownership n8n estão
  verdes. Canários reais Evolution/Meta, App Review e cutover continuam externos e pendentes.
- **P7 núcleo local code-complete:** health operacional protegido, alertas acionáveis, rate limit e
  QR compartilhados entre réplicas, integração cross-tenant e restore isolado de banco+mídia
  comprovados. Concorrência entre réplicas e falha fechada do rate limit foram provadas localmente;
  carga multi-account prolongada, alertas de produção/on-call e restore do backup real da VPS
  permanecem pendentes; portanto P7 global ainda não é PASS.
- **P8 núcleo local code-complete:** modos tipados, coorte determinístico, limites, horários,
  allowlists, auditoria, painel, draft humano persistido do modo `assist` e kill switch server-side
  antes e depois da IA e antes do provider. O draft nunca cria outbox automático e uso/edição/
  descarte/expiração são auditáveis.
  Evidência do delta: 114 arquivos/519 testes frontend verdes, Go test/vet verdes no escopo e lint
  do write-set sem erros. O build Nuxt de produção concluiu client, SSR, prerender e empacotamento
  Nitro com heap de 4,5 GiB; a imagem resultante iniciou isoladamente e respondeu HTTP 200. A API
  também foi reconstruída sem cache e aplicou localmente as migrations `0297`–`0301`.
  R0–R5, canário real e rollback por provider/workflow não foram executados; P8 global ainda não é
  PASS.
- **P5 continua fora de escopo:** hard delete/LGPD físico exige nova autorização explícita.
- Nenhum deploy, ativação de workflow ou envio para provider foi realizado nesta execução.

## 1. Objetivo

Este documento transforma a análise do módulo Omnichannel em pacotes executáveis. Ele deve permitir que qualquer agente implemente uma parte sem precisar redescobrir a arquitetura ou decidir regras de produto durante a execução.

Os resultados esperados são:

1. Limpar o histórico visível de somente uma conexão de WhatsApp, sem desconectar o número, sem apagar auditoria e sem afetar outra conexão, conta ou cliente.
2. Permitir que uma conexão da agência seja compartilhada ou restrita a usuários específicos.
3. Permitir que cada administrador autorizado opere somente o próprio WhatsApp, quando essa for a configuração escolhida.
4. Permitir que cada cliente tenha o próprio WhatsApp dentro da própria conta, quando o módulo e o limite estiverem liberados.
5. Fazer inbox, histórico, busca, mídia, ações, IA e realtime obedecerem exatamente ao mesmo escopo.
6. Fechar os itens restantes de integração, hardening, rollout e operação do módulo.

## 2. Como usar este documento

Antes de começar qualquer pacote:

1. Leia, nesta ordem:
   - AGENT.md da raiz;
   - back/internal/modules/omnichannel/AGENT.md, se tocar backend;
   - web/app/components/omnichannel/AGENT.md, se tocar frontend;
   - automation/AGENT.md, se tocar n8n;
   - docs/omnichannel/evolucao/CONTRATO_EXECUCAO_AGENTES.md;
   - este documento.
2. Execute somente um pacote ou uma faixa de arquivos explicitamente atribuída.
3. Não invente permissão, tabela, endpoint ou workflow diferente do contrato deste arquivo.
4. Se o código real contradizer este plano, pare o pacote e registre a divergência com arquivo, linha e impacto. Não improvise uma terceira solução.
5. O integrador é o único que resolve número de migration, alterações em module.go, contratos compartilhados e ordem de merge.
6. Um pacote só termina como PASS ou BLOCKED. Não use “passou com ressalva”.

Roteamento rápido:

| Tarefa recebida | Seções obrigatórias deste arquivo |
|---|---|
| P0 | 1–7, 16–20 e 23–25 |
| P1 | 1–6, 8, 16–20 e 23–25 |
| P2 | 1–6, 9, 16–20 e 23–25 |
| P3/P4 | 1–6, 10–11, 16–20 e 23–25 |
| P5 | 1–6, 12, 16–25 |
| P6A/P6B | 1–6, 13, 16–25 e o documento da fase |
| P7/P8 | 1–6, 14–25 |

## 3. Regras que não podem ser alteradas durante a execução

### 3.1 Autoridade e isolamento

- Go e PostgreSQL são a fonte autoritativa.
- n8n é apenas orquestrador stateless.
- n8n não envia mensagens diretamente aos canais.
- n8n não grava diretamente nas tabelas do produto.
- A conta vem do Principal autenticado e do provider global de conta.
- Nenhum endpoint aceita accountId ou tenantId do body como autoridade.
- Toda query de repository repete account_id, mesmo quando o service já validou o recurso.
- Recurso de outra conta retorna 404, sem confirmar sua existência.
- Módulo, permissão de ação e escopo de dados são gates cumulativos.
- Papel legado como ADMIN, OWNER, MANAGER ou DIRECTOR nunca substitui permissão efetiva.
- Nenhum usuário recebe acesso porque existe apenas uma instância.
- Instância sem responsável ou sem grants nunca se torna pública automaticamente.

### 3.2 Dados, credenciais e ações perigosas

- Não alterar senha de usuário.
- Não copiar, registrar ou exibir credenciais brutas.
- Não colocar segredo em issue, teste, fixture, screenshot, log ou documentação.
- A credencial exposta fora do repositório deve ser revogada/rotacionada antes de produção, sem ser repetida neste arquivo ou nos logs.
- Não executar hard delete para cumprir o botão de “limpar histórico visível”.
- Não excluir contatos, bindings, profiles ou a sessão do WhatsApp durante a limpeza lógica.
- A limpeza exige confirmação explícita da conexão selecionada e auditoria.
- Nenhum agente pode fazer commit, push, deploy, restart, importação ou ativação de workflow sem pedido explícito do usuário.

### 3.3 Workflows permitidos

Os únicos workflows n8n dentro do escopo Omnichannel são:

- automation/export/workflow-omnichannel-brain.json — owner omnichannel, ID omnibrain0000001;
- automation/export/workflow-instagram-first-contact.json — owner omnichannel, ID instafirst000001.

Não alterar workflows WAHA, calendário, automações gerais ou operação da plataforma.

### 3.4 Migrations

- Migrations são append-only.
- O próximo número é descoberto no momento da execução; não confiar no número visto durante esta análise.
- SQL deve ser idempotente, schema-qualified e sem goose Down.
- Não editar migration já aplicada.
- Depois de uma migration, atualizar o ERD/documentação canônica e o AGENT.md relevante.
- Como migrations estão embarcadas no binário Go, uma migration nova exige rebuild sem cache da API antes da aplicação no ambiente.

## 4. Estado atual confirmado

### 4.1 Limpeza existente

Já existe uma ação de limpeza no modal:

- web/app/components/omnichannel/inbox/OmnichannelWhatsAppSessionModal.vue;
- web/app/composables/omnichannel/useOmnichannelWhatsAppSession.ts.

Ela chama:

    POST /v1/omnichannel/tenant/whatsapp/conversations/clear

O backend atual está em:

- back/internal/modules/omnichannel/http_instances.go;
- back/internal/modules/omnichannel/service_instance_ops.go;
- back/internal/modules/omnichannel/store_instances.go.

Problemas confirmados:

- instanceId ausente significa conta inteira;
- o gate usa papel/admin legado em parte do fluxo;
- a implementação apaga audit_events, messages e conversations;
- a operação pode afetar toda a conta;
- o comportamento não cobre, de forma coerente, mídia, IA, jobs, outbox e realtime;
- FKs mais novas podem produzir falha parcial percebida pela interface;
- o botão está escondido em um lugar pouco descobrível.

Conclusão: o endpoint atual não deve continuar executando exclusão física.

### 4.2 Acesso atual

Problemas confirmados:

- assignedUserIds e userScopePolicy são gravados em provider_config, mas não são uma autoridade normalizada de acesso.
- Rotas de leitura, ações e realtime não usam exatamente o mesmo resolver.
- Instância sem responsável pode ficar visível para usuários indevidos.
- A exceção “uma única instância” abre acesso indevido.
- settings.manage atualmente pode ampliar a leitura de conversas.
- Queue membership não está aplicada de forma uniforme em todas as superfícies.
- O websocket é assinado por conta e pode transportar conteúdo para todos os assinantes da conta.
- A interface ainda usa gates como legacyRole === ADMIN e outros derivados de papel.
- Zero instâncias acessíveis pode cair em comportamento equivalente a “all” no frontend.
- GET /v1/omnichannel/ai/usage precisa ser revisado para exigir omnichannel.audit.view.

### 4.3 Cliente e agência

- messaging.channel_client_bindings representa atribuição histórica/operacional de um canal da agência a um cliente.
- Esse binding não autoriza usuários do cliente a entrar no inbox da agência.
- automation_profiles configura IA; não deve virar fonte de autorização.
- O caminho padrão seguro é o WhatsApp do cliente pertencer à própria account do cliente.
- A agência opera essa account somente quando possui membership, módulo, permissões e grants válidos nela.
- Um inbox delegado cross-account é um produto separado e não faz parte da entrega padrão.

### 4.4 Finalização geral

- As fases E1 a E8 possuem bastante código e documentação, mas ainda exigem evidência real de integração, smoke e cutover.
- E9, hardening e escala, não está concluída.
- E10, rollout controlado e kill switch, não está concluída.
- “100%” significa código, testes, integração real, observabilidade, rollback, restore e janela de estabilidade; não significa apenas build verde.

## 5. Modelo final de acesso

### 5.1 Três gates obrigatórios

Toda operação deve passar, nesta ordem:

1. Conta ativa, membership válida e módulo omnichannel habilitado.
2. Permissão efetiva da feature.
3. Escopo de dados da instância, fila, atribuição e privacidade.

Se qualquer gate falhar, a operação falha fechada.

### 5.2 Política da conexão

Cada instância terá uma política autoritativa:

| Política | Efeito |
|---|---|
| Compartilhada com a conta | Usuários da conta com conversations.view recebem view implícito; quem também tem conversations.reply recebe reply implícito. Fila e atribuição continuam restringindo conversas. Manage nunca é implícito. |
| Restrita | Somente usuários com grant ativo naquela instância entram no escopo. |

Os nomes técnicos recomendados são ACCOUNT_SHARED e RESTRICTED. A interface deve mostrar apenas os rótulos humanos acima.

Novas instâncias nascem RESTRICTED. Na mesma transação de criação, o ator que possui omnichannel.instances.manage recebe grant manage e vira responsável, salvo quando um responsible_user_id válido foi explicitamente escolhido; nesse caso, o responsável escolhido recebe manage. ACCOUNT_SHARED só nasce por ação explícita e auditada. Lista vazia, backfill ou instância sem dono nunca produz ACCOUNT_SHARED.

### 5.3 Grants da instância

Criar fonte relacional autoritativa em messaging.whatsapp_instance_user_grants.

Campos mínimos:

| Campo | Regra |
|---|---|
| account_id | obrigatório e repetido em toda FK/query |
| instance_id | instância da mesma account |
| user_id | membership ativa da mesma account |
| access_level | view, reply ou manage |
| is_active | true enquanto o grant estiver vigente |
| revision | revisão otimista da linha |
| granted_by_user_id / updated_by_user_id | atores da criação e última alteração |
| revoked_by_user_id / revoked_at | ator e horário da revogação; nulos enquanto ativo |
| created_at / updated_at | timestamps do servidor |

Chave única:

    (account_id, instance_id, user_id)

Semântica:

| Grant | Dados | Ações |
|---|---|---|
| view | entra no escopo da instância; ainda obedece fila/atribuição | somente features para as quais possui permissão |
| reply | inclui view e pode responder se também possuir conversations.reply | não gerencia sessão/configuração |
| manage | inclui reply e view; vê todas as conversas daquela instância e pode gerenciá-la se possuir as permissões de feature correspondentes | não concede permissão por si só |

Regras:

- Grant nunca substitui permissão.
- Permissão nunca substitui grant quando a conexão é RESTRICTED.
- responsible_user_id continua como responsável operacional e projeção de UI.
- responsible_user_id não é mais a regra de segurança.
- Usuário inativo perde acesso imediatamente, mesmo que o grant permaneça registrado.
- Remover o último manage/responsável exige transferência explícita.
- Revogar não apaga a linha: define is_active=false, incrementa revision e registra revoked_by_user_id/revoked_at.
- assignedUserIds deixa de ser lido para autorização após o rollout.

### 5.4 Fila e atribuição

Para grants view e reply, a conversa só é visível quando:

- está atribuída ao usuário; ou
- está em uma fila na qual ele é membro ativo.

Uma conversa sem fila e sem atribuição é visível somente a quem tem grant manage naquela instância.

O grant manage amplia o escopo somente dentro da instância concedida. Ele não amplia para outra instância ou conta.

### 5.5 Permissões existentes

Reutilizar as permissões existentes:

- omnichannel.conversations.view;
- omnichannel.conversations.reply;
- omnichannel.conversations.assign;
- omnichannel.conversations.close;
- omnichannel.contacts.manage;
- omnichannel.instances.manage;
- omnichannel.settings.manage;
- omnichannel.agents.manage;
- omnichannel.audit.view;
- omnichannel.conversations.privacy.manage.

Decisão fixa:

- A limpeza lógica usa omnichannel.conversations.privacy.manage e exige escopo manage na instância.
- Não criar uma permissão por botão nesta etapa.
- settings.manage não implica mais leitura ampla do inbox.
- instances.manage permite a feature de configuração, mas manage define quais instâncias o usuário pode configurar, inclusive quando ACCOUNT_SHARED. Compartilhamento afeta inbox, não administração.
- Exceção de platform_admin só existe onde o contrato central já a define. O gate de privacidade continua explícito.

Regra transitória de P0, antes de P1 existir:

- reset exige cumulativamente omnichannel.conversations.privacy.manage e omnichannel.instances.manage, ambas efetivas;
- a instância precisa ser resolvida por ID e account_id explícitos;
- não existe bypass por papel legado;
- depois que P1 for ativado, o mesmo endpoint passa também por grant manage.

### 5.6 Matriz resumida

| Ator | Requisitos | Resultado |
|---|---|---|
| Administrador da agência com WhatsApp próprio | membership da agência, módulo, permissões e grant manage somente na conexão dele | gerencia e vê somente essa conexão |
| Gestor da agência com vários números | mesmas permissões e grant manage em cada número autorizado | vê os números concedidos |
| Atendente | conversations.view/reply, view ou reply na instância e fila/atribuição | vê apenas seu escopo operacional |
| Cliente com número próprio | account do cliente, módulo/limite, permissões e grant | opera o número dentro da própria conta |
| Agência apoiando cliente | membership e grants explícitos na account do cliente | opera pelo seletor de conta existente |
| Usuário sem grant em conexão restrita | mesmo que tenha papel administrativo | não vê a instância nem suas conversas |
| Usuário sem módulo | qualquer papel | não entra no Omnichannel |

## 6. Preflight obrigatório para todo pacote

Executar a partir da raiz:

    git status --short
    git rev-parse HEAD
    docker compose config
    docker compose ps
    rg --files back/internal/platform/database/migrations | Sort-Object | Select-Object -Last 10

Registrar:

- SHA base;
- arquivos já alterados pelo usuário;
- serviços Docker disponíveis;
- última migration;
- pacote atribuído;
- allowlist de arquivos.

Neste momento existem alterações do usuário em arquivos de automação/deploy. Elas não pertencem a este plano e devem ser preservadas. O agente sempre deve confirmar o status real novamente; nunca assumir que a lista continua igual.

Proibições no preflight:

- não executar npm install automaticamente;
- não resetar, descartar ou formatar arquivos fora da allowlist;
- não alterar .env;
- não iniciar deploy;
- não mudar senha;
- não usar banco de produção para teste.

### 6.1 Manifesto bloqueante do pacote

Antes da primeira edição, o integrador registra:

    Pacote:
    Objetivo:
    Dependências já PASS:
    Owner do pacote:
    Revisor:
    SHA base:
    Write-set exato:
    Arquivos somente leitura:
    Contrato/fixture:
    Migration reservada:
    Flags:
    Comandos de teste:
    Resultado esperado:
    Rollback:

Sem write-set exato, o agente não começa. Se descobrir que precisa de outro arquivo, pausa, entrega a justificativa e espera o integrador atualizar o manifesto.

Registro inicial de write-set:

| Pacote | Write-set inicial |
|---|---|
| P0 | definido na seção 7.6 |
| P1 | definido na seção 8.6 |
| P2 | arquivos das seções 9.2 e 9.3; backend limitado a http_instances.go, service_instances.go, store_instance_grants.go e testes |
| P3 | arquivos da seção 10.2; provisionamento usa APIs existentes e só abre backend central se teste provar gap |
| P4 | http_channel_client_bindings.go, channel_client_binding_model.go, store_channel_client_bindings*.go, automation_model.go, automation_service.go, automation_store.go, ConfigChannelClientBindings.vue, useChannelClientBindings.ts, channel-client-bindings-api.ts e testes |
| P5 | purge_handler.go, store_retention.go, retention_media.go, retention_scheduler.go, migration/API novas e testes; somente após nova aprovação |
| P6A/P6B | definido pelo catálogo e documento da fase antes da execução; workflows somente pelo integrador |
| P7 | manifesto dividido em observabilidade, escala, segurança e restore; nunca um único diff amplo |
| P8 | manifesto dividido em schema/service, painel, kill switch e avaliação |

## 7. Pacote P0 — limpar histórico visível de uma conexão

Prioridade: primeira entrega
Codificação alvo com agentes em paralelo: 1,5 a 2,5 horas
Tempo esperado até PASS, incluindo integração, testes e revisão: 4 a 8 horas
Esforço agregado estimado: 12 a 20 agente-horas
Pode rodar em paralelo: frontend e backend após o integrador reservar a migration
Não inclui: exclusão física/LGPD

### 7.1 Resultado funcional

Ao confirmar “Limpar histórico visível desta conexão”:

- somente a instância selecionada fica visualmente zerada;
- outra instância da mesma conta não muda;
- outra conta ou cliente não muda;
- sessão e status de conexão não mudam;
- contatos não são apagados;
- auditoria e dados históricos permanecem no banco;
- uma mensagem nova faz a conversa reaparecer;
- apenas mensagens posteriores ao cutoff ficam visíveis;
- inbox, busca de mensagens, mídia direta, contexto operacional da IA e realtime não recuperam conteúdo anterior ou igual ao cutoff;
- CRM, audit_events, ai_runs e customer intelligence continuam preservados e acessíveis somente em suas superfícies permissionadas. P0 não é DSAR.

### 7.2 Migration e contrato da instância

O integrador deve criar a próxima migration livre, contendo:

1. Em messaging.whatsapp_instances:
   - history_visible_from timestamptz null;
   - history_reset_revision bigint not null default 0.
2. Permitir o tipo de auditoria WHATSAPP_INSTANCE_HISTORY_RESET.
3. Preservar integralmente a união atual do CHECK de audit_events; não substituir a lista por uma lista incompleta.
4. Nenhum backfill destrutivo. NULL mantém o comportamento anterior.

Expor nos contratos de instância usados pelo card e modal:

- historyVisibleFrom;
- historyResetRevision;
- myCapabilities.

Shape fixo:

    {
      "view": true,
      "reply": true,
      "manage": true,
      "resetHistory": true
    }

Antes de P1, manage e resetHistory são derivados das duas permissões transitórias descritas na seção 5.5. Depois de P1, também incorporam o grant.

Antes de escrever SQL:

    rg -n "audit_events|event_type|check" back/internal/platform/database/migrations
    rg -n "whatsapp_instances" back/internal/platform/database/migrations

### 7.3 Contrato HTTP

Adicionar:

    POST /v1/omnichannel/tenant/whatsapp/instances/{id}/history/reset

Body:

    {
      "confirmation": "instanceName exato",
      "reason": "opcional",
      "expectedRevision": 3
    }

Resposta 200:

    {
      "instanceId": "...",
      "hiddenBefore": "timestamp UTC do servidor",
      "resetRevision": 4
    }

Erros:

- 403: falta módulo/permissão;
- 404: instância fora do escopo ou inexistente;
- 409: expectedRevision desatualizada;
- 422: confirmação não corresponde ao nome da instância.

Confirmação:

- usa instanceName retornado pela API, não displayName;
- remove somente espaços no início/fim;
- comparação é case-sensitive e accent-sensitive;
- confirmation nunca entra em log ou audit payload;
- reason é trimado, limitado a 240 caracteres e não recebe segredo/PII desnecessária.

Compatibilidade do endpoint antigo:

- /tenant/whatsapp/conversations/clear nunca mais faz DELETE;
- ele nunca delega apenas com instanceId, pois não possui confirmation nem expectedRevision;
- responde 409 history_reset_moved apontando o novo endpoint;
- ausência de instanceId nunca admite escopo de conta;
- frontend e backend novos devem ser publicados na mesma release;
- remover o alias somente depois que nenhum cliente o chamar.

### 7.4 Transação backend e corrida

Implementar um único método autoritativo:

1. Resolver account pelo Principal.
2. Exigir módulo omnichannel.
3. Exigir omnichannel.conversations.privacy.manage.
4. Durante P0, exigir também omnichannel.instances.manage e resolver a instância por ID + account_id. Depois de P1, exigir grant manage.
5. Validar confirmation.
6. Bloquear a linha da instância com SELECT ... FOR UPDATE.
7. Comparar expectedRevision.
8. Definir history_visible_from com statement_timestamp() do PostgreSQL.
9. Incrementar history_reset_revision.
10. Gravar audit_event. Como o schema não possui coluna de instância, colocar em payload_json:
    - ator;
    - account;
    - instanceId e instanceName;
    - cutoff anterior e novo;
    - revisão anterior e nova;
    - reason, se fornecido.
    conversation_id e message_id ficam nulos.
11. Incrementar ai_generation das conversas afetadas.
12. Reinicializar projeções operacionais que reutilizem contexto antigo, incluindo extracted_fields e contadores de turno, preservando a trilha.
13. Cancelar ai_dispatches anteriores ao cutoff nos estados ainda canceláveis.
14. Levar outbox/mensagens PENDING correspondentes ao estado terminal já suportado pelo schema, com error_code history_reset. Não inventar enum novo.
15. Preservar todas essas linhas e sua auditoria.
16. Commit.
17. Publicar somente uma invalidação opaca após o commit.

Regras de concorrência:

- timestamp igual ao cutoff é histórico antigo: visibilidade usa created_at > cutoff;
- workers de IA/outbound releem cutoff e ai_generation depois do claim e imediatamente antes de chamar o provider;
- evento aceito pelo provider antes da revalidação não pode ser desfeito e continua auditado;
- inbound com created_at > cutoff é novo e faz a conversa reaparecer;
- o lock isolado da instância não substitui essas revalidações.

### 7.5 Regra de leitura

Criar um helper único de cutoff efetivo. Ele usa o maior timestamp entre:

- history_visible_from da instância;
- cutoff de supressão/privacidade do contato, quando existir.

Aplicar o helper em:

- lista de conversas;
- detalhe;
- histórico de mensagens;
- busca de mensagens;
- preview da última mensagem;
- mídia;
- contexto e contagem de turnos da IA;
- projeções operacionais de automação;
- ações por ID direto.

Uma conversa WhatsApp aparece quando possui mensagem com created_at > cutoff. Se não possuir, acesso direto retorna 404. Instagram e outros canais não usam o cutoff de instância WhatsApp.

P0 não oculta CRM, audit_events, ai_runs ou customer intelligence em suas telas próprias. O construtor de contexto operacional da IA não pode usar mensagens anteriores ou iguais ao cutoff.

### 7.6 Write-set e frontend

Write-set máximo de P0. Qualquer arquivo adicional exige liberação do integrador.

Backend/dados:

- a migration nova reservada pelo integrador;
- back/internal/modules/omnichannel/http_instances.go;
- back/internal/modules/omnichannel/service_instance_ops.go;
- back/internal/modules/omnichannel/service_admin.go;
- back/internal/modules/omnichannel/store_instances.go;
- back/internal/modules/omnichannel/store_postgres_admin.go;
- back/internal/modules/omnichannel/model.go;
- back/internal/modules/omnichannel/store_postgres.go;
- back/internal/modules/omnichannel/service_media.go;
- back/internal/modules/omnichannel/brain_context.go;
- back/internal/modules/omnichannel/store_ai_dispatch.go;
- back/internal/modules/omnichannel/outbound_handler.go;
- back/internal/modules/omnichannel/publisher.go;
- back/internal/modules/realtime/service_omnichannel.go;
- testes correspondentes;
- AGENT.md e ERD somente pelo integrador ao fechar o pacote.

Frontend:

- web/app/domain/omnichannel/instance-admin-api.ts;
- web/app/domain/omnichannel/config-types.ts;
- web/app/composables/omnichannel/useOmnichannelWhatsAppSession.ts;
- web/app/composables/omnichannel/useOmnichannelInbox.ts;
- web/app/composables/omnichannel/useOmnichannelInboxHistory.ts;
- web/app/composables/omnichannel/useOmnichannelInboxRealtime.ts;
- web/app/composables/omnichannel/useOmnichannelInboxStateMutators.ts;
- web/app/composables/omnichannel/useOmnichannelScopeInvalidation.ts, novo;
- web/app/components/omnichannel/inbox/OmnichannelWhatsAppSessionModal.vue;
- web/app/components/omnichannel/config/ConfigNumberCard.vue;
- web/app/components/omnichannel/config/ConfigNumbers.vue;
- web/app/components/omnichannel/OmnichannelInboxModule.vue;
- testes correspondentes.

Implementar:

- ação na zona de risco do card da conexão;
- manter a ação no modal usando a mesma função;
- texto exato “Limpar histórico visível desta conexão”;
- mostrar conta ativa, instanceName, displayName, telefone e provider;
- explicar que a sessão não será desconectada e os contatos serão preservados;
- exigir digitação de instanceName;
- usar myCapabilities.resetHistory, nunca legacyRole;
- bloquear clique duplicado;
- ao concluir, limpar conversa ativa e caches daquela instância;
- publicar o resultado local em useOmnichannelScopeInvalidation;
- card, modal e realtime usam a mesma invalidação;
- o inbox bloqueia merge de eventos enquanto refaz o fetch autorizado;
- 409 reidrata a instância e não repete a mutation automaticamente.

Contrato inicial de realtime para P0/P1:

- o tópico por account não entrega mensagem, telefone, preview, instanceId, conversationId nem messageId;
- publica somente omnichannel.invalidate com eventId, reason e occurredAt;
- reason permitido: message_changed, history_reset ou access_scope_changed;
- qualquer cliente inscrito refaz fetch pela API escopada;
- o autor do reset usa instanceId, hiddenBefore e resetRevision da resposta HTTP para limpar imediatamente só a instância local;
- eventId repetido é ignorado;
- reconnect sempre faz bootstrap REST completo;
- payload rico/per-instance só poderá voltar depois que houver filtragem por assinante provada por testes.

### 7.7 Testes obrigatórios de P0

Backend:

- reset somente da instância A;
- instância B da mesma conta intacta;
- conta B intacta;
- dados antigos continuam fisicamente existentes;
- audit_event anterior continua existente;
- novo evento de reset existe com payload_json correto;
- falta de uma das permissões transitórias retorna 403;
- depois de P1, grant inadequado retorna 404;
- confirmação errada retorna 422;
- revisão concorrente retorna 409;
- endpoint legado não altera nada e retorna 409;
- endpoint legado não executa DELETE;
- nova mensagem após cutoff reaparece;
- mensagem exatamente no cutoff permanece oculta;
- paginação não atravessa o cutoff;
- história, busca, mídia e contexto da IA não leem antes do cutoff;
- worker claimed antes do reset revalida antes do provider;
- outbox/dispatch cancelável anterior ao cutoff não envia;
- envio já aceito pelo provider permanece auditado.

Frontend:

- ação só aparece com resetHistory;
- envia o ID selecionado;
- nome incorreto não chama API;
- clique duplo é bloqueado;
- sucesso zera somente a instância;
- falha mantém o estado e mostra erro acionável;
- realtime não reinsere conversa antiga;
- 409 reidrata sem retry automático;
- evento em voo, reconnect e duas abas não restauram conteúdo antigo.

### 7.8 Aceite de P0

P0 é PASS somente quando:

- não existe DELETE no caminho do botão;
- nenhuma chamada aceita limpeza da conta inteira;
- outra instância e outra conta permanecem intactas;
- auditoria é preservada;
- UI, REST, busca, mídia, contexto operacional da IA e realtime respeitam o cutoff;
- corrida reset × inbound × IA × outbound está coberta;
- sessão permanece no estado anterior.

### 7.9 Rollback de P0

- Desabilitar a ação no frontend.
- Fazer o endpoint responder indisponível sem apagar as colunas.
- Usar flag temporária OMNICHANNEL_HISTORY_CUTOFF_ENFORCED no Go para desligar somente a aplicação do filtro em emergência.
- Não derrubar migration.
- Para um reset incorreto, restaurar history_visible_from somente por operação administrativa auditada e explicitamente aprovada.
- Testar rollback com migration presente: flag desligada mostra o histórico; flag ligada volta a aplicar o cutoff.

## 8. Pacote P1 — grants normalizados, resolver canônico e realtime seguro

Prioridade: alta
Codificação alvo com agentes em paralelo: 4 a 7 horas
Tempo esperado até PASS: 1 a 2 dias úteis
Esforço agregado estimado: 24 a 40 agente-horas
Dependência: integrador reserva a migration depois de P0

Estado de execução em 2026-08-28:

- **P1A local PASS:** migration `0298`, backfill/relatório, grants tenant-scoped, escrita com revisão
  otimista e criação/bootstrap atômicos com primeiro `manage`.
- **P1B local PASS:** resolver canônico ativo em REST, ações, mídia, contatos/CRM, IA, automação,
  bindings e lifecycle; realtime opaco com reautorização por escrita; frontend fail-closed nos
  quatro estados de `/instances/access`, preservando Instagram independente.
- Evidência direcionada: integrações reais P1A/P1B, realtime e regressão P0 de reset verdes;
  7 arquivos/32 testes Vitest verdes; ESLint direcionado com zero erros. Suíte global, build,
  smoke autenticado e deploy não fazem parte deste fechamento e não foram executados.

### 8.1 Migration de acesso

Adicionar em messaging.whatsapp_instances:

- access_policy text not null default 'RESTRICTED';
- CHECK para ACCOUNT_SHARED ou RESTRICTED;
- access_revision bigint not null default 0.

Adicionar messaging.whatsapp_instance_user_grants conforme a seção 5.3, incluindo:

- is_active boolean not null default true;
- revision bigint not null default 1;
- granted_by_user_id;
- updated_by_user_id;
- revoked_by_user_id;
- revoked_at;
- created_at e updated_at;
- unique (account_id, instance_id, user_id);
- CHECK de access_level.

Índices:

- account_id + user_id, parcial where is_active=true;
- account_id + instance_id, parcial where is_active=true;
- account_id + instance_id + access_level.

Adicionar o tipo de auditoria WHATSAPP_INSTANCE_ACCESS_CHANGED, preservando a união atual do CHECK. Mudança de política, criação, alteração, transferência e revogação usam esse evento com before/after em payload_json.

Toda escrita de policy/grants:

1. bloqueia whatsapp_instances com FOR UPDATE;
2. compara access_revision;
3. atualiza grants;
4. incrementa access_revision uma única vez;
5. grava auditoria;
6. publica invalidação somente depois do commit.

Não adivinhar FKs. Inspecionar as migrations que criaram core.account_users e whatsapp_instances. ACCOUNT_SHARED nunca é default e nunca nasce de backfill.

Depois de P1, toda resposta de instância consumida pelo inbox/configuração inclui:

- accessPolicy;
- accessRevision;
- historyVisibleFrom;
- historyResetRevision;
- myCapabilities calculado para o Principal atual.

myCapabilities nunca é persistido, recebido do frontend ou calculado para um userId arbitrário enviado pelo cliente.

### 8.2 Criação, revogação e backfill

Criação de instância:

- exige omnichannel.instances.manage;
- nasce RESTRICTED;
- cria instância e primeiro manage na mesma transação;
- responsável explícito válido recebe manage; sem responsável explícito, o ator recebe manage;
- responsible_user_id reflete um manage ativo.

Revogação:

- mantém a linha;
- define is_active=false;
- incrementa revision da linha;
- define revoked_by_user_id e revoked_at;
- incrementa access_revision da instância;
- último manage não pode ser revogado sem transferência na mesma transação.

Backfill idempotente:

- responsible_user_id ativo vira manage;
- assignedUserIds válidos viram reply;
- created_by_user_id ativo vira manage somente quando não existir responsável válido;
- usuário de outra account é ignorado e relatado;
- membership inativa é ignorada e relatada;
- toda instância existente inicia RESTRICTED;
- instância sem manage aparece no relatório de correção;
- nunca transformar instância sem dono em compartilhada;
- manter JSON por uma versão para rollback/compatibilidade;
- depois do enforcement, provider_config não participa da autorização nem recebe novas alterações de accessPolicy/grants.

Antes de ativar o enforcement, produzir:

- total de instâncias;
- compartilhadas;
- restritas;
- sem manage;
- grants ativos por nível;
- grants inválidos/inativos;
- usuários ignorados e motivo.

### 8.3 Resolver canônico

Criar um único ConversationAccessScope/InstanceAccessService reutilizado por todo o módulo.

Entrada mínima:

- account_id do Principal;
- user_id;
- permissões efetivas;
- membership/módulo;
- política da instância;
- grant ativo;
- filas ativas;
- atribuição;
- cutoff de histórico/privacidade.

Saída:

- instâncias visíveis;
- conversas visíveis;
- myCapabilities com view, reply, manage e resetHistory;
- motivo interno mascarado para auditoria;
- sem expor existência de recurso fora do escopo.

Ordem de decisão:

1. conta/membership/módulo;
2. permissão da feature;
3. ACCOUNT_SHARED ou grant da instância;
4. manage ou fila/atribuição;
5. cutoff/supressão;
6. account_id repetido no repository.

Remover:

- exceção de uma única instância;
- instância sem responsável pública;
- role/admin legado como autorização;
- settings.manage como “ver tudo”;
- SQL de visibilidade duplicado e divergente.

### 8.4 Matriz endpoint → permissão → escopo

| Superfície | Permissão de feature | Escopo de dados | Falha |
|---|---|---|---|
| Inbox/lista/detalhe/histórico/busca/mídia WhatsApp | conversations.view | ACCOUNT_SHARED ou grant view+; depois manage ou fila/atribuição; cutoff aplicado | lista vazia ou 404 no ID direto |
| Reply/send | conversations.reply | conversa visível e ACCOUNT_SHARED com reply implícito ou grant reply+ | 403 sem permissão; 404 fora do dado |
| Assign/transfer | conversations.assign | conversa visível; destinos válidos na fila/account | 403 ou 404 |
| Close/reopen | conversations.close | conversa visível | 403 ou 404 |
| Contato/CRM derivado da conversa | conversations.view para leitura; contacts.manage para escrita | ao menos uma conversa visível; endpoints próprios de customer_data mantêm o contrato do módulo deles | 403 ou 404 |
| /instances/access para inbox | conversations.view | somente instâncias autorizadas | array vazio; nunca all |
| Lista administrativa, status, capabilities e validate-endpoints | instances.manage | grant manage; platform_admin somente conforme contrato central | array vazio ou 404 |
| Create de instância | instances.manage | account ativa e limite comercial; cria RESTRICTED + manage | 403/409 |
| Update, credentials, QR, connect, logout e delete | instances.manage | grant manage | 403 sem feature; 404 fora do grant |
| Filas, setores e regras | settings.manage | account-scoped; não concede conteúdo de conversa | 403 |
| Agente/perfil de IA por instância | agents.manage | grant manage na instância vinculada | 403/404 |
| AI usage agregado | audit.view | account-scoped; não retorna conteúdo/PII por instância sem autorização | 403 |
| Audit de conversa/instância | audit.view | account + recurso resolvido; payload sensível segue o scope do recurso | 403/404 |
| Realtime | conversations.view | somente invalidação opaca; dados vêm das APIs acima | socket negado/fechado |

Alteração de limite de números continua na política comercial/platform admin e não nasce de instances.manage.

### 8.5 Contratos de repository

O repository deve oferecer métodos equivalentes a:

    ListAccessibleInstances(ctx, scope)
    GetAccessibleInstance(ctx, scope, instanceID, requiredLevel)
    ListVisibleConversations(ctx, scope, filter)
    GetVisibleConversation(ctx, scope, conversationID)
    ListVisibleMessages(ctx, scope, conversationID, filter)
    GetVisibleMessage(ctx, scope, conversationID, messageID)
    GetVisibleMediaDescriptor(ctx, scope, conversationID, messageID)
    ListVisibleContacts(ctx, scope, filter)

Não é obrigatório usar exatamente esses nomes. É obrigatório usar o mesmo objeto de scope e o mesmo fragmento de visibilidade.

### 8.6 Write-set máximo de P1

Novos:

- migration reservada;
- back/internal/modules/omnichannel/access_scope.go;
- back/internal/modules/omnichannel/store_instance_grants.go;
- testes correspondentes.

Existentes:

- back/internal/modules/omnichannel/service.go;
- back/internal/modules/omnichannel/service_actions.go;
- back/internal/modules/omnichannel/service_media.go;
- back/internal/modules/omnichannel/service_contacts.go;
- back/internal/modules/omnichannel/service_admin.go;
- back/internal/modules/omnichannel/service_instances.go;
- back/internal/modules/omnichannel/service_instance_ops.go;
- back/internal/modules/omnichannel/store_postgres.go;
- back/internal/modules/omnichannel/store_postgres_actions.go;
- back/internal/modules/omnichannel/store_postgres_admin.go;
- back/internal/modules/omnichannel/store_postgres_scope.go;
- back/internal/modules/omnichannel/store_instances.go;
- back/internal/modules/omnichannel/module.go, somente integrador;
- back/internal/modules/realtime/service_omnichannel.go;
- testes correspondentes.

Qualquer outro arquivo exige handoff ao integrador antes da edição.

### 8.7 Realtime fail-closed

Contrato inicial:

- tópico por account não é autorização de conteúdo;
- transportar somente omnichannel.invalidate opaco, conforme P0;
- nunca transportar texto, telefone, preview, mídia ou IDs de recursos;
- cada invalidação faz fetch por REST com o scope canônico;
- antes de escrever no socket, revalidar account ativa, membership, módulo e conversations.view;
- mudança de grant, membership, fila, atribuição ou permissão publica access_scope_changed;
- em access_scope_changed, descartar estado WhatsApp, refazer /instances/access e renovar a conexão/ticket;
- reconnect e troca de account sempre limpam estado e fazem bootstrap;
- payload rico só entra em fase posterior com filtro por assinante provado.

Estados obrigatórios no frontend para /instances/access:

- loading;
- resolved-empty;
- resolved-nonempty;
- error.

error falha fechado e mostra falha de sincronização. resolved-empty mostra zero conversas WhatsApp. Instagram continua no escopo próprio.

O valor all significa “todas as conexões autorizadas retornadas pela API”, nunca “toda a account”. Se a lista autorizada estiver vazia ou em erro, não enviar filtro all.

### 8.8 Testes obrigatórios de P1

Cobrir, em duas accounts:

- platform admin conforme contrato central;
- manager com manage na instância A;
- manager sem grant na B;
- attendant com reply e fila;
- attendant com grant, mas fora da fila;
- usuário sem grant;
- membership inativa;
- módulo desligado;
- ACCOUNT_SHARED com view/reply implícitos e sem manage implícito;
- RESTRICTED;
- conversa atribuída;
- conversa de fila;
- conversa sem fila;
- conversa sem instância;
- uma única instância;
- instância sem responsável;
- override allow e deny;
- create/update/credentials/QR/connect/logout/delete;
- /instances/access vazio;
- /instances/access com 403, 500 e falha de rede;
- ticket emitido antes da revogação;
- evento em voo, reconnect e duas abas.

Rodar a mesma matriz em lista, detalhe, mensagens, mídia, ações, contatos e realtime.

### 8.9 Aceite de P1

- A mesma identidade obtém o mesmo conjunto em REST; realtime só sinaliza refetch.
- Zero instâncias WhatsApp acessíveis significa zero conversas WhatsApp, sem afetar Instagram autorizado.
- Uma instância não significa acesso automático.
- grant e membership removidos surtam efeito sem entregar conteúdo residual.
- recurso de outra conta retorna 404.
- GET /v1/omnichannel/ai/usage exige omnichannel.audit.view.
- ciclo de vida completo da instância não usa caller.IsAdmin.

### 8.10 Rollout e rollback de P1

Flags temporárias:

- OMNICHANNEL_CONVERSATION_SCOPE_V2;
- OMNICHANNEL_INSTANCE_GRANTS_ENFORCED;
- OMNICHANNEL_REALTIME_OPAQUE_ONLY.

Sequência:

1. migration;
2. deploy com enforcement desligado e realtime opaco;
3. backfill;
4. relatório de inconsistências;
5. shadow comparison entre scope antigo e novo, sem PII;
6. corrigir instâncias sem manage;
7. ativar em conta de teste;
8. ativar na agência;
9. ativar em cliente piloto;
10. ativar gradualmente.

Rollback:

- manter realtime opaco;
- preservar tabela/colunas;
- antes da primeira ativação em produção, flags podem cancelar shadow/enforcement;
- depois que uma account entrar no scope V2, nunca voltar ao resolver legado nem ao JSON como autorização;
- em rollback pós-ativação, congelar escrita de grants, manter o resolver relacional em modo somente leitura e reverter apenas API/UI incompatível;
- se o resolver relacional ficar indisponível, falhar fechado com inbox WhatsApp indisponível; nunca abrir acesso;
- JSON legado pode continuar na resposta por compatibilidade durante uma versão, mas jamais volta a ser fonte de autorização;
- nunca remover migration aplicada.

## 9. Pacote P2 — painel de grants e acesso por usuário

Prioridade: alta
Codificação alvo com backend pronto: 2 a 4 horas
Tempo esperado até PASS: 4 a 8 horas
Esforço agregado estimado: 8 a 14 agente-horas
Dependência: P1

### 9.1 API

Evoluir os contratos de usuários da instância sem aceitar accountId do body:

    GET /v1/omnichannel/tenant/whatsapp/instances/{id}/users
    PUT /v1/omnichannel/tenant/whatsapp/instances/{id}/users

Resposta deve incluir:

- accessRevision;
- accessPolicy;
- responsibleUserId;
- grants;
- myCapabilities.

Escrita:

    {
      "accessRevision": 3,
      "accessPolicy": "RESTRICTED",
      "responsibleUserId": "...",
      "grants": [
        { "userId": "...", "accessLevel": "manage" },
        { "userId": "...", "accessLevel": "reply" }
      ]
    }

Regras:

- accessRevision divergente retorna 409;
- usuário precisa ser membro ativo da account;
- último manage não pode ser removido sem transferência;
- mudança é auditada;
- formato antigo userIds pode ser traduzido temporariamente para reply;
- resposta sempre reidrata o estado autoritativo.

Cada grant retornado inclui isActive e revision. O frontend não fabrica myCapabilities: usa exatamente:

    {
      "view": true,
      "reply": true,
      "manage": true,
      "resetHistory": true
    }

### 9.2 Frontend

Arquivos principais:

- web/app/domain/omnichannel/config-types.ts;
- web/app/domain/omnichannel/config-api.ts;
- web/app/components/omnichannel/config/ConfigNumbers.vue;
- web/app/components/omnichannel/config/ConfigNumberCard.vue;
- web/app/composables/omnichannel/useOmnichannelInboxBootstrapLoaders.ts;
- web/app/composables/omnichannel/useOmnichannelInboxRealtime.ts;
- web/app/composables/omnichannel/useOmnichannelInboxState.ts.

Interface:

- seletor “Compartilhado com a conta” ou “Restrito ao responsável e selecionados”;
- responsável principal;
- usuários adicionais;
- nível view, reply ou manage;
- resumo no card;
- alerta se o responsável não tiver manage;
- impedir usuário sem membership/módulo;
- remoção do último manage exige transferência;
- nenhuma lista vazia pode virar acesso global;
- usar conta ativa do shell; não criar outro seletor de tenant.

Estado por card:

- ConfigNumberCard chama GET .../instances/{id}/users para a própria instância;
- estados obrigatórios: idle, loading, ready, saving e error;
- accessRevision vem dessa resposta;
- salvar bloqueia duplo clique;
- 409 refaz GET e mostra conflito, sem reaplicar;
- falha não mantém grants inventados localmente;
- ao salvar, publicar access_scope_changed e reidratar /instances/access.

### 9.3 Remover gates de papel

Revisar:

- web/app/pages/omnichannel/index.vue;
- web/app/components/omnichannel/OmnichannelWorkspaceHeader.vue;
- web/app/components/omnichannel/OmnichannelInboxModule.vue;
- web/app/components/omnichannel/inbox/InboxChatHeader.vue;
- web/app/components/omnichannel/config/OmnichannelConfigDrawer.vue;
- web/app/components/omnichannel/automation/OmnichannelAutomationMvp.vue;
- web/app/composables/omnichannel/useOmnichannelWhatsAppSession.ts;
- web/app/composables/omnichannel/useOmnichannelInboxDerivedState.ts;
- web/app/domain/utils/permissions.ts;
- web/app/middleware/auth.global.ts;
- web/app/domain/utils/permissions.test.ts;
- web/app/domain/utils/permissions-workspaces.test.ts.

Substituir:

- legacyRole === ADMIN;
- user.role === ADMIN;
- canWriteInboxByRole;
- qualquer condição equivalente.

Usar capacidades autoritativas e permissões efetivas.

Estado de inbox:

- loading não renderiza “all”;
- resolved-empty mostra “Nenhuma conexão de WhatsApp acessível”;
- error mostra falha de sincronização e não consulta conversas account-wide;
- resolved-nonempty permite “Todas as conexões autorizadas”;
- instância salva em localStorage que saiu do scope é removida;
- troca de account zera conexão, conversa ativa, histórico e caches antes do novo bootstrap.

### 9.4 Testes frontend a criar/ajustar

- permissions/workspace: módulo, owner default e permissões efetivas;
- sessão/reset: confirmation, 409, duplo clique e invalidação;
- grants/card: load por instância, save, revoke, último manage e 409;
- bootstrap/access: quatro estados e sem fallback all;
- histórico: cutoff, paginação e seleção persistida;
- realtime: evento opaco, revogação, reconnect, duas abas e evento em voo;
- route gate: módulo sem conversations.view;
- troca de account: localStorage e caches.

### 9.5 Aceite de P2

- Um administrador autorizado configura somente suas instâncias concedidas.
- A tela explica claramente compartilhado versus restrito.
- Salvar e recarregar produz o mesmo estado.
- Remover grant retira a conversa da UI sem refresh manual.
- Adicionar grant passa a funcionar após reidratação.
- Nenhuma credencial bruta aparece.
- Binding de cliente nunca altera myCapabilities, /instances/access ou realtime.

## 10. Pacote P3 — WhatsApp próprio de cliente

Prioridade: depois de P1/P2
Codificação alvo por cliente piloto e fluxo de conta: 3 a 5 horas
Tempo esperado até PASS: 1 a 2 dias úteis
Esforço agregado estimado: 8 a 16 agente-horas
Dependência: P1 e P2

### 10.1 Modelo obrigatório

O WhatsApp próprio do cliente é criado dentro da account do cliente.

Pré-requisitos:

1. account do cliente ativa;
2. módulo omnichannel habilitado;
3. max_whatsapp_numbers maior ou igual a 1;
4. administrador do cliente como membership ativo;
5. papel/permissões efetivas;
6. instância criada nessa account;
7. grant manage para o responsável;
8. binding ativo para a própria account com source standalone_default;
9. automation_profile.client_account_id igual ao client_account_id do binding;
10. filas/grants adicionais configurados.

A criação do binding standalone_default deve usar o service existente e ser idempotente. Se profile, binding e account da instância divergirem, habilitar automação retorna 409 e o runtime falha fechado.

### 10.2 Agência operando o cliente e seletor de conta

- O usuário da agência precisa de core.account_users ativa, permissões e grants explícitos na account do cliente.
- Membership apenas na organização/agência não concede inbox do cliente.
- /v2/me/accounts continua sendo a fonte autoritativa das contas que o usuário pode selecionar.
- O CoreAccountSwitcher deve aparecer para qualquer usuário com mais de uma account autorizada, não apenas platform_admin.
- A opção “Plataforma (dev)” continua exclusiva de platform_admin.
- CoreAccountSwitcher nunca lista account que não veio de /v2/me/accounts.
- Trocar account chama /v2/me/context e limpa o estado Omnichannel antes de carregar o novo.
- Não montar X-Account-Id manualmente.
- Não consultar a account do cliente por binding da agência.
- Não misturar conversas de duas accounts no mesmo inbox.

Write-set adicional:

- web/layers/core/components/CoreAccountSwitcher.vue;
- web/layers/core/stores/account.ts;
- web/app/layouts/dashboard.vue;
- web/app/middleware/auth.global.ts;
- testes do store/switcher;
- endpoints /v2/me/accounts e /v2/me/context apenas se os testes mostrarem que não retornam todas as memberships ativas.

Alterar o endpoint central de contas exige leitura do AGENT.md do módulo access/core e revisão cross-tenant separada.

### 10.3 UI

Explicar dois cenários:

1. “Número próprio do cliente”: pertence à account do cliente e pode ser usado no portal do cliente.
2. “Número da agência dedicado ao cliente”: pertence à agência; o binding organiza CRM/roteamento, mas não dá acesso ao portal do cliente.

### 10.4 Testes de P3

- módulo desligado bloqueia;
- limite zero bloqueia criação;
- cliente A não vê cliente B;
- agência sem membership/grant não vê o cliente;
- agência com membership/grant vê somente a instância concedida;
- usuário com duas memberships vê somente essas duas accounts no switcher;
- usuário não-platform com duas accounts pode trocar entre elas;
- “Plataforma (dev)” não aparece para não-platform;
- realtime obedece à mesma regra;
- troca de conta limpa seleção e caches;
- logout/desconexão de um número não mostra histórico de outro.

### 10.5 Aceite de P3

- Cliente piloto conecta e opera seu próprio número.
- Agência só entra com acesso explícito.
- Self-binding e automation_profile permanecem coerentes.
- Nenhum dado cross-account aparece em REST, busca, mídia ou realtime.

## 11. Pacote P4 — bindings, CRM e automação coerentes

Prioridade: após P3
Codificação alvo: 1,5 a 3 horas
Tempo esperado até PASS: 4 a 8 horas
Esforço agregado estimado: 8 a 16 agente-horas
Dependência: P1

### 11.1 Regra

channel_client_bindings continua sendo histórico de atribuição do canal ao cliente. Ele não autoriza usuário.

Validar:

- account da instância;
- client_account_id do binding;
- snapshot do binding gravado na conversa;
- client_account_id do automation_profile;
- cliente usado por CRM/customer_data;
- vigência temporal do binding.

### 11.2 Reparo

- Criar preview antes de aplicar reparo.
- Mostrar quantidade e intervalo afetados.
- Exigir confirmação separada para aplicar.
- Auditar ator, motivo, before e after.
- Não reatribuir histórico antigo silenciosamente.
- Preservar o cliente original das conversas anteriores ao início de um novo binding.

### 11.3 Aceite de P4

- Automação, CRM e conversa apontam para o mesmo cliente efetivo.
- Reparos são previewáveis, auditados e tenant-safe.
- Binding não concede acesso de inbox.
- Divergência entre binding e automation_profile bloqueia automação com 409.

## 12. Pacote P5 opcional — exclusão física/LGPD

Status: fora da entrega inicial
Codificação alvo se aprovada separadamente: 6 a 10 horas
Tempo esperado até PASS, incluindo backup/restore: 3 a 5 dias úteis
Esforço agregado estimado: 24 a 40 agente-horas
Dependência: P0 estável, backup validado e decisão explícita do usuário

P0 resolve a necessidade operacional de zerar a interface preservando auditoria. Exclusão física é outro produto, com risco e irreversibilidade diferentes.

Se P5 for solicitado:

- reutilizar o mecanismo de retention/purge já existente;
- criar preview e request idempotente;
- preservar uma trilha mínima da operação;
- cancelar jobs/outbox antigos;
- scrub de IA;
- remover mídia e dependências na ordem das FKs;
- nunca apagar dados de outra account/instância;
- validar migrations 0244/0245 e todas as FKs posteriores;
- exigir backup e restore testado;
- usar worker assíncrono e status;
- documentar claramente o ponto irreversível.

Não ligar P5 ao botão de P0 sem nova aprovação.

## 13. Pacote P6 — evidências restantes das fases E0 a E8

Prioridade: depois do núcleo P0–P4
Codificação/ajustes alvo, com implementações existentes e credenciais prontas: 8 a 14 horas
Tempo esperado até PASS técnico, sem latência externa: 3 a 7 dias úteis
Esforço agregado estimado: 20 a 35 agente-horas
Latência de Meta/App Review: externa e não incluída

Não reimplementar fases concluídas. Para cada fase, abrir seu documento em docs/omnichannel/evolucao, verificar o código atual e produzir evidência.

### 13.1 P6A — E0 a E6

Codificação/validação alvo: 6 a 10 horas.

| Fase | Evidência que falta para fechar |
|---|---|
| E0 — ownership | sync, import e deploy alteram somente os dois workflows Omnichannel |
| E1 — Evolution | inbound/outbound real, mídia, quote, ACK, replay, restart, timeout e canário |
| E2 — cérebro n8n | shadow idempotente, retry/circuit breaker, sem envio direto e cancelamento humano × IA |
| E3 — multimodal | áudio, imagem e documento reais; limites; falha acionável; sem base64/chave persistida |
| E4 — CRM/atribuição | identidade, merge/undo, segmentos, consentimento e opt-out tenant-safe |
| E5 — handoff | take/release/transfer, SLA, outbox de aviso e corrida humano × IA |
| E6 — tools/conhecimento | allowlist, assinatura, timeout, rate limit, auditoria e prompt injection |

P6A é PASS quando:

- E0 a E6 não possuem teste crítico skipped;
- n8n não envia ao canal;
- Evolution real passou pelo canário controlado;
- rollback para atendimento humano foi ensaiado;
- owner, ID, hash e active dos workflows foram registrados antes/depois;
- falha externa não perde inbound.

### 13.2 P6B — E7 e E8

Codificação/validação alvo: 2 a 4 horas, depois que contas, credenciais e permissões externas estiverem prontas.

| Fase | Evidência que falta para fechar |
|---|---|
| E7 — WhatsApp Cloud | HMAC, challenge, janela/template, mídia, ACK, canário e um único provider ativo |
| E8 — Instagram | DM, comentário, private reply, prazo/tentativa e App Review |

P6B é PASS quando:

- nenhum número fica ativo em dois providers;
- assinatura/challenge inválidos não persistem evento;
- canário e rollback reais passaram;
- owner, ID, hash e active dos workflows foram registrados;
- permissões/App Review atuais foram documentados;
- nenhum workaround viola capabilities da Meta.

P6 só fecha quando P6A e P6B são PASS.

## 14. Pacote P7 — E9 hardening, escala e recuperação

Prioridade: começa em paralelo cedo e fecha antes de produção ampla
Codificação alvo: 16 a 28 horas
Tempo esperado até PASS: 7 a 12 dias úteis
Esforço agregado estimado: 40 a 70 agente-horas

### 14.1 Observabilidade

- métricas por webhook, dedupe, job, n8n, modelo, tool, handoff e outbox;
- health separado para processo, banco, provider e n8n;
- alertas para workflow inativo, backlog, dead-letter e provider degradado;
- dashboards por conta sem PII em labels;
- runbook e responsável para cada alerta.

### 14.2 Escala e resiliência

- rate limit compartilhado;
- QR/session cache compatível com múltiplas réplicas;
- fairness/backpressure por conta;
- circuit breakers;
- dead-letter observável e reprocessável;
- restart e webhook replay sem duplicação;
- provider, n8n ou modelo indisponível cai para humano.

### 14.3 Segurança e privacidade

- secret scan;
- revisão de PII em logs;
- SSRF e URLs privadas;
- isolamento por conta em store, job e realtime;
- consentimento, opt-out e DSAR conforme decisão jurídica;
- export privado/assíncrono, se aprovado.

### 14.4 Backup e restore

Cobrir:

- PostgreSQL;
- mídia do Omnichannel;
- configuração/dados necessários do n8n;
- estado/configuração Evolution.

Restaurar em ambiente isolado e provar:

- conversa abre;
- mídia abre;
- grants permanecem;
- outbox volta de forma segura;
- RPO e RTO são medidos.

### 14.5 Carga e fault injection

- carga multi-account;
- conta ruidosa não bloqueia outra;
- restart do worker;
- restart da API;
- falha do banco;
- timeout do provider;
- n8n indisponível;
- replay de webhook;
- corrida entre reset, IA e outbound.

P7 só é PASS com restore real, alertas acionáveis e zero vazamento cross-tenant.

## 15. Pacote P8 — E10 rollout controlado

Prioridade: última fase técnica antes de expansão
Codificação alvo: 10 a 18 horas
Tempo esperado até PASS: 4 a 7 dias úteis
Esforço agregado estimado: 25 a 45 agente-horas

O Go/PostgreSQL deve controlar:

- off;
- observe;
- shadow;
- assist;
- auto_pilot;
- active;
- paused.

Também controlar:

- account;
- instância;
- coorte;
- percentual;
- horário;
- tags;
- aprovador;
- motivo;
- métricas da promoção.

Requisitos:

- n8n apenas consulta/obedece ao rollout;
- paused impede novos dispatches/envios de IA rapidamente;
- inbox humano continua funcionando;
- coorte e percentual são determinísticos;
- kill switch é testado;
- rollback de provider, IA e workflow é ensaiado separadamente;
- promoção depende de evidência, não apenas de um clique.

## 16. Ordem de execução recomendada

### 16.1 Núcleo solicitado

1. G0: preflight, segredo revogado/rotacionado, baseline e banco de teste.
2. P0: reset lógico por instância.
3. P1: migration de grants, backfill e resolver canônico.
4. Realtime opaco/fail-closed do próprio P1.
5. P2: painel/API de grants e remoção de gates legados.
6. P3: cliente piloto com número na própria account.
7. P4: bindings/CRM/automação.
8. Browser matrix completa.
9. Revisão independente.

P0 frontend pode avançar em paralelo ao P0 backend. P2 frontend pode avançar com fixtures após o contrato de P1 congelar. Migrations e arquivos compartilhados continuam serializados.

### 16.2 Finalização

1. P7 observabilidade começa durante P1.
2. P6A valida E0–E6.
3. Preparação administrativa de E7/E8 começa cedo.
4. P7 fecha hardening/restore.
5. P6B faz canário E7/E8 quando providers/permissões estiverem prontos.
6. P8 implementa rollout e kill switch.
7. Produção entra em janela observada.

## 17. Divisão multiagente sem conflito

Com quatro slots:

| Responsável | Faixa principal |
|---|---|
| Integrador | número de migration, module.go, contrato de scope, workflows, merge e gates |
| Agente backend | service/store/http do pacote atribuído |
| Agente frontend | Vue, composables, tipos e testes web |
| Agente realtime/QA | módulo realtime, integração, matriz cross-tenant e revisão |

Arquivos que não podem ser editados em paralelo:

- back/internal/modules/omnichannel/module.go;
- qualquer migration;
- AGENT.md;
- tipos/API centrais compartilhados;
- os dois workflows n8n;
- package.json e scripts de deploy.

Regra de handoff:

- executor entrega diff e testes;
- revisor diferente inspeciona;
- integrador aplica ajustes compartilhados;
- ninguém aprova o próprio pacote.

## 18. Comandos de validação

### 18.1 Diff e higiene

    git status --short
    git diff --check
    git diff -- <allowlist do pacote>

### 18.2 Backend

A partir de back:

    go test ./internal/modules/omnichannel/...
    go run ./cmd/migrate status

Quando o pacote tocar realtime:

    go test ./internal/modules/realtime/...

Integration tests devem usar banco PostgreSQL isolado e todas as migrations reais. Não substituir por DDL manual reduzido.

### 18.3 Frontend

Usar o container já existente:

    docker compose exec -T web npm run test -- <arquivos de teste>
    docker compose exec -T web npm run typecheck
    docker compose exec -T web npm run lint
    docker compose exec -T web npm run build

Arquivos de teste mínimos do núcleo, existentes ou a criar:

- app/domain/utils/permissions.test.ts;
- app/domain/utils/permissions-workspaces.test.ts;
- app/composables/omnichannel/useOmnichannelWhatsAppSession.test.ts;
- app/components/omnichannel/config/ConfigNumberCard.test.ts;
- app/composables/omnichannel/useOmnichannelInboxBootstrapLoaders.test.ts;
- app/composables/omnichannel/useOmnichannelInboxHistory.test.ts;
- app/composables/omnichannel/useOmnichannelInboxRealtime.test.ts;
- app/pages/omnichannel/index.test.ts.

Não instalar dependências sem necessidade comprovada e autorização.

### 18.4 Compose

    docker compose config
    docker compose ps

### 18.5 Migration

Depois de migration nova e somente quando a etapa autorizada chegar:

    docker compose build --no-cache api
    docker compose up -d api

Build/recreate local, deploy e qualquer operação na VPS são ações diferentes. Executar apenas o que o usuário autorizou.

### 18.6 n8n

Executar testes de ownership previstos no repositório. Importar ou ativar workflow é ação de deploy e exige autorização explícita. Registrar ID, owner, hash e active antes/depois.

## 19. Matriz obrigatória no navegador

| Perfil | Instâncias esperadas | Conversas esperadas | Ações esperadas |
|---|---|---|---|
| platform admin | conforme bypass central e conta selecionada | conforme contrato central; nenhuma mistura de accounts | reset ainda exige privacy.manage |
| gestor da agência com manage em A | A | todas de A; nenhuma de B | configura A; reset se privacy.manage |
| administrador com manage somente em A | A | todas de A | configura A; não configura B |
| supervisor com reply em A e fila F | A | atribuídas a ele ou da F | reply/assign/close conforme permissões; sem config/reset |
| atendente atribuído em A | A | atribuídas ou filas ativas | view/reply conforme permissões |
| atendente sem grant/fila | nenhuma restrita | nenhuma WhatsApp | nenhuma ação |
| client owner com módulo e manage próprio | número da própria account | somente própria account | configura/opera conforme permissões |
| cliente sem módulo | nenhuma | nenhuma | rota bloqueada |
| usuário sem conversations.view | nenhuma no inbox | nenhuma | configuração independente somente se tiver contrato próprio |
| usuário com memberships A/B | somente accounts A/B no switcher | somente a account ativa | troca limpa estado antes do bootstrap |

Cenários:

- instância conectada;
- instância deslogada com histórico antigo;
- reset da A sem alterar B;
- nova mensagem após reset;
- grant adicionado;
- grant removido;
- membership removida;
- troca de conta;
- fila/atribuição;
- busca;
- abertura direta por ID;
- mídia;
- realtime;
- sessão/QR/login/logout;
- configuração de IA e binding.
- /instances/access vazio versus 403/500/rede;
- ticket aberto antes de revogar grant;
- evento em voo, reconnect e duas abas;
- localStorage com instância que perdeu acesso;
- 409 de historyResetRevision e accessRevision.

Evidência mínima:

- perfil;
- account;
- instância;
- resultado esperado;
- resultado observado;
- screenshot sem PII;
- request/status sem token;
- PASS ou BLOCKED.

## 20. Gate antes de deploy

Deploy só pode ser proposto quando:

- diff está limitado à allowlist;
- mudanças do usuário foram preservadas;
- nenhum segredo apareceu;
- migrations foram revisadas e aplicadas em banco isolado;
- testes unitários e de integração passaram;
- frontend typecheck/lint/build passou;
- browser matrix passou;
- cross-tenant passou;
- rollback foi ensaiado;
- backup/restore foi validado para qualquer operação destrutiva;
- workflows externos não mudaram;
- revisor independente aprovou.

O comando de deploy deve ser o comando canônico definido no package.json no momento da execução. Não assumir o conteúdo atual enquanto package.json tiver alterações do usuário.

## 21. Janela de produção

Tempo de engenharia não é igual a tempo de observação.

Sugestão:

- observe: 24 a 48 horas;
- shadow: 3 a 7 dias;
- assist: 3 a 7 dias;
- auto_pilot 10%: pelo menos 48 horas;
- 25%: 48 horas;
- 50%: 72 horas;
- 100%: somente com gates estáveis;
- mais 7 dias em 100% antes de declarar produção comprovada.

Essas janelas não significam trabalho contínuo do agente. Elas existem para observar tráfego real e detectar falhas raras.

Pausar imediatamente em caso de:

- vazamento cross-tenant;
- envio indevido;
- perda ou duplicação;
- segredo exposto;
- dois providers no mesmo número;
- kill switch inoperante;
- reset afetando outra instância.

## 22. Estimativas consolidadas

Codificação alvo é o período de implementação ativa com agentes em paralelo. Tempo até PASS inclui migrations serializadas, integração, suíte, browser, revisão independente e ensaio de rollback. Esforço agregado soma o trabalho de todos. Docker deve estar funcional e os contratos não podem mudar no meio.

| Entrega | Codificação alvo | Tempo até PASS | Esforço agregado |
|---|---:|---:|---:|
| P0 — reset lógico | 1,5–2,5 h | 4–8 h | 12–20 h |
| P1 — grants/resolver/realtime | 4–7 h | 1–2 dias úteis | 24–40 h |
| P2 — painel/gates | 2–4 h | 4–8 h | 8–14 h |
| P3 — cliente próprio/switcher | 3–5 h | 1–2 dias úteis | 8–16 h |
| P4 — binding/automação | 1,5–3 h | 4–8 h | 8–16 h |
| Núcleo P0–P4 completo | 12–20 h | 5–10 dias úteis | 60–105 h |
| P5 físico opcional | +6–10 h | +3–5 dias úteis | +24–40 h |
| P6A/P6B — evidências E0–E8 | 8–14 h | 3–7 dias úteis, sem Meta | 20–35 h |
| P7 — E9 hardening | 16–28 h | 7–12 dias úteis | 40–70 h |
| P8 — E10 rollout | 10–18 h | 4–7 dias úteis | 25–45 h |
| Engenharia completa restante | 45–75 h | 15–25 dias úteis | 200–360 h |

Prazo provável:

- núcleo solicitado: 5 a 10 dias úteis até PASS, embora a codificação ativa seja menor;
- engenharia completa: 15 a 25 dias úteis até release-ready;
- Meta/App Review, jurídico e soak podem acrescentar tempo externo;
- produção comprovada tende a 5 a 8 semanas no melhor cenário e exige a janela observada da seção 21.

## 23. Definition of Done

O núcleo só está concluído quando:

- reset lógico funciona por instância e preserva auditoria;
- endpoint antigo não apaga fisicamente;
- usuário sem grant não vê instância restrita;
- uma única instância não abre acesso automático;
- instância sem dono não fica pública;
- cada admin autorizado pode ter escopo próprio;
- cliente opera número na própria account;
- settings.manage não implica ler tudo;
- REST, busca, mídia, ações e IA usam o mesmo scope; realtime só invalida e força essas APIs;
- zero WhatsApp acessível vira zero conversas WhatsApp, sem abrir account-wide nem bloquear Instagram autorizado;
- remover grant/membership retira acesso imediatamente;
- troca de conta limpa caches;
- testes cross-tenant e browser passam;
- rollback está documentado e ensaiado.

O módulo completo só pode ser chamado de 100% quando, além do núcleo:

- E0–E8 possuem smokes reais;
- E9 possui métricas, alertas, carga, backup e restore;
- E10 possui rollout e kill switch;
- provider/cutover não duplica sender;
- zero teste crítico permanece skipped;
- ownership n8n foi provado ponta a ponta;
- matriz cross-tenant completa passou;
- RPO/RTO foram medidos;
- consentimento, opt-out e DSAR foram efetivamente fechados;
- todos os canários e rollbacks reais passaram;
- produção atravessou a janela observada sem incidente crítico.

Se Meta, jurídico, consentimento ou DSAR estiverem pendentes, o resultado máximo é piloto limitado/BLOCKED para conclusão integral. Não declarar 100%.

## 24. Modelo de handoff de cada agente

Preencher sempre:

    Pacote:
    Executor:
    Revisor:
    SHA base:
    Allowlist:
    Arquivos alterados:
    Migration:
    Decisões aplicadas:
    Testes executados:
    Resultado dos testes:
    Evidência cross-tenant:
    Evidência no navegador:
    Rollback ensaiado:
    Riscos/pêndencias:
    Resultado final: PASS ou BLOCKED

Se BLOCKED, informar:

- condição exata;
- arquivo/linha;
- tentativa feita;
- risco de continuar;
- decisão necessária.

## 25. Fora de escopo sem nova aprovação

- Inbox unificado cross-account.
- Exclusão física ligada ao botão de limpeza.
- Alteração de senha.
- Troca de provider em produção.
- Ativação de WhatsApp Cloud ou Instagram.
- Importação/ativação de workflow n8n.
- Deploy na VPS.
- Mudança em workflows que não pertencem ao Omnichannel.
- Criação de permissão nova por conveniência.
- Hard delete de account, cliente, instância ou contato.

Este documento é o contrato de execução. Alterações de produto devem primeiro atualizar este plano e receber aprovação; só depois entram em código.
