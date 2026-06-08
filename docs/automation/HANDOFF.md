# Prompt de handoff (colar no assistente do painel)

> Cole o bloco abaixo no Claude Code (ou IA equivalente) no projeto do painel.

---

Você vai me ajudar a **migrar e continuar** uma automação de atendimento de WhatsApp com IA, construída no n8n, agora dentro do meu painel/sistema principal (que tem banco de dados, back e front próprios).

## O que é
Assistente de WhatsApp com IA (n8n + WAHA), pensada pra ser **proativa** (não um chatbot reativo). Persona ativa: **Tony** (consultor objetivo, estilo WhatsApp real). Há outra persona pronta: **Pérola Buyer** (consultor de compras de joalheria).

## Pacote que estou trazendo — LEIA PRIMEIRO
- `docker-compose.yml` — containers: n8n **2.23.2**, WAHA **2026.5.1**, Postgres 16-alpine, Redis 8-alpine.
- `export/workflow-whatsapp.json` — **o workflow completo** (36 nós; persona + guardrails já embutidos no systemMessage).
- `export/credentials.decrypted.json` — credenciais (OpenAI, WAHA, Redis) ⚠️ com segredos.
- `docs/n8n/` — documentação: **SETUP.md** (runbook de subida), **AGENTS.md** (decisões/histórico), **WORKFLOW.md** (arquitetura/plano), **ROADMAP.md** (status), **MODELOS.md** (regras de modelos), personas (`gpt-tony.md`, `gpt-perola-buyer-assistant.md`) e `guardrails-resposta.md`.

👉 **Leia esses .md antes de mexer em qualquer coisa** — eles têm o histórico e os detalhes. Os caminhos aqui são da instalação antiga; no painel adapte como fizer sentido. O que importa é o conteúdo.

## O que o sistema faz (arquitetura do workflow)
`Webhook (WAHA) → normaliza dados → dedupe (por id) → filtro (só conversa 1:1; ignora grupo/canal/broadcast e mensagens próprias) → roteia por tipo:`
- **texto**: direto
- **áudio**: baixa a mídia → Whisper transcreve (vai com aviso "pode ter erro")
- **imagem**: baixa → visão (gpt-4o) descreve → junta com a legenda

`→ DEBOUNCE 7s (agrupa mensagens rápidas via Redis; responde 1x) → CLASSIFICADOR DE CONTEXTO (gpt-4o-mini decide ASSUNTO NOVO × CONTINUAÇÃO e reseta a memória curta por "segmento") → MEMÓRIA LONGA (resumo por contato, persiste entre assuntos) → AI AGENT (Tony) com memória Redis → NATURALIDADE (mostra "digitando", delay proporcional, divide a resposta em balões por parágrafo) → RESUMIDOR atualiza a memória longa.`

Modelos atuais: chat **gpt-5.3-chat-latest** · visão **gpt-4o** · áudio **whisper-1** · classificador/resumidor **gpt-4o-mini**.

## Tarefa imediata: subir e validar
Siga o **docs/n8n/SETUP.md**. Resumo:
1. Subir os containers (compose, ou o equivalente no painel).
2. **Instalar o community node `n8n-nodes-waha@2024.11.5`** no n8n e reiniciar (sem isso a importação quebra).
3. Importar **credenciais** e depois o **workflow** (nessa ordem; comandos no SETUP).
4. Conectar o WhatsApp na WAHA (escanear QR).
5. Ativar o workflow e testar com uma mensagem real.

## Gotchas técnicos (não esquecer)
- A **escrita de workflow via n8n-MCP está quebrada** nesta versão do n8n (API pública PUT é estrita). Edite via **PUT direto** com payload limpo `{name, nodes, connections, settings:{executionOrder}, staticData}` usando a API key do n8n. **Nunca** monte expressões com `$json` dentro de `node -e "..."` inline (o shell come o `$`); use arquivo `.js`.
- Modelos de raciocínio (gpt-5*, o-series) **exigem Responses API** e **não aceitam `temperature`**. E **não funcionam no nó de imagem** (operação analyze retorna só "reasoning") — por isso a visão usa **gpt-4o**.
- Expressões do n8n **não** suportam optional chaining (`?.`).
- `systemMessage` do AI Agent = persona (`gpt-tony.md`) + `guardrails-resposta.md` (sempre anexe os guardrails ao re-sincronizar). Guardrails forçam **resposta em PT-BR** e **texto puro** (sem markdown/`:::`), e definem quando dividir em balões.
- Memória de conversa, segmentos, memória longa e buffer do debounce ficam em **Redis + staticData** (runtime) — começam **zerados** no novo ambiente.

## Próxima fase (o motivo de migrar pro painel)
Adiadas de propósito pra fazer com o banco/back/front do painel:
- **Etapa 2 — mini-CRM no Postgres**: contatos, mensagens, estado do lead, follow-ups, compras (hoje a memória longa é uma versão "lite" no staticData).
- **Etapa 3 — motor proativo**: follow-up de quem não respondeu (cadência), pós-venda, nurture/upsell (depende de estado persistente).
- **Etapa 7 — painel de configuração**: escolher modelos, gerenciar/criar personas e prompts, ligar/desligar a automação, definir contexto temporário (ex.: "estou em gravação, confirmo após as 16h").
- **Tools do agente** (ainda não no roadmap): agenda, preços/estoque reais, web search, registrar lead/pedido.
- **Etapa 5 — RAG dos dados do RP**: pra o Pérola analisar estoque/vendas de verdade (preciso fornecer os arquivos).
- **Conectar tudo ao banco real do painel.**

## Como trabalhar
Itere com cuidado e **valide cada mudança pelos logs de execução do n8n**. Confirme comigo antes de **ativar** (responde no meu WhatsApp real). Mantenha a documentação (`ROADMAP.md`, `AGENTS.md`) atualizada a cada passo.

---
