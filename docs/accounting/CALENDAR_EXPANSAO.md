# CT-14 — preparação da expansão Calendar

Data: 2026-11-05. Estado: **parcial; integração de runtime bloqueada pelo aceite M1**.
Primeira superfície escolhida: Calendar. Não houve ativação, mudança de configuração,
rota, tabela, histórico ou visual. Meta Ads e Tasks não receberam nova expansão.
O pedido autoriza trabalhar CT-13/14; não constitui aprovação funcional do piloto.

## Dependências verificadas

CT-07 está implementado com limites registrados no handoff. CT-13 entregou avaliação
local e decisões independentes de não adoção, não um classificador opcional integrado.
O requisito de CT-14 é explícito: “Depende do aceite M1 e das decisões CT-13”.
[ACEITE_PILOTO](ACEITE_PILOTO.md) ainda registra nenhuma funcionalidade aprovada pelo
usuário. Portanto, inventário e desenho avançam; integração e qualificação não estão
concluídas. A ausência de Jeff ou MCP não impede uma futura implementação determinística.

## Matriz de equivalência e regressão

| Capacidade atual | Fonte existente | Invariante da expansão | Verificação necessária |
|---|---|---|---|
| Shell, atalhos e layout | CalendarChatPanel, useCalendarChatWindow | Preservar dimensões, posição, abertura e atalhos | Desktop/mobile, foco, Esc, recarga |
| Conversas e mensagens | chat_conversations.go, chat_store.go | Mesmos IDs, autores e histórico persistido | Ler conversa antiga e nova após rollback |
| Contexto e escopo de cliente | chat_access.go, runtime_context.go | Conta/cliente resolvidos no backend; página só sugere contexto novo | Duas contas, cliente revogado, escopo all/client |
| Superfície original | resolveChatTarget, immutableAssistantConversationSurface | Navegar Finance/Calendar não transforma conversa salva | Rejeitar superfície diferente; recuperar na original |
| Voz e transcrição | chat.go, assistant.go e composable atual | Mesma resolução de credenciais e escopo; sem herança fiscal | Permissão revogada e indisponibilidade do provedor |
| Anexos e mídias de eventos | fluxo Calendar existente | Mídia permanece no owner e escopo; não importar comprovantes como corpus | Upload, leitura e remoção autorizados |
| Propostas e confirmações | chat_proposal_execution*.go | Snapshot autoritativo, confirmação explícita, CAS e receipt | Mesmo/diferente hash, concorrência, stale e revogação |
| Notas/perfil/Tasks | sanitizers e executores atuais | Preservar limites atuais, inclusive operações indisponíveis | Não anunciar tool sem executor transacional |
| Configuração/modelos | configuração e cofre existentes | Fonte única; Calendar independente do módulo Accounting | Sem Accounting contratado; credenciais de contas distintas |
| Status e falha de IA | AssistantChatStatus e ChatAsk | Status deve distinguir leitura local de IA disponível | LLM/n8n/chave ausentes sem falso sucesso |

A matriz é um plano de regressão, não evidência de teste manual ou implementação nova.
Não extrair componentes comuns antes de provar dois consumidores e contrato equivalente.
O transporte e a persistência existentes já são compartilhados; não criar chat paralelo.

## Fatia proposta para implementação após o gate

Começar por leitura determinística de eventos de data explícita, com registry limitado
e schema fechado. Não implementar emissão fiscal, persona contábil, SQL gerado, URL
arbitrária ou escrita Calendar via texto de modelo. Pedidos incompletos/ambíguos recebem
pergunta de esclarecimento. A gramática exata e a cobertura precisam de teste próprio.

Opt-in por conta/superfície deve persistir na configuração canônica do Calendar,
desligado por padrão, sem seed ou habilitação em contas existentes. Não inserir o flag
no objeto de layout `chat`: useCalendarChatWindow o substitui ao salvar dimensões.
Novo campo e UI são proposta; **não existem nesta entrega**.

Resolver ator, associação atual, módulo habilitado, calendar.view, conversa e clientes
visíveis; revalidar revogação antes de retornar/persistir e no replay. Acesso negado a
recurso de outro escopo retorna 404. Uma lista all nunca autoriza clientes arbitrários.
Reutilizar projeções lean e limites/paginação; não retornar descrição, mídia ou campos
privados desnecessários. Nenhuma consulta pode depender do Accounting contratado.

O caminho local deve entrar antes dos gates de modelo/n8n de ChatAsk. O preflight da
interface hoje também depende do status; alterar só ChatAsk deixaria o recurso inacessível.
Status futuro precisa informar honestamente disponibilidade limitada, mantendo voz,
escritas e consultas legadas compatíveis. Esta leitura sem IA ainda não foi implementada.

Reutilizar Claim/Bind/Complete do pedido, mensagens e conversas existentes. Não presumir
atomicidade de várias chamadas ao store: validar recuperação de falha entre etapas.
Preservar confirmação transacional das escritas atuais. Replay exige autorização atual,
inclusive clientes removidos do escopo desde a primeira resposta.

## Rollout, observabilidade e rollback propostos

Após aceite M1: implementar/testar fatia desligada; qualificar com PostgreSQL descartável;
obter aceite específico Calendar; ativar só conta/superfície atribuída. Medir rota,
resultado, latência, fallback e chamadas IA, sem texto privado ou credenciais em logs.
Rollback pelo mesmo flag canônico; mensagens do piloto continuam legíveis no histórico
existente. Testar desativação após conversa nova e recuperação sem perda de IDs.
Não existe ainda flag/telemetria/adapter desta expansão no runtime.

## Evidência executada e próximos testes

[evaluations/CT-13-14.json](evaluations/CT-13-14.json) registra a regressão dos pacotes
Calendar/Finance com -race, sem TEST_DATABASE_URL. Skips estão individualizados.
Testes de superfície imutável, capabilities, módulo desligado, permissões não resolvidas,
transcrição revogada e hash idempotente executaram no código atual. Não são prova de
um adapter novo. A evidência PostgreSQL anterior de CT-10 permanece histórica.

Para implementar e qualificar: PostgreSQL novo integralmente migrado, fixtures de duas
contas/atores/clientes, flag off/on, revogação em voo e replay, IA indisponível, limites,
conversa antiga/nova, troca de página, confirmação stale/concorrente e rollback.
Roteiro visual: histórico antigo; pergunta local incompleta/completa; vazio/erro; troca
Finance/Calendar; áudio; propostas; teclado, mobile e recarga. Browser não executado.

Pendências e critérios de encerramento: [CT-15](prompts/CT-15.md), P15-01/25/27/30 e
P15-37. CT-14 só termina com uma superfície implementada e qualificada, não com esta matriz.
