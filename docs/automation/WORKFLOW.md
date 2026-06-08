# WORKFLOW — Visão e roadmap da automação

> Documento vivo. Descreve a aplicação alvo (não só o que existe hoje) e quebra a construção
> em etapas com os nodes do n8n que vamos criar/editar. Atualizar conforme avançamos.
> Última atualização: 2026-06-03.

---

## 1. Visão

Não queremos um chatbot **reativo e robotizado**. Queremos uma **assistente de vendas proativa**
para a Pérola Joias, que:

- Conversa de forma natural e consultiva (persona: ver [gpt-perola-buyer-assistant.md](gpt-perola-buyer-assistant.md)).
- Atende em **múltiplos canais**: WhatsApp e Instagram.
- Entende **múltiplos formatos**: texto, áudio, imagem e vídeo.
- Tem **memória** das conversas (o que o cliente disse E o que a IA respondeu).
- **Toma iniciativa**: faz follow-up de quem não respondeu, de quem comprou, e oferece sugestões
  futuras com gatilhos de relacionamento — trabalhando o lead ao longo do tempo.

## 2. Princípios

1. **Pipeline unificado**: toda mensagem, de qualquer canal/formato, vira um formato interno único
   antes de chegar no cérebro. Assim a lógica de IA não se importa se veio do WhatsApp ou Instagram.
2. **Proativo > reativo**: o sistema agenda e dispara contatos por conta própria (não só responde).
3. **Memória em duas camadas**: curto prazo (Redis, contexto da conversa) + longo prazo
   (Postgres, histórico do lead / mini-CRM).
4. **Multimodal**: áudio é transcrito (Whisper), imagem é interpretada (visão), vídeo é decomposto.
5. **Natural, não robô**: indicadores de digitação, ritmo humano, sem spam, respeito a horário e opt-out.

## 3. Arquitetura alvo (camadas)

```
[ Canais ]            WhatsApp (WAHA)        Instagram (Meta API)
                          │                       │
                          ▼                       ▼
[ Entrada ]        Adaptador WhatsApp       Adaptador Instagram
                          └──────────┬────────────┘
                                     ▼
[ Normalização ]   Formato interno único:
                   { canal, remetenteId, nome, tipo, texto, midiaUrl, timestamp, fromMe }
                                     ▼
[ Multimodal ]     Switch por tipo:
                     texto ─────────────────────────────────┐
                     áudio  → download → Whisper (transcrição)┤
                     imagem → download → Visão (OCR + produto)┤→ texto normalizado
                     vídeo  → download → áudio(Whisper)+frame ┘
                                     ▼
[ Cérebro ]        AI Agent (Pérola) + OpenAI + Memória Redis
                     + Tools: catálogo/RAG do RP, web search, ações de CRM
                                     ▼
[ Persistência ]   Postgres: contatos/leads, mensagens, estado do lead, follow-ups, compras
                                     ▼
[ Saída ]          Envio pelo canal de origem (WAHA / IG) + "visto"/digitando

[ Motor proativo ] (workflows AGENDADOS, separados do fluxo reativo)
   - Sem-resposta: re-contato com cadência (ex.: +1h, +1 dia, +3 dias)
   - Pós-venda: agradecimento + sugestões após fechamento
   - Nurture/upsell: sugestões futuras baseadas no histórico do lead
```

## 4. Estado atual (Etapa 0 — FEITO)

Fluxo único reativo, só WhatsApp, só texto:

`Webhook → Dados (Set) → Switch (event==message) → AI Agent (OpenAI gpt-4o-mini, persona Pérola)
→ Send Seen (WAHA) → Send Text (WAHA)`, com memória Redis por chatId.

Workflow "Whatsapp" (id `lzhb5JjN5kdcVuRR`), ATIVO. Detalhes e decisões em [AGENTS.md](AGENTS.md).

## 5. Roadmap

> Cada etapa lista o trabalho de nodes. "criar" = node novo; "editar" = ajustar existente.
> Ordem sugerida, mas dá pra repriorizar.

### Etapa 1 — Multimodal no WhatsApp
Objetivo: entender áudio, imagem e (depois) vídeo, não só texto.
- editar **Dados**: detectar o tipo da mensagem (texto/áudio/imagem/vídeo) a partir do payload da WAHA
  (`payload.hasMedia`, `payload.media.mimetype`, `_data.Info.Type`) e capturar a URL da mídia.
- criar **Switch "Tipo de mensagem"**: roteia por tipo.
- criar **HTTP Request** (ou node WAHA) para baixar a mídia.
- criar **OpenAI — Transcrever áudio (Whisper)**: áudio → texto.
- criar **OpenAI — Analisar imagem (visão gpt-4o)**: lê texto na imagem (OCR) e identifica o produto
  (ex.: aliança, solitário, bracelete) → descrição textual.
- criar **Merge/Set**: consolida tudo num único campo `message` para o AI Agent.
- (avançado) **Vídeo**: extrair áudio (Whisper) + frame-chave (visão). Pode exigir ffmpeg/serviço externo.
- pré-requisitos: confirmar que a WAHA entrega URL de mídia acessível pelo n8n.

### Etapa 2 — Memória longa + mini-CRM (Postgres)
Objetivo: histórico persistente do lead, base para o motor proativo.
- ligar o n8n ao **Postgres** (já existe no compose, hoje ocioso) — criar credencial Postgres.
- definir esquema: `contacts`, `messages`, `lead_state`, `follow_ups`, `purchases`.
- criar nodes **Postgres (insert)** para gravar cada mensagem (cliente e IA).
- editar memória: chave por `canal + remetenteId` (hoje é só `chatId`).
- criar nodes **Postgres (update)** para manter `lead_state` (status, última interação, contagem de follow-up).

### Etapa 3 — Motor proativo (follow-ups e gatilhos)
Objetivo: o sistema toma iniciativa. **Workflows separados** com Schedule Trigger.
- criar workflow **"Follow-up sem resposta"**: Schedule Trigger varre `lead_state` e dispara re-contato
  conforme cadência configurável (ex.: +1h, +1 dia, +3 dias); para após X tentativas.
- criar workflow **"Pós-venda"**: ao registrar compra, agenda agradecimento + sugestões futuras após N dias.
- criar workflow **"Nurture/upsell"**: sugestões periódicas baseadas no perfil/histórico.
- regras anti-spam: limite de tentativas, horário comercial, opt-out, não recontatar quem respondeu.
- decisão pendente: como detectar "fechamento/compra" (tool do agente? marcação manual? integração RP?).

### Etapa 4 — Instagram (segundo canal)
Objetivo: mesmo cérebro, novo canal.
- integração **Meta/Instagram Messaging** (webhook + verificação + tokens).
- criar **Adaptador de entrada IG** → mesmo formato normalizado da Etapa 1.
- criar **envio via IG API**.
- pré-requisitos: conta Instagram Business, página Facebook, app Meta, tokens de acesso.

### Etapa 5 — Cérebro com dados reais (RAG do RP) [era "Fase B"]
Objetivo: a assistente analisa estoque/vendas de verdade.
- ingestão dos **arquivos do RP** (estoque, vendas, fornecedores, giro) num **vector store**.
- criar **tool de consulta** no AI Agent (retrieval).
- definir atualização periódica dos dados.
- pré-requisito: você fornecer os arquivos do RP e definir o formato/atualização.

### Etapa 6 — Naturalidade (anti-robô)
- indicador "digitando", delays variáveis, divisão de mensagens longas, ritmo humano.
- ignorar mensagens próprias (`fromMe == true`) e dedupe de eventos repetidos da WAHA.

### Etapa 7 — Painel de configuração (admin)
Objetivo: configurar e operar a automação por uma **interface**, sem mexer no n8n na mão.
- **Modelos**: escolher modelo de chat e de visão pela tela (aplicando as regras do [MODELOS.md](MODELOS.md): Responses API / temperature ajustados automaticamente).
- **Personas/prompts**: criar, editar e listar prompts (como o Tony e o Pérola) **na plataforma** e escolher qual está **ATIVO** (hoje são arquivos `.md` sincronizados na mão).
- **Liga/desliga**: ativar/desativar a automação pela tela.
- **Contexto/status temporário**: ao ligar, definir um aviso de contexto (ex.: *"em gravação até 16h, não posso responder agora"*) que é **injetado no prompt**. O bot segue respondendo normal, mas ciente disso — ex.: pra confirmações, responde *"consigo confirmar depois das 16h"* em vez de prometer. Pode ter expiração automática.
- Dependências: precisa de **armazenamento** (Postgres da Etapa 2) e de o n8n **ler a config a cada execução**. Quando entrar back/banco/front, isto provavelmente vira parte do **painel/sistema principal do Mike** → possível migração de tudo pra lá (decisão já prevista no AGENTS.md).

### Etapa 8 — Gestão de contexto/memória (ajuste importante)
Problema observado: a memória (Redis, janela 10 por `chatId`, TTL 1h) faz **conversas "finalizadas" vazarem em conversas novas** (no teste, o assunto antigo de "uma frase" apareceu numa foto de bebê).
- Avaliar: **expiração por inatividade** (resetar contexto após X tempo sem mensagem), **detecção de novo assunto/encerramento**, **memória com resumo** (summary) em vez de janela crua, e/ou **comando/gatilho de reset**.
- Ajuste rápido possível já: reduzir janela/TTL. Ligado à Etapa 2 (memória longa no Postgres) e à Etapa 3 (estado do lead).

### Refinamento da Etapa 1 — confiança da transcrição de áudio
- Tratar a transcrição como **possivelmente imperfeita**: instruir o agente (no prompt) que áudio é transcrição e pode conter erros — interpretar com o contexto e, se ambíguo, confirmar gentilmente em vez de assumir.
- Opção mais cara (avaliar custo): um **node de IA leve pós-Whisper** pra revisar/normalizar a transcrição. Começar pelo jeito barato (instrução no prompt).

## 6. Como vamos editar (restrição técnica importante)

A escrita de workflow via **n8n-mcp está quebrada** nesta versão do n8n (a API pública PUT é estrita e
rejeita campos extras). Detalhes em [AGENTS.md](AGENTS.md). Portanto:
- **Leitura/validação/docs de nodes/credenciais**: via n8n-mcp (funciona).
- **Criar/editar nodes e workflows**: via **PUT direto com payload limpo** (helper Node com fetch).
  Avaliar versão mais nova do n8n-mcp depois.

## 7. Decisões em aberto (preciso de você)

1. **Instagram**: já tem conta Business + página Facebook + app Meta? Ou isso fica pra depois?
2. **Arquivos do RP** (Etapa 5): qual formato (CSV/Excel/JSON)? Com que frequência atualizam?
3. **Cadência de follow-up** (Etapa 3): quais tempos exatos de re-contato e quantas tentativas?
4. **Detecção de compra/fechamento** (Etapa 3): como o sistema sabe que o lead comprou?
5. **Vídeo** (Etapa 1): faz parte do MVP agora ou deixamos como avançado depois?
6. **Por onde começar**: sugiro Etapa 1 (multimodal) — alto impacto e independente das demais.

## 8. Referência — payloads de mídia da WAHA (capturado 2026-06-03)

Capturado com mensagens reais (engine GOWS 2026.5.1). Achados que mudam o design:

### Como detectar o tipo
- `payload._data.Info.Type` vem **sempre `"media"`** para áudio, imagem E vídeo — **NÃO serve** para distinguir.
- Distinguir pelo **`payload.media.mimetype`**:
  - áudio: `audio/ogg; codecs=opus` (arquivo `.oga`)
  - imagem: `image/jpeg` (`.jpeg`)
  - vídeo: `video/mp4` (`.mp4`)  ← **fica para depois (Etapa 1 avançado)**
- texto: `hasMedia: false`, conteúdo em `payload.body`. Para mídia, `body` é a legenda (ou vazio).

### URL da mídia e download (crítico)
- A URL vem em `payload.media.url`, ex.: `http://localhost:3000/api/files/default/<ID>.<ext>`.
- ⚠️ O host é `localhost:3000` (a própria WAHA), **inacessível de dentro do n8n**. Reescrever para
  **`http://waha:3000`** (nome do serviço na rede docker). Confirmado: `waha:3000` é alcançável do n8n.
- ⚠️ A WAHA **deleta a mídia pouco depois** (TTL curto; storage `/app/.media` esvazia). Logo, o n8n
  **precisa baixar na hora** que o webhook chega (na prática é imediato — sem problema).
- Caso de borda: pode vir `hasMedia: true, media: null` ou `hasMedia: false` (WAHA não baixou) → tratar
  como "mídia indisponível" (não quebrar o fluxo).

### Filtro de origem (bug encontrado)
- O bot estava respondendo **Newsletter/Canal** (`chatId` termina em `@newsletter`) — ex.: anúncio de
  produto — e a WAHA rejeitava o envio (erro 401/500). Também há risco com grupos (`@g.us`) e broadcast.
- Regra: só processar conversa 1:1 de usuário (`@c.us` ou `@lid`), `fromMe == false`, e ignorar
  `@newsletter`, `@g.us`, `@broadcast`, `status@broadcast`.

### Exemplo de payload de mídia (campos relevantes)
```json
{
  "event": "message",
  "session": "default",
  "payload": {
    "from": "192766906736845@lid",
    "fromMe": false,
    "hasMedia": true,
    "body": "",
    "media": { "url": "http://localhost:3000/api/files/default/<ID>.jpeg", "mimetype": "image/jpeg" },
    "_data": { "Info": { "Type": "media", "PushName": "..." } }
  }
}
```

