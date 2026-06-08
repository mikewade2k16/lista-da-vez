# 🤖 MODELOS — requisitos e como trocar

> Referência de qual modelo usa o quê. Quando você pedir "troca pro modelo X", eu sigo isto e já
> ajusto os parâmetros certos (Responses API, temperature, etc.) sem quebrar.
> Aprendido na prática (ver changelog em [ROADMAP.md](ROADMAP.md)). Atualizado: 2026-06-03.

## Onde cada modelo vive no workflow

| Função | Nó no n8n | Tipo | O que controla |
|---|---|---|---|
| Cérebro / resposta (chat) | **OpenAI Chat Model** | `lmChatOpenAi` | a conversa do AI Agent (Tony/Pérola) |
| Visão / imagem | **Analisar Imagem** | `openAi` (resource image, op analyze) | interpreta a foto |
| Transcrição de áudio | **Transcrever Audio** | `openAi` (resource audio, op transcribe) | Whisper — **modelo fixo** (`whisper-1`), não troca por chat model |

## Requisitos por modelo de CHAT (nó OpenAI Chat Model)

| Modelo | Responses API | `temperature` | Visão | Perfil |
|---|---|---|---|---|
| `gpt-4o-mini` | opcional (off ok) | ✅ aceita | ✅ | barato, rápido — bom p/ MVP/testes |
| `gpt-4o` | opcional (off ok) | ✅ aceita | ✅ | equilíbrio qualidade/custo/velocidade |
| `gpt-4.1` (se disponível) | opcional | ✅ aceita | ✅ | forte, ainda via chat completions |
| `gpt-5.5-pro` | ⚠️ **obrigatória (ON)** | ❌ **não aceita** (remover) | a confirmar | raciocínio: mais lento e caro, sem controle de temperatura |
| Modelos "pro"/raciocínio em geral (o-series, gpt-5*) | ⚠️ geralmente **obrigatória** | ❌ geralmente **não aceitam** | varia | reasoning |

### Regra que eu aplico ao trocar o modelo de chat
- **Modelo normal** (`gpt-4o`, `gpt-4o-mini`, `gpt-4.1`):
  - `responsesApiEnabled`: pode ficar `false`
  - `options.temperature`: pode definir (ex.: 0.2)
- **Modelo "pro"/raciocínio** (`gpt-5.5-pro`, o-series, gpt-5*):
  - `responsesApiEnabled`: **`true`** (senão: *"This model requires the Responses API"*)
  - **remover** `options.temperature` (senão: *"400 Unsupported parameter: 'temperature' is not supported with this model"*)

## Requisitos da VISÃO (nó Analisar Imagem)

- Campo do modelo: `modelId` (resourceLocator).
- ✅ Usar **gpt-4o-mini** (barato) ou **gpt-4o** — confirmados, retornam o texto certinho.
- 🚫 **NÃO usar modelos de raciocínio (gpt-5, gpt-5.5-pro, o-series) na operação `analyze`**: nesta versão do n8n eles retornam **só o item de `reasoning` com `content: []` (sem texto)** → a imagem quebra com "No prompt specified". Comprovado com gpt-5 em 2026-06-03.
- A saída (modelos não-raciocínio) vem no formato Responses API; o texto é extraído pelo nó **"Texto da imagem"** de forma robusta: `($json.find(x => x.type==='message')||{}).content` → filtra `output_text` → junta os `text`. (Também junta a legenda da pessoa.)
- Não envio `temperature` nesse nó.
- ⚠️ **Recusa em fotos de pessoas/bebês:** modelos de visão (gpt-4o-mini) às vezes recusam descrever pessoas ("Desculpe, mas não posso ajudar com isso") → o agente acha que não tem imagem. Fix: prompt **neutro** mandando descrever de forma **genérica** (sem identificar ninguém) e **nunca recusar**. Se ainda recusar muito, subir pra gpt-4o.

## Áudio (nó Transcrever Audio)

- Modelo **fixo: Whisper (`whisper-1`)** — embutido na operação `transcribe`. Não troca por gpt-*.
- Params: `binaryPropertyName: data`, `options.language: pt`. Aceita ogg/mp3/m4a/wav/etc.
- Áudio sempre passa por aqui antes do cérebro; o modelo de chat é independente do Whisper.

## Como a config é montada (JSON do nó)

```jsonc
// OpenAI Chat Model
{
  "model": { "__rl": true, "mode": "list", "value": "<id>", "cachedResultName": "<id>" },
  "responsesApiEnabled": true,        // true para pro/raciocínio
  "options": {}                        // { "temperature": 0.2 } só p/ modelos que aceitam
}
```

## Se um modelo novo der erro

A própria API diz o que falta — eu leio o log da execução e ajusto:
- *"requires the Responses API"* → ligo `responsesApiEnabled`.
- *"temperature is not supported"* → removo `options.temperature`.
- *"not a chat model" / "model not found"* → modelo errado p/ a função; troco/volto.

## Regra operacional (importante)

Eu edito o workflow via **API (PUT direto)**, porque a escrita do n8n-MCP está quebrada nesta versão
(ver [AGENTS.md](AGENTS.md)). Se você tiver a **aba do n8n aberta** e publicar por lá, sobrescreve o que
fiz. **Depois que eu editar, dê F5 na aba do n8n antes de mexer/publicar.**
