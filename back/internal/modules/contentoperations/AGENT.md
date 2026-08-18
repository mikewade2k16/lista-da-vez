# Operação de conteúdo

Módulo de leitura que calcula alertas operacionais a partir de `tasks.tasks.ui_metadata.checklist` e `calendar.events`.

## Estado atual — incompleto e pausado

Em 2026-08-18, por decisão do dono do produto, a implementação foi pausada antes da
homologação final. O módulo está **em andamento** e não deve ser apresentado como
concluído, validado integralmente ou pronto para produção.

Já existe:

- endpoint de leitura `GET /v1/content-operations/brief`;
- cálculo de prioridades por cliente usando Tasks e Calendário;
- estados e datas opcionais nos itens de checklist usados pelo cálculo;
- página `/operacao-conteudo` com resumo e links para a origem;
- exibição dos lembretes na central global de notificações, identificados como
  `Operação de conteúdo`;
- contexto de leitura limitado para o Crow responder perguntas, sem torná-lo responsável
  por gerar ou disparar alertas;
- testes unitários iniciais das regras do service e teste do agregador global de
  notificações.

Ainda falta antes de considerar o módulo concluído:

- completar testes HTTP, repository, permissões, isolamento multi-tenant e recorte por cliente;
- cobrir limites de data/fuso e os modos de segunda-feira, acompanhamento e fechamento;
- adicionar testes frontend do composable, página, estados vazio/erro/loading e navegação
  para Tasks/Calendário;
- executar E2E com dados representativos de clientes ativos e validar falsos positivos e
  falsos negativos de cada regra;
- homologar o comportamento integrado após alteração real em task, item e evento de calendário;
- definir persistência/histórico, confirmação, adiar/dispensar e preferências de alerta;
- implementar disparo fora de uma sessão aberta, caso o produto deva avisar por e-mail,
  WhatsApp ou outro canal; hoje a central global consulta os dados enquanto o sistema está aberto;
- realizar a validação final local e, somente depois de tudo fechado, validar na VPS mediante
  autorização explícita.

O acompanhamento canônico desta pausa está na fase `CO-F0` do roadmap.

## Invariantes

- Não depende do Crow Assistant para disparar alertas.
- A rota `/v1/content-operations/brief` é gateada por `content_operations` no Module Registry.
- Não grava estado derivado: PostgreSQL de Tasks e Calendário continua sendo a fonte autoritativa.
- Exige, ao mesmo tempo, `tasks.tasks.view` e `calendar.view` na account ativa.
- Toda query repete `account_id` de armazenamento e restringe `client_id` ao escopo resolvido pelo Calendário.
- O Crow pode consumir somente o `Brief` pronto para responder perguntas; nunca aciona nem altera os alertas.
- Datas e modos usam `America/Sao_Paulo`; segunda é planejamento e sexta a domingo é fechamento.
