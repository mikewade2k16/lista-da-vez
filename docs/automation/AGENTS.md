# Projeto: n8n whatsapp

## Migracao para o Omni (2026-06-04)

- Pacote trazido para dentro do projeto principal (Omni / fila-atendimento) como modulo
  `automation/`. Docs em `docs/automation/`. Doc do modulo: `automation/AGENT.md`.
- Containers `n8n`, `waha`, `redis` mesclados no `docker-compose.yml` da raiz sob
  `profiles: ["automation"]` (mesmo projeto/rede do Omni). Sobem so com
  `docker compose --profile automation up -d`. Runbook atualizado em SETUP.md.
- Decisoes da fusao: (1) sem Postgres dedicado do n8n — segue em SQLite; o mini-CRM da
  Etapa 2 usara o Postgres do Omni num schema `automation.*`. (2) WAHA fala com o n8n por
  `http://n8n:5678` (rede interna), nao mais `host.docker.internal`. (3) Portas no host:
  n8n 5680, waha 3010, redis 6380 (nao colidem com api 9091 / web 3003 / postgres 5432 do Omni).
- Volumes do projeto Omni sao NOVOS — a stack comeca do zero (reimportar workflow/credenciais,
  reinstalar community node, reescanear QR). Esperado; ver SETUP secao 5.
- A stack so entrou no `docker-compose.yml` (dev local). Deploy na VPS continua em aberto
  (era a "Fase 2" abaixo) e nao foi adicionado ao `docker-compose.prod.yml`.
- As fases que viram modulo Go/banco (Etapa 2 em diante) aguardam o fechamento da branch
  `refactor/multi-tenant-complete` (regra do MULTITENANT_COMPLETION_PLAN). A migracao de
  infra/pastas nao toca no core multi-tenant.

## Decisao atual

- Fase 1: subir um MVP local no Windows com Docker Desktop, mudando o minimo possivel para continuar seguindo o tutorial.
- Fase 2: fazer o upgrade para a VPS da Hostinger com hardening, URLs corretas e servicos separados quando o MVP estiver validado.

## Estado atual (2026-06-03)

- Prioridade absoluta: validar e aprovar tudo LOCALMENTE antes de qualquer deploy. VPS e o ultimo passo, so depois do MVP aprovado.
- Este e um projeto teste/solto. Destino provavel: portar a logica para o painel proprio ja existente (subdominio de crowvisuals.com.br), em vez de manter n8n rodando na VPS. Decisao em aberto.
- Dominio do futuro: crowvisuals.com.br (ja tem site + painel ativo em outro subdominio). Proxy reverso sugerido: Caddy. Tudo isso so na fase final.
- Gerenciamento dos workflows: via MCP (servidor n8n-mcp), nao por UI manual. Sem versionamento em git por enquanto.
- MCP configurado em .mcp.json apontando para http://localhost:5680 com API key do n8n. Carrega no boot do Claude Code (precisa aprovar o servidor de projeto).

### Workflow atual no n8n local

- "Whatsapp" (id lzhb5JjN5kdcVuRR), hoje INATIVO. n8n versao 2.23.2, usando SQLite interno (Postgres do compose ainda nao ligado ao n8n).
- Pipeline: Webhook (POST /webhook) -> Set "Dados" (session, chatId, pushName, payload_id, event, message, fromMe) -> Switch (event == message) -> AI Agent -> Send Seen (WAHA) -> Send a text message (WAHA).
- AI Agent: systemMessage de Engenheiro/CTO. Modelo: Google Gemini (temp 0.2). Memoria: Redis Chat (sessionKey = chatId, TTL 3600, janela 10).
- Credenciais: googlePalmApi (Gemini), redis, wahaApi. Community node: n8n-nodes-waha.

### Decisao: trocar Gemini -> OpenAI (GPT) [2026-06-03]

- Objetivo do Mike: usar o Custom GPT dele "Perola Buyer Assistant" (consultor de compras p/ joalheria Perola Joias).
- IMPORTANTE: Custom GPT do ChatGPT NAO e acessivel por API. Nao da pra linkar por ID/URL. Reproduzimos no n8n via system prompt + (depois) RAG.
- Especificacao completa do GPT (instrucoes verbatim, quebra-gelos, conhecimento, recursos): ver arquivo gpt-perola-buyer-assistant.md.
- Esse GPT e um RAG de dados (le exportacoes do RP da Perola). O valor real depende dos arquivos de dados, que AINDA NAO foram fornecidos.
- Plano em 2 fases:
  - Fase A (agora): trocar node Google Gemini -> OpenAI Chat Model (lmChatOpenAi, typeVersion 1.3) com modelo BARATO (ex.: gpt-4o-mini) so pra validar o fluxo WhatsApp -> GPT -> resposta. Colar as instrucoes do Perola no systemMessage do AI Agent. Manter memoria Redis. Nesta fase o bot responde no personagem mas SEM os dados reais.
  - Fase B (depois): ingestao dos arquivos do RP como RAG (vector store) ou OpenAI Assistant (file_search) p/ analise real. Avaliar web search como tool. Ajustar modelo (subir de gpt-4o-mini p/ algo mais forte) so depois do fluxo validado.
- Credencial OpenAI: tipo openAiApi, criada no n8n via MCP. A API key e da plataforma platform.openai.com (billing/creditos proprios, separado do ChatGPT Plus). Key NAO fica salva em arquivo do projeto, so na credencial do n8n.
- Pendencia operacional: reload da janela do VSCode p/ o MCP carregar WEBHOOK_SECURITY_MODE=moderate e liberar as tools de API (criar credencial, editar workflow).

### FEITO — Fase A concluida [2026-06-03]

- Credencial openAiApi criada no n8n: "OpenAI account" (id sCzmqFisO8bdeZ9B). API key testada direto na OpenAI: HTTP 200, gpt-4o-mini responde, tem creditos.
- Workflow "Whatsapp" atualizado: node Google Gemini removido; adicionado "OpenAI Chat Model" (lmChatOpenAi v1.3, modelo gpt-4o-mini, temperature 0.2, responsesApiEnabled false) ligado ao AI Agent via ai_languageModel.
- systemMessage do AI Agent trocado pelas instrucoes do Perola Buyer Assistant (4463 chars). Memoria Redis mantida. Workflow valido (0 erros). pinData do Webhook preservado.
- LEMBRETE Fase A: o bot responde NO PERSONAGEM mas SEM dados reais (sem os arquivos do RP). Isso e esperado; serve so pra validar o fluxo.

### FEITO — Etapa 1 (multimodal) + persona Tony [2026-06-03]

- Etapa 1 construida: Dados agora captura mimetype, mediaUrl (host reescrito localhost:3000 -> waha:3000) e hasMedia. Switch virou FILTRO (so user 1:1, fromMe=false, ignora @newsletter/@g.us/@broadcast). Novo Switch "Tipo" roteia por mimetype: audio -> Baixar Audio -> Transcrever (Whisper, pt); imagem -> Baixar Imagem -> Analisar Imagem (visao gpt-4o-mini); texto -> direto. Tudo converge no AI Agent (text = message || text || content).
- Bug corrigido: nao responde mais Newsletter/Canal/grupo (antes dava erro 401 ao tentar responder @newsletter).
- Nota n8n: optional chaining (?.) NAO e suportado nas expressoes desta versao; usar (obj || {}).campo.
- Midia da WAHA tem TTL curto (storage esvazia) -> baixar na hora (no fluxo e imediato, ok). Ver WORKFLOW.md secao 8.
- Persona ATIVA trocada: de Perola Buyer Assistant -> Tony (ver gpt-tony.md). Tony e consultor objetivo, humano, estilo WhatsApp, respostas curtas. Perola continua documentado em gpt-perola-buyer-assistant.md para uso futuro.
- Modelo do AI Agent subido de gpt-4o-mini -> gpt-4o (respostas melhores/mais naturais para os testes). Whisper e visao seguem como estao.
- Video continua fora (Etapa 1 avancado). Workflow valido, ATIVO.

### Modelos: requisitos e troca

- Referencia completa em MODELOS.md (o que cada modelo exige + como trocar).
- Resumo: modelos "pro"/raciocinio (gpt-5.5-pro, o-series, gpt-5*) EXIGEM responsesApiEnabled=true e NAO aceitam options.temperature (remover). Modelos normais (gpt-4o, gpt-4o-mini, gpt-4.1) funcionam com chat completions e aceitam temperature.
- Nos: chat = OpenAI Chat Model (lmChatOpenAi); visao = Analisar Imagem (openAi image/analyze, saida em $json[0].content[0].text); audio = Transcrever Audio (Whisper whisper-1, fixo).

### LIMITACAO IMPORTANTE do n8n-mcp com n8n 2.23.2

- As tools de ESCRITA de workflow do n8n-mcp (n8n_update_partial_workflow, provavelmente create/update_full) FALHAM nesta versao do n8n: a API publica PUT /workflows e estrita (additionalProperties:false) e so aceita name, nodes, connections, settings(executionOrder...), staticData. O n8n-mcp reenvia campos extras (pinData, meta, tags, e settings.binaryMode) e a API rejeita com "request/body must NOT have additional properties".
- O que do n8n-mcp FUNCIONA: leitura (get/list/validate workflow), docs de nodes (search_nodes/get_node), health_check, e credenciais (n8n_manage_credentials create funcionou).
- WORKAROUND para editar/criar workflow: PUT direto na API com payload limpo (script Node com fetch). Foi assim que a troca do modelo foi aplicada.
- Acao futura: avaliar versao mais nova do n8n-mcp ou flag que limpe o payload; ou manter um helper local de PUT limpo para escritas.

## MVP local

- Manter a stack atual: Redis, Postgres, WAHA e n8n.
- Manter `host.docker.internal` apenas no ambiente local, porque isso preserva o fluxo atual do tutorial.
- Trocar somente as portas expostas no host para evitar conflito com outros projetos da maquina.
- Portas escolhidas para o MVP local:
  - n8n: `5680 -> 5678`
  - WAHA: `3010 -> 3000`
  - Postgres: `5440 -> 5432`
  - Redis: `6380 -> 6379`
- O WAHA continua enviando webhook para o n8n local usando `host.docker.internal`, mas agora pela porta `5680`.

## Resumo da pesquisa

### O que esta ok para o local

- A stack atual e suficiente para validar um MVP local de automacao de WhatsApp com n8n + IA.
- Faz sentido usar o n8n localmente enquanto a VPS hospeda outros servicos como APIs, webhooks, banco e componentes auxiliares.
- Isso ajuda a evoluir o produto sem forcar a migracao completa logo no inicio.

### O que nao deve ir assim para a VPS

- Uso de `host.docker.internal`
- Imagens com `latest`
- Credenciais default
- Portas internas expostas sem necessidade
- WAHA sem autenticacao
- n8n com `N8N_SECURE_COOKIE=false`
- n8n com log em `debug`
- n8n sem uso explicito do Postgres que ja existe no compose

### Ajuste descoberto no teste local

- `postgres:latest` pode quebrar mesmo no ambiente local por causa da mudanca de layout dos volumes na linha 18+.
- Para o MVP local, fixamos o Postgres em `postgres:16-alpine` para manter compatibilidade e previsibilidade.

## Passo 2: upgrade para VPS

- Trocar `host.docker.internal` por comunicacao entre containers via nome de servico
- Colocar reverse proxy com HTTPS e dominio publico
- Fixar versoes das imagens
- Colocar secrets reais
- Expor publicamente apenas o que precisar
- Ligar o n8n no Postgres
- Avaliar Redis + queue mode apenas quando houver volume para justificar
- Definir backup para sessoes do WAHA, banco e dados do n8n

## n8n gratuito self-hosted

- O n8n Community Edition self-hosted nao tem o mesmo limite por plano do n8n Cloud.
- O limite real fica na infraestrutura: CPU, RAM, banco, fila e qualidade dos workflows.
- Queue mode existe no self-hosted.
- As limitacoes mais relevantes no futuro tendem a ser de recursos enterprise, como SSO, sharing, projects, environments, external secrets, log streaming, Git nativo e external storage de binarios.

## Observacao de arquitetura

- Sim: usar o n8n localmente e a VPS para outros servicos de webhook, APIs e integracoes faz sentido.
- Isso permite evoluir o produto por camadas, sem travar o MVP local.
- O maior ponto de atencao futuro tende a ser a camada WhatsApp, especialmente estabilidade de sessao, bloqueios e compliance.