package automation

import _ "embed"

// defaultPersona e o comportamento padrao (Tony / Crow Visuals): instrucoes +
// conhecimento, verbatim. Gerado de docs/automation/persona-tony-crowvisuals.md.
// Usado para semear a persona ativa da automacao quando ainda nao existe.
//
//go:embed defaults/persona.md
var defaultPersona string

// defaultGuardrails sao as regras de resposta de WhatsApp (PT-BR, texto puro,
// baloes, emoji raro). Gerado de docs/automation/guardrails-resposta.md. Anexado
// ao final do systemMessage no runtime-config — nunca editado junto da persona.
//
//go:embed defaults/guardrails.md
var defaultGuardrails string

// omniChatPersona e a persona DEDICADA do Omni Chat (chat interno do painel de
// Operacao) — copiloto de vendas/conhecimento da Perola Joias. Independente do Tony
// (defaultPersona), que e customer-facing do WhatsApp. Gerada de
// defaults/omni_chat_persona.md. Usada verbatim como systemMessage (sem guardrails de
// WhatsApp), sem consultar banco/persona.
//
//go:embed defaults/omni_chat_persona.md
var omniChatPersona string
