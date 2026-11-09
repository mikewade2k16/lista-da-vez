# Cliente LLM compartilhado

Providers nativos openai/gemini/glm, allowlist SSRF e validação JSON no Go.
Credencial vem do cofre pelo consumidor, nunca env/prompt/log.

CT-07 adiciona Request.MaxOutputTokens opcional (0 mantém legado) e
NoFormatFallback (false mantém retry legado de response_format400). Plus usa limite
explícito e uma única tentativa. OpenAI envia max_completion_tokens; Gemini/GLM
max_tokens. Provider/modelo incompatível falha; não reconfigurar Crow legado.
Response.UsageKnown distingue ausência de metadados de uso de zero explícito.
Erro de schema conserva apenas tokens/modelo/latência, sem texto bruto.
Testes interceptam HTTP sem rede; smoke com provider real depende de autorização.
