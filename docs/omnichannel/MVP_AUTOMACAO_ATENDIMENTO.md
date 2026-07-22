# MVP — Automação de atendimento WhatsApp

Status: **MVP-01 e núcleo MVP-02 concluídos localmente; primeira tela do MVP-03 disponível**.
Nenhuma migration foi aplicada, nenhum container/deploy foi alterado e somente o workflow
`workflow-omnichannel-brain.json`, pertencente ao Omnichannel, recebeu o contrato v3.

## 1. Resultado do MVP

Uma nova página `/omnichannel/automacao` permitirá configurar e acompanhar o primeiro
atendimento feito pela IA, sem responder mensagens pelo painel. O operador continua usando
WhatsApp no celular ou WhatsApp Web. O painel serve para configurar a automação, enxergar os
casos que exigem intervenção e acompanhar indicadores básicos.

O MVP cobre somente WhatsApp e três resultados operacionais:

1. a IA inicia e conduz o primeiro atendimento;
2. a IA sugere o encerramento e o Go decide se pode encerrar;
3. a IA solicita transferência e o Go cria o handoff para a fila humana.

Instagram e comentários permanecem fora deste MVP. A conexão pela API oficial do WhatsApp
entra depois da validação do MVP, reutilizando a mesma instância lógica, CRM, FSM e outbox.

## 2. Autoridade por camada

| Go/PostgreSQL | n8n |
|---|---|
| Evolution/webhooks e futuro WhatsApp Cloud | Debounce e agrupamento |
| Dedupe e idempotência | Montagem do contexto recebido do Go |
| Perfil cliente–número–agente | Chamada ao modelo |
| CRM, mensagens, mídias e estados | Transcrição e visão |
| Filas, responsáveis e handoff | Consulta a ferramentas autorizadas |
| Validação de encerramento | Resposta estruturada sugerida |
| Auditoria, custo, outbox e envio | Nunca envia direto ao canal |

Nenhum workflow de Automation/WAHA, Calendário ou outro módulo pode ser editado. O único
workflow WhatsApp pertencente ao Omnichannel é `workflow-omnichannel-brain.json`.

## 3. Perfil de automação por cliente

Fonte do cliente: a mesma lista permission-scoped de `/v1/tenants` usada pelo Calendário.
O Omnichannel não consulta `calendar.*` por SQL. Um adapter fino, montado em `platform/app`,
expõe somente o perfil estratégico necessário ao atendimento.

Cardinalidade do MVP:

- um perfil por `account_id + client_account_id`;
- um número/instância associado a apenas um cliente;
- um agente publicado associado ao perfil;
- o provider vem da instância e pode mudar de Evolution para WhatsApp Cloud sem mudar o perfil.

O cadastro estratégico continua tendo uma única fonte em `calendar.client_profiles`. A tela
nova pode reutilizar o template visual do modal do Calendário, mas não cria uma segunda cópia.

## 4. Encerramento automático configurável

A IA nunca grava `closed`. Ela devolve uma sugestão estruturada; o Go aceita somente quando
todas as condições habilitadas passam.

| Condição | Configurável | Default de teste |
|---|---:|---:|
| fechamento automático habilitado | sim | `false` |
| confiança mínima | sim | `0.90` |
| exigir todos os campos obrigatórios | sim | `true` |
| bloquear quando houve pedido de humano | sim | `true` |
| bloquear assunto sensível | sim | `true` |
| geração da conversa ainda válida | não desligável | `true` |

A lease `conversations.ai_generation` é uma trava estrutural, apresentada no contrato como
`validGenerationRequired: true`. Permitir desligá-la aceitaria resposta atrasada depois de um
humano assumir a conversa; por isso ela não é um toggle do painel.

Cada tentativa de encerramento registra os gates avaliados, o resultado e a geração capturada
em `messaging.ai_close_evaluations`, sem persistir prompt, chave ou conteúdo da conversa.

## 5. APIs do MVP

- `GET /v1/omnichannel/automation/profiles`: todos os clientes visíveis, configurados ou não;
- `GET /v1/omnichannel/automation/profiles/{clientId}`: configuração e contexto estratégico;
- `PUT /v1/omnichannel/automation/profiles/{clientId}`: full replace tenant-scoped.
- `GET /v1/omnichannel/automation/interventions`: handoffs pendentes/na fila, filtráveis por
  cliente visível, expondo somente as chaves dos campos coletados.

Todas exigem `RequireAuthWithAccount` e `omnichannel.settings.manage`. `account_id` vem do
Principal; `clientId` vem do path e precisa pertencer à lista de clientes visíveis. Número e
agente são validados dentro da mesma conta. Recurso fora do escopo responde 404.

## 6. Fases executáveis

### MVP-01 — contrato e configuração por cliente

- migration `0228_messaging_automation_profiles.sql`;
- APIs tenant-scoped;
- adapter de leitura do perfil estratégico do Calendário;
- resolução do agente pelo número com fallback temporário somente para contas ainda sem perfil;
- testes de defaults, acesso, validação e isolamento lógico.

### MVP-02 — iniciar, finalizar e transferir

- incluir `client` e `businessContext` no contrato versionado do cérebro;
- detectar resposta humana `fromMe` e invalidar a geração antes de qualquer nova resposta da IA;
- acrescentar decisão estruturada `close` sem permitir mudança direta de estado pelo n8n;
- implementar o avaliador Go dos seis gates de encerramento;
- usar handoff existente para transferência, resumo e campos coletados;
- emitir eventos operacionais idempotentes para os cards.

Entregue no núcleo local:

- `brain.request/result.v3`, preservando `brain.v2` por versão do agente;
- contexto estratégico do cliente resolvido pelo número e injetado no cérebro;
- takeover `fromMe` atômico para celular/WhatsApp Web;
- avaliador Go dos gates, auditoria `0229` e encerramento com resposta final/outbox atômicos;
- handoff real e idempotente para pedido humano, bloqueio de close e falhas da IA.

Também entregue localmente: endpoint/read model dos cards e teste de contrato para takeover
`fromMe` idempotente. Pendente desta fase: eventos realtime específicos e teste integrado com
as migrations aplicadas em banco descartável/controlado.

### MVP-03 — página `/omnichannel/automacao`

- seletor de cliente idêntico ao Calendário;
- abas Visão geral, Configuração e Intervenções;
- associação número/agente, prompt/versionamento, modelo e chave mascarada;
- template do perfil estratégico reutilizado do Calendário;
- política de fechamento com avisos claros e defaults seguros.

Primeira versão entregue localmente: navegação permission-scoped, seletor de cliente, visão
geral, configuração cliente–número–agente, política de fechamento, contexto estratégico,
configuração existente do agente e aba de intervenções. Não existe composer de mensagem.

### MVP-04 — cards de intervenção

- aguardando humano, baixa confiança, assunto sensível, erro de IA e limite atingido;
- filtros por cliente, número, setor e tempo de espera;
- ações mínimas: marcar como visto, abrir no WhatsApp e atribuir responsável;
- realtime apenas como invalidação; a API refaz a leitura autoritativa.

Entregue no recorte atual: cards vindos do handoff autoritativo, filtro pelo cliente selecionado,
tempo de espera, motivo, resumo seguro, quantidade de campos e ação para abrir o WhatsApp.
Marcar como visto, atribuição pelo card, filtros avançados e realtime permanecem posteriores.

### MVP-05 — teste controlado

- shadow sem resposta automática;
- piloto interno com um número Evolution;
- testes de primeira mensagem, debounce, duplicata, mídia, pedido humano, transferência e fechamento;
- medir tempo de primeira resposta, handoff, erro, duplicata e custo;
- congelar os thresholds depois dos resultados.

### Pós-MVP — WhatsApp Cloud API

- cadastrar/verificar o número oficial e webhooks Meta;
- migrar um número por vez, nunca ativo simultaneamente em Evolution e Cloud;
- manter os mesmos perfis, conversa, CRM, FSM e outbox;
- validar janela de 24h, templates, mídia, status e rollback.

## 7. Fora de escopo imediato

- responder pelo painel;
- CRM avançado do atendente;
- Instagram DM/comentários no MVP;
- KPIs definitivos antes do piloto;
- remoção de workflows ou WAHA pertencentes a outros módulos;
- deploy, import/export de workflow ou alteração de containers.
