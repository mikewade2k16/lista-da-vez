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
