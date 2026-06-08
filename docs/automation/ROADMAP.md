# 🗺️ ROADMAP — n8n WhatsApp

> Quadro de status vivo. Lido automaticamente por **roadmap.html** (cada etapa vira um collapse).
> Plano em [WORKFLOW.md](WORKFLOW.md) · decisões/histórico em [AGENTS.md](AGENTS.md) · modelos em [MODELOS.md](MODELOS.md).
> Última atualização: **2026-06-03**.

## ⚙️ Config atual

| Item | Valor |
|---|---|
| Persona ATIVA | **Tony** — consultor objetivo, humano, estilo WhatsApp (`gpt-tony.md`) |
| Persona alternativa (inativa) | Pérola Buyer Assistant — joalheria (`gpt-perola-buyer-assistant.md`) |
| Modelo do agente (cérebro) | **gpt-5.3-chat-latest** (otimizado p/ conversa natural; Responses API ligada) |
| Transcrição de áudio | Whisper (`whisper-1`), pt + aviso de "pode conter erros" |
| Análise de imagem (visão) | **gpt-4o** (custo-benefício que funciona; modelos de raciocínio gpt-5.x NÃO funcionam no nó de imagem) |
| Memória | Curto prazo: Redis por `chatId_<segmento>` (reseta em assunto novo). Longo prazo: resumo por contato no `staticData` (resumidor gpt-4o-mini) que persiste entre assuntos e lembra artefatos (ex.: legendas) |
| Canal ativo | WhatsApp (WAHA, sessão `default`) |
| Workflow | "Whatsapp" · id `lzhb5JjN5kdcVuRR` · **ATIVO** |
| ⚠️ Atenção | roda no WhatsApp pessoal — responde contatos 1:1 quando ativo |

## ✅ O que já funciona (pode testar agora)

- ✅ **Texto** (1:1) → Tony responde
- ✅ **Áudio** → transcrição (Whisper, com ressalva de erro) → Tony responde
- ✅ **Imagem** (com ou sem legenda) → visão descreve → Tony responde
- ✅ **Filtro anti-ruído** → ignora Newsletter/Canal, grupos, broadcast e mensagens próprias
- ✅ **Memória** de conversa por contato (Redis), com **reset automático em assunto novo** (classificador da Etapa 8) — conversas finalizadas não vazam mais

## 🚧 Em andamento / a decidir

- ✅ Etapas 8 (memória) e 6 (naturalidade) COMPLETAS.
- 🚧 Próximas n8n possíveis: **vídeo** (Etapa 1 avançado), **Instagram** (Etapa 4, precisa creds Meta), **RAG do RP** (Etapa 5, precisa dos arquivos).

## 🚦 Ordem de execução (decisão 2026-06-03: n8n primeiro)

Fazer tudo que dá **dentro do n8n** antes de mexer em back/banco/front.
- **Agora (n8n):** Etapa 8 (memória/contexto) → Etapa 6 (naturalidade) → vídeo (Etapa 1 avançado) → Etapa 4 (Instagram, se tiver credenciais Meta) → Etapa 5 (RAG, quando tiver os arquivos do RP).
- **Depois (back/banco/front):** Etapa 2 (Postgres/CRM), Etapa 3 completa (proativo persistente), Etapa 7 (painel) — e a possível migração pro sistema principal.

## 🧩 Etapas (clique para expandir)

<!--ETAPAS-->
### ✅ Etapa 0 — Base reativa (WhatsApp, texto)
Ponto de partida (tutorial). Fluxo mínimo reativo, só texto.
- Webhook (WAHA) → Set "Dados" → Switch → AI Agent → Send Seen → Send Text.
- Memória Redis por `chatId`.
- **Status:** feito.

### ✅ Etapa A — Troca Gemini → OpenAI + persona
- Removido o Google Gemini; adicionado OpenAI Chat Model; credencial OpenAI criada.
- Persona inicial: Pérola Buyer; depois trocada para Tony.
- **Status:** feito.

### ✅ Etapa 1 — Multimodal (áudio + imagem)
Entender áudio e imagem, não só texto.
- "Dados" detecta o tipo pelo `mimetype` e captura a URL da mídia (reescrita `localhost:3000`→`waha:3000`).
- Switch "Tipo" roteia: **áudio** → Baixar → Whisper (pt) → aviso "pode conter erros" → agente; **imagem** → Baixar → Visão (gpt-4o-mini, prompt neutro) → "Texto da imagem" → agente; **texto** → direto.
- Filtro: ignora `@newsletter`, `@g.us`, `@broadcast` e `fromMe`.
- **Falta (avançado):** vídeo (extrair áudio + frame).
- **Status:** feito (vídeo pendente).

### ⏸️ Etapa 2 — Memória longa + mini-CRM (Postgres) — ADIADA (back/banco)
Histórico persistente do lead, base para o motor proativo.
- Ligar o n8n ao Postgres (já existe no compose, hoje ocioso) — criar credencial.
- Tabelas: `contacts`, `messages`, `lead_state`, `follow_ups`, `purchases`.
- Gravar cada mensagem (cliente e IA); manter `lead_state` (status, última interação, contagem de follow-up).
- Chave de memória por **canal + remetente** (hoje é só `chatId`).
- **Status:** planejado.

### ⏸️ Etapa 3 — Motor proativo (follow-up / gatilhos) — ADIADA (precisa de estado persistente)
O sistema toma iniciativa (não só responde). Workflows **agendados**, separados do reativo.
- "Follow-up sem resposta": varre `lead_state` e re-contata com cadência configurável (ex.: +1h, +1 dia, +3 dias); para após X tentativas.
- "Pós-venda": ao registrar compra, agenda agradecimento + sugestões futuras após N dias.
- "Nurture/upsell": sugestões periódicas pelo perfil/histórico.
- Anti-spam: horário comercial, limite de tentativas, opt-out, não recontatar quem respondeu.
- A definir: como detectar "compra/fechamento".
- **Status:** planejado.

### ⏳ Etapa 4 — Instagram (2º canal)
Mesmo cérebro, novo canal.
- Integração Meta/Instagram Messaging (webhook + verificação + tokens).
- Adaptador de entrada IG → mesmo formato normalizado da Etapa 1; envio via IG API.
- Pré-req: conta Instagram Business + página Facebook + app Meta.
- **Status:** planejado.

### ⏳ Etapa 5 — RAG dos dados reais do RP
Dá "valor real" ao Pérola (analisar estoque/vendas de verdade).
- Ingestão dos arquivos do RP (estoque, vendas, fornecedores, giro) num **vector store**.
- Tool de consulta (retrieval) no AI Agent; atualização periódica dos dados.
- Pré-req: Mike fornecer os arquivos e definir formato/atualização.
- **Status:** planejado.

### ✅ Etapa 6 — Naturalidade (anti-robô) — COMPLETA (6a+6b+6c)
- **6a (FEITO/testado):** ✅ "digitando…" (HTTP `waha:3000/api/startTyping`, onError continue) → ✅ delay proporcional (nó Wait) → envia. + ✅ dedupe por `payload.id` no `staticData`.
- **6b (FEITO/testado):** `Dividir` quebra a resposta em até 3 balões — por `|||` OU por **parágrafo (linha em branco)**, já que o agente raramente usa `|||` e separa por parágrafos; `Loop` (SplitInBatches) manda cada um com "digitando" + pausa. (bug corrigido: Send Text lê `$('Loop').first().json.text`, pois o HTTP Digitando substitui o `$json`.)
- **6c (construído, em teste):** debounce de 7s no Redis. Cada msg: `Fila push` (RPUSH no buffer `buf:chatId`) + `Fila token` (SET `tok:chatId`=playload_id) → `Wait` 7s → `Fila token atual` (GET) → `Eh ultima` (se token != o meu, aborta) → `Fila buffer` (lê lista) → `Fila limpar` (DEL) → `Juntar` (concatena) → segue. Junta mensagens rápidas e responde 1x; resolve a corrida de concorrência. Janela ajustável (7s).

### ⏸️ Etapa 7 — Painel de configuração (admin) — ADIADA (back/banco/front)
Configurar e operar a automação por uma **interface**, sem mexer no n8n.
- **Modelos:** escolher chat e visão pela tela (regras do MODELOS.md aplicadas sozinhas — Responses API/temperature).
- **Personas/prompts:** criar, editar e listar (como Tony e Pérola) e escolher o **ATIVO** (hoje são `.md` sincronizados na mão).
- **Liga/desliga:** ativar/desativar a automação pela tela.
- **Contexto/status temporário:** ao ligar, definir um aviso (ex.: *"em gravação até 16h"*) injetado no prompt → o bot adia confirmações em vez de prometer. Com expiração.
- Depende de armazenamento (Postgres, Etapa 2). Provavelmente **nasce dentro do sistema principal do Mike** (back/banco/front) → ponto natural de migração.
- **Status:** planejado.

### ✅ Etapa 8 — Gestão de contexto/memória (FUNCIONANDO)
Resolver o vazamento: conversas "finalizadas" influenciando conversas novas.
- **Como ficou (2 camadas):**
  - **Curto prazo:** janela de 10 msgs **por SEGMENTO** (`chatId_<seg>`). O classificador gpt-4o-mini decide NOVO×CONTINUA; em assunto novo abre um segmento limpo (sem bleed).
  - **Longo prazo:** resumo por contato (resumidor gpt-4o-mini no `staticData`) que **persiste entre segmentos** e guarda artefatos/decisões; sempre injetado no agente → recall entre assuntos.
- ⚠️ A janela de 10 é POR SEGMENTO: ao trocar de assunto o curto prazo zera **de propósito** (é o que evita o vazamento). Recall de coisas antigas vem da **memória longa**.
- ⚠️ Artefatos criados ANTES desta feature não estão na memória longa (ela começou vazia em 2026-06-03).
- **Status:** planejado.

### ⏳ Refinamento — confiança da transcrição de áudio
- ✅ Feito (versão barata): a transcrição vai pro agente com aviso "pode conter erros; interprete pelo contexto".
- Pendente (se precisar): node de IA leve pós-Whisper pra revisar/normalizar a transcrição (avaliar custo).
- **Status:** parcial.
<!--/ETAPAS-->

## ❌ Ainda NÃO disponível

- ❌ Vídeo · ❌ Instagram · ❌ análise de dados reais do RP (RAG) · ❌ follow-up automático · ❌ painel

## 🧪 Como testar (hoje)

1. O workflow precisa estar **ATIVO** (peça pra ligar/desligar — roda no WhatsApp pessoal).
2. De **outro número**, mande pro bot: um **texto**, um **áudio** e uma **imagem**.
3. O Tony responde no tom dele: curto, direto, humano.

## 🛠️ Arquitetura (resumo)

```
Webhook → Dados → Switch (filtro 1:1) → Tipo (por mimetype):
   ├─ áudio  → Baixar → Whisper → Texto do áudio ─┐
   ├─ imagem → Baixar → Visão  → Texto da imagem ─┤→ AI Agent (Tony, gpt-5) → Send Seen → Send Text
   └─ texto  ─────────────────────────────────────┘   + memória Redis
```
Edição via PUT direto na API (escrita do n8n-MCP quebrada nesta versão — ver AGENTS.md).

## 📒 Histórico (changelog)

### 2026-06-03 — Pacote de migração
- Fase de migração iniciada (levar pro sistema principal do Mike). Exportados `export/workflow-whatsapp.json` (36 nós) e `export/credentials.decrypted.json`. Versões fixadas no docker-compose (n8n 2.23.2, waha 2026.5.1, postgres 16-alpine, redis 8-alpine). Community node `n8n-nodes-waha@2024.11.5`. Runbook completo em **SETUP.md**. `.gitignore` protege segredos.

### 2026-06-03
- MCP do n8n configurado; descoberta a limitação de escrita (workaround: PUT direto).
- Gemini → OpenAI; persona Pérola → Tony; modelo passou por gpt-4o → 4o-mini → 5.5-pro → 5-mini → **gpt-5**.
- Etapa 1 (multimodal): áudio (Whisper) + imagem (visão) + filtro de canal/grupo/broadcast.
- Bugs corrigidos: resposta a Newsletter (401); parsing da visão; optional chaining; gpt-5.5-pro exige Responses API e não aceita temperature; gpt-5 não serve pro nó de imagem; visão recusava fotos de pessoas (prompt neutro).
- Áudio agora vai com aviso de "transcrição pode ter erro". (corrigido bug onde a expressão perdeu o `$json` ao ser salva via `node -e` inline — bash comia o `$`.)
- Visão subida de gpt-4o-mini → **gpt-4o** (melhor custo-benefício que funciona; gpt-5.x de raciocínio continuam sem funcionar no nó de imagem).
- Chat: gpt-5 parecia "burrinho"/sem sentido (em parte por memória vazando) → trocado para **gpt-5.3-chat-latest** (otimizado p/ conversa).
- ✅ **Etapa 8 feita e funcionando**: classificador gpt-4o-mini decide NOVO×CONTINUA por mensagem; assunto novo reseta a memória via `sessionKey = chatId_<segmento>`. Estado guardado no `staticData` do workflow (sem precisar de banco). Testado: separou "aliança" de "vídeo/post" corretamente.
- ✅ Memória v2 (longa) + guardrails TESTADOS OK: recall da legenda entre assuntos funcionou; saída limpa sem `:::`.
- 🌐 gpt-tony reescrito em inglês (regras anti-clichê/anti-travessão); guardrail força resposta em PT-BR. Balões: não-obrigatório (padrão 1 balão, pode juntar perguntas), divide por parágrafo até 5.
- ✅ Etapa 6a TESTADA OK: "digitando…" disparou (WAHA result:true), delay (Wait) retomou normal, dedupe ativo. Fluxo completo ~12s (classificador + gpt-5.3 + wait + resumidor).
- 🧹 Guardrails de resposta (`guardrails-resposta.md`) anexados ao systemMessage: proíbem markdown/`:::`/blocos de escrita (saíam `:::writing block` no WhatsApp) e a postura de criticar o pedido ("tá muito genérico"). systemMessage ativo = persona + guardrails.
- 🔧 Ajuste pós-teste (memória v2 / opção A): reset duro estava cortando recall legítimo (esqueceu a legenda criada 3 assuntos antes). Adicionada **memória longa por contato** (resumidor gpt-4o-mini → resumo no staticData, persiste entre segmentos e é injetado no agente). É uma versão lite da Etapa 2 (memória longa) feita no n8n, sem banco; o CRM estruturado (tabelas) fica pra fase de banco.
- Roadmap detalhado por etapa + página com collapses (Bootstrap 5). Adicionadas Etapas 7 (painel) e 8 (gestão de contexto).
