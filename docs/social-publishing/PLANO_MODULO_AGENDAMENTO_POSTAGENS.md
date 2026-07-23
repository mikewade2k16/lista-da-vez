# Módulo de Agendamento de Postagens

Status: planejamento aprovado para implementação isolada em 2026-07-23.

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
- Fila durável no PostgreSQL com idempotência, retry e dead letter.
- Coleta de analytics por publicação e visão geral do período.
- Contexto somente leitura para consumo futuro pelo Crow Assistant.
- Isolamento completo por `account_id` no handler, service e repository.

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

## Fluxo de publicação

1. O service valida conta, conexão, mídia, legenda, timezone e horário.
2. Uma transação grava a publicação e um job `social.publish`.
3. O worker Go reivindica o job somente quando `run_after` vence.
4. O adapter cria o container de mídia e solicita `media_publish` no Instagram.
5. O resultado externo é persistido antes de concluir o job.
6. Jobs `social.analytics.refresh` coletam métricas em janelas posteriores.

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

## Estratégia de teste e retorno

O módulo será validado em camadas: testes unitários de domínio e adapter HTTP,
testes de repository/handler quando o ambiente permitir, typecheck/lint/build do
frontend e smoke visual da nova rota.

As alterações são agrupadas por diretórios e por uma única migration aditiva.
Até a integração futura, o retorno consiste em desabilitar o módulo no Module
Registry e remover a rota. Nenhuma tabela ou código do Calendário depende desta
fundação.

## Fases

1. Fundação isolada: schema, módulo Go, fila, API, permissões e workspace.
2. Piloto técnico: conectar credencial de teste e publicar imagem controlada.
3. Homologação: observar retries, idempotência e atualização dos analytics.
4. Integração com Calendário: somente após novo comando.
5. Integração com Crow Assistant e expansão de formatos: somente após o contrato
   do piloto estar estável.

