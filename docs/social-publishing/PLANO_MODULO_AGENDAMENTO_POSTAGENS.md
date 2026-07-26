# Módulo de Agendamento de Postagens

Status: fundação isolada implementada e validada localmente em 2026-07-23.
Homologação externa com uma conta Meta de teste controlada permanece pendente.

## Objetivo

Criar um workspace semelhante ao Buffer para conectar uma conta profissional do
Instagram, preparar e agendar publicações e acompanhar seus resultados. Nesta
primeira etapa o módulo nasce isolado, com banco, API e página próprios.

O Calendário e o Crow Assistant não serão alterados nesta etapa. O módulo apenas
publicará contratos estáveis para que essas integrações sejam feitas depois dos
testes, mediante comando explícito.

## Recorte do primeiro piloto

- Rota do painel: `/postagens`.
- Módulo e schema PostgreSQL: `social_publishing`.
- Canal inicial: Instagram profissional, com publicação de imagem única.
- Conexão técnica por token da Instagram API, cifrado pelo `secretbox`.
- Rascunho, agendamento, edição, cancelamento e tentativa manual de publicação.
- Filas duráveis no PostgreSQL com idempotência, retry e dead letter:
  `social_publishing.outbox` para publicação e
  `social_publishing.analytics_outbox` para coleta de métricas.
- Coleta de analytics por publicação e visão geral do período.
- Contexto somente leitura para consumo futuro pelo Crow Assistant.
- Isolamento completo por `account_id` no handler, service e repository.

O contrato de contexto permanece interno nesta etapa. Nenhuma rota de runtime do
Crow Assistant é registrada antes do comando explícito de integração.

## Fora deste piloto

- Qualquer alteração em `calendar.*`, `/calendario` ou seus componentes.
- Acionamento do módulo pelo Calendário.
- Exibição de analytics dentro do Calendário.
- Alteração de prompts, tools ou workflows do Crow Assistant.
- Reels, Stories, carrossel e publicação em outras redes.
- Upload duplicado de mídia. Durante o piloto será usada uma URL HTTPS pública;
  futuramente a origem será o arquivo autorizado pelo Calendário.
- OAuth/Embedded Signup para onboarding de clientes. O piloto começa com uma
  conexão técnica real; o fluxo guiado entra antes da liberação ampla.

## Contratos para a integração futura

Cada publicação possui `source_type` e `source_ref`. O par é idempotente por
conta, permitindo que um evento do Calendário ou uma ação do Crow Assistant seja
reprocessado sem gerar postagem duplicada.

Valores previstos para `source_type`:

- `manual`: criado na página do módulo.
- `calendar`: reservado para uma integração futura.
- `crow_assistant`: reservado para uma integração futura.

O módulo é o único dono da publicação, do estado da fila, dos IDs externos e dos
analytics. O Calendário continuará dono do evento editorial e do arquivo. O Crow
Assistant será consumidor autorizado desses dados, não uma segunda fonte de
verdade.

Na integração, o Calendário deverá entregar uma referência durável do arquivo,
não uma URL assinada que possa expirar antes do horário agendado. O adapter
futuro resolverá uma URL pública válida perto da execução, sem duplicar a mídia
ou transferir sua propriedade para o Calendário.

## Fluxo de publicação

1. O service valida conta, conexão, mídia, legenda, timezone e horário.
2. Uma transação grava a publicação e um job `social.publish` na outbox de
   publicação.
3. O pool de workers Go reivindica o job somente quando `run_after` vence.
4. O adapter cria o container de mídia e solicita `media_publish` no Instagram.
5. O resultado externo é persistido antes de concluir o job.
6. Jobs `social.analytics.refresh`, em uma outbox e worker próprios, coletam
   métricas em janelas posteriores.

O n8n não envia ao Instagram. Se usado no futuro, será apenas orquestrador que
chama a API Go autenticada; PostgreSQL e Go permanecem autoritativos.

## Segurança e confiabilidade

- Token nunca volta ao frontend e nunca entra em logs ou mensagens de erro.
- Credencial cifrada com `OMNI_SECRETS_KEY`.
- Permissões separadas para visualizar, gerenciar, conectar e sincronizar.
- URLs de mídia exigem HTTPS e não aceitam credenciais embutidas.
- Jobs e referências externas têm chaves de idempotência por conta.
- Erros do provedor são sanitizados antes de persistir ou responder.
- Publicação já enviada não é apagada remotamente por um cancelamento local.
- Imediatamente antes de `media_publish`, o backend registra a tentativa. Se a
  resposta externa ficar ambígua por timeout, queda do worker ou falha ao
  persistir o resultado, o post recebe `publish_outcome_unknown` e não é
  reenviado automaticamente. O operador precisa conferir o Instagram; a
  reconciliação assistida será uma evolução do piloto.

## Ativação controlada

A migration cadastra o módulo, mas não o habilita em massa para todas as contas.
O piloto deve ser ativado pelo fluxo administrativo de módulos somente nas
contas escolhidas. Desabilitar `social_publishing` na mesma tela é o rollback
operacional: a rota e o worker ficam bloqueados, enquanto o histórico permanece
preservado para auditoria.

## Validação executada e retorno

A fundação isolada foi validada em camadas:

- `go test ./...` em todo o backend e `golangci-lint` no módulo: aprovados.
- 32 testes dirigidos de domínio, API e store no frontend: aprovados.
- ESLint dirigido no módulo e nos gates: sem erros.
- Build de produção Nuxt completo: aprovado.
- Smoke visual em desktop e mobile da fila, analytics, conexão e compositor:
  aprovado.
- Migration validada em banco PostgreSQL 16 vazio, no upgrade `0237 -> 0238`,
  em reexecução literal e com escrita compatível pelo binário anterior:
  aprovada.
- O typecheck global ainda aponta o passivo preexistente do frontend, sem
  ocorrência relacionada a `social-publishing` ou `/postagens`.

A migration `0237` permanece como a fundação original e imutável; a correção
aditiva `0238` introduz as garantias de confiabilidade sem quebrar rollback do
binário. Até a integração futura, o retorno operacional consiste em desabilitar
`social_publishing` no Module Registry. A rota e os workers ficam bloqueados e o
histórico permanece disponível para auditoria. Nenhuma tabela ou código do
Calendário depende desta fundação.

Não foi feita uma publicação real no Instagram nem coleta real de métricas nesta
etapa, pois isso exige uma credencial Meta e uma conta profissional de teste
controladas. Essa homologação deve ocorrer antes da ativação para clientes.

## Fases

1. Fundação isolada: schema, módulo Go, filas, API, permissões e workspace —
   concluída localmente.
2. Piloto técnico: conectar credencial de teste e publicar imagem controlada —
   pendente de homologação Meta.
3. Homologação: observar retries, idempotência e atualização dos analytics.
4. Integração com Calendário: somente após novo comando.
5. Integração com Crow Assistant e expansão de formatos: somente após o contrato
   do piloto estar estável.
